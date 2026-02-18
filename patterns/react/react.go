package react

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/core/overview"
	"github.com/leofalp/aigo/core/parse"
	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/memory"
	"github.com/leofalp/aigo/providers/observability"
	"github.com/leofalp/aigo/providers/tool"
)

// ReAct is a type-safe ReAct (Reasoning + Acting) pattern implementation.
// The generic parameter T defines the expected structure of the final answer.
//
// This pattern automatically handles the tool execution loop with reasoning,
// and parses the final result into type T.
//
// Example:
//
//	type MathResult struct {
//	    Answer int    `json:"answer"`
//	    Steps  string `json:"steps"`
//	}
//
//	agent := react.New[MathResult](baseClient)
//	result, err := agent.Execute(ctx, "What is 42 * 17?")
//	fmt.Printf("Answer: %d\n", result.Data.Answer)
type ReAct[T any] struct {
	client                     *client.Client
	maxIterations              int
	stopOnError                bool
	withSystemPromptAnnotation bool
	schema                     *jsonschema.Schema
	state                      map[string]interface{}
}

// Option is a functional option for configuring ReAct.
type Option func(*ReAct[any])

// WithMaxIterations sets the maximum number of tool execution iterations.
// Default: 10
func WithMaxIterations(max int) Option {
	return func(rc *ReAct[any]) {
		rc.maxIterations = max
	}
}

// WithStopOnError configures whether to stop execution on tool errors.
// Default: false
func WithStopOnError(stop bool) Option {
	return func(rc *ReAct[any]) {
		rc.stopOnError = stop
	}
}

// New creates a new type-safe ReAct pattern that wraps a base client.
// The base client should be configured with memory, tools, and observer.
//
// Memory is required for the ReAct pattern to work (the LLM needs to see tool results).
// If the client is in stateless mode, this function will return an error.
//
// The generic parameter T defines the expected structure of the final answer.
// A JSON schema is automatically generated from T and injected into the system prompt
// to guide the LLM's final response format.
//
// Example:
//
//	type MathResult struct {
//	    Answer      int    `json:"answer" jsonschema:"required"`
//	    Explanation string `json:"explanation" jsonschema:"required"`
//	}
//
//	baseClient, _ := client.New(
//	    provider,
//	    client.WithMemory(memory),
//	    client.WithTools(tool1, tool2),
//	    client.WithObserver(observer),
//	)
//
//	agent, _ := react.New[MathResult](
//	    baseClient,
//	    react.WithMaxIterations(5),
//	    react.WithStopOnError(true),
//	)
func New[T any](baseClient *client.Client, opts ...Option) (*ReAct[T], error) {
	// Validate that memory is configured (required for ReAct)
	if baseClient.Memory() == nil {
		return nil, fmt.Errorf("ReAct pattern requires memory: client must be configured with WithMemory()")
	}

	// Generate JSON schema for type T
	schema := jsonschema.GenerateJSONSchema[T]()

	// Create ReAct with defaults
	rc := &ReAct[T]{
		client:                     baseClient,
		maxIterations:              10,
		stopOnError:                false,
		withSystemPromptAnnotation: true,
		schema:                     schema,
		state:                      map[string]interface{}{},
	}

	// Apply options (using type erasure for the option functions)
	for _, opt := range opts {
		opt((*ReAct[any])(rc))
	}

	if rc.withSystemPromptAnnotation {
		baseClient.AppendToSystemPrompt("Use the ReAct (Reasoning + Acting) pattern to answer user queries with " + strconv.Itoa(rc.maxIterations) + " iterations maximum. ")
	}

	// Inject schema constraint into system prompt from the start
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}
	schemaPrompt := fmt.Sprintf(
		"\n\nWhen providing your final answer (no tool calls), format it as valid JSON matching this schema:\n%s",
		string(schemaJSON),
	)
	baseClient.AppendToSystemPrompt(schemaPrompt)

	return rc, nil
}

