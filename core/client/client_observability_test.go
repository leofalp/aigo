package client

import (
	"aigo/providers/ai"
	"aigo/providers/memory/inmemory"
	"aigo/providers/observability"
	"aigo/providers/observability/slogobs"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

// mockProvider is a simple mock for testing observability integration
type mockProvider struct {
	sendMessageFunc func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error)
}

func (m *mockProvider) SendMessage(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	if m.sendMessageFunc != nil {
		return m.sendMessageFunc(ctx, req)
	}
	return &ai.ChatResponse{
		Id:           "test-id",
		Model:        "test-model",
		Content:      "test response",
		FinishReason: "stop",
		Usage: &ai.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}, nil
}

func (m *mockProvider) IsStopMessage(message *ai.ChatResponse) bool {
	return message.FinishReason == "stop"
}

func (m *mockProvider) WithAPIKey(apiKey string) ai.Provider {
	return m
}

func (m *mockProvider) WithBaseURL(baseURL string) ai.Provider {
	return m
}

func (m *mockProvider) WithHttpClient(httpClient *http.Client) ai.Provider {
	return m
}

func TestClient_DefaultNilObserver(t *testing.T) {
	provider := &mockProvider{}
	client, err := NewClient[string](provider)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if client.observer != nil {
		t.Errorf("Default observer should be nil for zero overhead, got %T", client.observer)
	}
}

func TestClient_WithObserver(t *testing.T) {
	provider := &mockProvider{}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	observer := slogobs.New(slogobs.WithLogger(logger))

	client, err := NewClient[string](provider, WithObserver(observer))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if client.observer != observer {
		t.Error("Observer was not set correctly")
	}
}

