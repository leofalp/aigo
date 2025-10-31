package ai

import (
	"context"
	"net/http"
)

// Provider is the generic interface that all LLM providers must implement
type Provider interface {
	// SendSingleMessage sends a chat request and returns the response
	SendMessage(ctx context.Context, request ChatRequest) (*ChatResponse, error)

	IsStopMessage(message *ChatResponse) bool

	// WithAPIKey sets the API key used for authenticating requests.
	WithAPIKey(apiKey string) Provider

	// WithBaseURL overrides the default base URL for API requests.
	WithBaseURL(baseURL string) Provider

	// WithHttpClient sets the HTTP client used for outbound requests.
	WithHttpClient(httpClient *http.Client) Provider
}
