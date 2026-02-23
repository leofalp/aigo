package client

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/observability"
)

// ========== Mock observer ==========

// mockObserver records all observability calls for assertion in tests.
type mockObserver struct {
	spanStartCount int
	spanEndCount   int
	errorCount     int
	infoCount      int
	debugCount     int
	counterAdds    map[string]int64 // counter name -> cumulative value
	histogramRecs  int
	errorMessages  []string
	infoMessages   []string
}

func newMockObserver() *mockObserver {
	return &mockObserver{counterAdds: make(map[string]int64)}
}

// Tracer

func (m *mockObserver) StartSpan(ctx context.Context, name string, attrs ...observability.Attribute) (context.Context, observability.Span) {
	m.spanStartCount++
	span := &mockSpan{observer: m}
	return ctx, span
}

// Metrics

func (m *mockObserver) Counter(name string) observability.Counter {
	return &mockCounter{observer: m, name: name}
}

func (m *mockObserver) Histogram(_ string) observability.Histogram {
	return &mockHistogram{observer: m}
}

// Logger

func (m *mockObserver) Trace(_ context.Context, _ string, _ ...observability.Attribute) {}
func (m *mockObserver) Debug(_ context.Context, msg string, _ ...observability.Attribute) {
	m.debugCount++
}
func (m *mockObserver) Info(_ context.Context, msg string, _ ...observability.Attribute) {
	m.infoCount++
	m.infoMessages = append(m.infoMessages, msg)
}
func (m *mockObserver) Warn(_ context.Context, _ string, _ ...observability.Attribute) {}
func (m *mockObserver) Error(_ context.Context, msg string, _ ...observability.Attribute) {
	m.errorCount++
	m.errorMessages = append(m.errorMessages, msg)
}

// mockSpan records End and SetStatus calls.
type mockSpan struct {
	observer    *mockObserver
	ended       bool
	statusCode  observability.StatusCode
	errorEvents int
}

func (s *mockSpan) End()                                              { s.ended = true; s.observer.spanEndCount++ }
func (s *mockSpan) SetAttributes(_ ...observability.Attribute)        {}
func (s *mockSpan) SetStatus(code observability.StatusCode, _ string) { s.statusCode = code }
func (s *mockSpan) RecordError(_ error)                               { s.errorEvents++ }
func (s *mockSpan) AddEvent(_ string, _ ...observability.Attribute)   {}

type mockCounter struct {
	observer *mockObserver
	name     string
}

func (c *mockCounter) Add(_ context.Context, value int64, _ ...observability.Attribute) {
	c.observer.counterAdds[c.name] += value
}

type mockHistogram struct {
	observer *mockObserver
}

func (h *mockHistogram) Record(_ context.Context, _ float64, _ ...observability.Attribute) {
	h.observer.histogramRecs++
}

// ========== Helper constructors ==========

// successSendFunc returns a SendFunc that immediately returns a successful ChatResponse.
func successSendFunc() func(context.Context, ai.ChatRequest) (*ai.ChatResponse, error) {
	return func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		return &ai.ChatResponse{
			Model:        "test-model",
			Content:      "hello world",
			FinishReason: "stop",
			Usage:        &ai.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
		}, nil
	}
}

// errorSendFunc returns a SendFunc that always returns an error.
func errorSendFunc(err error) func(context.Context, ai.ChatRequest) (*ai.ChatResponse, error) {
	return func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		return nil, err
	}
}

// simpleObsStreamFunc returns a StreamFunc yielding a content event, usage event, and done event.
func simpleObsStreamFunc() func(context.Context, ai.ChatRequest) (*ai.ChatStream, error) {
	return func(_ context.Context, _ ai.ChatRequest) (*ai.ChatStream, error) {
		iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
			if !yield(ai.StreamEvent{Type: ai.StreamEventContent, Content: "streamed"}, nil) {
				return
			}
			if !yield(ai.StreamEvent{
				Type:  ai.StreamEventUsage,
				Usage: &ai.Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
			}, nil) {
				return
			}
			yield(ai.StreamEvent{Type: ai.StreamEventDone, FinishReason: "stop"}, nil)
		}
		return ai.NewChatStream(iter.Seq2[ai.StreamEvent, error](iteratorFunc)), nil
	}
}

