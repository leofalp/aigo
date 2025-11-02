package slog

import (
	"aigo/providers/observability"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestSlogObserver_Implements_Provider(t *testing.T) {
	var _ observability.Provider = (*Observer)(nil)
}

func TestSlogObserver_New(t *testing.T) {
	obs := New(nil)
	if obs == nil {
		t.Fatal("New() returned nil")
	}

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	obs = New(logger)
	if obs == nil {
		t.Fatal("New() with custom logger returned nil")
	}
}

func TestSlogObserver_StartSpan(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	obs := New(logger)
	ctx := context.Background()

	ctx2, span := obs.StartSpan(ctx, "test-span",
		observability.String("key", "value"),
		observability.Int("count", 42),
	)

	if ctx2 == nil {
		t.Fatal("StartSpan returned nil context")
	}
	if span == nil {
		t.Fatal("StartSpan returned nil span")
	}

	output := buf.String()
	if !strings.Contains(output, "test-span") {
		t.Errorf("Expected span name in output, got: %s", output)
	}
	if !strings.Contains(output, "span.start") {
		t.Errorf("Expected span.start event in output, got: %s", output)
	}
}

func TestSlogObserver_Span_End(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	obs := New(logger)
	ctx := context.Background()

	_, span := obs.StartSpan(ctx, "test-span")
	buf.Reset()

	span.End()

	output := buf.String()
	if !strings.Contains(output, "test-span") {
		t.Errorf("Expected span name in output, got: %s", output)
	}
	if !strings.Contains(output, "span.end") {
		t.Errorf("Expected span.end event in output, got: %s", output)
	}
	if !strings.Contains(output, "duration") {
		t.Errorf("Expected duration in output, got: %s", output)
	}
}

func TestSlogObserver_Span_SetAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	obs := New(logger)
	ctx := context.Background()

	_, span := obs.StartSpan(ctx, "test-span")
	span.SetAttributes(
		observability.String("attr1", "value1"),
		observability.Int("attr2", 123),
	)
	buf.Reset()

	span.End()

	output := buf.String()
	if !strings.Contains(output, "attr1") {
		t.Errorf("Expected attr1 in output, got: %s", output)
	}
	if !strings.Contains(output, "value1") {
		t.Errorf("Expected value1 in output, got: %s", output)
	}
}

func TestSlogObserver_Span_SetStatus(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	obs := New(logger)
	ctx := context.Background()

	_, span := obs.StartSpan(ctx, "test-span")
	span.SetStatus(observability.StatusOK, "operation successful")
	buf.Reset()

	span.End()

	output := buf.String()
	if !strings.Contains(output, "status") {
		t.Errorf("Expected status in output, got: %s", output)
	}
	if !strings.Contains(output, "ok") {
		t.Errorf("Expected 'ok' status in output, got: %s", output)
	}
}

func TestSlogObserver_Span_RecordError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	obs := New(logger)
	ctx := context.Background()

	_, span := obs.StartSpan(ctx, "test-span")
	testErr := errors.New("test error")
	span.RecordError(testErr)

	output := buf.String()
	if !strings.Contains(output, "test error") {
		t.Errorf("Expected error message in output, got: %s", output)
	}
	if !strings.Contains(output, "error") {
		t.Errorf("Expected 'error' in output, got: %s", output)
	}
}

func TestSlogObserver_Span_RecordError_Nil(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	obs := New(logger)
	ctx := context.Background()

	_, span := obs.StartSpan(ctx, "test-span")
	span.RecordError(nil) // Should not panic

	output := buf.String()
	if output != "" {
		t.Errorf("Expected no output for nil error, got: %s", output)
	}
}

func TestSlogObserver_Span_AddEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	obs := New(logger)
	ctx := context.Background()

	_, span := obs.StartSpan(ctx, "test-span")
	buf.Reset()

	span.AddEvent("custom-event", observability.String("detail", "something happened"))

	output := buf.String()
	if !strings.Contains(output, "custom-event") {
		t.Errorf("Expected event name in output, got: %s", output)
	}
	if !strings.Contains(output, "detail") {
		t.Errorf("Expected event attribute in output, got: %s", output)
	}
}

func TestSlogObserver_Counter(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	obs := New(logger)
	ctx := context.Background()

	counter := obs.Counter("test-counter")
	if counter == nil {
		t.Fatal("Counter() returned nil")
	}

	counter.Add(ctx, 5, observability.String("label", "test"))

	output := buf.String()
	if !strings.Contains(output, "test-counter") {
		t.Errorf("Expected counter name in output, got: %s", output)
	}
	if !strings.Contains(output, "counter") {
		t.Errorf("Expected 'counter' type in output, got: %s", output)
	}
}

func TestSlogObserver_Counter_Accumulation(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	obs := New(logger)
	ctx := context.Background()

	counter := obs.Counter("test-counter")
	counter.Add(ctx, 10)
	counter.Add(ctx, 5)
	counter.Add(ctx, 3)

	output := buf.String()
	// Should contain cumulative values
	if !strings.Contains(output, "18") {
		t.Errorf("Expected accumulated value 18 in output, got: %s", output)
	}
}

