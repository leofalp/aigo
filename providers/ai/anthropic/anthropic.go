package anthropic

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
	// defaultBaseURL is the canonical base URL for Anthropic's Messages API.
	defaultBaseURL = "https://api.anthropic.com/v1"

	// messagesEndpoint is the path for the Messages API endpoint.
	messagesEndpoint = "/messages"

	// anthropicVersion is the required anthropic-version header value.
	// Anthropic uses this to version-lock response formats independently of the URL.
	anthropicVersion = "2023-06-01"
)

// AnthropicProvider implements [ai.Provider] for Anthropic's Messages API.
// It supports extended thinking, prompt caching, vision, tool use, and PDF input
// through the [Capabilities] struct. Use [New] to construct a ready-to-use instance.
type AnthropicProvider struct {
	apiKey       string
	baseURL      string
	client       *http.Client
	capabilities Capabilities
}

// New returns an [AnthropicProvider] initialized from environment variables.
// It reads ANTHROPIC_API_KEY for authentication and ANTHROPIC_API_BASE_URL for
// the endpoint base (defaulting to https://api.anthropic.com/v1 when unset).
// Use [AnthropicProvider.WithAPIKey] and [AnthropicProvider.WithBaseURL] to
// override these values after construction.
func New() *AnthropicProvider {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	baseURL := os.Getenv("ANTHROPIC_API_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &AnthropicProvider{
		apiKey:       apiKey,
		baseURL:      baseURL,
		client:       &http.Client{},
		capabilities: Capabilities{},
	}
}

// WithAPIKey sets the API key used for authenticating requests and returns the
// provider so calls can be chained. It overrides the value read from ANTHROPIC_API_KEY.
func (p *AnthropicProvider) WithAPIKey(apiKey string) ai.Provider {
	p.apiKey = apiKey
	return p
}

// WithBaseURL overrides the API base URL and returns the provider so calls can
// be chained. Use this when targeting a proxy or local testing endpoint.
func (p *AnthropicProvider) WithBaseURL(baseURL string) ai.Provider {
	p.baseURL = baseURL
	return p
}

// WithHttpClient replaces the default [http.Client] used for API calls and
// returns the provider so calls can be chained. Useful for injecting custom
// timeouts, transport layers, or test doubles.
func (p *AnthropicProvider) WithHttpClient(httpClient *http.Client) ai.Provider {
	p.client = httpClient
	return p
}

// WithCapabilities replaces the current [Capabilities] with a caller-supplied
// value and returns *AnthropicProvider (not ai.Provider) so the Capabilities
// type remains accessible without an interface cast. This mirrors the OpenAI pattern.
func (p *AnthropicProvider) WithCapabilities(capabilities Capabilities) *AnthropicProvider {
	p.capabilities = capabilities
	return p
}

// GetCapabilities returns the [Capabilities] currently in effect for this provider.
func (p *AnthropicProvider) GetCapabilities() Capabilities {
	return p.capabilities
}

// buildHeaders constructs the HTTP headers required for every Anthropic request.
// x-api-key carries the credential (Anthropic does not use Bearer tokens),
// anthropic-version pins the wire format, and anthropic-beta is added only when
// beta features are configured so the header is absent for standard requests.
func (p *AnthropicProvider) buildHeaders() []utils.HeaderOption {
	headers := []utils.HeaderOption{
		{Key: "x-api-key", Value: p.apiKey},
		{Key: "anthropic-version", Value: anthropicVersion},
	}

	if betaValue := p.capabilities.betaHeaderValue(); betaValue != "" {
		headers = append(headers, utils.HeaderOption{Key: "anthropic-beta", Value: betaValue})
	}

	return headers
}

// SendMessage implements [ai.Provider] by sending a synchronous chat request to
// Anthropic's Messages API and returning the full response mapped to the generic
// [ai.ChatResponse] format. It returns an error if the API key is unset, the HTTP
// request fails, or the response body is empty.
func (p *AnthropicProvider) SendMessage(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
	// Enrich span if observability is wired into the context.
	span := observability.SpanFromContext(ctx)
	observer := observability.ObserverFromContext(ctx)

	// Anthropic has no SDK-level default model; the caller must supply one.
	model := request.Model

	if span != nil {
		span.AddEvent(observability.EventLLMRequestStart)
		span.SetAttributes(
			observability.String(observability.AttrLLMProvider, "anthropic"),
			observability.String(observability.AttrLLMEndpoint, p.baseURL),
			observability.String(observability.AttrLLMModel, model),
		)
		defer span.AddEvent(observability.EventLLMRequestEnd)
	}

	if observer != nil {
		observer.Trace(ctx, "Anthropic provider preparing request",
			observability.String(observability.AttrLLMProvider, "anthropic"),
			observability.String(observability.AttrLLMEndpoint, p.baseURL),
			observability.String(observability.AttrLLMModel, model),
			observability.Int(observability.AttrRequestMessagesCount, len(request.Messages)),
			observability.Int(observability.AttrRequestToolsCount, len(request.Tools)),
		)
	}

	// Guard against missing credentials before making a network call.
	if p.apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}

	url := p.baseURL + messagesEndpoint

	// Convert the generic request to the Anthropic Messages wire format.
	anthropicReq, err := requestToAnthropic(request, p.capabilities)
	if err != nil {
		return nil, fmt.Errorf("failed to build Anthropic request: %w", err)
	}

	// Pass empty apiKey so DoPostSync does not inject a Bearer token;
	// Anthropic authenticates via x-api-key instead.
	httpResponse, resp, err := utils.DoPostSync[anthropicResponse](
		ctx,
		p.client,
		url,
		"",
		anthropicReq,
		p.buildHeaders()...,
	)
	if err != nil {
		if observer != nil {
			observer.Trace(ctx, "HTTP request failed", observability.Error(err))
		}
		return nil, err
	}

	if resp == nil {
		return nil, fmt.Errorf("empty response from Anthropic API: %s", httpResponse.Status)
	}

	// Convert the Anthropic-specific response to the provider-agnostic format.
	result := anthropicToGeneric(*resp)

	// Anthropic often echoes the model name in the response, but when it is
	// absent (e.g., certain beta endpoints) we fall back to the request model
	// so callers always have a non-empty Model field.
	if result.Model == "" {
		result.Model = request.Model
	}

	// Enrich span with response details now that we have a decoded result.
	if span != nil {
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

// IsStopMessage reports whether message represents a terminal response that
// requires no further action. A nil message, a response whose FinishReason is
// "stop", "length", or "content_filter", or a response with no content and no
// media output are all treated as stop signals. Responses that contain tool calls
// are never considered stops even when FinishReason is "stop", because some
// Anthropic models set stop_reason to "end_turn" alongside tool_use blocks.
func (p *AnthropicProvider) IsStopMessage(message *ai.ChatResponse) bool {
	if message == nil {
		return true
	}

	// Tool calls take priority over finish_reason — tools need to be executed.
	if len(message.ToolCalls) > 0 {
		return false
	}

	// Check canonical finish reasons that indicate the model has completed.
	if message.FinishReason == "stop" || message.FinishReason == "length" || message.FinishReason == "content_filter" {
		return true
	}

	// If there is no content and no media outputs, treat as an implicit stop.
	if message.Content == "" && len(message.Images) == 0 && len(message.Audio) == 0 && len(message.Videos) == 0 {
		return true
	}

	return false
}
