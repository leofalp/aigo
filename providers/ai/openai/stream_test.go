package openai

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

// writeSSEDone writes the [DONE] sentinel to signal end of stream.
func writeSSEDone(writer http.ResponseWriter) {
	fmt.Fprintf(writer, "data: [DONE]\n\n")
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

// TestStreamMessage_ContentStreaming verifies that content deltas are correctly
// streamed and can be collected into a complete response.
func TestStreamMessage_ContentStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		// Send content in multiple chunks
		writeSSE(writer, `{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`)
		writeSSE(writer, `{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`)
		writeSSE(writer, `{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}`)

		// Send usage in final chunk
		writeSSE(writer, `{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`)

		// Send finish reason
		writeSSE(writer, `{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)

		writeSSEDone(writer)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "gpt-4",
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
	if response.Usage.PromptTokens != 10 {
		t.Errorf("expected 10 prompt tokens, got %d", response.Usage.PromptTokens)
	}
	if response.Usage.CompletionTokens != 3 {
		t.Errorf("expected 3 completion tokens, got %d", response.Usage.CompletionTokens)
	}
}

// TestStreamMessage_ToolCallStreaming verifies that incremental tool call deltas
// are correctly accumulated into complete tool calls.
func TestStreamMessage_ToolCallStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		// First chunk: tool call start with ID and name
		writeSSE(writer, `{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_abc123","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`)

		// Subsequent chunks: argument fragments
		writeSSE(writer, `{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]},"finish_reason":null}]}`)
		writeSSE(writer, `{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"London\"}"}}]},"finish_reason":null}]}`)

		// Finish
		writeSSE(writer, `{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`)
		writeSSEDone(writer)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "gpt-4",
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
	if toolCall.ID != "call_abc123" {
		t.Errorf("expected tool call ID 'call_abc123', got '%s'", toolCall.ID)
	}
	if toolCall.Function.Name != "get_weather" {
		t.Errorf("expected function name 'get_weather', got '%s'", toolCall.Function.Name)
	}
	expectedArgs := `{"city":"London"}`
	if toolCall.Function.Arguments != expectedArgs {
		t.Errorf("expected arguments '%s', got '%s'", expectedArgs, toolCall.Function.Arguments)
	}
	if response.FinishReason != "tool_calls" {
		t.Errorf("expected finish_reason 'tool_calls', got '%s'", response.FinishReason)
	}
}

// TestStreamMessage_ErrorMidStream verifies that a malformed SSE payload
// mid-stream is correctly propagated as an error through Collect.
func TestStreamMessage_ErrorMidStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		// Valid first chunk
		writeSSE(writer, `{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"Start"},"finish_reason":null}]}`)

		// Malformed JSON chunk
		writeSSE(writer, `{invalid json}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "gpt-4",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned error: %v", err)
	}

	response, err := stream.Collect()
	if err == nil {
		t.Fatal("expected error from Collect, got nil")
	}

	// The partial content from before the error should be accumulated
	if response.Content != "Start" {
		t.Errorf("expected partial content 'Start', got '%s'", response.Content)
	}

	if !strings.Contains(err.Error(), "failed to parse streaming chunk") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

// TestStreamMessage_ContextCancellation verifies that cancelling the context
// terminates the stream with a context error.
func TestStreamMessage_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		// Send one chunk then hang (simulate slow server)
		writeSSE(writer, `{"id":"chatcmpl-4","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`)

		// Block until request is cancelled
		<-request.Context().Done()
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	ctx, cancel := context.WithCancel(context.Background())

	stream, err := provider.StreamMessage(ctx, ai.ChatRequest{
		Model:    "gpt-4",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned error: %v", err)
	}

	// Read one event, then cancel
	eventCount := 0
	for event, iterErr := range stream.Iter() {
		if iterErr != nil {
			// Expected: context cancellation error
			break
		}
		eventCount++
		if event.Type == ai.StreamEventContent {
			cancel() // Cancel after receiving first content
		}
	}

	if eventCount == 0 {
		t.Error("expected at least one event before cancellation")
	}
}

