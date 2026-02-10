package gemini

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
	defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	defaultModel   = "gemini-2.0-flash-lite" // Most cost-effective model
)

// GeminiProvider implements the ai.Provider interface for Google's Gemini API.
type GeminiProvider struct {
	apiKey       string
	baseURL      string
	client       *http.Client
	capabilities Capabilities
}

// New creates a new Gemini provider instance with default values from environment.
// Environment variables:
//   - GEMINI_API_KEY: API key for authentication
//   - GEMINI_API_BASE_URL: Base URL for API (optional, defaults to Google's API)
func New() *GeminiProvider {
	apiKey := os.Getenv("GEMINI_API_KEY")
	baseURL := os.Getenv("GEMINI_API_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &GeminiProvider{
		apiKey:       apiKey,
		baseURL:      baseURL,
		client:       &http.Client{},
		capabilities: detectCapabilities(),
	}
}

// WithAPIKey sets the API key for the provider.
func (p *GeminiProvider) WithAPIKey(apiKey string) ai.Provider {
	p.apiKey = apiKey
	return p
}

// WithBaseURL sets the base URL for the API.
func (p *GeminiProvider) WithBaseURL(baseURL string) ai.Provider {
	p.baseURL = baseURL
	return p
}

// WithHttpClient sets a custom HTTP client.
func (p *GeminiProvider) WithHttpClient(httpClient *http.Client) ai.Provider {
	p.client = httpClient
	return p
}

// GetCapabilities returns the current capabilities (informational only).
func (p *GeminiProvider) GetCapabilities() Capabilities {
	return p.capabilities
}

// SendMessage implements the ai.Provider interface.
// It sends a chat request to the Gemini API and returns the response.
func (p *GeminiProvider) SendMessage(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
	// Get observability context
	span := observability.SpanFromContext(ctx)
	observer := observability.ObserverFromContext(ctx)

	// Determine model
	model := request.Model
	if model == "" {
		model = defaultModel
	}

	if span != nil {
		span.AddEvent(observability.EventLLMRequestStart)
		span.SetAttributes(
			observability.String(observability.AttrLLMProvider, "gemini"),
			observability.String(observability.AttrLLMEndpoint, p.baseURL),
			observability.String(observability.AttrLLMModel, model),
		)
		defer span.AddEvent(observability.EventLLMRequestEnd)
	}

	if observer != nil {
		observer.Trace(ctx, "Gemini provider preparing request",
			observability.String(observability.AttrLLMProvider, "gemini"),
			observability.String(observability.AttrLLMEndpoint, p.baseURL),
			observability.String(observability.AttrLLMModel, model),
			observability.Int(observability.AttrRequestMessagesCount, len(request.Messages)),
			observability.Int(observability.AttrRequestToolsCount, len(request.Tools)),
		)
	}

	// Validate API key
	if p.apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not set")
	}

	// Build request URL
	url := fmt.Sprintf("%s/models/%s:generateContent", p.baseURL, model)

	// Convert request to Gemini format
	geminiReq := requestToGemini(request)

	// Send request with Gemini-specific authentication header
	httpResponse, resp, err := utils.DoPostSync[generateContentResponse](
		ctx,
		p.client,
		url,
		"", // Empty apiKey for DoPostSync's default Bearer auth
		geminiReq,
		utils.HeaderOption{Key: "x-goog-api-key", Value: p.apiKey},
	)
	if err != nil {
		if observer != nil {
			observer.Trace(ctx, "HTTP request failed", observability.Error(err))
		}
		return nil, err
	}

	if resp == nil {
		return nil, fmt.Errorf("empty response from Gemini API: %s", httpResponse.Status)
	}

	// Convert response to generic format
	result := geminiToGeneric(*resp)
	result.Model = model // Ensure model is set even if not in response

	// Enrich span with response details
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

// IsStopMessage reports whether the given chat response should be treated as a stop/end signal.
func (p *GeminiProvider) IsStopMessage(message *ai.ChatResponse) bool {
	if message == nil {
		return true
	}

	// Tool calls take priority - if tools need execution, we're not done
	if len(message.ToolCalls) > 0 {
		return false
	}

	// Check explicit finish reasons that indicate completion
	switch message.FinishReason {
	case "stop", "length", "content_filter":
		return true
	}

	// If there's no content, no images, and no tool calls, treat as stop
	if message.Content == "" && len(message.Images) == 0 {
		return true
	}

	return false
}