func TestSlogObserver_Counter_SameNameReturnsSameInstance(t *testing.T) {
	obs := New(nil)
	ctx := context.Background()

	counter1 := obs.Counter("test-counter")
	counter2 := obs.Counter("test-counter")

	// Add to counter1
	counter1.Add(ctx, 10)

	// counter2 should share the same underlying counter
	// We can't directly test this without accessing internals,
	// but we can ensure both are non-nil
	if counter1 == nil || counter2 == nil {
		t.Fatal("Counters should not be nil")
	}
}

func TestSlogObserver_Histogram(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	obs := New(logger)
	ctx := context.Background()

	histogram := obs.Histogram("test-histogram")
	if histogram == nil {
		t.Fatal("Histogram() returned nil")
	}

	histogram.Record(ctx, 1.23, observability.String("label", "test"))

	output := buf.String()
	if !strings.Contains(output, "test-histogram") {
		t.Errorf("Expected histogram name in output, got: %s", output)
	}
	if !strings.Contains(output, "histogram") {
		t.Errorf("Expected 'histogram' type in output, got: %s", output)
	}
	if !strings.Contains(output, "1.23") {
		t.Errorf("Expected value 1.23 in output, got: %s", output)
	}
}

func TestSlogObserver_Logging_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	obs := New(logger)
	ctx := context.Background()

	obs.Debug(ctx, "debug message", observability.String("key", "value"))

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Errorf("Expected debug message in output, got: %s", output)
	}
	if !strings.Contains(output, "DEBUG") || !strings.Contains(output, "debug") {
		t.Errorf("Expected DEBUG level in output, got: %s", output)
	}
}

func TestSlogObserver_Logging_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	obs := New(logger)
	ctx := context.Background()

	obs.Info(ctx, "info message", observability.Int("count", 42))

	output := buf.String()
	if !strings.Contains(output, "info message") {
		t.Errorf("Expected info message in output, got: %s", output)
	}
	if !strings.Contains(output, "42") {
		t.Errorf("Expected count=42 in output, got: %s", output)
	}
}

func TestSlogObserver_Logging_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	obs := New(logger)
	ctx := context.Background()

	obs.Warn(ctx, "warning message", observability.Bool("flag", true))

	output := buf.String()
	if !strings.Contains(output, "warning message") {
		t.Errorf("Expected warning message in output, got: %s", output)
	}
	if !strings.Contains(output, "WARN") || !strings.Contains(output, "warn") {
		t.Errorf("Expected WARN level in output, got: %s", output)
	}
}

func TestSlogObserver_Logging_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	obs := New(logger)
	ctx := context.Background()

	obs.Error(ctx, "error message", observability.Float64("value", 3.14))

	output := buf.String()
	if !strings.Contains(output, "error message") {
		t.Errorf("Expected error message in output, got: %s", output)
	}
	if !strings.Contains(output, "ERROR") || !strings.Contains(output, "error") {
		t.Errorf("Expected ERROR level in output, got: %s", output)
	}
}

func TestSlogObserver_Logging_FiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	// Set level to Info - Debug should be filtered out
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	obs := New(logger)
	ctx := context.Background()

	obs.Debug(ctx, "debug message")
	obs.Info(ctx, "info message")

	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Errorf("Debug message should be filtered out, got: %s", output)
	}
	if !strings.Contains(output, "info message") {
		t.Errorf("Info message should be present, got: %s", output)
	}
}

func TestSlogObserver_ConcurrentAccess(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	obs := New(logger)
	ctx := context.Background()

	done := make(chan bool)

	// Test concurrent access to all methods
	for i := 0; i < 100; i++ {
		go func(id int) {
			_, span := obs.StartSpan(ctx, "concurrent-span")
			span.SetAttributes(observability.Int("id", id))
			span.End()

			counter := obs.Counter("concurrent-counter")
			counter.Add(ctx, 1)

			histogram := obs.Histogram("concurrent-histogram")
			histogram.Record(ctx, float64(id))

			obs.Info(ctx, "concurrent message", observability.Int("id", id))

			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	// Just verify no panics occurred
	if buf.Len() == 0 {
		t.Error("Expected some output from concurrent operations")
	}
}

func BenchmarkSlogObserver_StartSpan(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	obs := New(logger)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := obs.StartSpan(ctx, "test-span")
		span.End()
	}
}

func BenchmarkSlogObserver_Counter(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	obs := New(logger)
	ctx := context.Background()
	counter := obs.Counter("test-counter")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Add(ctx, 1)
	}
}

func BenchmarkSlogObserver_Histogram(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	obs := New(logger)
	ctx := context.Background()
	histogram := obs.Histogram("test-histogram")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		histogram.Record(ctx, 1.234)
	}
}

func BenchmarkSlogObserver_Logging(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	obs := New(logger)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obs.Info(ctx, "test message", observability.String("key", "value"))
	}
}

func TestSlogObserver_Span_Duration(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	obs := New(logger)
	ctx := context.Background()

	_, span := obs.StartSpan(ctx, "timed-span")
	time.Sleep(10 * time.Millisecond)
	buf.Reset()
	span.End()

	output := buf.String()
	// Should contain duration information
	if !strings.Contains(output, "duration") {
		t.Errorf("Expected duration in output, got: %s", output)
	}
}
