package client

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
)

// mockStreamProvider is a mock that implements both ai.Provider and ai.StreamProvider.
// It embeds mockProvider so all non-streaming methods are inherited; the
// streamMessageFunc field allows individual tests to control streaming behaviour.
type mockStreamProvider struct {
	mockProvider
	streamMessageFunc func(ctx context.Context, req ai.ChatRequest) (*ai.ChatStream, error)
}

// StreamMessage implements ai.StreamProvider. If streamMessageFunc is set it
// delegates to it; otherwise it returns a single-event stream wrapping the
// default mockProvider response.
func (m *mockStreamProvider) StreamMessage(ctx context.Context, req ai.ChatRequest) (*ai.ChatStream, error) {
	if m.streamMessageFunc != nil {
		return m.streamMessageFunc(ctx, req)
	}
	response := &ai.ChatResponse{
		Content:      "streamed response",
		FinishReason: "stop",
	}
	return ai.NewSingleEventStream(response), nil
}

// makeClientStream is a test helper that creates a ChatStream yielding the
// provided content string as a single content event followed by a done event.
func makeClientStream(content string) *ai.ChatStream {
	events := []ai.StreamEvent{
		{Type: ai.StreamEventContent, Content: content},
		{Type: ai.StreamEventDone, FinishReason: "stop"},
	}
	iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
		for _, event := range events {
			if !yield(event, nil) {
				return
			}
		}
	}
	return ai.NewChatStream(iter.Seq2[ai.StreamEvent, error](iteratorFunc))
}

// collectStream drains a ChatStream and returns all events, failing the test on
// any mid-stream error.
func collectStream(t *testing.T, stream *ai.ChatStream) []ai.StreamEvent {
	t.Helper()
	var events []ai.StreamEvent
	for event, err := range stream.Iter() {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
		events = append(events, event)
	}
	return events
}

// ========== StreamMessage Tests ==========

// TestStreamMessage_WithStreamProvider verifies that when the provider implements
// ai.StreamProvider the client delegates to StreamMessage rather than SendMessage.
func TestStreamMessage_WithStreamProvider(t *testing.T) {
	nativeStreamCalled := false
	provider := &mockStreamProvider{
		streamMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatStream, error) {
			nativeStreamCalled = true
			return makeClientStream("native stream"), nil
		},
	}

	client, err := New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	stream, err := client.StreamMessage(context.Background(), "hello")
	if err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}

	events := collectStream(t, stream)

	if !nativeStreamCalled {
		t.Error("expected native StreamMessage to be called")
	}
	// The helper emits a content event + done event.
	if len(events) < 1 {
		t.Fatalf("expected at least 1 event, got %d", len(events))
	}
}

// TestStreamMessage_FallbackToSync verifies that when the provider only implements
// ai.Provider (no streaming), the client falls back to a single-event stream.
func TestStreamMessage_FallbackToSync(t *testing.T) {
	provider := &mockProvider{} // Does NOT implement StreamProvider.

	client, err := New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	stream, err := client.StreamMessage(context.Background(), "hello")
	if err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}

	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	// The default mockProvider returns "test response".
	if response.Content != "test response" {
		t.Errorf("expected fallback content %q, got %q", "test response", response.Content)
	}
}

// TestStreamMessage_EmptyPrompt verifies that an empty prompt is rejected before
// the provider is ever called.
func TestStreamMessage_EmptyPrompt(t *testing.T) {
	provider := &mockProvider{}

	client, err := New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_, err = client.StreamMessage(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty prompt, got nil")
	}
	if !strings.Contains(err.Error(), "prompt cannot be empty") {
		t.Errorf("expected 'prompt cannot be empty' in error, got: %v", err)
	}
}

// TestStreamMessage_WithMemory verifies that when a memory provider is configured,
// the user message is appended before the stream request is sent.
func TestStreamMessage_WithMemory(t *testing.T) {
	var capturedRequest ai.ChatRequest
	provider := &mockStreamProvider{
		streamMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatStream, error) {
			capturedRequest = req
			return makeClientStream("ok"), nil
		},
	}

	memoryProvider := inmemory.New()
	client, err := New(provider, WithMemory(memoryProvider))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_, err = client.StreamMessage(context.Background(), "first message")
	if err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}

	if len(capturedRequest.Messages) != 1 {
		t.Fatalf("expected 1 message in request, got %d", len(capturedRequest.Messages))
	}
	if capturedRequest.Messages[0].Content != "first message" {
		t.Errorf("expected message content %q, got %q", "first message", capturedRequest.Messages[0].Content)
	}
	// The memory provider should have the user message persisted.
	count, countErr := memoryProvider.Count(context.Background())
	if countErr != nil {
		t.Fatalf("Count returned unexpected error: %v", countErr)
	}
	if count != 1 {
		t.Errorf("expected 1 message in memory, got %d", count)
	}
}

