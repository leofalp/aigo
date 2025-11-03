package client

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/memory"
	"github.com/leofalp/aigo/providers/observability"
	"github.com/leofalp/aigo/providers/tool"
)

const (
	envDefaultModel = "AIGO_DEFAULT_LLM_MODEL"
)

// Client is an immutable orchestrator for LLM interactions.
// All configuration must be provided at construction time via Options.
type Client[T any] struct {
	systemPrompt     string
	defaultModel     string
	llmProvider      ai.Provider
	memoryProvider   memory.Provider
	observer         observability.Provider // nil if not set (zero overhead)
	toolCatalog      *tool.Catalog
	toolDescriptions []ai.ToolDescription
	outputSchema     *jsonschema.Schema
}

// ClientOptions contains all configuration for a Client.
type ClientOptions struct {
	// Required
	LlmProvider ai.Provider

	// Optional with sensible defaults
	DefaultModel                       string                 // Model to use for requests (can be overridden per-request in future)
	MemoryProvider                     memory.Provider        // Optional: if nil, client operates in stateless mode
	Observer                           observability.Provider // Defaults to nil (zero overhead)
	SystemPrompt                       string                 // System prompt for all requests
	Tools                              []tool.GenericTool     // Tools available to the LLM
	EnrichSystemPromptWithToolDescr    bool                   // If true, automatically append tool descriptions to system prompt (default: false)
	EnrichSystemPromptWithOutputSchema bool                   // If true, automatically append output structure guidance to system prompt (default: false)
}

// Functional option pattern for ergonomic API

func WithDefaultModel(model string) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.DefaultModel = model
	}
}

func WithMemory(memProvider memory.Provider) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.MemoryProvider = memProvider
	}
}

func WithObserver(observer observability.Provider) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.Observer = observer
	}
}

func WithSystemPrompt(prompt string) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.SystemPrompt = prompt
	}
}

func WithTools(tools ...tool.GenericTool) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.Tools = append(o.Tools, tools...)
	}
}

// WithEnrichSystemPromptWithToolsDescriptions enables automatic enrichment of the system prompt
// with tool descriptions. When enabled, the client will append detailed
// information about available tools to the system prompt, helping the LLM
// understand when and how to use them.
//
// This is disabled by default to maintain backward compatibility and give
// users full control over system prompts.
//
// Example usage:
//
//	client.NewClient(provider,
//	    client.WithSystemPrompt("You are a helpful assistant."),
//	    client.WithTools(calcTool, searchTool),
//	    client.WithEnrichSystemPromptWithToolsDescriptions(), // Adds tool guidance automatically
//	)
func WithEnrichSystemPromptWithToolsDescriptions() func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.EnrichSystemPromptWithToolDescr = true
	}
}

// WithEnrichSystemPromptWithOutputSchema enables automatic enrichment of the system prompt
// with output schema guidance. When enabled, the client will append information
// about the expected output structure based on the generic type T.
//
// This is disabled by default to maintain backward compatibility and give
// users full control over system prompts.
//
// Example usage:
//
//	client.NewClient[MyResponse](provider,
//	    client.WithSystemPrompt("You are a helpful assistant."),
//	    client.WithEnrichSystemPromptWithOutputSchema(), // Adds output schema guidance automatically
//	)
func WithEnrichSystemPromptWithOutputSchema() func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.EnrichSystemPromptWithOutputSchema = true
	}
}

