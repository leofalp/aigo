package middleware

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/leofalp/aigo/providers/ai"
)

// ========== Helpers ==========

// makeSendFunc returns a SendFunc that sleeps for the given duration before
// returning, simulating a slow provider.
func makeSendFunc(sleep time.Duration, resp *ai.ChatResponse, err error) func(context.Context, ai.ChatRequest) (*ai.ChatResponse, error) {
	return func(ctx context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		select {
		case <-time.After(sleep):
			return resp, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// makeStreamFunc returns a StreamFunc that sleeps for the given duration before
// yielding events.
func makeStreamFunc(sleep time.Duration) func(context.Context, ai.ChatRequest) (*ai.ChatStream, error) {
	return func(ctx context.Context, _ ai.ChatRequest) (*ai.ChatStream, error) {
		iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
			select {
			case <-time.After(sleep):
				yield(ai.StreamEvent{Type: ai.StreamEventContent, Content: "hello"}, nil)
				yield(ai.StreamEvent{Type: ai.StreamEventDone, FinishReason: "stop"}, nil)
			case <-ctx.Done():
				yield(ai.StreamEvent{}, ctx.Err())
			}
		}

		return ai.NewChatStream(iter.Seq2[ai.StreamEvent, error](iteratorFunc)), nil
	}
}

// ========== Send timeout tests ==========

// TestTimeoutMiddleware_SendCompletesBeforeTimeout verifies that a fast provider
// returns its response successfully.
func TestTimeoutMiddleware_SendCompletesBeforeTimeout(t *testing.T) {
	fast := makeSendFunc(
		0,
		&ai.ChatResponse{Content: "ok", FinishReason: "stop"},
		nil,
	)

	mw := NewTimeoutMiddleware(100 * time.Millisecond)
	chain := mw.Send(fast)

	resp, err := chain(context.Background(), ai.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "ok" {
		t.Errorf("expected 'ok', got %q", resp.Content)
	}
}

// TestTimeoutMiddleware_SendExceedsTimeout verifies that a slow provider causes
// the send middleware to return a DeadlineExceeded error.
func TestTimeoutMiddleware_SendExceedsTimeout(t *testing.T) {
	slow := makeSendFunc(200*time.Millisecond, nil, nil)

	mw := NewTimeoutMiddleware(20 * time.Millisecond)
	chain := mw.Send(slow)

	_, err := chain(context.Background(), ai.ChatRequest{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

// TestTimeoutMiddleware_ExistingShorterDeadline verifies that when the caller's
// context already has a deadline shorter than the middleware's timeout, the
// caller's deadline wins.
func TestTimeoutMiddleware_ExistingShorterDeadline(t *testing.T) {
	slow := makeSendFunc(200*time.Millisecond, nil, nil)

	// Middleware timeout is 100ms but caller deadline is only 20ms.
	mw := NewTimeoutMiddleware(100 * time.Millisecond)
	chain := mw.Send(slow)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := chain(ctx, ai.ChatRequest{})
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}

	// Should have cancelled closer to 20ms (caller deadline), not 100ms.
	if elapsed > 80*time.Millisecond {
		t.Errorf("expected cancellation near 20ms, elapsed %v", elapsed)
	}
}

// ========== Stream timeout tests ==========

// TestTimeoutMiddleware_StreamCompletesBeforeTimeout verifies that a fast stream
// is delivered without error.
func TestTimeoutMiddleware_StreamCompletesBeforeTimeout(t *testing.T) {
	fastStream := makeStreamFunc(0)

	mw := NewTimeoutMiddleware(100 * time.Millisecond)
	chain := mw.Stream(fastStream)

	stream, err := chain(context.Background(), ai.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	response, collectErr := stream.Collect()
	if collectErr != nil {
		t.Fatalf("Collect error: %v", collectErr)
	}

	if response.Content != "hello" {
		t.Errorf("expected 'hello', got %q", response.Content)
	}
}

// TestTimeoutMiddleware_StreamExceedsTimeout verifies that the timeout fires if
// the stream is too slow to produce its first event.
func TestTimeoutMiddleware_StreamExceedsTimeout(t *testing.T) {
	slowStream := makeStreamFunc(200 * time.Millisecond)

	mw := NewTimeoutMiddleware(20 * time.Millisecond)
	chain := mw.Stream(slowStream)

	stream, err := chain(context.Background(), ai.ChatRequest{})
	if err != nil {
		// Pre-stream error is also acceptable.
		if errors.Is(err, context.DeadlineExceeded) {
			return
		}

		t.Fatalf("unexpected non-deadline error: %v", err)
	}

	// The timeout should surface as a mid-stream error.
	for _, iterErr := range collectErrors(stream) {
		if errors.Is(iterErr, context.DeadlineExceeded) {
			return // Correct behavior.
		}
	}

	t.Error("expected DeadlineExceeded either as a stream error or pre-stream error")
}

// TestTimeoutMiddleware_StreamNilField verifies that NewTimeoutMiddleware sets a
// non-nil Stream field (unlike RetryMiddleware which leaves it nil).
func TestTimeoutMiddleware_StreamNilField(t *testing.T) {
	mw := NewTimeoutMiddleware(time.Second)
	if mw.Stream == nil {
		t.Error("expected non-nil Stream field for timeout middleware")
	}
}

// collectErrors drains a ChatStream and returns all non-nil iterator errors.
func collectErrors(stream *ai.ChatStream) []error {
	var errs []error

	for _, err := range stream.Iter() {
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}