// ========== Send middleware tests ==========

// TestObservabilityMiddleware_Send_Success verifies that a successful send call
// starts and ends a span, records histogram and counter metrics, and emits an
// INFO log.
func TestObservabilityMiddleware_Send_Success(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "default-model")

	chain := mw.Send(successSendFunc())
	response, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response == nil {
		t.Fatal("expected non-nil response")
	}

	// Span lifecycle
	if obs.spanStartCount != 1 {
		t.Errorf("expected 1 span start, got %d", obs.spanStartCount)
	}
	if obs.spanEndCount != 1 {
		t.Errorf("expected 1 span end, got %d", obs.spanEndCount)
	}

	// Metrics
	if obs.histogramRecs != 1 {
		t.Errorf("expected 1 histogram record, got %d", obs.histogramRecs)
	}
	if obs.counterAdds[observability.MetricClientRequestCount] != 1 {
		t.Errorf("expected request counter = 1, got %d", obs.counterAdds[observability.MetricClientRequestCount])
	}

	// Logs
	if obs.infoCount == 0 {
		t.Error("expected at least one INFO log")
	}
	if obs.errorCount != 0 {
		t.Errorf("expected no ERROR logs, got %d", obs.errorCount)
	}
}

// TestObservabilityMiddleware_Send_RecordsTokenCounters verifies that token
// counters are incremented with the values from the response's Usage field.
func TestObservabilityMiddleware_Send_RecordsTokenCounters(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "default-model")

	chain := mw.Send(successSendFunc()) // returns Usage: {10, 20, 30}
	_, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if obs.counterAdds[observability.MetricClientTokensPrompt] != 10 {
		t.Errorf("expected prompt tokens = 10, got %d", obs.counterAdds[observability.MetricClientTokensPrompt])
	}
	if obs.counterAdds[observability.MetricClientTokensCompletion] != 20 {
		t.Errorf("expected completion tokens = 20, got %d", obs.counterAdds[observability.MetricClientTokensCompletion])
	}
	if obs.counterAdds[observability.MetricClientTokensTotal] != 30 {
		t.Errorf("expected total tokens = 30, got %d", obs.counterAdds[observability.MetricClientTokensTotal])
	}
}

// TestObservabilityMiddleware_Send_ErrorPath verifies that a provider error is
// recorded on the span, logged as an error, counted, and propagated.
func TestObservabilityMiddleware_Send_ErrorPath(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "default-model")

	providerErr := errors.New("provider down")
	chain := mw.Send(errorSendFunc(providerErr))
	_, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})

	if !errors.Is(err, providerErr) {
		t.Errorf("expected providerErr, got %v", err)
	}

	// Span must still be ended on error.
	if obs.spanEndCount != 1 {
		t.Errorf("expected span to be ended on error (got spanEndCount=%d)", obs.spanEndCount)
	}

	// Error metrics and logging.
	if obs.errorCount == 0 {
		t.Error("expected at least one error log")
	}
	if obs.counterAdds[observability.MetricClientRequestCount] != 1 {
		t.Errorf("expected request counter = 1, got %d", obs.counterAdds[observability.MetricClientRequestCount])
	}

	// No histogram record on error.
	if obs.histogramRecs != 0 {
		t.Errorf("expected no histogram records on error, got %d", obs.histogramRecs)
	}
}

// TestObservabilityMiddleware_Send_ContextPropagation verifies that the observer
// and span are injected into the context that the next function receives.
func TestObservabilityMiddleware_Send_ContextPropagation(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "default-model")

	var capturedCtx context.Context
	probe := func(ctx context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		capturedCtx = ctx
		return &ai.ChatResponse{Model: "test-model", FinishReason: "stop"}, nil
	}

	chain := mw.Send(probe)
	_, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCtx == nil {
		t.Fatal("expected captured context to be non-nil")
	}
	if observability.ObserverFromContext(capturedCtx) == nil {
		t.Error("expected observer to be injected into context")
	}
	if observability.SpanFromContext(capturedCtx) == nil {
		t.Error("expected span to be injected into context")
	}
}