// TestStreamMessage_RangeIteration verifies that the stream works correctly
// with a for-range loop, collecting tokens one at a time.
func TestStreamMessage_RangeIteration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)

		writeSSE(writer, `{"id":"chatcmpl-5","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"A"},"finish_reason":null}]}`)
		writeSSE(writer, `{"id":"chatcmpl-5","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"B"},"finish_reason":null}]}`)
		writeSSE(writer, `{"id":"chatcmpl-5","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"C"},"finish_reason":null}]}`)
		writeSSE(writer, `{"id":"chatcmpl-5","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
		writeSSEDone(writer)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("test-key")

	stream, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "gpt-4",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("StreamMessage returned error: %v", err)
	}

	var contentTokens []string
	var finishReason string

	for event, iterErr := range stream.Iter() {
		if iterErr != nil {
			t.Fatalf("unexpected error: %v", iterErr)
		}
		switch event.Type {
		case ai.StreamEventContent:
			contentTokens = append(contentTokens, event.Content)
		case ai.StreamEventDone:
			finishReason = event.FinishReason
		}
	}

	if len(contentTokens) != 3 {
		t.Errorf("expected 3 content tokens, got %d", len(contentTokens))
	}
	if strings.Join(contentTokens, "") != "ABC" {
		t.Errorf("expected 'ABC', got '%s'", strings.Join(contentTokens, ""))
	}
	if finishReason != "stop" {
		t.Errorf("expected finish_reason 'stop', got '%s'", finishReason)
	}
}

// TestStreamMessage_PreStreamError verifies that pre-stream errors (e.g., HTTP errors)
// are returned from StreamMessage directly (not through the iterator).
func TestStreamMessage_PreStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, `{"error":{"message":"Invalid API key"}}`)
	}))
	defer server.Close()

	provider := New()
	provider.WithBaseURL(server.URL)
	provider.WithAPIKey("bad-key")

	_, err := provider.StreamMessage(context.Background(), ai.ChatRequest{
		Model:    "gpt-4",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error, got: %v", err)
	}
}

// TestUnmarshalStreamChunk verifies that raw SSE JSON payloads are correctly
// decoded into chatCompletionStreamChunk structs, covering content deltas,
// reasoning deltas, tool-call deltas, usage-only chunks, finish-reason chunks,
// and malformed JSON.
func TestUnmarshalStreamChunk(t *testing.T) {
	contentStr := "Hello"
	emptyStr := ""

	testCases := []struct {
		name        string
		data        string
		wantErr     bool
		checkResult func(t *testing.T, chunk *chatCompletionStreamChunk)
	}{
		{
			name:    "content delta chunk",
			data:    `{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
			wantErr: false,
			checkResult: func(t *testing.T, chunk *chatCompletionStreamChunk) {
				if chunk.ID != "chatcmpl-1" {
					t.Errorf("ID = %q, want %q", chunk.ID, "chatcmpl-1")
				}
				if len(chunk.Choices) != 1 {
					t.Fatalf("len(Choices) = %d, want 1", len(chunk.Choices))
				}
				if chunk.Choices[0].Delta.Content == nil || *chunk.Choices[0].Delta.Content != contentStr {
					t.Errorf("Choices[0].Delta.Content = %v, want %q", chunk.Choices[0].Delta.Content, contentStr)
				}
				if chunk.Choices[0].FinishReason != nil {
					t.Errorf("FinishReason = %v, want nil", chunk.Choices[0].FinishReason)
				}
			},
		},
		{
			name:    "reasoning delta chunk",
			data:    `{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1700000000,"model":"o1","choices":[{"index":0,"delta":{"reasoning":"step 1"},"finish_reason":null}]}`,
			wantErr: false,
			checkResult: func(t *testing.T, chunk *chatCompletionStreamChunk) {
				if len(chunk.Choices) != 1 {
					t.Fatalf("len(Choices) = %d, want 1", len(chunk.Choices))
				}
				if chunk.Choices[0].Delta.Reasoning == nil || *chunk.Choices[0].Delta.Reasoning != "step 1" {
					t.Errorf("Reasoning = %v, want %q", chunk.Choices[0].Delta.Reasoning, "step 1")
				}
			},
		},
		{
			name:    "tool call delta chunk",
			data:    `{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"Calculator","arguments":""}}]},"finish_reason":null}]}`,
			wantErr: false,
			checkResult: func(t *testing.T, chunk *chatCompletionStreamChunk) {
				if len(chunk.Choices) != 1 {
					t.Fatalf("len(Choices) = %d, want 1", len(chunk.Choices))
				}
				toolCalls := chunk.Choices[0].Delta.ToolCalls
				if len(toolCalls) != 1 {
					t.Fatalf("len(ToolCalls) = %d, want 1", len(toolCalls))
				}
				if toolCalls[0].ID != "call_abc" {
					t.Errorf("ToolCall.ID = %q, want %q", toolCalls[0].ID, "call_abc")
				}
				if toolCalls[0].Function.Name != "Calculator" {
					t.Errorf("ToolCall.Function.Name = %q, want %q", toolCalls[0].Function.Name, "Calculator")
				}
			},
		},
		{
			name:    "usage-only final chunk",
			data:    `{"id":"chatcmpl-4","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`,
			wantErr: false,
			checkResult: func(t *testing.T, chunk *chatCompletionStreamChunk) {
				if chunk.Usage == nil {
					t.Fatal("Usage is nil, want non-nil")
				}
				if chunk.Usage.PromptTokens != 10 {
					t.Errorf("Usage.PromptTokens = %d, want 10", chunk.Usage.PromptTokens)
				}
				if chunk.Usage.CompletionTokens != 20 {
					t.Errorf("Usage.CompletionTokens = %d, want 20", chunk.Usage.CompletionTokens)
				}
			},
		},
		{
			name:    "finish reason stop chunk",
			data:    `{"id":"chatcmpl-5","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}]}`,
			wantErr: false,
			checkResult: func(t *testing.T, chunk *chatCompletionStreamChunk) {
				if len(chunk.Choices) != 1 {
					t.Fatalf("len(Choices) = %d, want 1", len(chunk.Choices))
				}
				if chunk.Choices[0].FinishReason == nil || *chunk.Choices[0].FinishReason != "stop" {
					t.Errorf("FinishReason = %v, want %q", chunk.Choices[0].FinishReason, "stop")
				}
				// Empty string content should be present (not nil)
				if chunk.Choices[0].Delta.Content == nil || *chunk.Choices[0].Delta.Content != emptyStr {
					t.Errorf("Delta.Content = %v, want empty string pointer", chunk.Choices[0].Delta.Content)
				}
			},
		},
		{
			name:    "malformed JSON returns error",
			data:    `{"id": "broken", "choices": [`,
			wantErr: true,
			checkResult: func(t *testing.T, chunk *chatCompletionStreamChunk) {
				// chunk should be nil on error; nothing to check
			},
		},
		{
			name:    "empty JSON object",
			data:    `{}`,
			wantErr: false,
			checkResult: func(t *testing.T, chunk *chatCompletionStreamChunk) {
				if chunk.ID != "" {
					t.Errorf("ID = %q, want empty", chunk.ID)
				}
				if chunk.Usage != nil {
					t.Errorf("Usage = %v, want nil", chunk.Usage)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			chunk, err := unmarshalStreamChunk(testCase.data)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("unmarshalStreamChunk() error = %v, wantErr = %v", err, testCase.wantErr)
			}
			if !testCase.wantErr {
				testCase.checkResult(t, chunk)
			}
		})
	}
}