// Execute runs the type-safe ReAct (Reasoning + Acting) pattern loop:
//
// 1. First iteration: Send user prompt to LLM using SendMessage()
//   - Schema is already injected in system prompt from construction
//
// 2. LLM analyzes and decides if it needs tools to answer
// 3. If LLM requests tool calls:
//   - Execute each tool and append results to memory
//   - Use ContinueConversation() to let LLM process tool results
//   - LLM may request more tools or provide final answer
//
// 4. Repeat steps 3 until LLM provides final answer (no tool calls) or max iterations reached
// 5. Parse the final answer into type T
//   - If parsing fails, request JSON format explicitly (one retry)
//   - If still fails, return error
//
// Key implementation details:
//   - Schema is injected once at construction time (not per-request)
//   - First iteration uses SendMessage(ctx, prompt) to add user message
//   - Subsequent iterations use ContinueConversation(ctx) to process tool results
//   - Automatic retry with explicit JSON request on parse failure
//
// Returns a StructuredOverview[T] containing both the parsed data and execution statistics.
func (r *ReAct[T]) Execute(ctx context.Context, prompt string) (*overview.StructuredOverview[T], error) {
	var response *ai.ChatResponse
	var err error

	// Get memory and tool catalog from client
	iteration := 0
	iterationTimer := utils.NewTimer()
	execTimer := utils.NewTimer()
	reactMemory := r.client.Memory()
	toolCatalog := r.client.ToolCatalog()
	executionOverview := overview.OverviewFromContext(&ctx)

	// Start execution timing for compute cost tracking
	executionOverview.StartExecution()
	defer executionOverview.EndExecution()

	// Start top-level ReAct span
	observer := r.client.Observer()
	if observer == nil {
		observer = observability.ObserverFromContext(ctx)
	}

	r.state["observer"] = observer
	r.state["iterationTimer"] = iterationTimer
	r.state["execTimer"] = execTimer

	r.observeInit(&ctx, prompt, toolCatalog)

	execTimer.Start()

	// Main ReAct loop
	for iteration < r.maxIterations {
		iteration++

		r.observeStartIteration(&ctx, iteration)

		// Step 1: Send message to LLM
		// First iteration: SendMessage adds user message to memory
		// Subsequent iterations: ContinueConversation processes tool results without new user message
		iterationTimer.Start()

		if iteration == 1 {
			response, err = r.client.SendMessage(ctx, prompt)
		} else {
			// Continue conversation to allow LLM to process tool results from reactMemory.
			// ContinueConversation() sends all messages (including tool results) to the LLM
			// without adding a new user message, maintaining proper conversation flow.
			response, err = r.client.ContinueConversation(ctx)
		}

		iterationTimer.Stop()

		if err != nil {
			r.observeIterationError(&ctx, err, iteration)
			return nil, fmt.Errorf("iteration %d failed: %w", iteration, err)
		}

		// Step 2: Check if we're done (no tool calls = final answer)
		if len(response.ToolCalls) == 0 {
			// No more tool calls - this is the final answer
			// Try to parse the response into type T
			data, parseErr := parse.ParseStringAs[T](response.Content)

			if parseErr != nil {
				// Parse failed - request explicit JSON format and retry once
				r.observeParseError(&ctx, parseErr, response.Content)
				r.observeRequestingStructuredFinalAnswer(&ctx, iteration)

				// Add assistant's response to memory first
				reactMemory.AppendMessage(ctx, &ai.Message{
					Role:    ai.RoleAssistant,
					Content: response.Content,
				})

				// Request JSON format explicitly
				retryPrompt := "Please provide your answer in valid JSON format only, with no additional text."
				retryResponse, err := r.client.SendMessage(ctx, retryPrompt)
				if err != nil {
					r.observeIterationError(&ctx, err, iteration)
					return nil, fmt.Errorf("failed to request JSON format: %w", err)
				}

				// Try parsing again
				data, parseErr = parse.ParseStringAs[T](retryResponse.Content)
				if parseErr != nil {
					r.observeParseError(&ctx, parseErr, retryResponse.Content)
					return nil, fmt.Errorf("failed to parse final answer after retry into type %T: %w", data, parseErr)
				}

				// Success after retry
				r.observeSuccess(&ctx, retryResponse, iteration)
				// Get updated overview from context (includes all responses added by client)
				finalOverview := overview.OverviewFromContext(&ctx)
				return &overview.StructuredOverview[T]{
					Overview: *finalOverview,
					Data:     &data,
				}, nil
			}

			// Parse succeeded on first try
			r.observeSuccess(&ctx, response, iteration)
			// Get updated overview from context (includes all responses added by client)
			finalOverview := overview.OverviewFromContext(&ctx)
			return &overview.StructuredOverview[T]{
				Overview: *finalOverview,
				Data:     &data,
			}, nil
		}

		// Step 3: Execute tool calls
		r.observeTools(&ctx, response, iteration)

		// Add assistant message to memory (with tool calls, reasoning, and refusal)
		reactMemory.AppendMessage(ctx, &ai.Message{
			Role:      ai.RoleAssistant,
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
			Reasoning: response.Reasoning,
			Refusal:   response.Refusal,
		})

		toolsExecuted := 0
		for _, toolCall := range response.ToolCalls {
			err := r.executeToolCall(ctx, observer, reactMemory, toolCatalog, toolCall)

			if err != nil {
				r.observeToolError(&ctx, err, iteration, toolCall.Function.Name)
				if r.stopOnError {
					r.observeStopOnError(&ctx, iteration, err)
					return nil, fmt.Errorf("tool execution failed at iteration %d: %w", iteration, err)
				}
			} else {
				toolsExecuted++
			}
		}

		r.observeNextIteration(&ctx, iteration, toolsExecuted, response)
	}

	execTimer.Stop()

	r.observeMaxIteration(&ctx)

	return nil, fmt.Errorf("reached maximum iterations (%d) without final answer", r.maxIterations)
}

