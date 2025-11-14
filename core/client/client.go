package client

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/patterns"
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
type Client struct {
	systemPrompt        string
	defaultModel        string
	defaultOutputSchema *jsonschema.Schema // Optional: JSON schema for structured output (applied to all requests unless overridden)
	llmProvider         ai.Provider
	memoryProvider      memory.Provider
	observer            observability.Provider // nil if not set (zero overhead)
	toolCatalog         *tool.Catalog
	toolDescriptions    []ai.ToolDescription
	requiredTools       []ai.ToolDescription
	state               map[string]any
}

// ClientOptions contains all configuration for a Client.
type ClientOptions struct {
	// Required
	LlmProvider ai.Provider

	// Optional with sensible defaults
	DefaultModel                    string                 // Model to use for requests (can be overridden per-request in future)
	DefaultOutputSchema             *jsonschema.Schema     // Optional: JSON schema for structured output (applied to all requests unless overridden)
	MemoryProvider                  memory.Provider        // Optional: if nil, client operates in stateless mode
	Observer                        observability.Provider // Defaults to nil (zero overhead)
	SystemPrompt                    string                 // System prompt for all requests
	Tools                           []tool.GenericTool     // Tools available to the LLM
	RequiredTools                   []tool.GenericTool
	EnrichSystemPromptWithToolDescr bool // If true, automatically append tool descriptions to system prompt (default: false)
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

func WithRequiredTool(tools ...tool.GenericTool) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.RequiredTools = append(o.RequiredTools, tools...)
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
//	client := NewClient(provider,
//	    WithSystemPrompt("You are a helpful assistant."),
//	    WithTools(calcTool, searchTool),
//	    WithEnrichSystemPromptWithToolsDescriptions(), // Adds tool guidance automatically
//	)
func WithEnrichSystemPromptWithToolsDescriptions() func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.EnrichSystemPromptWithToolDescr = true
	}
}

// WithDefaultOutputSchema sets a default JSON schema for structured output.
// This schema will be applied to all requests unless overridden per-request
// using WithOutputSchema in SendMessage or ContinueConversation.
//
// This is useful when you want consistent structured output across all interactions,
// such as in ReAct patterns or multi-turn conversations.
//
// Example usage:
//
//	type MyResponse struct {
//	    Answer string `json:"answer" jsonschema:"required"`
//	    Confidence float64 `json:"confidence" jsonschema:"required"`
//	}
//
//	client := NewClient(provider,
//	    WithMemory(memory),
//	    WithDefaultOutputSchema(jsonschema.GenerateJSONSchema[MyResponse]()),
//	)
//
// Note: For most use cases, consider using StructuredClient[T] instead,
// which provides type-safe parsing in addition to schema application.
func WithDefaultOutputSchema(schema *jsonschema.Schema) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.DefaultOutputSchema = schema
	}
}

// NewClient creates a new immutable Client instance.
// The llmProvider is required as the first argument.
// All other configuration is provided via functional options.
//
// Example:
//
//	client, err := NewClient(
//	    openaiProvider,
//	    WithDefaultModel("gpt-4"),
//	    WithObserver(myObserver),
//	    WithSystemPrompt("You are a helpful assistant"),
//	    WithTools(tool1, tool2),
//	    WithEnrichSystemPromptWithToolsDescriptions(), // Optionally add tool guidance
//	)
func NewClient(llmProvider ai.Provider, opts ...func(*ClientOptions)) (*Client, error) {
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

	options.Tools = append(options.Tools, options.RequiredTools...)
	// Build tool catalog and descriptions
	toolCatalog := tool.NewCatalogWithTools(options.Tools...)
	toolDescriptions := make([]ai.ToolDescription, 0, len(options.Tools))
	requiredTools := make([]ai.ToolDescription, 0, len(options.RequiredTools))

	for _, t := range options.Tools {
		toolDescriptions = append(toolDescriptions, t.ToolInfo())
	}

	for _, t := range options.RequiredTools {
		requiredTools = append(requiredTools, t.ToolInfo())
	}

	// Enrich system prompt if enabled
	systemPrompt := options.SystemPrompt
	if options.EnrichSystemPromptWithToolDescr && len(toolDescriptions) > 0 {
		systemPrompt = enrichSystemPromptWithTools(options.SystemPrompt, toolDescriptions)
	}

	return &Client{
		systemPrompt:        systemPrompt,
		defaultModel:        options.DefaultModel,
		defaultOutputSchema: options.DefaultOutputSchema,
		llmProvider:         options.LlmProvider,
		memoryProvider:      options.MemoryProvider,
		observer:            options.Observer,
		toolCatalog:         toolCatalog,
		toolDescriptions:    toolDescriptions,
		requiredTools:       requiredTools,
		state:               map[string]any{},
	}, nil
}

