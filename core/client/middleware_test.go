package client

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/leofalp/aigo/providers/ai"
)

// ========== Chain construction helpers ==========

// callRecorder records whether a middleware was invoked, in what order, and
// whether the next function was called.
type callRecorder struct {
	order        *[]string
	name         string
	calledSend   bool
	calledStream bool
}

func newCallRecorder(name string, sharedOrder *[]string) *callRecorder {
	return &callRecorder{order: sharedOrder, name: name}
}

func (rec *callRecorder) sendMiddleware() Middleware {
	return func(next SendFunc) SendFunc {
		return func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			rec.calledSend = true
			*rec.order = append(*rec.order, rec.name)

			return next(ctx, request)
		}
	}
}

func (rec *callRecorder) streamMiddleware() StreamMiddleware {
	return func(next StreamFunc) StreamFunc {
		return func(ctx context.Context, request ai.ChatRequest) (*ai.ChatStream, error) {
			rec.calledStream = true
			*rec.order = append(*rec.order, rec.name+"-stream")

			return next(ctx, request)
		}
	}
}

// ========== buildSendChain tests ==========

// TestBuildSendChain_EmptyMiddlewares verifies that an empty slice results in a
// direct provider call.
func TestBuildSendChain_EmptyMiddlewares(t *testing.T) {
	provider := &mockProvider{}
	chain := buildSendChain(provider, nil)

	resp, err := chain(context.Background(), ai.ChatRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Content != "test response" {
		t.Errorf("expected 'test response', got %q", resp.Content)
	}
}

// TestBuildSendChain_SingleMiddleware verifies that a single middleware wraps the
// provider call correctly.
func TestBuildSendChain_SingleMiddleware(t *testing.T) {
	provider := &mockProvider{}
	order := []string{}
	rec := newCallRecorder("mw1", &order)

	chain := buildSendChain(provider, []MiddlewareConfig{
		{Send: rec.sendMiddleware()},
	})

	_, err := chain(context.Background(), ai.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !rec.calledSend {
		t.Error("expected middleware to be called")
	}
}

// TestBuildSendChain_MultipleMiddlewares verifies outermost-first execution order.
func TestBuildSendChain_MultipleMiddlewares(t *testing.T) {
	provider := &mockProvider{}
	order := []string{}
	rec1 := newCallRecorder("mw1", &order)
	rec2 := newCallRecorder("mw2", &order)
	rec3 := newCallRecorder("mw3", &order)

	chain := buildSendChain(provider, []MiddlewareConfig{
		{Send: rec1.sendMiddleware()},
		{Send: rec2.sendMiddleware()},
		{Send: rec3.sendMiddleware()},
	})

	_, err := chain(context.Background(), ai.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"mw1", "mw2", "mw3"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}

	for i, name := range expected {
		if order[i] != name {
			t.Errorf("position %d: expected %q, got %q", i, name, order[i])
		}
	}
}

// TestBuildSendChain_ShortCircuit verifies that a middleware can return early
// without calling next.
func TestBuildSendChain_ShortCircuit(t *testing.T) {
	provider := &mockProvider{}
	shortCircuitError := errors.New("short-circuit")

	shortCircuit := Middleware(func(next SendFunc) SendFunc {
		return func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			return nil, shortCircuitError
		}
	})

	order := []string{}
	rec := newCallRecorder("after-short-circuit", &order)

	chain := buildSendChain(provider, []MiddlewareConfig{
		{Send: shortCircuit},
		{Send: rec.sendMiddleware()},
	})

	_, err := chain(context.Background(), ai.ChatRequest{})
	if !errors.Is(err, shortCircuitError) {
		t.Fatalf("expected short-circuit error, got %v", err)
	}

	if rec.calledSend {
		t.Error("middleware after short-circuit should not be called")
	}
}

// ========== buildStreamChain tests ==========

// TestBuildStreamChain_NilStreamFields verifies that middleware with nil Stream
// fields are skipped in the stream chain.
func TestBuildStreamChain_NilStreamFields(t *testing.T) {
	provider := &mockStreamProvider{}
	order := []string{}
	rec := newCallRecorder("send-only", &order)

	// Stream is nil — should be skipped
	chain := buildStreamChain(provider, []MiddlewareConfig{
		{Send: rec.sendMiddleware(), Stream: nil},
	})

	_, err := chain(context.Background(), ai.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The stream-only recorder should NOT have been called (its Stream field is nil)
	if rec.calledStream {
		t.Error("stream middleware with nil Stream should not be invoked")
	}
}

// TestBuildStreamChain_WithStreamMiddleware verifies that non-nil Stream fields
// are applied in the correct order.
func TestBuildStreamChain_WithStreamMiddleware(t *testing.T) {
	provider := &mockStreamProvider{}
	order := []string{}
	rec1 := newCallRecorder("mw1", &order)
	rec2 := newCallRecorder("mw2", &order)

	chain := buildStreamChain(provider, []MiddlewareConfig{
		{Send: rec1.sendMiddleware(), Stream: rec1.streamMiddleware()},
		{Send: rec2.sendMiddleware(), Stream: rec2.streamMiddleware()},
	})

	_, err := chain(context.Background(), ai.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedOrder := []string{"mw1-stream", "mw2-stream"}
	if len(order) != len(expectedOrder) {
		t.Fatalf("expected %d stream calls, got %d: %v", len(expectedOrder), len(order), order)
	}

	for i, name := range expectedOrder {
		if order[i] != name {
			t.Errorf("position %d: expected %q, got %q", i, name, order[i])
		}
	}
}

// ========== WithMiddleware client option tests ==========

// TestWithMiddleware_ClientCallsChain verifies that SendMessage routes through
// the middleware chain when one is configured.
func TestWithMiddleware_ClientCallsChain(t *testing.T) {
	provider := &mockProvider{}
	order := []string{}
	rec := newCallRecorder("mw", &order)

	c, err := New(provider, WithMiddleware(MiddlewareConfig{Send: rec.sendMiddleware()}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, sendErr := c.SendMessage(context.Background(), "hello")
	if sendErr != nil {
		t.Fatalf("SendMessage: %v", sendErr)
	}

	if !rec.calledSend {
		t.Error("expected middleware to be called on SendMessage")
	}
}

// TestWithMiddleware_ContinueConversationCallsChain verifies that
// ContinueConversation also routes through the middleware chain.
func TestWithMiddleware_ContinueConversationCallsChain(t *testing.T) {
	provider := &mockProvider{}
	order := []string{}
	rec := newCallRecorder("mw", &order)

	memProvider := &mockMemoryProvider{}
	c, err := New(provider,
		WithMemory(memProvider),
		WithMiddleware(MiddlewareConfig{Send: rec.sendMiddleware()}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.ContinueConversation(context.Background())
	if err != nil {
		t.Fatalf("ContinueConversation: %v", err)
	}

	if !rec.calledSend {
		t.Error("expected middleware to be called on ContinueConversation")
	}
}

// TestWithMiddleware_StreamMessageCallsChain verifies that StreamMessage routes
// through the stream middleware chain.
func TestWithMiddleware_StreamMessageCallsChain(t *testing.T) {
	provider := &mockStreamProvider{}
	order := []string{}
	rec := newCallRecorder("mw", &order)

	c, err := New(provider, WithMiddleware(MiddlewareConfig{
		Send:   rec.sendMiddleware(),
		Stream: rec.streamMiddleware(),
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.StreamMessage(context.Background(), "hello")
	if err != nil {
		t.Fatalf("StreamMessage: %v", err)
	}

	if !rec.calledStream {
		t.Error("expected stream middleware to be called on StreamMessage")
	}
}

// TestWithMiddleware_StreamContinueConversationCallsChain verifies that
// StreamContinueConversation routes through the stream middleware chain.
func TestWithMiddleware_StreamContinueConversationCallsChain(t *testing.T) {
	provider := &mockStreamProvider{}
	order := []string{}
	rec := newCallRecorder("mw", &order)

	memProvider := &mockMemoryProvider{}
	c, err := New(provider,
		WithMemory(memProvider),
		WithMiddleware(MiddlewareConfig{
			Send:   rec.sendMiddleware(),
			Stream: rec.streamMiddleware(),
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.StreamContinueConversation(context.Background())
	if err != nil {
		t.Fatalf("StreamContinueConversation: %v", err)
	}

	if !rec.calledStream {
		t.Error("expected stream middleware to be called on StreamContinueConversation")
	}
}

// TestWithMiddleware_NoMiddleware verifies that the direct provider path works
// when no middleware is configured (sendChain == nil).
func TestWithMiddleware_NoMiddleware(t *testing.T) {
	provider := &mockProvider{}
	c, err := New(provider)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if c.sendChain != nil {
		t.Error("expected sendChain to be nil when no middleware configured")
	}

	if c.streamChain != nil {
		t.Error("expected streamChain to be nil when no middleware configured")
	}

	resp, err := c.SendMessage(context.Background(), "hello")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if resp.Content != "test response" {
		t.Errorf("expected 'test response', got %q", resp.Content)
	}
}

// TestWithMiddleware_OnlySendNoStreamChain verifies that streamChain is nil when
// all middleware entries have a nil Stream field.
func TestWithMiddleware_OnlySendNoStreamChain(t *testing.T) {
	provider := &mockProvider{}
	order := []string{}
	rec := newCallRecorder("mw", &order)

	c, err := New(provider, WithMiddleware(MiddlewareConfig{
		Send:   rec.sendMiddleware(),
		Stream: nil, // no stream component
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if c.sendChain == nil {
		t.Error("expected sendChain to be non-nil")
	}

	// streamChain should be nil because no Stream middleware was provided
	if c.streamChain != nil {
		t.Error("expected streamChain to be nil when no Stream middleware is provided")
	}
}

// ========== Mock memory provider (minimal) ==========

// mockMemoryProvider is a minimal in-memory implementation sufficient for the
// middleware tests that call ContinueConversation / StreamContinueConversation.
type mockMemoryProvider struct {
	messages []*ai.Message
}

func (m *mockMemoryProvider) AppendMessage(_ context.Context, msg *ai.Message) {
	if msg == nil {
		return
	}

	m.messages = append(m.messages, msg)
}

func (m *mockMemoryProvider) AllMessages(_ context.Context) ([]ai.Message, error) {
	result := make([]ai.Message, len(m.messages))
	for i, msg := range m.messages {
		result[i] = *msg
	}

	return result, nil
}

func (m *mockMemoryProvider) Count(_ context.Context) (int, error) {
	return len(m.messages), nil
}

func (m *mockMemoryProvider) ClearMessages(_ context.Context) {
	m.messages = nil
}

func (m *mockMemoryProvider) LastMessages(_ context.Context, n int) ([]ai.Message, error) {
	if n <= 0 {
		return []ai.Message{}, nil
	}

	start := len(m.messages) - n
	if start < 0 {
		start = 0
	}

	result := make([]ai.Message, 0, len(m.messages)-start)
	for _, msg := range m.messages[start:] {
		result = append(result, *msg)
	}

	return result, nil
}

func (m *mockMemoryProvider) PopLastMessage(_ context.Context) (*ai.Message, error) {
	if len(m.messages) == 0 {
		return nil, nil
	}

	last := m.messages[len(m.messages)-1]
	m.messages = m.messages[:len(m.messages)-1]

	return last, nil
}

func (m *mockMemoryProvider) FilterByRole(_ context.Context, role ai.MessageRole) ([]ai.Message, error) {
	var result []ai.Message

	for _, msg := range m.messages {
		if msg.Role == role {
			result = append(result, *msg)
		}
	}

	return result, nil
}

// ========== Nil Send validation tests ==========

// TestNew_NilSendField_ReturnsError verifies that New returns a descriptive
// error when a MiddlewareConfig has a nil Send field, rather than panicking
// later at call time.
func TestNew_NilSendField_ReturnsError(t *testing.T) {
	provider := &mockProvider{}

	_, err := New(provider, WithMiddleware(MiddlewareConfig{Send: nil}))
	if err == nil {
		t.Fatal("expected error for nil Send field, got nil")
	}

	expected := "middleware[0] has a nil Send field"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error to contain %q, got: %v", expected, err)
	}
}

// TestNew_NilSendField_ReportsCorrectIndex verifies that the error message
// reports the index of the offending middleware when multiple are registered.
func TestNew_NilSendField_ReportsCorrectIndex(t *testing.T) {
	provider := &mockProvider{}
	order := []string{}
	rec := newCallRecorder("mw", &order)

	_, err := New(provider, WithMiddleware(
		MiddlewareConfig{Send: rec.sendMiddleware()}, // index 0: valid
		MiddlewareConfig{Send: nil},                  // index 1: invalid
	))
	if err == nil {
		t.Fatal("expected error for nil Send field at index 1, got nil")
	}

	expected := "middleware[1] has a nil Send field"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error to contain %q, got: %v", expected, err)
	}
}

// containsString reports whether s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
