package gemini

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leofalp/aigo/providers/ai"
)

// writeSSE is a test helper that writes an SSE data line to the response writer and flushes.
func writeSSE(writer http.ResponseWriter, data string) {
	fmt.Fprintf(writer, "data: %s\n\n", data)
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

// TestGeminiStreamMessage_ContentStreaming verifies that Gemini's cumulative text responses
// are correctly converted to incremental deltas and can be collected.
func TestGeminiStreamMessage_ContentStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		// Gemini sends cumulative text in each chunk (not deltas)
		writeSSE(writer, `{"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"}}]}`)
		writeSSE(writer, `{"candidates":[{"content":{"parts":[{"text":"Hello world"}],"role":"model"}}]}`)
		writeSSE(writer, `{"candidates":[{"content":{"parts":[{"text":"Hello world!"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.5-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned error: %v", err)
	}

	// Collect all events
	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if response.Content != "Hello world!" {
		t.Errorf("expected content 'Hello world!', got '%s'", response.Content)
	}

	if response.FinishReason != "stop" {
		t.Errorf("expected finish_reason 'stop', got '%s'", response.FinishReason)
	}

	if response.Usage == nil {
		t.Fatal("expected usage to be present")
	}
	if response.Usage.PromptTokens != 5 {
		t.Errorf("expected 5 prompt tokens, got %d", response.Usage.PromptTokens)
	}
	if response.Usage.CompletionTokens != 3 {
		t.Errorf("expected 3 completion tokens, got %d", response.Usage.CompletionTokens)
	}
}

// TestGeminiStreamMessage_FunctionCall verifies that function calls from streaming
// responses are correctly extracted as tool call events.
func TestGeminiStreamMessage_FunctionCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		// Gemini sends function calls as complete parts (not incremental)
		writeSSE(writer, `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"get_weather","args":{"city":"London"}}}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.5-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "What's the weather?"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned error: %v", err)
	}

	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if len(response.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(response.ToolCalls))
	}

	toolCall := response.ToolCalls[0]
	if toolCall.Function.Name != "get_weather" {
		t.Errorf("expected function name 'get_weather', got '%s'", toolCall.Function.Name)
	}
	if !strings.Contains(toolCall.Function.Arguments, "London") {
		t.Errorf("expected arguments to contain 'London', got '%s'", toolCall.Function.Arguments)
	}
}

// TestGeminiStreamMessage_UsageMetadata verifies that usage metadata from the
// final streaming chunk is correctly captured.
func TestGeminiStreamMessage_UsageMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		// First chunk without usage
		writeSSE(writer, `{"candidates":[{"content":{"parts":[{"text":"Result"}],"role":"model"}}]}`)
		// Final chunk with usage and finish reason
		writeSSE(writer, `{"candidates":[{"content":{"parts":[{"text":"Result"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":10,"totalTokenCount":30,"thoughtsTokenCount":5,"cachedContentTokenCount":8}}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.5-pro",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Think about this"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned error: %v", err)
	}

	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if response.Usage == nil {
		t.Fatal("expected usage to be present")
	}
	if response.Usage.PromptTokens != 20 {
		t.Errorf("expected 20 prompt tokens, got %d", response.Usage.PromptTokens)
	}
	if response.Usage.CompletionTokens != 10 {
		t.Errorf("expected 10 completion tokens, got %d", response.Usage.CompletionTokens)
	}
	if response.Usage.TotalTokens != 30 {
		t.Errorf("expected 30 total tokens, got %d", response.Usage.TotalTokens)
	}
	if response.Usage.ReasoningTokens != 5 {
		t.Errorf("expected 5 reasoning tokens, got %d", response.Usage.ReasoningTokens)
	}
	if response.Usage.CachedTokens != 8 {
		t.Errorf("expected 8 cached tokens, got %d", response.Usage.CachedTokens)
	}
}

// TestGeminiStreamMessage_RangeIteration verifies that the stream works correctly
// with a for-range loop, yielding incremental deltas from Gemini's cumulative format.
func TestGeminiStreamMessage_RangeIteration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		writeSSE(writer, `{"candidates":[{"content":{"parts":[{"text":"A"}],"role":"model"}}]}`)
		writeSSE(writer, `{"candidates":[{"content":{"parts":[{"text":"AB"}],"role":"model"}}]}`)
		writeSSE(writer, `{"candidates":[{"content":{"parts":[{"text":"ABC"}],"role":"model"},"finishReason":"STOP"}]}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.5-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Count"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned error: %v", err)
	}

	var contentDeltas []string
	for event, iterErr := range stream.Iter() {
		if iterErr != nil {
			t.Fatalf("unexpected error: %v", iterErr)
		}
		if event.Type == ai.StreamEventContent {
			contentDeltas = append(contentDeltas, event.Content)
		}
	}

	// Each chunk should yield only the new portion
	if len(contentDeltas) != 3 {
		t.Fatalf("expected 3 content deltas, got %d: %v", len(contentDeltas), contentDeltas)
	}
	if contentDeltas[0] != "A" {
		t.Errorf("expected first delta 'A', got '%s'", contentDeltas[0])
	}
	if contentDeltas[1] != "B" {
		t.Errorf("expected second delta 'B', got '%s'", contentDeltas[1])
	}
	if contentDeltas[2] != "C" {
		t.Errorf("expected third delta 'C', got '%s'", contentDeltas[2])
	}
}

// TestGeminiStreamMessage_PreStreamError verifies that pre-stream HTTP errors
// are returned directly from StreamMessage.
func TestGeminiStreamMessage_PreStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		fmt.Fprint(writer, `{"error":{"message":"API key invalid"}}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("bad-key")

	_, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.5-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 error, got: %v", err)
	}
}

// TestGeminiStreamMessage_ContextCancellation verifies that cancelling the context
// terminates the Gemini stream.
func TestGeminiStreamMessage_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		writeSSE(writer, `{"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"}}]}`)

		// Block until cancelled
		<-request.Context().Done()
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	ctx, cancel := context.WithCancel(context.Background())

	stream, err := provider.StreamMessage(ctx, ai.ChatRequest{
		Model:    "gemini-2.5-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned error: %v", err)
	}

	eventCount := 0
	for event, iterErr := range stream.Iter() {
		if iterErr != nil {
			break // Expected: context cancellation
		}
		eventCount++
		if event.Type == ai.StreamEventContent {
			cancel()
		}
	}

	if eventCount == 0 {
		t.Error("expected at least one event before cancellation")
	}
}
