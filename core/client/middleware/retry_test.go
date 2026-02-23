package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/leofalp/aigo/providers/ai"
)

// ========== Mock helpers ==========

// mockSendFunc is a helper that builds a client.SendFunc-compatible function
// with a configurable return sequence. Each call pops the next element.
type mockSendSequence struct {
	responses []*ai.ChatResponse
	errors    []error
	callCount int
}

func (m *mockSendSequence) next(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
	index := m.callCount
	m.callCount++

	if index < len(m.errors) && m.errors[index] != nil {
		return nil, m.errors[index]
	}

	if index < len(m.responses) {
		return m.responses[index], nil
	}

	return &ai.ChatResponse{Content: "default", FinishReason: "stop"}, nil
}

// newRetryProvider builds a mock ai.Provider for retry integration tests.
// It is only used when we need a full provider (not just a SendFunc).
type stubProvider struct {
	callCount int
	responses []*ai.ChatResponse
	errors    []error
}

func (s *stubProvider) SendMessage(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
	index := s.callCount
	s.callCount++

	if index < len(s.errors) && s.errors[index] != nil {
		return nil, s.errors[index]
	}

	if index < len(s.responses) {
		return s.responses[index], nil
	}

	return &ai.ChatResponse{Content: "ok", FinishReason: "stop"}, nil
}

func (s *stubProvider) IsStopMessage(resp *ai.ChatResponse) bool  { return resp.FinishReason == "stop" }
func (s *stubProvider) WithAPIKey(_ string) ai.Provider           { return s }
func (s *stubProvider) WithBaseURL(_ string) ai.Provider          { return s }
func (s *stubProvider) WithHttpClient(_ *http.Client) ai.Provider { return s }

// ========== NewRetryMiddleware tests ==========

// TestRetryMiddleware_SuccessOnFirstTry verifies that when the provider succeeds
// immediately, no retry is performed and the response is returned as-is.
func TestRetryMiddleware_SuccessOnFirstTry(t *testing.T) {
	seq := &mockSendSequence{
		responses: []*ai.ChatResponse{{Content: "ok", FinishReason: "stop"}},
	}

	mw := NewRetryMiddleware(RetryConfig{MaxRetries: 3})
	chain := mw.Send(seq.next)

	resp, err := chain(context.Background(), ai.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "ok" {
		t.Errorf("expected 'ok', got %q", resp.Content)
	}

	if seq.callCount != 1 {
		t.Errorf("expected 1 call, got %d", seq.callCount)
	}
}

// TestRetryMiddleware_RetryThenSuccess verifies that the middleware retries on a
// retryable error and eventually returns the successful response.
func TestRetryMiddleware_RetryThenSuccess(t *testing.T) {
	retryableErr := fmt.Errorf("status 429: rate limited")
	seq := &mockSendSequence{
		errors:    []error{retryableErr, nil},
		responses: []*ai.ChatResponse{nil, {Content: "ok", FinishReason: "stop"}},
	}

	mw := NewRetryMiddleware(RetryConfig{
		MaxRetries:     3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	})
	chain := mw.Send(seq.next)

	resp, err := chain(context.Background(), ai.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "ok" {
		t.Errorf("expected 'ok', got %q", resp.Content)
	}

	if seq.callCount != 2 {
		t.Errorf("expected 2 calls, got %d", seq.callCount)
	}
}

// TestRetryMiddleware_ExhaustsRetries verifies that after MaxRetries the
// middleware returns ErrRetryExhausted wrapping the last error.
func TestRetryMiddleware_ExhaustsRetries(t *testing.T) {
	retryableErr := fmt.Errorf("status 503: unavailable")

	callCount := 0
	alwaysFail := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		callCount++
		return nil, retryableErr
	}

	mw := NewRetryMiddleware(RetryConfig{
		MaxRetries:     3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
	})
	chain := mw.Send(alwaysFail)

	_, err := chain(context.Background(), ai.ChatRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrRetryExhausted) {
		t.Errorf("expected ErrRetryExhausted, got %v", err)
	}

	if !errors.Is(err, retryableErr) {
		t.Errorf("expected original error to be wrapped, got %v", err)
	}

	// 1 original + MaxRetries
	if callCount != 4 {
		t.Errorf("expected 4 total calls, got %d", callCount)
	}
}

// TestRetryMiddleware_NonRetryableError verifies that a non-retryable error is
// propagated immediately without any retry.
func TestRetryMiddleware_NonRetryableError(t *testing.T) {
	nonRetryableErr := errors.New("permanent failure")

	callCount := 0
	alwaysFail := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		callCount++
		return nil, nonRetryableErr
	}

	mw := NewRetryMiddleware(RetryConfig{
		MaxRetries:     3,
		InitialBackoff: time.Millisecond,
	})
	chain := mw.Send(alwaysFail)

	_, err := chain(context.Background(), ai.ChatRequest{})
	if !errors.Is(err, nonRetryableErr) {
		t.Fatalf("expected nonRetryableErr, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected exactly 1 call for non-retryable error, got %d", callCount)
	}
}