// NewClient creates a new immutable Client instance.
// The llmProvider is required as the first argument.
// All other configuration is provided via functional options.
//
// Example:
//
//	client, err := NewClient[MyResponse](
//	    openaiProvider,
//	    WithDefaultModel("gpt-4"),
//	    WithObserver(myObserver),
//	    WithSystemPrompt("You are a helpful assistant"),
//	    WithTools(tool1, tool2),
//	    WithEnrichSystemPromptWithToolsDescriptions(), // Optionally add tool guidance
//	)
func NewClient[T any](llmProvider ai.Provider, opts ...func(*ClientOptions)) (*Client[T], error) {
	options := &ClientOptions{
		LlmProvider: llmProvider,
		// MemoryProvider is optional (nil = stateless mode)
	}

	for _, opt := range opts {
		opt(options)
	}

	// Validation
	if options.LlmProvider == nil {
		return nil, errors.New("llmProvider is required and cannot be nil")
	}

	// Use default model from environment if not specified
	if options.DefaultModel == "" {
		options.DefaultModel = os.Getenv(envDefaultModel)
	}

	// Build tool catalog and descriptions
	toolCatalog := tool.NewCatalogWithTools(options.Tools...)
	toolDescriptions := make([]ai.ToolDescription, 0, len(options.Tools))

	for _, t := range options.Tools {
		info := t.ToolInfo()
		toolDescriptions = append(toolDescriptions, info)
	}

	// Enrich system prompt  if enabled
	systemPrompt := options.SystemPrompt
	if options.EnrichSystemPromptWithToolDescr && len(toolDescriptions) > 0 {
		systemPrompt = enrichSystemPromptWithTools(options.SystemPrompt, toolDescriptions)
	}
	if options.EnrichSystemPromptWithOutputSchema {
		systemPrompt = enrichSystemPromptWithOutputSchema[T](systemPrompt)
	}

	var outputSchema *jsonschema.Schema
	// Generate output schema only for non-string types
	if reflect.TypeOf((*T)(nil)).Elem().Kind() != reflect.String {
		outputSchema = jsonschema.GenerateJSONSchema[T]()
	}
	return &Client[T]{
		systemPrompt:     systemPrompt,
		defaultModel:     options.DefaultModel,
		llmProvider:      options.LlmProvider,
		memoryProvider:   options.MemoryProvider,
		observer:         options.Observer,
		toolCatalog:      toolCatalog,
		toolDescriptions: toolDescriptions,
		outputSchema:     outputSchema,
	}, nil
}

// Memory returns the memory provider configured for this client.
// Returns nil if the client is in stateless mode (no memory configured).
func (c *Client[T]) Memory() memory.Provider {
	return c.memoryProvider
}

// ToolCatalog returns the tool catalog for this client.
// The returned map is a copy to prevent external modifications.
// Keys are tool names (as registered), values are tool instances.
func (c *Client[T]) ToolCatalog() *tool.Catalog {
	// Return a clone to maintain immutability
	return c.toolCatalog.Clone()
}

// Observer returns the observability provider configured for this client.
// Returns nil if no observer is configured (zero overhead mode).
func (c *Client[T]) Observer() observability.Provider {
	return c.observer
}

// enrichSystemPromptWithTools appends tool usage guidance to the system prompt.
// This helps LLMs understand when and how to use available tools.
func enrichSystemPromptWithTools(basePrompt string, tools []ai.ToolDescription) string {
	if len(tools) == 0 {
		return basePrompt
	}

	enrichment := "\n\n## Available Tools\n\n"
	enrichment += "You have access to the following tools. Use them when appropriate to provide accurate and helpful responses:\n\n"

	for i, singleTool := range tools {
		enrichment += strconv.Itoa(i+1) + ". **" + singleTool.Name + "**"
		if singleTool.Description != "" {
			enrichment += "\n   - Description: " + singleTool.Description
		}

		// Add parameter information if available
		if singleTool.Parameters != nil {
			if paramsJSON, err := json.Marshal(singleTool.Parameters); err == nil {
				enrichment += "\n   - Parameters: " + string(paramsJSON)
			}
		}
		// TODO describe also the output

		enrichment += "\n"
	}

	enrichment += "\n**Important:** When you need to use a tool, call it using the function calling format. "
	enrichment += "The system will execute the tool and provide you with the results, which you should then use to formulate your final response."

	return basePrompt + enrichment
}

// enrichSystemPromptWithTools appends tool usage guidance to the system prompt.
// This helps LLMs understand when and how to use available tools.
func enrichSystemPromptWithOutputSchema[T any](basePrompt string) string {
	if reflect.TypeOf((*T)(nil)).Elem().Kind() == reflect.String {
		return basePrompt
	}
	enrichment := "\n\n## Output Format\n\n"
	enrichment += "Please structure your responses in strict accordance with the following JSON schema:\n\n"

	schema := jsonschema.GenerateJSONSchema[T]()
	if schemaJSON, err := json.MarshalIndent(schema, "", "  "); err == nil {
		enrichment += "```json\n" + string(schemaJSON) + "\n```\n"
	}

	enrichment += "\nEnsure that all your responses conform exactly to this schema. This will help in parsing and utilizing your responses effectively."
	return basePrompt + enrichment
}