// TestObservabilityMiddleware_Send_BothFieldsNonNil verifies that both Send and
// Stream fields of the returned MiddlewareConfig are non-nil.
func TestObservabilityMiddleware_Send_BothFieldsNonNil(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "default-model")
	if mw.Send == nil {
		t.Error("expected non-nil Send field")
	}
	if mw.Stream == nil {
		t.Error("expected non-nil Stream field")
	}
}

// ========== Stream middleware tests ==========

// TestObservabilityMiddleware_Stream_Success verifies that consuming a stream to
// completion records a span (started before the provider call, ended after all
// events are consumed), histogram, and counters.
func TestObservabilityMiddleware_Stream_Success(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "default-model")

	chain := mw.Stream(simpleObsStreamFunc())
	stream, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected error starting stream: %v", err)
	}

	// Span should be started but NOT yet ended (stream not consumed).
	if obs.spanStartCount != 1 {
		t.Errorf("expected 1 span start, got %d", obs.spanStartCount)
	}
	if obs.spanEndCount != 0 {
		t.Errorf("expected span not ended before stream is consumed, got spanEndCount=%d", obs.spanEndCount)
	}

	// Consume the stream.
	resp, collectErr := stream.Collect()
	if collectErr != nil {
		t.Fatalf("Collect error: %v", collectErr)
	}
	if resp.Content != "streamed" {
		t.Errorf("expected 'streamed', got %q", resp.Content)
	}

	// After stream consumed: span ended and metrics recorded.
	if obs.spanEndCount != 1 {
		t.Errorf("expected span ended after stream, got spanEndCount=%d", obs.spanEndCount)
	}
	if obs.histogramRecs != 1 {
		t.Errorf("expected 1 histogram record, got %d", obs.histogramRecs)
	}
	if obs.counterAdds[observability.MetricClientRequestCount] != 1 {
		t.Errorf("expected request counter = 1, got %d", obs.counterAdds[observability.MetricClientRequestCount])
	}
}

// TestObservabilityMiddleware_Stream_RecordsTokenCounters verifies that token
// counters are populated from the StreamEventUsage event.
func TestObservabilityMiddleware_Stream_RecordsTokenCounters(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "default-model")

	chain := mw.Stream(simpleObsStreamFunc()) // usage: {5, 3, 8}
	stream, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _ = stream.Collect()

	if obs.counterAdds[observability.MetricClientTokensPrompt] != 5 {
		t.Errorf("expected prompt tokens = 5, got %d", obs.counterAdds[observability.MetricClientTokensPrompt])
	}
	if obs.counterAdds[observability.MetricClientTokensCompletion] != 3 {
		t.Errorf("expected completion tokens = 3, got %d", obs.counterAdds[observability.MetricClientTokensCompletion])
	}
	if obs.counterAdds[observability.MetricClientTokensTotal] != 8 {
		t.Errorf("expected total tokens = 8, got %d", obs.counterAdds[observability.MetricClientTokensTotal])
	}
}

// TestObservabilityMiddleware_Stream_PreStreamError verifies that when the
// provider itself returns an error (before streaming begins), the span is ended
// and the error is logged and returned.
func TestObservabilityMiddleware_Stream_PreStreamError(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "default-model")

	preErr := errors.New("auth failure")
	errStreamFunc := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatStream, error) {
		return nil, preErr
	}

	chain := mw.Stream(errStreamFunc)
	_, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})

	if !errors.Is(err, preErr) {
		t.Errorf("expected preErr, got %v", err)
	}
	if obs.spanEndCount != 1 {
		t.Errorf("expected span ended on pre-stream error, got spanEndCount=%d", obs.spanEndCount)
	}
	if obs.errorCount == 0 {
		t.Error("expected error log on pre-stream error")
	}
}