// executeToolCall executes a single tool call and adds the result to memory.
func (r *ReAct[T]) executeToolCall(
	ctx context.Context,
	observer observability.Provider,
	mem memory.Provider,
	toolCatalog *tool.Catalog,
	toolCall ai.ToolCall,
) error {
	// Get overview for cost tracking
	executionOverview := overview.OverviewFromContext(&ctx)
	var span observability.Span

	if observer != nil {
		ctx, span = observer.StartSpan(ctx, "react.execute_tool",
			observability.String("tool_name", toolCall.Function.Name),
		)
		defer span.End()
	}

	start := time.Now()
	// Look up the tool in the catalog (catalog is case-insensitive by design)
	toolInstance, exists := toolCatalog.Get(toolCall.Function.Name)
	if !exists {
		err := fmt.Errorf("tool '%s' not found in catalog (case-insensitive lookup)", toolCall.Function.Name)
		duration := time.Since(start)
		if observer != nil {
			span.RecordError(err)
			span.SetStatus(observability.StatusError, "Tool not found")
			observer.Error(ctx, "Tool call failed - not found",
				observability.String("tool", toolCall.Function.Name),
				observability.Duration("duration", duration),
				observability.Error(err),
			)
		}

		// Add error as structured ToolResult to memory
		toolResult := ai.NewToolResultError(
			"tool_not_found",
			fmt.Sprintf("Tool '%s' not found. Available tools: %s",
				toolCall.Function.Name,
				strings.Join(getToolNames(toolCatalog), ", ")),
		)
		resultJSON, err := toolResult.ToJSON()
		if err != nil {
			resultJSON = fmt.Sprintf(`{"error":"failed to serialize tool result: %s"}`, err.Error())
		}
		mem.AppendMessage(ctx, &ai.Message{
			Role:       ai.RoleTool,
			Content:    resultJSON,
			ToolCallID: toolCall.ID,
			Name:       toolCall.Function.Name,
		})

		return err
	}

	// Execute tool
	result, err := toolInstance.Call(ctx, toolCall.Function.Arguments)
	duration := time.Since(start)

	// Prepare compact log attributes
	logAttrs := []observability.Attribute{
		observability.String("tool", toolCall.Function.Name),
		observability.Duration("duration", duration),
	}

	// Parse and add arguments as structured attributes
	var argsMap map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap); jsonErr == nil {
		for k, v := range argsMap {
			logAttrs = append(logAttrs, observability.String("in."+k, fmt.Sprintf("%v", v)))
		}
	} else {
		// Fallback to truncated string if not valid JSON
		logAttrs = append(logAttrs, observability.String("input", utils.TruncateString(toolCall.Function.Arguments, 100)))
	}

	if err != nil {
		logAttrs = append(logAttrs, observability.Error(err))

		if observer != nil {
			span.RecordError(err)
			span.SetStatus(observability.StatusError, "Tool execution error")
			observer.Error(ctx, "Tool call failed", logAttrs...)
		}

		// Add error as structured ToolResult to memory
		toolResult := ai.NewToolResultError("tool_execution_failed", err.Error())
		resultJSON, err := toolResult.ToJSON()
		if err != nil {
			resultJSON = fmt.Sprintf(`{"error":"failed to serialize tool result: %s"}`, err.Error())
		}
		mem.AppendMessage(ctx, &ai.Message{
			Role:       ai.RoleTool,
			Content:    resultJSON,
			ToolCallID: toolCall.ID,
			Name:       toolCall.Function.Name,
		})

		return err
	}

	// Add successful result to memory
	mem.AppendMessage(ctx, &ai.Message{
		Role:       ai.RoleTool,
		Content:    result,
		ToolCallID: toolCall.ID,
		Name:       toolCall.Function.Name,
	})

	// Track tool execution cost if available
	if toolMetrics := toolInstance.GetMetrics(); toolMetrics != nil {
		executionOverview.AddToolExecutionCost(toolCall.Function.Name, toolMetrics)
	}

	// Parse and add result as structured attributes if it's JSON
	var resultMap map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(result), &resultMap); jsonErr == nil {
		for k, v := range resultMap {
			logAttrs = append(logAttrs, observability.String("out."+k, fmt.Sprintf("%v", v)))
		}
	} else {
		// Fallback to truncated string if not valid JSON
		logAttrs = append(logAttrs, observability.String("output", utils.TruncateString(result, 100)))
	}

	if observer != nil {
		span.SetStatus(observability.StatusOK, "Tool executed successfully")
		observer.Info(ctx, "Tool call completed", logAttrs...)
	}

	return nil
}