// TestStreamMessage_WithSystemPrompt verifies that a per-request system prompt
// overrides the client-level system prompt in the outgoing request.
func TestStreamMessage_WithSystemPrompt(t *testing.T) {
	var capturedRequest ai.ChatRequest
	provider := &mockStreamProvider{
		streamMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatStream, error) {
			capturedRequest = req
			return makeClientStream("ok"), nil
		},
	}

	client, err := New(provider, WithSystemPrompt("default prompt"))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_, err = client.StreamMessage(
		context.Background(),
		"hello",
		WithEphemeralSystemPrompt("ephemeral prompt"),
	)
	if err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}

	if capturedRequest.SystemPrompt != "ephemeral prompt" {
		t.Errorf("expected system prompt %q, got %q", "ephemeral prompt", capturedRequest.SystemPrompt)
	}
}

// ========== StreamContinueConversation Tests ==========

// TestStreamContinueConversation_NoMemory verifies that calling
// StreamContinueConversation without a memory provider returns an error.
func TestStreamContinueConversation_NoMemory(t *testing.T) {
	provider := &mockProvider{}

	client, err := New(provider) // No memory provider.
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_, err = client.StreamContinueConversation(context.Background())
	if err == nil {
		t.Fatal("expected error when no memory provider is configured, got nil")
	}
	if !strings.Contains(err.Error(), "StreamContinueConversation requires a memory provider") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestStreamContinueConversation_WithStreamProvider verifies that when the
// provider implements ai.StreamProvider, StreamContinueConversation uses it.
func TestStreamContinueConversation_WithStreamProvider(t *testing.T) {
	nativeStreamCalled := false
	provider := &mockStreamProvider{
		streamMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatStream, error) {
			nativeStreamCalled = true
			return makeClientStream("continued"), nil
		},
	}

	memoryProvider := inmemory.New()
	ctx := context.Background()
	memoryProvider.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "initial"})

	client, err := New(provider, WithMemory(memoryProvider))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	stream, err := client.StreamContinueConversation(ctx)
	if err != nil {
		t.Fatalf("StreamContinueConversation failed: %v", err)
	}

	// Drain the stream to verify it is readable.
	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if !nativeStreamCalled {
		t.Error("expected native StreamMessage to be called")
	}
	if response.Content != "continued" {
		t.Errorf("expected content %q, got %q", "continued", response.Content)
	}
}

// TestStreamContinueConversation_FallbackToSync verifies that when the provider
// does not implement ai.StreamProvider, the method falls back to a single-event
// stream wrapping the synchronous response.
func TestStreamContinueConversation_FallbackToSync(t *testing.T) {
	provider := &mockProvider{} // Does NOT implement StreamProvider.

	memoryProvider := inmemory.New()
	ctx := context.Background()
	memoryProvider.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "initial"})

	client, err := New(provider, WithMemory(memoryProvider))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	stream, err := client.StreamContinueConversation(ctx)
	if err != nil {
		t.Fatalf("StreamContinueConversation failed: %v", err)
	}

	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	// Default mockProvider returns "test response".
	if response.Content != "test response" {
		t.Errorf("expected fallback content %q, got %q", "test response", response.Content)
	}
}

// TestStreamMessage_ProviderError verifies that a pre-stream provider error is
// propagated as a normal error (not through the iterator).
func TestStreamMessage_ProviderError(t *testing.T) {
	providerErr := errors.New("provider stream error")
	provider := &mockStreamProvider{
		streamMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatStream, error) {
			return nil, providerErr
		},
	}

	client, err := New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_, err = client.StreamMessage(context.Background(), "hello")
	if !errors.Is(err, providerErr) {
		t.Errorf("expected provider error, got %v", err)
	}
}