// Memory returns the memory provider configured for this client.
// Returns nil if the client is in stateless mode (no memory configured).
func (c *Client) Memory() memory.Provider {
	return c.memoryProvider
}

// ToolCatalog returns the tool catalog for this client.
// The returned map is a copy to prevent external modifications.
// Keys are tool names (as registered), values are tool instances.
func (c *Client) ToolCatalog() *tool.Catalog {
	// Return a clone to maintain immutability
	return c.toolCatalog.Clone()
}

// Observer returns the observability provider configured for this client.
// Returns nil if no observer is configured (zero overhead mode).
func (c *Client) Observer() observability.Provider {
	return c.observer
}

// AppendToSystemPrompt appends additional text to the existing system prompt.
// This can be used to dynamically modify the system prompt after client creation.
func (c *Client) AppendToSystemPrompt(appendix string) {
	c.systemPrompt += "\n" + appendix
}

// SetDefaultOutputSchema sets the default JSON schema for structured output.
// This schema will be applied to all requests unless overridden per-request
// using WithOutputSchema in SendMessage or ContinueConversation.
func (c *Client) SetDefaultOutputSchema(schema *jsonschema.Schema) {
	c.defaultOutputSchema = schema
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

// SendMessageOptions contains optional parameters for SendMessage.
type SendMessageOptions struct {
	OutputSchema *jsonschema.Schema // Optional: JSON schema for structured output
}

// SendMessageOption is a functional option for SendMessage.
type SendMessageOption func(*SendMessageOptions)

// WithOutputSchema sets a JSON schema for structured output for this specific request.
// The schema guides the LLM to produce responses matching the specified structure.
//
// Example usage:
//
//	type MyResponse struct {
//	    Answer string `json:"answer"`
//	    Confidence float64 `json:"confidence"`
//	}
//
//	resp, _ := client.SendMessage(ctx, "What is 2+2?",
//	    client.WithOutputSchema(jsonschema.GenerateJSONSchema[MyResponse]()),
//	)
//	parsed, _ := utils.ParseStringAs[ProductReview](resp.Content)
func WithOutputSchema(schema *jsonschema.Schema) SendMessageOption {
	return func(o *SendMessageOptions) {
		o.OutputSchema = schema
	}
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
func (c *Client) SendMessage(ctx context.Context, prompt string, opts ...SendMessageOption) (*ai.ChatResponse, error) {
	// Validate prompt is non-empty
	if prompt == "" {
		return nil, errors.New("prompt cannot be empty; use ContinueConversation() to continue without adding a user message")
	}

	// Apply options
	options := &SendMessageOptions{}
	for _, opt := range opts {
		opt(options)
	}
	// Start tracing span (only if observer is set)
	span := c.observeInit(&ctx, "sending message")
	overview := patterns.OverviewFromContext(&ctx)
	timer := utils.NewTimer()

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

	// Add response format if output schema is provided
	// Priority: per-request schema > default schema
	schema := options.OutputSchema
	if schema == nil {
		schema = c.defaultOutputSchema
	}

	if schema != nil {
		request.ResponseFormat = &ai.ResponseFormat{
			Type:         "json_schema",
			OutputSchema: schema,
		}
		// TODO: consider do add hints to the LLM into the system prompt about the expected structure
	}

	// Send to LLM provider
	response, err := c.llmProvider.SendMessage(ctx, request)

	timer.Stop()

	if err != nil {
		c.observeError(&ctx, &span, err, timer, "sending message")

		return nil, err
	}

	overview.AddRequest(&request)
	overview.AddResponse(response)
	overview.IncludeUsage(response.Usage)

	c.observeSuccess(&ctx, &span, response, timer, "sending message")

	return response, nil
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
func (c *Client) ContinueConversation(ctx context.Context, opts ...SendMessageOption) (*ai.ChatResponse, error) {
	// Validate that memory provider is configured
	if c.memoryProvider == nil {
		return nil, errors.New("ContinueConversation requires a memory provider; create client with WithMemory() option")
	}

	// Apply options
	options := &SendMessageOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Start tracing span (only if observer is set)
	span := c.observeInit(&ctx, "continue conversation")
	overview := patterns.OverviewFromContext(&ctx)
	timer := utils.NewTimer()
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

	// Add response format if output schema is provided for this request
	// Add response format if output schema is provided
	// Priority: per-request schema > default schema
	schema := options.OutputSchema
	if schema == nil {
		schema = c.defaultOutputSchema
	}

	if schema != nil {
		request.ResponseFormat = &ai.ResponseFormat{
			Type:         "json_schema",
			OutputSchema: schema,
		}
	}

	// Send to LLM provider
	response, err := c.llmProvider.SendMessage(ctx, request)

	timer.Stop()

	if err != nil {
		c.observeError(&ctx, &span, err, timer, "continue conversation")

		return nil, err
	}

	overview.AddRequest(&request)
	overview.AddResponse(response)
	if response.Usage != nil {
		overview.IncludeUsage(response.Usage)
	}

	c.observeSuccess(&ctx, &span, response, timer, "continue conversation")

	return response, nil
}
func (c *Client) observeInit(ctx *context.Context, subject string) observability.Span {
	if c.observer == nil {
		return nil
	}

	var span observability.Span

	*ctx, span = c.observer.StartSpan(*ctx, observability.SpanClientSendMessage,
		observability.String(observability.AttrLLMModel, c.defaultModel),
		observability.Bool(observability.AttrClientContinuingConversation, true),
	)

	// Put span and observer in context for downstream propagation
	*ctx = observability.ContextWithSpan(*ctx, span)
	*ctx = observability.ContextWithObserver(*ctx, c.observer)

	// DEBUG: Operation starting
	c.observer.Debug(*ctx, subject,
		observability.String(observability.AttrLLMModel, c.defaultModel),
		observability.Int(observability.AttrClientToolsCount, len(c.toolDescriptions)),
	)

	return span
}

func (c *Client) observeError(ctx *context.Context, span *observability.Span, err error, timer *utils.Timer, subject string) {
	if c.observer == nil {
		return
	}

	(*span).RecordError(err)
	(*span).SetStatus(observability.StatusError, "Failed to "+subject)

	// ERROR: Operation failed
	c.observer.Error(*ctx, "Failed to "+subject+" with LLM",
		observability.Error(err),
		observability.Duration(observability.AttrDuration, timer.GetDuration()),
		observability.String(observability.AttrLLMModel, c.defaultModel),
	)

	// Metrics at DEBUG level
	c.observer.Counter(observability.MetricClientRequestCount).Add(*ctx, 1,
		observability.String(observability.AttrStatus, "error"),
		observability.String(observability.AttrLLMModel, c.defaultModel),
	)

}

func (c *Client) observeSuccess(ctx *context.Context, span *observability.Span, response *ai.ChatResponse, timer *utils.Timer, subject string) {
	if c.observer == nil {
		return
	}

	// Record metrics
	c.observer.Histogram(observability.MetricClientRequestDuration).Record(*ctx, timer.GetDuration().Seconds(),
		observability.String(observability.AttrLLMModel, c.defaultModel),
	)

	c.observer.Counter(observability.MetricClientRequestCount).Add(*ctx, 1,
		observability.String(observability.AttrStatus, "success"),
		observability.String(observability.AttrLLMModel, c.defaultModel),
	)

	// Prepare compact log attributes
	logAttrs := []observability.Attribute{
		observability.String(observability.AttrLLMModel, c.defaultModel),
		observability.String(observability.AttrLLMFinishReason, response.FinishReason),
		observability.Duration(observability.AttrDuration, timer.GetDuration()),
		observability.Int(observability.AttrClientToolCalls, len(response.ToolCalls)),
	}

	// Add token usage if available
	if response.Usage != nil {
		c.observer.Counter(observability.MetricClientTokensTotal).Add(*ctx, int64(response.Usage.TotalTokens),
			observability.String(observability.AttrLLMModel, c.defaultModel),
		)
		c.observer.Counter(observability.MetricClientTokensPrompt).Add(*ctx, int64(response.Usage.PromptTokens),
			observability.String(observability.AttrLLMModel, c.defaultModel),
		)
		c.observer.Counter(observability.MetricClientTokensCompletion).Add(*ctx, int64(response.Usage.CompletionTokens),
			observability.String(observability.AttrLLMModel, c.defaultModel),
		)

		(*span).SetAttributes(
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
			observability.String("response", utils.TruncateString(response.Content, 100)),
		)
	}

	// Single INFO log with all information
	c.observer.Info(*ctx, subject+": LLM call completed ", logAttrs...)

	(*span).SetStatus(observability.StatusOK, subject+" success")
	(*span).End()
}
