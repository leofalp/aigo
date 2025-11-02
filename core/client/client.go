package client

import (
	"aigo/internal/jsonschema"
	"aigo/providers/ai"
	"aigo/providers/memory"
	"aigo/providers/observability"
	"aigo/providers/tool"
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"time"
)

type Client[T any] struct {
	systemPrompt   string
	defaultModel   string
	llmProvider    ai.Provider //TODO: Enforce requirement of llmProvider
	memoryProvider memory.Provider
	observer       observability.Provider // nil if not set (zero overhead)
	// for fast accessing tool by name
	toolCatalog map[string]tool.GenericTool
	// for passing tool info to LLM without processing all tools each time
	toolDescriptions []ai.ToolDescription
	outputSchema     *jsonschema.Schema
}

type funcClientOptions struct {
	DefaultModel string
	Observer     observability.Provider
}

func WithDefaultModel(defaultModel string) func(tool *funcClientOptions) {
	return func(tool *funcClientOptions) {
		tool.DefaultModel = defaultModel
	}
}

func WithObserver(observer observability.Provider) func(tool *funcClientOptions) {
	return func(tool *funcClientOptions) {
		tool.Observer = observer
	}
}

func NewClient[T any](llmProvider ai.Provider, options ...func(tool *funcClientOptions)) *Client[T] {
	toolOptions := &funcClientOptions{}
	for _, o := range options {
		o(toolOptions)
	}

	return &Client[T]{
		defaultModel: toolOptions.DefaultModel,
		llmProvider:  llmProvider,
		observer:     toolOptions.Observer, // can be nil (zero overhead)
		toolCatalog:  map[string]tool.GenericTool{},
		outputSchema: jsonschema.GenerateJSONSchema[T](),
	}
}

func (c *Client[T]) WithLlmProvider(llmProvider ai.Provider) *Client[T] {
	c.llmProvider = llmProvider
	return c
}

func (c *Client[T]) WithMemoryProvider(memoryProvider memory.Provider) *Client[T] {
	c.memoryProvider = memoryProvider
	return c
}

func (c *Client[T]) AddSystemPrompt(content string) *Client[T] {
	c.systemPrompt += content + "\n"
	return c
}

func (c *Client[T]) AddTools(tools []tool.GenericTool) *Client[T] {
	for i, t := range tools {
		c.toolCatalog[t.ToolInfo().Name] = tools[i]
		c.toolDescriptions = append(c.toolDescriptions, t.ToolInfo())
	}

	return c
}

func (c *Client[T]) SendMessage(prompt string) (*ai.ChatResponse, error) {
	ctx := context.Background()

	// Start tracing span (only if observer is set)
	var span observability.Span
	if c.observer != nil {
		ctx, span = c.observer.StartSpan(ctx, observability.SpanClientSendMessage,
			observability.String(observability.AttrLLMModel, c.defaultModel),
			observability.String("prompt", prompt),
		)
		defer span.End()

		// Put span in context for downstream propagation
		ctx = observability.ContextWithSpan(ctx, span)

		c.observer.Debug(ctx, "Sending message to LLM",
			observability.String(observability.AttrLLMModel, c.defaultModel),
			observability.Int("tools_count", len(c.toolDescriptions)),
		)
	}

	start := time.Now()

	c.memoryProvider.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: prompt})
	response, err := c.llmProvider.SendMessage(ctx, ai.ChatRequest{
		// TODO
	})

	duration := time.Since(start)

	if err != nil {
		if c.observer != nil {
			span.RecordError(err)
			span.SetStatus(observability.StatusError, "Failed to send message")

			c.observer.Error(ctx, "Failed to send message to LLM",
				observability.Error(err),
				observability.Duration("duration", duration),
			)

			c.observer.Counter("aigo.client.request.count").Add(ctx, 1,
				observability.String("status", "error"),
				observability.String("model", c.defaultModel),
			)
		}

		return nil, err
	}

	if c.observer != nil {
		// Record metrics
		c.observer.Histogram("aigo.client.request.duration").Record(ctx, duration.Seconds(),
			observability.String("model", c.defaultModel),
		)

		c.observer.Counter("aigo.client.request.count").Add(ctx, 1,
			observability.String("status", "success"),
			observability.String("model", c.defaultModel),
		)

		if response.Usage != nil {
			c.observer.Counter("aigo.client.tokens.total").Add(ctx, int64(response.Usage.TotalTokens),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)
			c.observer.Counter("aigo.client.tokens.prompt").Add(ctx, int64(response.Usage.PromptTokens),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)
			c.observer.Counter("aigo.client.tokens.completion").Add(ctx, int64(response.Usage.CompletionTokens),
				observability.String(observability.AttrLLMModel, c.defaultModel),
			)

			span.SetAttributes(
				observability.Int(observability.AttrLLMTokensTotal, response.Usage.TotalTokens),
				observability.Int(observability.AttrLLMTokensPrompt, response.Usage.PromptTokens),
				observability.Int(observability.AttrLLMTokensCompletion, response.Usage.CompletionTokens),
			)
		}

		c.observer.Info(ctx, "Message sent successfully",
			observability.String("finish_reason", response.FinishReason),
			observability.Duration("duration", duration),
			observability.Int("tool_calls", len(response.ToolCalls)),
		)

		span.SetStatus(observability.StatusOK, "Message sent successfully")
	}

	return c.responseParser(response)
}

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
		response.Content = "[Waring] Could not parse response: " + err.Error() + " --> providing raw response content as fallback.\n\n" + response.Content
	}

	return response, nil
}
