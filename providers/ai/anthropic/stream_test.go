package anthropic

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leofalp/aigo/providers/ai"
)

// writeSSE is a test helper that writes a typed SSE event to the response writer
// and flushes the buffer so the client receives it immediately.
// Anthropic's SSE protocol uses "event:" lines as discriminators; the data
// payload contains a JSON object with a redundant "type" field so that our
// unmarshalStreamEvent function can work from the "data:" line alone.
func writeSSE(writer http.ResponseWriter, eventType, data string) {
	fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", eventType, data)
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

// TestStreamMessage_ContentStreaming verifies that a standard text response is
// streamed correctly: content deltas accumulate, usage is reported with the
// right token counts, and the done event carries the mapped finish reason.
func TestStreamMessage_ContentStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		writeSSE(writer, "message_start",
			`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"usage":{"input_tokens":25,"output_tokens":0}}}`)

		writeSSE(writer, "content_block_start",
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)

		writeSSE(writer, "content_block_delta",
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`)

		writeSSE(writer, "content_block_delta",
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`)

		writeSSE(writer, "content_block_stop",
			`{"type":"content_block_stop","index":0}`)

		writeSSE(writer, "message_delta",
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`)

		writeSSE(writer, "message_stop",
			`{"type":"message_stop"}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned unexpected error: %v", err)
	}

	// Collect all events to simplify assertion logic.
	var contentEvents []ai.StreamEvent
	var usageEvent *ai.StreamEvent
	var doneEvent *ai.StreamEvent

	for event, iterErr := range stream.Iter() {
		if iterErr != nil {
			t.Fatalf("stream iterator returned unexpected error: %v", iterErr)
		}
		switch event.Type {
		case ai.StreamEventContent:
			copied := event
			contentEvents = append(contentEvents, copied)
		case ai.StreamEventUsage:
			copied := event
			usageEvent = &copied
		case ai.StreamEventDone:
			copied := event
			doneEvent = &copied
		}
	}

	// --- content assertions ---
	if len(contentEvents) != 2 {
		t.Fatalf("expected 2 content events, got %d", len(contentEvents))
	}
	if contentEvents[0].Content != "Hello" {
		t.Errorf("first content event: got %q, want %q", contentEvents[0].Content, "Hello")
	}
	if contentEvents[1].Content != " world" {
		t.Errorf("second content event: got %q, want %q", contentEvents[1].Content, " world")
	}

	// --- usage assertions ---
	if usageEvent == nil {
		t.Fatal("expected a usage event, got none")
	}
	if usageEvent.Usage == nil {
		t.Fatal("usage event has nil Usage")
	}
	if usageEvent.Usage.PromptTokens != 25 {
		t.Errorf("PromptTokens: got %d, want 25", usageEvent.Usage.PromptTokens)
	}
	if usageEvent.Usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens: got %d, want 5", usageEvent.Usage.CompletionTokens)
	}

	// --- done assertions ---
	if doneEvent == nil {
		t.Fatal("expected a done event, got none")
	}
	if doneEvent.FinishReason != "stop" {
		t.Errorf("FinishReason: got %q, want %q", doneEvent.FinishReason, "stop")
	}
}

