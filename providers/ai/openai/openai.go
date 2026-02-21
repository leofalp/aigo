package openai

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/observability"
)

const (
	defaultBaseURL          = "https://api.openai.com/v1"
	responsesEndpoint       = "/responses"
	chatCompletionsEndpoint = "/chat/completions"
)

// OpenAIProvider implements [ai.Provider] and [ai.StreamProvider] for OpenAI-compatible APIs.
// It targets real OpenAI, Azure OpenAI, Ollama, OpenRouter, and any other host that
// exposes an OpenAI-compatible REST interface. Capabilities are detected automatically
// from the base URL; use [OpenAIProvider.WithCapabilities] to override them manually.
type OpenAIProvider struct {
	apiKey       string
	baseURL      string
	client       *http.Client
	capabilities Capabilities
}

// New returns an [OpenAIProvider] initialized from environment variables.
// It reads OPENAI_API_KEY for authentication and OPENAI_API_BASE_URL for the
// endpoint base (defaulting to https://api.openai.com/v1 when unset). Provider
// capabilities are derived automatically by inspecting the base URL.
// Use [OpenAIProvider.WithAPIKey] and [OpenAIProvider.WithBaseURL] to override
// these values after construction.
func New() *OpenAIProvider {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_API_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &OpenAIProvider{
		apiKey:       apiKey,
		baseURL:      baseURL,
		client:       &http.Client{},
		capabilities: detectCapabilities(baseURL),
	}
}

// WithAPIKey sets the bearer token used for API authentication and returns the
// provider so calls can be chained. It overrides the value read from OPENAI_API_KEY.
func (p *OpenAIProvider) WithAPIKey(apiKey string) ai.Provider {
	p.apiKey = apiKey
	return p
}

// WithBaseURL overrides the API base URL and re-runs capability detection against
// the new host. It returns the provider so calls can be chained.
func (p *OpenAIProvider) WithBaseURL(baseURL string) ai.Provider {
	p.baseURL = baseURL
	p.capabilities = detectCapabilities(baseURL)
	return p
}

// WithHttpClient replaces the default [http.Client] used for API calls and returns
// the provider so calls can be chained. Useful for injecting custom timeouts,
// transport layers, or test doubles.
func (p *OpenAIProvider) WithHttpClient(httpClient *http.Client) ai.Provider {
	p.client = httpClient
	return p
}

// WithCapabilities replaces the auto-detected [Capabilities] with a caller-supplied
// value. This is useful when connecting to a provider whose base URL is not
// recognized by the built-in heuristic, or when testing specific feature flags.
// It returns *OpenAIProvider (not ai.Provider) to keep the Capabilities type
// accessible without an interface cast.
func (p *OpenAIProvider) WithCapabilities(capabilities Capabilities) *OpenAIProvider {
	p.capabilities = capabilities
	return p
}

// GetCapabilities returns the [Capabilities] currently in effect for this provider,
// whether detected automatically or set via [OpenAIProvider.WithCapabilities].
func (p *OpenAIProvider) GetCapabilities() Capabilities {
	return p.capabilities
}

// SendMessage implements [ai.Provider] by sending a synchronous chat request and
// returning the full response. It automatically routes to
// [OpenAIProvider.SendMessageViaResponses] when the provider supports the
// /v1/responses endpoint, and falls back to
// [OpenAIProvider.SendMessageViaChatCompletions] otherwise.
// Returns an error if the API key is unset, the HTTP request fails, or the
// response contains no usable output.
func (p *OpenAIProvider) SendMessage(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
	// Enrich span if present in context
	span := observability.SpanFromContext(ctx)
	observer := observability.ObserverFromContext(ctx)

	if span != nil {
		span.AddEvent(observability.EventLLMRequestStart)
		span.SetAttributes(
			observability.String(observability.AttrLLMProvider, "openai"),
			observability.String(observability.AttrLLMEndpoint, p.baseURL),
			observability.String(observability.AttrLLMModel, request.Model),
		)
		defer span.AddEvent(observability.EventLLMRequestEnd)
	}

	// TRACE: Log provider-level details
	if observer != nil {
		observer.Trace(ctx, "OpenAI provider preparing request",
			observability.String(observability.AttrLLMProvider, "openai"),
			observability.String(observability.AttrLLMEndpoint, p.baseURL),
			observability.String(observability.AttrLLMModel, request.Model),
			observability.Int(observability.AttrRequestMessagesCount, len(request.Messages)),
			observability.Int(observability.AttrRequestToolsCount, len(request.Tools)),
		)
	}

	// Check API key
	if p.apiKey == "" {
		return nil, fmt.Errorf("API key is not set")
	}

	// Decide which endpoint to use
	if p.capabilities.SupportsResponses {
		return p.SendMessageViaResponses(ctx, request)
	}
	return p.SendMessageViaChatCompletions(ctx, request)
}

