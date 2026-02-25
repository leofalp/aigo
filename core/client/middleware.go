package client

import (
	"context"

	"github.com/leofalp/aigo/providers/ai"
)

// SendFunc is a function that sends a chat request to the LLM provider and returns
// the completed response. It is the base unit threaded through the send middleware chain.
type SendFunc func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error)

// StreamFunc is a function that sends a chat request to the LLM provider and returns
// a ChatStream for real-time token delivery. It is the base unit threaded through the
// stream middleware chain.
type StreamFunc func(ctx context.Context, request ai.ChatRequest) (*ai.ChatStream, error)

// Middleware intercepts and optionally transforms LLM send requests and responses.
// Each Middleware receives the next SendFunc in the chain and returns a new SendFunc
// that wraps it. Middlewares are applied outermost-first: the first middleware in
// the slice is the outermost wrapper.
type Middleware func(next SendFunc) SendFunc

// StreamMiddleware is the streaming counterpart of Middleware. It intercepts
// stream requests and may wrap the returned ChatStream to observe or transform
// the event sequence. If nil in a MiddlewareConfig, streaming calls skip this
// particular middleware in the chain.
type StreamMiddleware func(next StreamFunc) StreamFunc

// MiddlewareConfig pairs a send middleware with its optional streaming counterpart.
// The Send field is required; a nil Send causes [New] to return a descriptive
// error. The Stream field is optional: a nil value means streaming calls bypass
// this middleware entry entirely (the stream chain falls through to the next entry).
type MiddlewareConfig struct {
	// Send is the middleware applied to SendMessage and ContinueConversation calls.
	// Required — a nil Send causes New to return an error.
	Send Middleware

	// Stream is the optional middleware applied to StreamMessage and
	// StreamContinueConversation calls. A nil value means streaming bypasses
	// this middleware.
	Stream StreamMiddleware
}

// buildSendChain constructs the linear send middleware chain from the slice of
// MiddlewareConfig values. The base function calls the provider directly. Middlewares
// are applied in reverse order so that the first entry in the slice becomes the
// outermost wrapper, i.e. the first to execute on an incoming request.
func buildSendChain(provider ai.Provider, middlewares []MiddlewareConfig) SendFunc {
	// Base function: direct provider call.
	var chain SendFunc = func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
		return provider.SendMessage(ctx, request)
	}

	// Apply middlewares in reverse so that middlewares[0] is outermost.
	for i := len(middlewares) - 1; i >= 0; i-- {
		chain = middlewares[i].Send(chain)
	}

	return chain
}

// buildStreamChain constructs the linear stream middleware chain from the slice of
// MiddlewareConfig values. The base function attempts a native stream via
// ai.StreamProvider; if the provider does not implement that interface it falls
// back to a synchronous SendMessage wrapped in a single-event stream. Middlewares
// with a nil Stream field are skipped; only those with a non-nil Stream are applied.
func buildStreamChain(provider ai.Provider, middlewares []MiddlewareConfig) StreamFunc {
	// Base function: native streaming with sync fallback.
	var chain StreamFunc = func(ctx context.Context, request ai.ChatRequest) (*ai.ChatStream, error) {
		if streamProvider, ok := provider.(ai.StreamProvider); ok {
			return streamProvider.StreamMessage(ctx, request)
		}

		response, err := provider.SendMessage(ctx, request)
		if err != nil {
			return nil, err
		}

		return ai.NewSingleEventStream(response), nil
	}

	// Apply only non-nil stream middlewares in reverse so that the first entry
	// with a non-nil Stream field is the outermost wrapper.
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i].Stream != nil {
			chain = middlewares[i].Stream(chain)
		}
	}

	return chain
}