func (r *ReAct[T]) observeMaxIteration(ctx *context.Context) {
	if r.state["observer"] == nil {
		return
	}
	observer, ok := r.state["observer"].(observability.Provider)
	if !ok {
		return
	}
	span, ok := r.state["span"].(observability.Span)
	if !ok {
		return
	}
	timer, ok := r.state["execTimer"].(*utils.Timer)
	if !ok {
		return
	}

	span.SetStatus(observability.StatusError, "Max iterations reached")
	observer.Warn(*ctx, "ReAct pattern reached max iterations without final answer",
		observability.Int("max_iterations", r.maxIterations),
		observability.Duration("total_duration", timer.GetDuration()),
	)

	observer.Counter("react.executions.total").Add(*ctx, 1,
		observability.String("status", "max_iterations_reached"),
	)

	span.End()
}

func (r *ReAct[T]) observeNextIteration(ctx *context.Context, iteration int, toolsExecuted int, response *ai.ChatResponse) {
	if r.state["observer"] == nil {
		return
	}
	observer, ok := r.state["observer"].(observability.Provider)
	if !ok {
		return
	}
	timer, ok := r.state["iterationTimer"].(*utils.Timer)
	if !ok {
		return
	}

	observer.Info(*ctx, "ReAct iteration completed - continuing to next iteration",
		observability.Int("iteration", iteration),
		observability.Int("tools_executed", toolsExecuted),
		observability.Int("tools_failed", len(response.ToolCalls)-toolsExecuted),
		observability.Duration("iteration_duration", timer.GetDuration()),
	)

	// Record iteration metrics
	observer.Counter("react.iterations.total").Add(*ctx, 1)
	observer.Counter("react.tools_executed.total").Add(*ctx, int64(toolsExecuted))
}