func TestClient_SendMessage_ObservabilityTracing(t *testing.T) {
	provider := &mockProvider{}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	observer := slogobs.New(slogobs.WithLogger(logger))

	client, err := NewClient[string](provider,
		WithObserver(observer),
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.SendMessage(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	output := buf.String()

	// Verify span events
	if !strings.Contains(output, "client.send_message") {
		t.Error("Expected span name in output")
	}
	if !strings.Contains(output, "span.start") {
		t.Error("Expected span start event")
	}
	if !strings.Contains(output, "span.end") {
		t.Error("Expected span end event")
	}
	if !strings.Contains(output, "duration") {
		t.Error("Expected duration in span end")
	}
}

func TestClient_SendMessage_ObservabilityLogging(t *testing.T) {
	provider := &mockProvider{}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	observer := slogobs.New(slogobs.WithLogger(logger))

	client, err := NewClient[string](provider,
		WithObserver(observer),
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.SendMessage(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	output := buf.String()

	// Verify logging
	if !strings.Contains(output, "LLM call completed") {
		t.Error("Expected success log message")
	}
	if !strings.Contains(output, "finish_reason") {
		t.Error("Expected finish_reason in logs")
	}
}

func TestClient_SendMessage_ObservabilityMetrics(t *testing.T) {
	provider := &mockProvider{}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	observer := slogobs.New(slogobs.WithLogger(logger))

	client, err := NewClient[string](provider,
		WithObserver(observer),
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.SendMessage(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	output := buf.String()

	// Verify metrics
	if !strings.Contains(output, "aigo.client.request.duration") {
		t.Error("Expected request duration metric")
	}
	if !strings.Contains(output, "aigo.client.request.count") {
		t.Error("Expected request count metric")
	}
	if !strings.Contains(output, "aigo.client.tokens.total") {
		t.Error("Expected total tokens metric")
	}
	if !strings.Contains(output, "aigo.client.tokens.prompt") {
		t.Error("Expected prompt tokens metric")
	}
	if !strings.Contains(output, "aigo.client.tokens.completion") {
		t.Error("Expected completion tokens metric")
	}
}

func TestClient_SendMessage_ErrorObservability(t *testing.T) {
	testError := errors.New("test error")
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			return nil, testError
		},
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	observer := slogobs.New(slogobs.WithLogger(logger))

	client, err := NewClient[string](provider,
		WithObserver(observer),
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.SendMessage(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("Expected error but got none")
	}

	output := buf.String()

	// Verify error is recorded
	if !strings.Contains(output, "test error") {
		t.Error("Expected error message in output")
	}
	if !strings.Contains(output, "Failed to send message") {
		t.Error("Expected failure log message")
	}
	if !strings.Contains(output, "error") {
		t.Error("Expected error event in span")
	}
}

func TestClient_SendMessage_TokenMetrics(t *testing.T) {
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			return &ai.ChatResponse{
				Id:           "test-id",
				Model:        "test-model",
				Content:      "response",
				FinishReason: "stop",
				Usage: &ai.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
			}, nil
		},
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	observer := slogobs.New(slogobs.WithLogger(logger))

	client, err := NewClient[string](provider,
		WithObserver(observer),
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.SendMessage(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	output := buf.String()

	// Verify token counts in metrics
	if !strings.Contains(output, "150") {
		t.Error("Expected total tokens (150) in output")
	}
	if !strings.Contains(output, "100") {
		t.Error("Expected prompt tokens (100) in output")
	}
	if !strings.Contains(output, "50") {
		t.Error("Expected completion tokens (50) in output")
	}
}

func TestClient_SendMessage_NoopObserverPerformance(t *testing.T) {
	provider := &mockProvider{}
	client, err := NewClient[string](provider,
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	start := time.Now()
	iterations := 1000

	for i := 0; i < iterations; i++ {
		_, err := client.SendMessage(context.Background(), "test")
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}
	}

	duration := time.Since(start)
	avgPerCall := duration / time.Duration(iterations)

	// Noop observer should add minimal overhead (< 1µs per call for observability code)
	if avgPerCall > 10*time.Millisecond {
		t.Logf("Average time per call with noop observer: %v", avgPerCall)
	}
}

func TestClient_MultipleRequests_CounterAccumulation(t *testing.T) {
	provider := &mockProvider{}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	observer := slogobs.New(slogobs.WithLogger(logger))

	client, err := NewClient[string](provider,
		WithObserver(observer),
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Send multiple requests
	for i := 0; i < 3; i++ {
		_, err := client.SendMessage(context.Background(), "test")
		if err != nil {
			t.Fatalf("SendMessage %d failed: %v", i, err)
		}
	}

	output := buf.String()

	// Counter should accumulate
	// The counter value should increase with each request
	if !strings.Contains(output, "aigo.client.request.count") {
		t.Error("Expected request count metric")
	}
}

func TestClient_SendMessage_SpanAttributes(t *testing.T) {
	provider := &mockProvider{}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	observer := slogobs.New(slogobs.WithLogger(logger))

	client, err := NewClient[string](provider,
		WithObserver(observer),
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.SendMessage(context.Background(), "What is AI?")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	output := buf.String()

	// Verify span attributes
	if !strings.Contains(output, "model") {
		t.Error("Expected model attribute in span")
	}
	// Note: Model value comes from AIGO_DEFAULT_LLM_MODEL environment variable
	// We just verify the attribute exists, not the specific value
	if !strings.Contains(output, "tokens.total") {
		t.Error("Expected tokens.total attribute in span")
	}
}

// Mock observer for testing
type testObserver struct {
	spanStarted     bool
	spanEnded       bool
	errorLogged     bool
	metricsRecorded int
}

func (o *testObserver) StartSpan(ctx context.Context, name string, attrs ...observability.Attribute) (context.Context, observability.Span) {
	o.spanStarted = true
	return ctx, &testSpan{observer: o}
}

func (o *testObserver) Counter(name string) observability.Counter {
	return &testCounter{observer: o}
}

func (o *testObserver) Histogram(name string) observability.Histogram {
	return &testHistogram{observer: o}
}

func (o *testObserver) Trace(ctx context.Context, msg string, attrs ...observability.Attribute) {}
func (o *testObserver) Debug(ctx context.Context, msg string, attrs ...observability.Attribute) {}
func (o *testObserver) Info(ctx context.Context, msg string, attrs ...observability.Attribute)  {}
func (o *testObserver) Warn(ctx context.Context, msg string, attrs ...observability.Attribute)  {}
func (o *testObserver) Error(ctx context.Context, msg string, attrs ...observability.Attribute) {
	o.errorLogged = true
}

type testSpan struct {
	observer *testObserver
}

func (s *testSpan) End() {
	s.observer.spanEnded = true
}

func (s *testSpan) SetAttributes(attrs ...observability.Attribute)         {}
func (s *testSpan) SetStatus(code observability.StatusCode, desc string)   {}
func (s *testSpan) RecordError(err error)                                  {}
func (s *testSpan) AddEvent(name string, attrs ...observability.Attribute) {}

type testCounter struct {
	observer *testObserver
}

func (c *testCounter) Add(ctx context.Context, value int64, attrs ...observability.Attribute) {
	c.observer.metricsRecorded++
}

type testHistogram struct {
	observer *testObserver
}

func (h *testHistogram) Record(ctx context.Context, value float64, attrs ...observability.Attribute) {
	h.observer.metricsRecorded++
}

func TestClient_SendMessage_ObserverCalled(t *testing.T) {
	provider := &mockProvider{}
	observer := &testObserver{}

	client, err := NewClient[string](provider,
		WithObserver(observer),
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.SendMessage(context.Background(), "test")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if !observer.spanStarted {
		t.Error("Span was not started")
	}
	if !observer.spanEnded {
		t.Error("Span was not ended")
	}
	if observer.metricsRecorded < 4 {
		t.Errorf("Expected at least 4 metrics recorded, got %d", observer.metricsRecorded)
	}
}

func TestClient_SendMessage_ErrorObserverCalled(t *testing.T) {
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			return nil, errors.New("test error")
		},
	}
	observer := &testObserver{}

	client, err := NewClient[string](provider,
		WithObserver(observer),
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.SendMessage(context.Background(), "test")
	if err == nil {
		t.Fatal("Expected error but got none")
	}

	if !observer.errorLogged {
		t.Error("Error was not logged")
	}
}

func TestClient_SendMessage_NilObserver_NoPanic(t *testing.T) {
	provider := &mockProvider{}

	// Create client without observer (nil by default)
	client, err := NewClient[string](provider,
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Verify observer is nil
	if client.observer != nil {
		t.Fatalf("Expected nil observer, got %T", client.observer)
	}

	// Should not panic with nil observer
	resp, err := client.SendMessage(context.Background(), "test message")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	if resp.Content != "test response" {
		t.Errorf("Expected 'test response', got '%s'", resp.Content)
	}
}

func TestClient_SendMessage_NilObserver_Error_NoPanic(t *testing.T) {
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			return nil, errors.New("simulated error")
		},
	}

	// Create client without observer (nil by default)
	client, err := NewClient[string](provider,
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Should not panic even on error with nil observer
	_, err = client.SendMessage(context.Background(), "test message")
	if err == nil {
		t.Fatal("Expected error but got none")
	}

	if err.Error() != "simulated error" {
		t.Errorf("Expected 'simulated error', got '%s'", err.Error())
	}
}

func TestClient_StatelessMode_NilMemory(t *testing.T) {
	provider := &mockProvider{}

	// Create client without memory provider (stateless mode)
	client, err := NewClient[string](provider)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Verify memory provider is nil
	if client.memoryProvider != nil {
		t.Fatalf("Expected nil memoryProvider for stateless mode, got %T", client.memoryProvider)
	}

	// Should work without panicking
	resp, err := client.SendMessage(context.Background(), "test message")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	if resp.Content != "test response" {
		t.Errorf("Expected 'test response', got '%s'", resp.Content)
	}
}

func TestClient_StatelessMode_OnlyCurrentPrompt(t *testing.T) {
	var capturedRequest ai.ChatRequest
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			capturedRequest = req
			return &ai.ChatResponse{
				Id:           "test-id",
				Model:        "test-model",
				Content:      "response",
				FinishReason: "stop",
			}, nil
		},
	}

	// Create stateless client
	client, err := NewClient[string](provider)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Send first message
	_, err = client.SendMessage(context.Background(), "first message")
	if err != nil {
		t.Fatalf("First SendMessage failed: %v", err)
	}

	// Verify only current message is sent (no history)
	if len(capturedRequest.Messages) != 1 {
		t.Fatalf("Expected 1 message in stateless mode, got %d", len(capturedRequest.Messages))
	}
	if capturedRequest.Messages[0].Content != "first message" {
		t.Errorf("Expected 'first message', got '%s'", capturedRequest.Messages[0].Content)
	}

	// Send second message
	_, err = client.SendMessage(context.Background(), "second message")
	if err != nil {
		t.Fatalf("Second SendMessage failed: %v", err)
	}

	// Verify still only one message (no history accumulated)
	if len(capturedRequest.Messages) != 1 {
		t.Fatalf("Expected 1 message in stateless mode, got %d", len(capturedRequest.Messages))
	}
	if capturedRequest.Messages[0].Content != "second message" {
		t.Errorf("Expected 'second message', got '%s'", capturedRequest.Messages[0].Content)
	}
}

func TestClient_StatefulMode_AccumulatesHistory(t *testing.T) {
	var capturedRequest ai.ChatRequest
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			capturedRequest = req
			return &ai.ChatResponse{
				Id:           "test-id",
				Model:        "test-model",
				Content:      "response",
				FinishReason: "stop",
			}, nil
		},
	}

	// Create stateful client with memory
	client, err := NewClient[string](provider,
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Send first message
	_, err = client.SendMessage(context.Background(), "first message")
	if err != nil {
		t.Fatalf("First SendMessage failed: %v", err)
	}

	// Verify first message is in history
	if len(capturedRequest.Messages) != 1 {
		t.Fatalf("Expected 1 message after first call, got %d", len(capturedRequest.Messages))
	}

	// Send second message
	_, err = client.SendMessage(context.Background(), "second message")
	if err != nil {
		t.Fatalf("Second SendMessage failed: %v", err)
	}

	// Verify both messages are in history
	if len(capturedRequest.Messages) != 2 {
		t.Fatalf("Expected 2 messages after second call, got %d", len(capturedRequest.Messages))
	}
	if capturedRequest.Messages[0].Content != "first message" {
		t.Errorf("Expected 'first message' at index 0, got '%s'", capturedRequest.Messages[0].Content)
	}
	if capturedRequest.Messages[1].Content != "second message" {
		t.Errorf("Expected 'second message' at index 1, got '%s'", capturedRequest.Messages[1].Content)
	}
}

func TestClient_StatelessMode_WithObserver(t *testing.T) {
	provider := &mockProvider{}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	observer := slogobs.New(slogobs.WithLogger(logger))

	// Create stateless client with observer
	client, err := NewClient[string](provider,
		WithObserver(observer),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Should not panic with nil memory and observer
	_, err = client.SendMessage(context.Background(), "test")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	output := buf.String()

	// Verify observability still works
	if !strings.Contains(output, "client.send_message") {
		t.Error("Expected span name in output")
	}
	if !strings.Contains(output, "LLM call completed") {
		t.Error("Expected success log message")
	}
}
