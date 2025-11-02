package client

import (
	"aigo/internal/jsonschema"
	"aigo/providers/ai"
	"aigo/providers/memory"
	"aigo/providers/observability"
	"aigo/providers/tool"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"time"
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
	toolCatalog      map[string]tool.GenericTool
	toolDescriptions []ai.ToolDescription
	outputSchema     *jsonschema.Schema
}

// ClientOptions contains all configuration for a Client.
type ClientOptions struct {
	// Required
	LlmProvider ai.Provider

	// Optional with sensible defaults
	DefaultModel   string                 // Model to use for requests (can be overridden per-request in future)
	MemoryProvider memory.Provider        // Optional: if nil, client operates in stateless mode
	Observer       observability.Provider // Defaults to nil (zero overhead)
	SystemPrompt   string                 // System prompt for all requests
	Tools          []tool.GenericTool     // Tools available to the LLM
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
	toolCatalog := make(map[string]tool.GenericTool, len(options.Tools))
	toolDescriptions := make([]ai.ToolDescription, 0, len(options.Tools))

	for i, t := range options.Tools {
		info := t.ToolInfo()
		toolCatalog[info.Name] = options.Tools[i]
		toolDescriptions = append(toolDescriptions, info)
	}

	return &Client[T]{
		systemPrompt:     options.SystemPrompt,
		defaultModel:     options.DefaultModel,
		llmProvider:      options.LlmProvider,
		memoryProvider:   options.MemoryProvider,
		observer:         options.Observer,
		toolCatalog:      toolCatalog,
		toolDescriptions: toolDescriptions,
		outputSchema:     jsonschema.GenerateJSONSchema[T](),
	}, nil
}

// SendMessage sends a user message to the LLM and returns the response.
// This is a basic orchestration method that:
// 1. Appends the user message to memory (if memory provider is set)
// 2. Sends messages + tools + schema to the LLM provider
// 3. Records observability data
// 4. Returns the raw response
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

		// INFO: High-level operation starting
		c.observer.Info(ctx, "Sending message to LLM",
			observability.String(observability.AttrLLMModel, c.defaultModel),
			observability.Int(observability.AttrClientToolsCount, len(c.toolDescriptions)),
		)

		// DEBUG: Include truncated prompt content for debugging
		c.observer.Debug(ctx, "Message content",
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
		// DEBUG: Detailed metrics
		c.observer.Histogram(observability.MetricClientRequestDuration).Record(ctx, duration.Seconds(),
			observability.String(observability.AttrLLMModel, c.defaultModel),
		)

		c.observer.Counter(observability.MetricClientRequestCount).Add(ctx, 1,
			observability.String(observability.AttrStatus, "success"),
			observability.String(observability.AttrLLMModel, c.defaultModel),
		)

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

			// Set span attributes for token usage
			span.SetAttributes(
				observability.Int(observability.AttrLLMTokensTotal, response.Usage.TotalTokens),
				observability.Int(observability.AttrLLMTokensPrompt, response.Usage.PromptTokens),
				observability.Int(observability.AttrLLMTokensCompletion, response.Usage.CompletionTokens),
			)

			// DEBUG: Token usage details
			c.observer.Debug(ctx, "Token usage",
				observability.Int(observability.AttrLLMTokensTotal, response.Usage.TotalTokens),
				observability.Int(observability.AttrLLMTokensPrompt, response.Usage.PromptTokens),
				observability.Int(observability.AttrLLMTokensCompletion, response.Usage.CompletionTokens),
			)
		}

		// INFO: Operation completed successfully with key summary info
		c.observer.Info(ctx, "Message sent successfully",
			observability.String(observability.AttrLLMModel, c.defaultModel),
			observability.String(observability.AttrLLMFinishReason, response.FinishReason),
			observability.Duration(observability.AttrDuration, duration),
			observability.Int(observability.AttrClientToolCalls, len(response.ToolCalls)),
		)

		// DEBUG: Include truncated response content
		if response.Content != "" {
			c.observer.Debug(ctx, "Response content",
				observability.String(observability.AttrResponseContent, observability.TruncateStringDefault(response.Content)),
			)
		}

		span.SetStatus(observability.StatusOK, "Message sent successfully")
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
