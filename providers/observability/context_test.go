package observability

import (
	"context"
	"reflect"
	"testing"
)

// testContextKey is a custom type for context keys in tests to avoid collisions.
type testContextKey string

func TestSpanFromContext_Nil(t *testing.T) {
	span := SpanFromContext(context.Background())
	if span != nil {
		t.Errorf("Expected nil span from nil context, got %v", span)
	}
}

func TestSpanFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	span := SpanFromContext(ctx)
	if span != nil {
		t.Errorf("Expected nil span from empty context, got %v", span)
	}
}

func TestSpanFromContext_WithSpan(t *testing.T) {
	ctx := context.Background()
	mockSpan := &mockSpan{name: "test-span"}

	ctx = ContextWithSpan(ctx, mockSpan)
	span := SpanFromContext(ctx)

	if span == nil {
		t.Fatal("Expected span from context, got nil")
	}

	if span != mockSpan {
		t.Errorf("Expected same span instance, got different span")
	}
}

func TestContextWithSpan_NilContext(t *testing.T) {
	mockSpan := &mockSpan{name: "test-span"}
	ctx := ContextWithSpan(context.Background(), mockSpan)

	if ctx == nil {
		t.Fatal("Expected non-nil context, got nil")
	}

	span := SpanFromContext(ctx)
	if span != mockSpan {
		t.Errorf("Expected span to be stored in context")
	}
}

func TestContextWithSpan_NilSpan(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithSpan(ctx, nil)

	span := SpanFromContext(ctx)
	if span != nil {
		t.Errorf("Expected nil span, got %v", span)
	}
}

func TestContextWithSpan_Overwrite(t *testing.T) {
	ctx := context.Background()
	span1 := &mockSpan{name: "span-1"}
	span2 := &mockSpan{name: "span-2"}

	ctx = ContextWithSpan(ctx, span1)
	ctx = ContextWithSpan(ctx, span2)

	span := SpanFromContext(ctx)
	if span != span2 {
		t.Errorf("Expected span2, got different span")
	}
}

func TestSpanFromContext_WrongType(t *testing.T) {
	ctx := context.Background()
	// Put a non-Span value with the same key (simulate collision)
	ctx = context.WithValue(ctx, spanContextKey, "not a span")

	span := SpanFromContext(ctx)
	if span != nil {
		t.Errorf("Expected nil when value is not a Span, got %v", span)
	}
}

func TestContextPropagation_Nested(t *testing.T) {
	ctx := context.Background()
	span := &mockSpan{name: "parent-span"}

	ctx = ContextWithSpan(ctx, span)

	// Simulate passing context through multiple layers
	ctx2 := context.WithValue(ctx, testContextKey("key"), "value")
	ctx3 := context.WithValue(ctx2, testContextKey("another"), "data")

	// Span should still be accessible
	retrievedSpan := SpanFromContext(ctx3)
	if retrievedSpan != span {
		t.Errorf("Expected span to survive context wrapping")
	}
}

func TestContextWithSpan_Concurrent(t *testing.T) {
	ctx := context.Background()
	span := &mockSpan{name: "concurrent-span"}

	done := make(chan bool)

	// Concurrent writes and reads
	for i := 0; i < 100; i++ {
		go func() {
			newCtx := ContextWithSpan(ctx, span)
			retrievedSpan := SpanFromContext(newCtx)
			if retrievedSpan != span {
				t.Errorf("Concurrent access failed")
			}
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

// mockSpan for testing
type mockSpan struct {
	name string
}

func (m *mockSpan) End()                                          {}
func (m *mockSpan) SetAttributes(attrs ...Attribute)              {}
func (m *mockSpan) SetStatus(code StatusCode, description string) {}
func (m *mockSpan) RecordError(err error)                         {}
func (m *mockSpan) AddEvent(name string, attrs ...Attribute)      {}

// mockProvider is a minimal Provider implementation used exclusively for
// Observer context round-trip tests. It carries an identifying label so
// that test assertions can confirm the exact same instance was stored and
// retrieved.
type mockProvider struct {
	label string
}

func (m *mockProvider) StartSpan(ctx context.Context, _ string, _ ...Attribute) (context.Context, Span) {
	return ctx, nil
}
func (m *mockProvider) Counter(_ string) Counter                          { return nil }
func (m *mockProvider) Histogram(_ string) Histogram                      { return nil }
func (m *mockProvider) Trace(_ context.Context, _ string, _ ...Attribute) {}
func (m *mockProvider) Debug(_ context.Context, _ string, _ ...Attribute) {}
func (m *mockProvider) Info(_ context.Context, _ string, _ ...Attribute)  {}
func (m *mockProvider) Warn(_ context.Context, _ string, _ ...Attribute)  {}
func (m *mockProvider) Error(_ context.Context, _ string, _ ...Attribute) {}

// --- Observer context tests ---

// TestContextWithObserver_RoundTrip verifies that a Provider stored via
// ContextWithObserver is the exact same instance returned by ObserverFromContext.
func TestContextWithObserver_RoundTrip(t *testing.T) {
	observer := &mockProvider{label: "round-trip-observer"}
	ctx := ContextWithObserver(context.Background(), observer)

	retrieved := ObserverFromContext(ctx)
	if retrieved == nil {
		t.Fatal("ObserverFromContext returned nil; expected the stored observer")
	}
	if retrieved != observer {
		t.Errorf("ObserverFromContext returned a different instance; pointer equality expected")
	}

	// Confirm identity via the label field as an extra safety check.
	mock, ok := retrieved.(*mockProvider)
	if !ok {
		t.Fatalf("Retrieved observer is not *mockProvider, got %T", retrieved)
	}
	if mock.label != "round-trip-observer" {
		t.Errorf("Expected label 'round-trip-observer', got %q", mock.label)
	}
}

// TestObserverFromContext_MissingKey ensures that a plain context with no
// observer stored returns nil without error.
func TestObserverFromContext_MissingKey(t *testing.T) {
	observer := ObserverFromContext(context.Background())
	if observer != nil {
		t.Errorf("Expected nil from context without observer, got %v", observer)
	}
}

// TestObserverFromContext_NilContext ensures passing a nil context does not
// panic and returns nil.
func TestObserverFromContext_NilContext(t *testing.T) {
	//nolint:staticcheck // intentionally passing nil to verify defensive guard
	observer := ObserverFromContext(nil)
	if observer != nil {
		t.Errorf("Expected nil from nil context, got %v", observer)
	}
}

// TestStringSlice verifies that the StringSlice attribute constructor
// correctly stores the key and a []string value.
func TestStringSlice(t *testing.T) {
	input := []string{"a", "b", "c"}
	attr := StringSlice("tools", input)

	if attr.Key != "tools" {
		t.Errorf("Expected key 'tools', got %q", attr.Key)
	}

	value, ok := attr.Value.([]string)
	if !ok {
		t.Fatalf("Expected Value to be []string, got %T", attr.Value)
	}
	if !reflect.DeepEqual(value, input) {
		t.Errorf("Expected value %v, got %v", input, value)
	}
}
