package react

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/memory"
	"github.com/leofalp/aigo/providers/observability"
	"github.com/leofalp/aigo/providers/tool"
)

// ReactPattern wraps a base client and adds ReAct pattern behavior:
// automatic tool execution loop with reasoning.
type ReactPattern[T any] struct {
	client        *client.Client[T]
	maxIterations int
	stopOnError   bool
}

// Option is a functional option for configuring ReactPattern.
type Option func(*ReactPattern[any])

// WithMaxIterations sets the maximum number of tool execution iterations.
// Default: 10
func WithMaxIterations(max int) Option {
	return func(rc *ReactPattern[any]) {
		rc.maxIterations = max
	}
}

// WithStopOnError configures whether to stop execution on tool errors.
// Default: true
func WithStopOnError(stop bool) Option {
	return func(rc *ReactPattern[any]) {
		rc.stopOnError = stop
	}
}

// NewReactPattern creates a new ReAct pattern that wraps a base client.
// The base client should be configured with memory, tools, and observer.
//
// Memory is required for the ReAct pattern to work (the LLM needs to see tool results).
// If the client is in stateless mode, this function will return an error.
//
// Example:
//
//	baseClient, _ := client.NewClient[string](
//	    provider,
//	    client.WithMemory(memory),
//	    client.WithTools(tool1, tool2),
//	    client.WithObserver(observer),
//	)
//
//	reactClient, _ := react.NewReactPattern(
//	    baseClient,
//	    react.WithMaxIterations(5),
//	    react.WithStopOnError(true),
//	)
func NewReactPattern[T any](baseClient *client.Client[T], opts ...Option) (*ReactPattern[T], error) {
	// Validate that memory is configured (required for ReAct)
	if baseClient.Memory() == nil {
		return nil, fmt.Errorf("ReAct pattern requires memory: client must be configured with WithMemory()")
	}

	// Create ReactPattern with defaults
	rc := &ReactPattern[T]{
		client:        baseClient,
		maxIterations: 10,
		stopOnError:   true,
	}

	// Apply options (type-erased to work with generic type)
	for _, opt := range opts {
		opt((*ReactPattern[any])(unsafe.Pointer(rc)))
	}

	return rc, nil
}