// TestStreamMessage_ToolCallStreaming verifies that a tool-use response is
// correctly streamed: the initial content_block_start emits a tool call header
// (with ID and Name), the input_json_delta events emit argument chunks, and
// the done event carries the mapped "tool_calls" finish reason.
func TestStreamMessage_ToolCallStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		writeSSE(writer, "message_start",
			`{"type":"message_start","message":{"id":"msg_2","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"usage":{"input_tokens":30,"output_tokens":0}}}`)

		// content_block_start for a tool_use block — carries ID and Name.
		writeSSE(writer, "content_block_start",
			`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_1","name":"get_weather","input":{}}}`)

		// Two input_json_delta events that together form a complete JSON argument.
		writeSSE(writer, "content_block_delta",
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}`)

		writeSSE(writer, "content_block_delta",
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"NYC\"}"}}`)

		writeSSE(writer, "content_block_stop",
			`{"type":"content_block_stop","index":0}`)

		writeSSE(writer, "message_delta",
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":20}}`)

		writeSSE(writer, "message_stop",
			`{"type":"message_stop"}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "What's the weather?"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned unexpected error: %v", err)
	}

	var toolCallEvents []ai.StreamEvent
	var doneEvent *ai.StreamEvent

	for event, iterErr := range stream.Iter() {
		if iterErr != nil {
			t.Fatalf("stream iterator returned unexpected error: %v", iterErr)
		}
		switch event.Type {
		case ai.StreamEventToolCall:
			copied := event
			toolCallEvents = append(toolCallEvents, copied)
		case ai.StreamEventDone:
			copied := event
			doneEvent = &copied
		}
	}

	// Expect: 1 header event (from content_block_start) + 2 argument-chunk events.
	if len(toolCallEvents) != 3 {
		t.Fatalf("expected 3 tool call events, got %d", len(toolCallEvents))
	}

	// First event: header with ID and Name, no arguments yet.
	firstEvent := toolCallEvents[0]
	if firstEvent.ToolCall == nil {
		t.Fatal("first tool call event has nil ToolCall")
	}
	if firstEvent.ToolCall.Index != 0 {
		t.Errorf("first tool call Index: got %d, want 0", firstEvent.ToolCall.Index)
	}
	if firstEvent.ToolCall.ID != "call_1" {
		t.Errorf("first tool call ID: got %q, want %q", firstEvent.ToolCall.ID, "call_1")
	}
	if firstEvent.ToolCall.Name != "get_weather" {
		t.Errorf("first tool call Name: got %q, want %q", firstEvent.ToolCall.Name, "get_weather")
	}

	// Second and third events: argument chunks, same index, no ID/Name.
	for i, argEvent := range toolCallEvents[1:] {
		if argEvent.ToolCall == nil {
			t.Fatalf("argument event %d has nil ToolCall", i+1)
		}
		if argEvent.ToolCall.Index != 0 {
			t.Errorf("argument event %d Index: got %d, want 0", i+1, argEvent.ToolCall.Index)
		}
		if argEvent.ToolCall.Arguments == "" {
			t.Errorf("argument event %d has empty Arguments", i+1)
		}
	}

	if doneEvent == nil {
		t.Fatal("expected a done event, got none")
	}
	if doneEvent.FinishReason != "tool_calls" {
		t.Errorf("FinishReason: got %q, want %q", doneEvent.FinishReason, "tool_calls")
	}
}

// TestStreamMessage_ThinkingStreaming verifies that extended thinking blocks
// generate StreamEventReasoning events and are followed by normal text content
// in a single response.
func TestStreamMessage_ThinkingStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		writeSSE(writer, "message_start",
			`{"type":"message_start","message":{"id":"msg_3","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"usage":{"input_tokens":15,"output_tokens":0}}}`)

		// Thinking block
		writeSSE(writer, "content_block_start",
			`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`)

		writeSSE(writer, "content_block_delta",
			`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me think..."}}`)

		writeSSE(writer, "content_block_stop",
			`{"type":"content_block_stop","index":0}`)

		// Text block follows thinking
		writeSSE(writer, "content_block_start",
			`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`)

		writeSSE(writer, "content_block_delta",
			`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"The answer is 42."}}`)

		writeSSE(writer, "content_block_stop",
			`{"type":"content_block_stop","index":1}`)

		writeSSE(writer, "message_delta",
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":12}}`)

		writeSSE(writer, "message_stop",
			`{"type":"message_stop"}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "What is 6*7?"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned unexpected error: %v", err)
	}

	var reasoningEvents []string
	var contentEvents []string

	for event, iterErr := range stream.Iter() {
		if iterErr != nil {
			t.Fatalf("stream iterator returned unexpected error: %v", iterErr)
		}
		switch event.Type {
		case ai.StreamEventReasoning:
			reasoningEvents = append(reasoningEvents, event.Reasoning)
		case ai.StreamEventContent:
			contentEvents = append(contentEvents, event.Content)
		}
	}

	if len(reasoningEvents) == 0 {
		t.Fatal("expected at least one StreamEventReasoning, got none")
	}
	if strings.Join(reasoningEvents, "") == "" {
		t.Error("reasoning events produced empty reasoning string")
	}

	if len(contentEvents) == 0 {
		t.Fatal("expected at least one StreamEventContent, got none")
	}
	if strings.Join(contentEvents, "") != "The answer is 42." {
		t.Errorf("content: got %q, want %q", strings.Join(contentEvents, ""), "The answer is 42.")
	}
}

// TestStreamMessage_ErrorMidStream verifies that an Anthropic "error" event
// received mid-stream is propagated as an error through the iterator.
func TestStreamMessage_ErrorMidStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		writeSSE(writer, "message_start",
			`{"type":"message_start","message":{"id":"msg_4","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}`)

		// Server sends an error event instead of continuing the stream.
		writeSSE(writer, "error",
			`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned unexpected pre-stream error: %v", err)
	}

	// Iterate manually so we can capture the mid-stream error.
	var iterErr error
	for _, iterErr = range stream.Iter() {
		// Keep iterating; the last value of iterErr will hold the error.
	}

	if iterErr == nil {
		t.Fatal("expected a mid-stream error, got nil")
	}
	if !strings.Contains(iterErr.Error(), "Overloaded") {
		t.Errorf("error message should contain %q, got: %v", "Overloaded", iterErr)
	}
}

// TestStreamMessage_PreStreamError verifies that a non-2xx HTTP response causes
// StreamMessage itself to return an error, with no ChatStream created.
func TestStreamMessage_PreStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(writer, `{"type":"error","error":{"type":"rate_limit_error","message":"Rate limited"}}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	_, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for non-2xx response, got nil")
	}
	// DoPostStream formats non-2xx as "non-2xx status NNN: ..."
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("expected error to contain status code 429, got: %v", err)
	}
}

// TestStreamMessage_NoAPIKey verifies that StreamMessage returns an error
// immediately when no API key has been configured, without making a network call.
func TestStreamMessage_NoAPIKey(t *testing.T) {
	// Use a server that would succeed to confirm the error is local only.
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Error("server should not have been called when API key is missing")
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	// Explicitly clear the API key — New() may have read a real key from the
	// environment when running alongside integration tests.
	provider.apiKey = ""

	_, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY is not set") {
		t.Errorf("expected API key error, got: %v", err)
	}
}