func (r *ReAct[T]) observeIterationError(ctx *context.Context, err error, iteration int) {
	if r.state["observer"] == nil {
		return
	}
	observer, ok := r.state["observer"].(observability.Provider)
	if !ok {
		return
	}
	span, ok := r.state["span"].(observability.Span)
	if !ok {
		return
	}
	timer, ok := r.state["iterationTimer"].(*utils.Timer)
	if !ok {
		return
	}
	execTimer, ok := r.state["execTimer"].(*utils.Timer)
	if !ok {
		return
	}

	execTimer.Stop()
	span.RecordError(err)
	span.SetStatus(observability.StatusError, "ReAct iteration failed")
	observer.Error(*ctx, "Iteration failed",
		observability.Int("iteration", iteration),
		observability.Duration("duration", timer.GetDuration()),
		observability.Error(err),
	)
	span.End()
}

func (r *ReAct[T]) observeToolError(ctx *context.Context, err error, iteration int, functionName string) {
	if r.state["observer"] == nil {
		return
	}
	observer, ok := r.state["observer"].(observability.Provider)
	if !ok {
		return
	}
	span, ok := r.state["span"].(observability.Span)
	if !ok {
		return
	}

	if r.stopOnError {
		span.RecordError(err)
		span.SetStatus(observability.StatusError, "Tool execution failed")
		observer.Error(*ctx, "Tool execution failed, stopping ReAct loop",
			observability.Error(err),
			observability.String("tool_name", functionName),
			observability.Int("iteration", iteration),
		)
		return
	}

	observer.Warn(*ctx, "Tool execution failed, continuing",
		observability.Error(err),
		observability.String("tool_name", functionName),
	)
}

func (r *ReAct[T]) observeStopOnError(ctx *context.Context, iteration int, err error) {
	if r.state["observer"] == nil {
		return
	}
	observer, ok := r.state["observer"].(observability.Provider)
	if !ok {
		return
	}
	span, ok := r.state["span"].(observability.Span)
	if !ok {
		return
	}
	timer, ok := r.state["execTimer"].(*utils.Timer)
	if !ok {
		return
	}

	timer.Stop()

	span.RecordError(err)
	span.SetStatus(observability.StatusError, "ReAct pattern stopped due to tool error")
	observer.Error(*ctx, "ReAct pattern terminated due to tool error",
		observability.Error(err),
		observability.Int("iteration", iteration),
	)
	observer.Counter("react.executions.total").Add(*ctx, 1,
		observability.String("status", "error"),
	)
	span.End()
}

func (r *ReAct[T]) observeStartIteration(ctx *context.Context, iteration int) {
	if r.state["observer"] == nil {
		return
	}
	observer, ok := r.state["observer"].(observability.Provider)
	if !ok {
		return
	}

	observer.Debug(*ctx, "ReAct iteration",
		observability.Int("iteration", iteration),
	)
}

func (r *ReAct[T]) observeRequestingStructuredFinalAnswer(ctx *context.Context, iteration int) {
	if r.state["observer"] == nil {
		return
	}
	observer, ok := r.state["observer"].(observability.Provider)
	if !ok {
		return
	}

	observer.Debug(*ctx, "Requesting structured final answer",
		observability.Int("iteration", iteration),
		observability.String("schema_type", fmt.Sprintf("%T", *new(T))),
	)
}