// SendMessageViaResponses sends a chat request to the OpenAI /v1/responses endpoint
// and returns the normalised response. This endpoint is only available on the real
// OpenAI API (not Azure or third-party compatible hosts). Prefer
// [OpenAIProvider.SendMessage], which selects the endpoint automatically.
// Returns an error if the HTTP call fails or the response body contains no output items.
func (p *OpenAIProvider) SendMessageViaResponses(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
	span := observability.SpanFromContext(ctx)
	observer := observability.ObserverFromContext(ctx)

	if span != nil {
		span.SetAttributes(
			observability.String(observability.AttrLLMEndpointType, "responses"),
		)
	}

	if observer != nil {
		observer.Trace(ctx, "Using /v1/responses endpoint",
			observability.String(observability.AttrLLMEndpoint, p.baseURL+responsesEndpoint),
		)
	}

	req := requestToResponses(request)
	httpResponse, resp, err := utils.DoPostSync[responseCreateResponse](ctx, p.client, p.baseURL+responsesEndpoint, p.apiKey, req)
	if err != nil {
		if observer != nil {
			observer.Trace(ctx, "HTTP request failed",
				observability.Error(err),
			)
		}
		return nil, err
	}

	if resp == nil {
		return nil, fmt.Errorf("empty response from OpenAI Responses API: %s", httpResponse.Status)
	}

	if len(resp.Output) == 0 {
		return nil, fmt.Errorf("no output items in response")
	}

	result := responsesToGeneric(*resp)

	// Enrich span with response details
	if span != nil && result != nil {
		span.SetAttributes(
			observability.String(observability.AttrLLMResponseID, result.Id),
			observability.String(observability.AttrLLMFinishReason, result.FinishReason),
		)
		if result.Usage != nil {
			span.AddEvent(observability.EventTokensReceived,
				observability.Int(observability.AttrLLMTokensTotal, result.Usage.TotalTokens),
			)
		}
	}

	return result, nil
}

// SendMessageViaChatCompletions sends a chat request to the /v1/chat/completions
// endpoint and returns the normalised response. This endpoint is supported by
// virtually all OpenAI-compatible providers. The legacy functions wire format is
// used automatically when [Capabilities.ToolCallMode] is [ToolCallModeFunctions].
// Returns an error if the HTTP call fails or the response contains no choices.
func (p *OpenAIProvider) SendMessageViaChatCompletions(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
	span := observability.SpanFromContext(ctx)
	observer := observability.ObserverFromContext(ctx)

	if span != nil {
		span.SetAttributes(
			observability.String(observability.AttrLLMEndpointType, "chat_completions"),
		)
	}

	// Determine if we should use legacy functions format
	useLegacyFunctions := (p.capabilities.ToolCallMode == ToolCallModeFunctions)

	if observer != nil {
		observer.Trace(ctx, "Using /v1/chat/completions endpoint",
			observability.String(observability.AttrLLMEndpoint, p.baseURL+chatCompletionsEndpoint),
			observability.Bool(observability.AttrUseLegacyFunctions, useLegacyFunctions),
		)
	}

	req := requestToChatCompletion(request, useLegacyFunctions)
	httpResponse, resp, err := utils.DoPostSync[chatCompletionResponse](ctx, p.client, p.baseURL+chatCompletionsEndpoint, p.apiKey, req)
	if err != nil {
		if observer != nil {
			observer.Trace(ctx, "HTTP request failed", observability.Error(err))
		}
		return nil, err
	}

	if resp == nil {
		return nil, fmt.Errorf("empty response from Chat Completions API: %s", httpResponse.Status)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	result := chatCompletionToGeneric(*resp)

	// Enrich span with response details
	if span != nil && result != nil {
		span.SetAttributes(
			observability.String(observability.AttrLLMResponseID, result.Id),
			observability.String(observability.AttrLLMFinishReason, result.FinishReason),
			observability.Int(observability.AttrHTTPStatusCode, httpResponse.StatusCode),
		)
		if result.Usage != nil {
			span.AddEvent(observability.EventTokensReceived,
				observability.Int(observability.AttrLLMTokensTotal, result.Usage.TotalTokens),
			)
		}
	}

	return result, nil
}

// IsStopMessage reports whether message represents a terminal response that requires
// no further action. A nil message, a response whose finish reason is "stop",
// "length", or "content_filter", or a response with no content and no media output
// are all treated as stop signals. Responses that contain tool calls are never
// considered stops, even when finish_reason is "stop", because some providers
// (e.g. certain OpenRouter models) set that field incorrectly.
func (p *OpenAIProvider) IsStopMessage(message *ai.ChatResponse) bool {
	if message == nil {
		return true
	}

	// IMPORTANT: Tool calls take priority over finish_reason
	// Some providers (e.g., OpenRouter with certain models) return finish_reason="stop"
	// even when they want to call tools, so we must check tool calls first
	if len(message.ToolCalls) > 0 {
		return false // Not stopped - tools need to be executed
	}

	// Check explicit finish reasons that indicate completion
	if message.FinishReason == "stop" || message.FinishReason == "length" || message.FinishReason == "content_filter" {
		return true
	}

	// If there's no content, no media outputs, and no tool calls, treat as stop
	if message.Content == "" && len(message.Images) == 0 && len(message.Audio) == 0 && len(message.Videos) == 0 {
		return true
	}

	return false
}