// TestRetryMiddleware_ContextCancellation verifies that a canceled context stops
// retries early and returns ctx.Err().
func TestRetryMiddleware_ContextCancellation(t *testing.T) {
	retryableErr := fmt.Errorf("status 429: rate limited")

	callCount := 0
	alwaysFail := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		callCount++
		return nil, retryableErr
	}

	mw := NewRetryMiddleware(RetryConfig{
		MaxRetries:     10,
		InitialBackoff: 50 * time.Millisecond, // long enough to be cancelled
		MaxBackoff:     200 * time.Millisecond,
	})
	chain := mw.Send(alwaysFail)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := chain(ctx, ai.ChatRequest{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}

	// Should have attempted exactly once before the deadline.
	if callCount < 1 {
		t.Errorf("expected at least 1 call before cancellation, got %d", callCount)
	}
}

// TestRetryMiddleware_CustomRetryableFunc verifies that a user-supplied
// RetryableFunc controls which errors are retried.
func TestRetryMiddleware_CustomRetryableFunc(t *testing.T) {
	sentinel := errors.New("custom-retryable")
	other := errors.New("not retryable")

	callCount := 0
	returnSentinel := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		callCount++
		if callCount == 1 {
			return nil, sentinel
		}

		return nil, other
	}

	mw := NewRetryMiddleware(RetryConfig{
		MaxRetries:     3,
		InitialBackoff: time.Millisecond,
		RetryableFunc: func(err error) bool {
			return errors.Is(err, sentinel)
		},
	})
	chain := mw.Send(returnSentinel)

	_, err := chain(context.Background(), ai.ChatRequest{})
	// Second call returns "other" (non-retryable) → should propagate immediately.
	if !errors.Is(err, other) {
		t.Errorf("expected 'other' error to propagate, got %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

// TestRetryMiddleware_DefaultConfig verifies that zero-valued RetryConfig gets
// sensible defaults applied (no panic, at least 1 retry).
func TestRetryMiddleware_DefaultConfig(t *testing.T) {
	retryableErr := fmt.Errorf("status 429: rate limited")

	callCount := 0
	alwaysFail := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		callCount++
		return nil, retryableErr
	}

	// Zero value — all defaults should be applied.
	mw := NewRetryMiddleware(RetryConfig{
		// Use tiny backoffs so the test doesn't take 30+ seconds.
		InitialBackoff: time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
	})
	chain := mw.Send(alwaysFail)

	_, err := chain(context.Background(), ai.ChatRequest{})
	if !errors.Is(err, ErrRetryExhausted) {
		t.Fatalf("expected ErrRetryExhausted, got %v", err)
	}

	// Default MaxRetries is 3 → 4 total calls.
	if callCount != 4 {
		t.Errorf("expected 4 total calls with default MaxRetries=3, got %d", callCount)
	}
}

// TestRetryMiddleware_ExponentialBackoff verifies that the backoff grows with
// each attempt by measuring elapsed wall time across attempts.
func TestRetryMiddleware_ExponentialBackoff(t *testing.T) {
	retryableErr := fmt.Errorf("status 429: rate limited")
	attempts := 0
	timestamps := make([]time.Time, 0, 4)

	recordCall := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		timestamps = append(timestamps, time.Now())
		attempts++
		return nil, retryableErr
	}

	mw := NewRetryMiddleware(RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 20 * time.Millisecond,
		MaxBackoff:     200 * time.Millisecond,
		BackoffFactor:  2.0,
		JitterFraction: 0, // No jitter for deterministic timing test.
	})
	chain := mw.Send(recordCall)

	_, _ = chain(context.Background(), ai.ChatRequest{})

	if len(timestamps) != 3 {
		t.Fatalf("expected 3 timestamps, got %d", len(timestamps))
	}

	// Gap between attempt 0→1 should be ~20ms; between 1→2 should be ~40ms.
	gap01 := timestamps[1].Sub(timestamps[0])
	gap12 := timestamps[2].Sub(timestamps[1])

	if gap12 <= gap01 {
		t.Errorf("expected gap12 (%v) > gap01 (%v) for exponential backoff", gap12, gap01)
	}
}

// TestRetryMiddleware_StreamIsNil verifies that the Stream field of the returned
// MiddlewareConfig is nil (streaming bypasses retry).
func TestRetryMiddleware_StreamIsNil(t *testing.T) {
	mw := NewRetryMiddleware(RetryConfig{})
	if mw.Stream != nil {
		t.Error("expected Stream field to be nil for retry middleware")
	}
}