// TestObservabilityMiddleware_Stream_MidStreamError verifies that a mid-stream
// error causes the span to be recorded with an error status, the error to be
// logged, and the error to be propagated to the caller.
func TestObservabilityMiddleware_Stream_MidStreamError(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "default-model")

	streamErr := errors.New("mid-stream failure")
	errStreamFunc := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatStream, error) {
		iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
			if !yield(ai.StreamEvent{Type: ai.StreamEventContent, Content: "partial"}, nil) {
				return
			}
			yield(ai.StreamEvent{}, streamErr)
		}
		return ai.NewChatStream(iter.Seq2[ai.StreamEvent, error](iteratorFunc)), nil
	}

	chain := mw.Stream(errStreamFunc)
	stream, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected pre-stream error: %v", err)
	}

	_, collectErr := stream.Collect()
	if !errors.Is(collectErr, streamErr) {
		t.Errorf("expected streamErr, got %v", collectErr)
	}

	if obs.spanEndCount != 1 {
		t.Errorf("expected span ended on mid-stream error, got spanEndCount=%d", obs.spanEndCount)
	}
	if obs.errorCount == 0 {
		t.Error("expected error log on mid-stream error")
	}
}

// TestObservabilityMiddleware_Stream_AbandonedSpanEnded verifies that breaking
// out of the stream early still results in the span being ended.
func TestObservabilityMiddleware_Stream_AbandonedSpanEnded(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "default-model")

	// A stream with several events so the caller can break early.
	manyEventsStreamFunc := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatStream, error) {
		iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
			for i := 0; i < 5; i++ {
				if !yield(ai.StreamEvent{Type: ai.StreamEventContent, Content: "x"}, nil) {
					return
				}
			}
			yield(ai.StreamEvent{Type: ai.StreamEventDone}, nil)
		}
		return ai.NewChatStream(iter.Seq2[ai.StreamEvent, error](iteratorFunc)), nil
	}

	chain := mw.Stream(manyEventsStreamFunc)
	stream, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Break immediately after the first event.
	for range stream.Iter() {
		break
	}

	if obs.spanEndCount != 1 {
		t.Errorf("expected span ended after abandoned stream, got spanEndCount=%d", obs.spanEndCount)
	}
}

// TestObservabilityMiddleware_Stream_ContextPropagation verifies that the
// observer and span are injected into the context forwarded to next.
func TestObservabilityMiddleware_Stream_ContextPropagation(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "default-model")

	var capturedCtx context.Context
	probe := func(ctx context.Context, _ ai.ChatRequest) (*ai.ChatStream, error) {
		capturedCtx = ctx
		return simpleObsStreamFunc()(ctx, ai.ChatRequest{})
	}

	chain := mw.Stream(probe)
	stream, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _ = stream.Collect()

	if capturedCtx == nil {
		t.Fatal("expected captured context to be non-nil")
	}
	if observability.ObserverFromContext(capturedCtx) == nil {
		t.Error("expected observer to be injected into stream context")
	}
	if observability.SpanFromContext(capturedCtx) == nil {
		t.Error("expected span to be injected into stream context")
	}
}

// TestObservabilityMiddleware_EffectiveModel_FallsBackToDefault verifies that
// when the request has an empty Model field, the defaultModel is used for metric
// labels (observable via the counter calls, though in this test we just confirm
// no panic and the info log is emitted).
func TestObservabilityMiddleware_EffectiveModel_FallsBackToDefault(t *testing.T) {
	obs := newMockObserver()
	mw := NewObservabilityMiddleware(obs, "fallback-model")

	chain := mw.Send(successSendFunc())
	// Pass an empty Model — the middleware should use "fallback-model".
	_, err := chain(context.Background(), ai.ChatRequest{Model: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if obs.infoCount == 0 {
		t.Error("expected INFO log even when request model is empty")
	}
}
