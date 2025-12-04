package observability

import (
	"context"
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