func (r *ReAct[T]) observeParseError(ctx *context.Context, err error, content string) {
	if r.state["observer"] == nil {
		return
	}
	observer, ok := r.state["observer"].(observability.Provider)
	if !ok {
		return
	}
	span, ok := r.state["span"].(observability.Span)
	if !ok {
		return
	}

	span.RecordError(err)
	observer.Error(*ctx, "Failed to parse final answer into structured type",
		observability.Error(err),
		observability.String("response_content", utils.TruncateString(content, 200)),
		observability.String("target_type", fmt.Sprintf("%T", *new(T))),
	)
}

func (r *ReAct[T]) observeSuccess(ctx *context.Context, response *ai.ChatResponse, iteration int) {
	if r.state["observer"] == nil {
		return
	}
	observer, ok := r.state["observer"].(observability.Provider)
	if !ok {
		return
	}
	span, ok := r.state["span"].(observability.Span)
	if !ok {
		return
	}
	timer, ok := r.state["iterationTimer"].(*utils.Timer)
	if !ok {
		return
	}
	execTimer, ok := r.state["execTimer"].(*utils.Timer)
	if !ok {
		return
	}

	execTimer.Stop()
	totalDuration := execTimer.GetDuration()

	span.SetStatus(observability.StatusOK, "ReAct completed successfully")
	observer.Info(*ctx, "ReAct pattern completed - final answer parsed successfully",
		observability.Int("total_iterations", iteration),
		observability.Duration("total_duration", totalDuration),
		observability.Duration("last_iteration_duration", timer.GetDuration()),
		observability.String("finish_reason", response.FinishReason),
		observability.String("result_type", fmt.Sprintf("%T", *new(T))),
	)

	// Record metrics
	observer.Counter("react.executions.total").Add(*ctx, 1,
		observability.String("status", "success"),
	)
	observer.Histogram("react.iterations.count").Record(*ctx, float64(iteration))
	observer.Histogram("react.duration.seconds").Record(*ctx, totalDuration.Seconds())

	span.End()
}

func (r *ReAct[T]) observeTools(ctx *context.Context, response *ai.ChatResponse, iteration int) {
	if r.state["observer"] == nil {
		return
	}
	observer, ok := r.state["observer"].(observability.Provider)
	if !ok {
		return
	}

	// Log tool calls as a list
	toolNames := make([]string, len(response.ToolCalls))
	for i, tc := range response.ToolCalls {
		toolNames[i] = tc.Function.Name
	}
	observer.Debug(*ctx, "Executing tools from LLM response",
		observability.Int("iteration", iteration),
		observability.StringSlice("tools", toolNames),
	)
}

func (r *ReAct[T]) observeInit(ctx *context.Context, prompt string, toolCatalog *tool.Catalog) {
	if r.state["observer"] == nil {
		return
	}
	observer, ok := r.state["observer"].(observability.Provider)
	if !ok {
		return
	}
	var span observability.Span

	*ctx, span = observer.StartSpan(*ctx, "react.execute",
		observability.String("pattern", "react"),
		observability.String("prompt", utils.TruncateStringDefault(prompt)),
		observability.Int("max_iterations", r.maxIterations),
		observability.String("result_type", fmt.Sprintf("%T", *new(T))),
	)

	*ctx = observability.ContextWithSpan(*ctx, span)
	*ctx = observability.ContextWithObserver(*ctx, observer)

	observer.Debug(*ctx, "Starting type-safe ReAct pattern",
		observability.Int("max_iterations", r.maxIterations),
		observability.Int("tools_available", toolCatalog.Size()),
		observability.String("result_type", fmt.Sprintf("%T", *new(T))),
	)

	r.state["span"] = span
}

// getToolNames returns a list of tool names from the catalog.
func getToolNames(catalog *tool.Catalog) []string {
	tools := catalog.Tools()
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	return names
}