// Execute runs the ReAct pattern loop:
// 1. Send user prompt to LLM
// 2. If LLM requests tool calls, execute them and add results to memory
// 3. Repeat until LLM provides final answer or max iterations reached
//
// Returns the final response from the LLM after the reasoning loop completes.
func (r *ReactPattern[T]) Execute(ctx context.Context, prompt string) (*ai.ChatResponse, error) {
	// Get memory and tool catalog from client
	reactMemory := r.client.Memory()
	toolCatalog := r.client.ToolCatalog()

	// Start top-level ReAct span
	observer := r.client.Observer()
	if observer == nil {
		observer = observability.ObserverFromContext(ctx)
	}
	var span observability.Span
	if observer != nil {
		ctx, span = observer.StartSpan(ctx, "react.execute",
			observability.String("pattern", "react"),
			observability.String("prompt", observability.TruncateStringDefault(prompt)),
			observability.Int("max_iterations", r.maxIterations),
		)
		defer span.End()

		ctx = observability.ContextWithSpan(ctx, span)
		ctx = observability.ContextWithObserver(ctx, observer)

		observer.Debug(ctx, "Starting ReAct pattern",
			observability.Int("max_iterations", r.maxIterations),
			observability.Int("tools_available", toolCatalog.Size()),
		)
	}

	startTime := time.Now()
	iteration := 0
	var response *ai.ChatResponse
	var err error

	// Main ReAct loop
	for iteration < r.maxIterations {
		iteration++

		if observer != nil {
			observer.Debug(ctx, "ReAct iteration",
				observability.Int("iteration", iteration),
			)
		}

		// Step 1: Send message to LLM (first iteration uses prompt, subsequent use empty string)
		iterationStart := time.Now()
		var message string
		if iteration == 1 {
			message = prompt
		} else {
			// Empty message allows LLM to process tool results from reactMemory
			message = ""
		}

		response, err = r.client.SendMessage(ctx, message)
		iterationDuration := time.Since(iterationStart)

		if err != nil {
			if observer != nil {
				span.RecordError(err)
				span.SetStatus(observability.StatusError, "ReAct iteration failed")
				observer.Error(ctx, "Iteration failed",
					observability.Int("iteration", iteration),
					observability.Duration("duration", iterationDuration),
					observability.Error(err),
				)
			}
			return nil, fmt.Errorf("iteration %d failed: %w", iteration, err)
		}

		// Step 2: Check if we're done (no tool calls = final answer)
		if len(response.ToolCalls) == 0 {
			totalDuration := time.Since(startTime)

			if observer != nil {
				span.SetStatus(observability.StatusOK, "ReAct completed successfully")
				observer.Info(ctx, "ReAct pattern completed - no tool calls, final answer received",
					observability.Int("total_iterations", iteration),
					observability.Duration("total_duration", totalDuration),
					observability.String("finish_reason", response.FinishReason),
					observability.Bool("has_content", response.Content != ""),
				)

				// Record metrics
				observer.Counter("react.executions.total").Add(ctx, 1,
					observability.String("status", "success"),
				)
				observer.Histogram("react.iterations.count").Record(ctx, float64(iteration))
				observer.Histogram("react.duration.seconds").Record(ctx, totalDuration.Seconds())
			}

			return response, nil
		}

		// Step 3: Execute tool calls
		if observer != nil {
			// Log tool calls as a list
			toolNames := make([]string, len(response.ToolCalls))
			for i, tc := range response.ToolCalls {
				toolNames[i] = tc.Function.Name
			}
			observer.Debug(ctx, "Executing tools from LLM response",
				observability.Int("iteration", iteration),
				observability.StringSlice("tools", toolNames),
			)
		}

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
				if r.stopOnError {
					if observer != nil {
						span.RecordError(err)
						span.SetStatus(observability.StatusError, "Tool execution failed")
						observer.Error(ctx, "Tool execution failed, stopping ReAct loop",
							observability.Error(err),
							observability.String("tool_name", toolCall.Function.Name),
							observability.Int("iteration", iteration),
						)
					}
					return nil, fmt.Errorf("tool execution failed at iteration %d: %w", iteration, err)
				}

				// Continue with error message in reactMemory
				if observer != nil {
					observer.Warn(ctx, "Tool execution failed, continuing",
						observability.Error(err),
						observability.String("tool_name", toolCall.Function.Name),
					)
				}
			} else {
				toolsExecuted++
			}
		}

		if observer != nil {
			observer.Info(ctx, "ReAct iteration completed - continuing to next iteration",
				observability.Int("iteration", iteration),
				observability.Int("tools_executed", toolsExecuted),
				observability.Int("tools_failed", len(response.ToolCalls)-toolsExecuted),
				observability.Duration("iteration_duration", iterationDuration),
			)

			// Record iteration metrics
			observer.Counter("react.iterations.total").Add(ctx, 1)
			observer.Counter("react.tools_executed.total").Add(ctx, int64(toolsExecuted))
		}
	}

	// Max iterations reached without final answer
	totalDuration := time.Since(startTime)

	if observer != nil {
		span.SetStatus(observability.StatusError, "Max iterations reached")
		observer.Warn(ctx, "ReAct pattern reached max iterations without final answer",
			observability.Int("max_iterations", r.maxIterations),
			observability.Duration("total_duration", totalDuration),
		)

		observer.Counter("react.executions.total").Add(ctx, 1,
			observability.String("status", "max_iterations_reached"),
		)
	}

	return response, fmt.Errorf("reached maximum iterations (%d) without final answer", r.maxIterations)
}

// executeToolCall executes a single tool call and adds the result to memory.
func (r *ReactPattern[T]) executeToolCall(
	ctx context.Context,
	observer observability.Provider,
	mem memory.Provider,
	toolCatalog *tool.Catalog,
	toolCall ai.ToolCall,
) error {
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
		resultJSON, _ := toolResult.ToJSON()
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
		logAttrs = append(logAttrs, observability.String("input", observability.TruncateString(toolCall.Function.Arguments, 100)))
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
		resultJSON, _ := toolResult.ToJSON()
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

	// Parse and add result as structured attributes if it's JSON
	var resultMap map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(result), &resultMap); jsonErr == nil {
		for k, v := range resultMap {
			logAttrs = append(logAttrs, observability.String("out."+k, fmt.Sprintf("%v", v)))
		}
	} else {
		// Fallback to truncated string if not valid JSON
		logAttrs = append(logAttrs, observability.String("output", observability.TruncateString(result, 100)))
	}

	if observer != nil {
		span.SetStatus(observability.StatusOK, "Tool executed successfully")
		observer.Info(ctx, "Tool call completed", logAttrs...)
	}

	return nil
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