// SendMessage sends a user message to the LLM and returns the response.
// This is a basic orchestration method that:
// 1. Appends the user message to memory (if memory provider is set)
// 2. Sends messages + tools + schema to the LLM provider
// 3. Records observability data
// 4. Returns the raw response
//
// The prompt parameter must be non-empty. If you need to continue a conversation
// without adding a new user message (e.g., after tool execution), use ContinueConversation() instead.
//
// If no memory provider is configured, the client operates in stateless mode,
// sending only the current prompt as a single user message.
//
// TODO: Future enhancements:
// - Accept per-request options (model override, temperature, etc.)
// - Support streaming responses
// - Add timeout/cancellation handling
// - Support context propagation for distributed tracing
//
// Note: This method does NOT execute tool calls automatically.
// Tool execution loops should be implemented as higher-level patterns.
func (c *Client[T]) SendMessage(ctx context.Context, prompt string) (*ai.ChatResponse, error) {
	// Validate prompt is non-empty
	if prompt == "" {
		return nil, errors.New("prompt cannot be empty; use ContinueConversation() to continue without adding a user message")
	}
	// Start tracing span (only if observer is set)
	var span observability.Span
	if c.observer != nil {
		// Truncate prompt for span attributes to avoid huge attribute values
		truncatedPrompt := observability.TruncateStringDefault(prompt)

		ctx, span = c.observer.StartSpan(ctx, observability.SpanClientSendMessage,
			observability.String(observability.AttrLLMModel, c.defaultModel),
			observability.String(observability.AttrClientPrompt, truncatedPrompt),
		)
		defer span.End()

		// Put span and observer in context for downstream propagation
		ctx = observability.ContextWithSpan(ctx, span)
		ctx = observability.ContextWithObserver(ctx, c.observer)

		// DEBUG: Operation starting with prompt
		c.observer.Debug(ctx, "Sending message to LLM",
			observability.String(observability.AttrLLMModel, c.defaultModel),
			observability.Int(observability.AttrClientToolsCount, len(c.toolDescriptions)),
			observability.String(observability.AttrClientPrompt, truncatedPrompt),
		)
	}

	start := time.Now()

	// Build messages list based on memory provider availability
	var messages []ai.Message
	if c.memoryProvider != nil {
		// Stateful mode: append to memory and use all messages
		c.memoryProvider.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: prompt})
		messages = c.memoryProvider.AllMessages()

		if c.observer != nil {
			c.observer.Debug(ctx, "Using stateful mode with memory",
				observability.Int(observability.AttrMemoryTotalMessages, len(messages)),
			)
		}
	} else {
		// Stateless mode: use only the current prompt
		messages = []ai.Message{
			{Role: ai.RoleUser, Content: prompt},
		}

		if c.observer != nil {
			c.observer.Debug(ctx, "Using stateless mode (no memory)")
		}
	}

	// Build complete request with all configuration
	request := ai.ChatRequest{
		Model:        c.defaultModel,
		Messages:     messages,
		SystemPrompt: c.systemPrompt,
		Tools:        c.toolDescriptions,
	}

	// Add response format if output schema is defined (for structured output)
	if c.outputSchema != nil {
		request.ResponseFormat = &ai.ResponseFormat{
			Type:         "json_schema",
			OutputSchema: c.outputSchema,
		}
	}

	// Send to LLM provider
	response, err := c.llmProvider.SendMessage(ctx, request)

	duration := time.Since(start)

	if err != nil {
		if c.observer != nil {
			span.RecordError(err)
			span.SetStatus(observability.StatusError, "Failed to send message")

			// ERROR: Operation failed
			c.observer.Error(ctx, "Failed to send message to LLM",
				observability.Error(err),
				observability.Duration(observability.AttrDuration, duration),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)

			// Metrics at DEBUG level
			c.observer.Counter(observability.MetricClientRequestCount).Add(ctx, 1,
				observability.String(observability.AttrStatus, "error"),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)
		}

		return nil, err
	}

	// Record success metrics and observability
	if c.observer != nil {
		// Record metrics
		c.observer.Histogram(observability.MetricClientRequestDuration).Record(ctx, duration.Seconds(),
			observability.String(observability.AttrLLMModel, c.defaultModel),
		)

		c.observer.Counter(observability.MetricClientRequestCount).Add(ctx, 1,
			observability.String(observability.AttrStatus, "success"),
			observability.String(observability.AttrLLMModel, c.defaultModel),
		)

		// Prepare compact log attributes
		logAttrs := []observability.Attribute{
			observability.String(observability.AttrLLMModel, c.defaultModel),
			observability.String(observability.AttrLLMFinishReason, response.FinishReason),
			observability.Duration(observability.AttrDuration, duration),
			observability.Int(observability.AttrClientToolCalls, len(response.ToolCalls)),
		}

		// Add token usage if available
		if response.Usage != nil {
			c.observer.Counter(observability.MetricClientTokensTotal).Add(ctx, int64(response.Usage.TotalTokens),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)
			c.observer.Counter(observability.MetricClientTokensPrompt).Add(ctx, int64(response.Usage.PromptTokens),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)
			c.observer.Counter(observability.MetricClientTokensCompletion).Add(ctx, int64(response.Usage.CompletionTokens),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)

			span.SetAttributes(
				observability.Int(observability.AttrLLMTokensTotal, response.Usage.TotalTokens),
				observability.Int(observability.AttrLLMTokensPrompt, response.Usage.PromptTokens),
				observability.Int(observability.AttrLLMTokensCompletion, response.Usage.CompletionTokens),
			)

			logAttrs = append(logAttrs,
				observability.Int(observability.AttrLLMTokensPrompt, response.Usage.PromptTokens),
				observability.Int(observability.AttrLLMTokensCompletion, response.Usage.CompletionTokens),
				observability.Int(observability.AttrLLMTokensTotal, response.Usage.TotalTokens),
			)
		}

		// Add tool call names if present
		if len(response.ToolCalls) > 0 {
			toolNames := make([]string, len(response.ToolCalls))
			for i, tc := range response.ToolCalls {
				toolNames[i] = tc.Function.Name
			}
			logAttrs = append(logAttrs,
				observability.StringSlice("tool_calls", toolNames),
			)
		}

		// Add response content preview if present
		if response.Content != "" {
			logAttrs = append(logAttrs,
				observability.String("response", observability.TruncateString(response.Content, 100)),
			)
		}

		// Single INFO log with all information
		c.observer.Info(ctx, "LLM call completed", logAttrs...)

		span.SetStatus(observability.StatusOK, "Message sent successfully")
	}

	return c.responseParser(response)
}

