package middleware

import (
	"context"
	"time"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/ai"
)

// NewTimeoutMiddleware creates a MiddlewareConfig that enforces a per-request
// deadline on both synchronous and streaming provider calls.
//
// For send requests (SendMessage / ContinueConversation) the implementation
// wraps the context with context.WithTimeout and defers cancel() — the context
// is automatically canceled once the provider returns or the deadline expires.
//
// For streaming requests (StreamMessage / StreamContinueConversation) the
// timeout wraps the context before calling next, but the cancel function is NOT
// deferred immediately. Instead it is called once the stream is fully consumed
// (StreamEventDone), a mid-stream error occurs, or the iterator is abandoned.
// This ensures the timeout governs the complete lifetime of the stream, not
// just the time to receive the first byte.
//
// If the caller supplies a context that already has a shorter deadline, that
// shorter deadline wins as per normal context semantics.
func NewTimeoutMiddleware(timeout time.Duration) client.MiddlewareConfig {
	return client.MiddlewareConfig{
		Send:   buildSendTimeout(timeout),
		Stream: buildStreamTimeout(timeout),
	}
}

// buildSendTimeout constructs the send middleware that adds a deadline.
func buildSendTimeout(timeout time.Duration) client.Middleware {
	return func(next client.SendFunc) client.SendFunc {
		return func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			return next(ctx, request)
		}
	}
}

// buildStreamTimeout constructs the stream middleware that adds a deadline and
// wraps the resulting ChatStream so the cancel function is called at the
// appropriate moment.
func buildStreamTimeout(timeout time.Duration) client.StreamMiddleware {
	return func(next client.StreamFunc) client.StreamFunc {
		return func(ctx context.Context, request ai.ChatRequest) (*ai.ChatStream, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)

			stream, err := next(ctx, request)
			if err != nil {
				// Pre-stream error — cancel immediately.
				cancel()
				return nil, err
			}

			// Wrap the iterator so cancel is called when the stream ends.
			wrapped := wrapStreamWithCancel(stream, cancel)
			return wrapped, nil
		}
	}
}

// wrapStreamWithCancel returns a new ChatStream whose iterator calls cancel once
// the stream finishes (done event), errors, or the caller breaks out of the loop.
func wrapStreamWithCancel(stream *ai.ChatStream, cancel context.CancelFunc) *ai.ChatStream {
	iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
		defer cancel()

		for event, err := range stream.Iter() {
			if !yield(event, err) {
				// The caller broke out of the range loop early.
				return
			}

			if err != nil {
				return
			}

			if event.Type == ai.StreamEventDone {
				return
			}
		}
	}

	return ai.NewChatStream(iteratorFunc)
}
