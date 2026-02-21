package ai

import (
	"context"
	"net/http"
)

// StreamProvider is an optional interface that providers can implement to support
// streaming (SSE-based) responses. Callers detect streaming support via type
// assertion: provider.(StreamProvider). If the provider does not implement this
// interface, callers should fall back to the synchronous SendMessage method.
type StreamProvider interface {
	Provider
	// StreamMessage sends a chat request and returns a ChatStream that yields
	// incremental deltas as they arrive from the API. Pre-stream errors
	// (auth, bad request, network) are returned as a normal error. Mid-stream
	// errors are yielded through the iterator.
	StreamMessage(ctx context.Context, request ChatRequest) (*ChatStream, error)
}

// Provider is the core interface that every LLM provider implementation must
// satisfy. It covers the full lifecycle of a single request: authentication,
// endpoint configuration, message dispatch, and response interpretation.
// Use [StreamProvider] in addition when the provider supports streaming.
type Provider interface {
	// SendMessage sends a chat request to the provider and returns the
	// completed response. Returns an error if the provider call fails,
	// the context is cancelled, or the response cannot be decoded.
	SendMessage(ctx context.Context, request ChatRequest) (*ChatResponse, error)

	// IsStopMessage reports whether the response represents a terminal
	// completion (i.e. the model has nothing more to say and no further
	// tool calls are expected). Providers use their own finish-reason
	// semantics to implement this check.
	IsStopMessage(message *ChatResponse) bool

	// WithAPIKey sets the API key used for authenticating requests.
	WithAPIKey(apiKey string) Provider

	// WithBaseURL overrides the default base URL for API requests.
	WithBaseURL(baseURL string) Provider

	// WithHttpClient sets the HTTP client used for outbound requests.
	WithHttpClient(httpClient *http.Client) Provider
}