// ContinueConversation continues the conversation without adding a new user message.
// This is useful after tool execution to let the LLM process tool results that are
// already in memory.
//
// This method only works in stateful mode (when a memory provider is configured).
// It will return an error if called on a client without memory.
//
// Example usage in a tool execution loop:
//
//	// User asks a question
//	resp1, _ := client.SendMessage(ctx, "What is 42 * 17?")
//
//	// LLM requests a tool call, execute it and add result to memory
//	toolResult := executeTool(resp1.ToolCalls[0])
//	client.Memory().AppendMessage(ctx, &ai.Message{
//	    Role:       ai.RoleTool,
//	    Content:    toolResult,
//	    ToolCallID: resp1.ToolCalls[0].ID,
//	    Name:       resp1.ToolCalls[0].Function.Name,
//	})
//
//	// Continue conversation to let LLM process the tool result
//	resp2, _ := client.ContinueConversation(ctx)
//	// resp2 now contains the final answer using the tool result
func (c *Client[T]) ContinueConversation(ctx context.Context) (*ai.ChatResponse, error) {
	// Validate that memory provider is configured
	if c.memoryProvider == nil {
		return nil, errors.New("ContinueConversation requires a memory provider; create client with WithMemory() option")
	}

	// Start tracing span (only if observer is set)
	var span observability.Span
	if c.observer != nil {
		ctx, span = c.observer.StartSpan(ctx, observability.SpanClientSendMessage,
			observability.String(observability.AttrLLMModel, c.defaultModel),
			observability.Bool(observability.AttrClientContinuingConversation, true),
		)
		defer span.End()

		// Put span and observer in context for downstream propagation
		ctx = observability.ContextWithSpan(ctx, span)
		ctx = observability.ContextWithObserver(ctx, c.observer)

		// DEBUG: Operation starting
		c.observer.Debug(ctx, "Continuing conversation without new user message",
			observability.String(observability.AttrLLMModel, c.defaultModel),
			observability.Int(observability.AttrClientToolsCount, len(c.toolDescriptions)),
		)
	}

	start := time.Now()

	// Get all messages from memory
	messages := c.memoryProvider.AllMessages()

	if c.observer != nil {
		c.observer.Debug(ctx, "Using stateful mode with memory",
			observability.Int(observability.AttrMemoryTotalMessages, len(messages)),
			observability.Bool(observability.AttrClientContinuingConversation, true),
		)
	}

	// Build complete request with all configuration
	request := ai.ChatRequest{
		Model:        c.defaultModel,
		Messages:     messages,
		SystemPrompt: c.systemPrompt,
		Tools:        c.toolDescriptions,
	}

	// Add response format if output schema is defined (for structured output)
	if c.outputSchema != nil {
		request.ResponseFormat = &ai.ResponseFormat{
			Type:         "json_schema",
			OutputSchema: c.outputSchema,
		}
	}

	// Send to LLM provider
	response, err := c.llmProvider.SendMessage(ctx, request)

	duration := time.Since(start)

	if err != nil {
		if c.observer != nil {
			span.RecordError(err)
			span.SetStatus(observability.StatusError, "Failed to continue conversation")

			// ERROR: Operation failed
			c.observer.Error(ctx, "Failed to continue conversation with LLM",
				observability.Error(err),
				observability.Duration(observability.AttrDuration, duration),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)

			// Metrics at DEBUG level
			c.observer.Counter(observability.MetricClientRequestCount).Add(ctx, 1,
				observability.String(observability.AttrStatus, "error"),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)
		}

		return nil, err
	}

	// Record success metrics and observability
	if c.observer != nil {
		// Record metrics
		c.observer.Histogram(observability.MetricClientRequestDuration).Record(ctx, duration.Seconds(),
			observability.String(observability.AttrLLMModel, c.defaultModel),
		)

		c.observer.Counter(observability.MetricClientRequestCount).Add(ctx, 1,
			observability.String(observability.AttrStatus, "success"),
			observability.String(observability.AttrLLMModel, c.defaultModel),
		)

		// Prepare compact log attributes
		logAttrs := []observability.Attribute{
			observability.String(observability.AttrLLMModel, c.defaultModel),
			observability.String(observability.AttrLLMFinishReason, response.FinishReason),
			observability.Duration(observability.AttrDuration, duration),
			observability.Int(observability.AttrClientToolCalls, len(response.ToolCalls)),
		}

		// Add token usage if available
		if response.Usage != nil {
			c.observer.Counter(observability.MetricClientTokensTotal).Add(ctx, int64(response.Usage.TotalTokens),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)
			c.observer.Counter(observability.MetricClientTokensPrompt).Add(ctx, int64(response.Usage.PromptTokens),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)
			c.observer.Counter(observability.MetricClientTokensCompletion).Add(ctx, int64(response.Usage.CompletionTokens),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)

			span.SetAttributes(
				observability.Int(observability.AttrLLMTokensTotal, response.Usage.TotalTokens),
				observability.Int(observability.AttrLLMTokensPrompt, response.Usage.PromptTokens),
				observability.Int(observability.AttrLLMTokensCompletion, response.Usage.CompletionTokens),
			)

			logAttrs = append(logAttrs,
				observability.Int(observability.AttrLLMTokensPrompt, response.Usage.PromptTokens),
				observability.Int(observability.AttrLLMTokensCompletion, response.Usage.CompletionTokens),
				observability.Int(observability.AttrLLMTokensTotal, response.Usage.TotalTokens),
			)
		}

		// Add tool call names if present
		if len(response.ToolCalls) > 0 {
			toolNames := make([]string, len(response.ToolCalls))
			for i, tc := range response.ToolCalls {
				toolNames[i] = tc.Function.Name
			}
			logAttrs = append(logAttrs,
				observability.StringSlice("tool_calls", toolNames),
			)
		}

		// Add response content preview if present
		if response.Content != "" {
			logAttrs = append(logAttrs,
				observability.String("response", observability.TruncateString(response.Content, 100)),
			)
		}

		// Single INFO log with all information
		c.observer.Info(ctx, "LLM call completed (continued conversation)", logAttrs...)

		span.SetStatus(observability.StatusOK, "Conversation continued successfully")
	}

	return c.responseParser(response)
}

// responseParser validates and parses the response content according to the generic type T.
// It attempts to parse the response into the expected type, providing a fallback with warning
// if parsing fails.
func (c *Client[T]) responseParser(response *ai.ChatResponse) (*ai.ChatResponse, error) {
	var typedVar T
	var err error

	switch reflect.TypeOf(typedVar).Kind().String() {
	case "string":
		return response, nil
	case "bool":
		_, err = strconv.ParseBool(response.Content)
	case "float32", "float64":
		_, err = strconv.ParseFloat(response.Content, 64)
	case "int", "int8", "int16", "int32", "int64":
		_, err = strconv.ParseInt(response.Content, 10, 64)
	default:
		err = json.Unmarshal([]byte(response.Content), &typedVar)
	}

	if err != nil {
		response.Content = "[Warning] Could not parse response: " + err.Error() + " --> providing raw response content as fallback.\n\n" + response.Content
	}

	return response, nil
}
