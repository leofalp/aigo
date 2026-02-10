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

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs
type OpenAIProvider struct {
	apiKey       string
	baseURL      string
	client       *http.Client
	capabilities Capabilities
}

// New creates a new OpenAI provider instance with default values
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

// WithAPIKey sets the API key for the provider
func (p *OpenAIProvider) WithAPIKey(apiKey string) ai.Provider {
	p.apiKey = apiKey
	return p
}

// WithBaseURL sets the base URL for the API and re-detects capabilities
func (p *OpenAIProvider) WithBaseURL(baseURL string) ai.Provider {
	p.baseURL = baseURL
	p.capabilities = detectCapabilities(baseURL)
	return p
}

// WithHttpClient sets a custom HTTP client
func (p *OpenAIProvider) WithHttpClient(httpClient *http.Client) ai.Provider {
	p.client = httpClient
	return p
}

// WithCapabilities manually overrides detected capabilities (for advanced users)
func (p *OpenAIProvider) WithCapabilities(capabilities Capabilities) *OpenAIProvider {
	p.capabilities = capabilities
	return p
}

// GetCapabilities returns the current capabilities
func (p *OpenAIProvider) GetCapabilities() Capabilities {
	return p.capabilities
}

// SendMessage implements the Provider interface
// It automatically chooses the best endpoint based on capabilities
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

// SendMessageViaResponses uses the /v1/responses endpoint (OpenAI only)
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

// SendMessageViaChatCompletions uses the /v1/chat/completions endpoint (universal)
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

// IsStopMessage reports whether the given chat response should be treated as a stop/end signal.
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

	// If there's no content, no images, and no tool calls, treat as stop
	if message.Content == "" && len(message.Images) == 0 {
		return true
	}

	return false
}
