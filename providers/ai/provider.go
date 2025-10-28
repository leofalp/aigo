package ai

import (
	"context"
	"net/http"
)

// Provider is the generic interface that all LLM providers must implement
type Provider interface {
	// SendSingleMessage sends a chat request and returns the response
	SendSingleMessage(ctx context.Context, request ChatRequest) (*ChatResponse, error)

	IsStopMessage(message *ChatResponse) bool

	// GetModelName returns the name of the model being used
	GetModelName() string

	// WithAPIKey sets the API key used for authenticating requests.
	WithAPIKey(apiKey string) Provider

	// WithModel sets the model identifier to use for chat requests.
	WithModel(model string) Provider

	// WithBaseURL overrides the default base URL for API requests.
	WithBaseURL(baseURL string) Provider

	// WithSystemPrompt sets the default system prompt for conversations.
	WithSystemPrompt(prompt string) Provider

	// WithHttpClient sets the HTTP client used for outbound requests.
	WithHttpClient(httpClient *http.Client) Provider
}
