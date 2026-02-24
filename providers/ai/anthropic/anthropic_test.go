package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leofalp/aigo/providers/ai"
)

// TestNew verifies that New() returns a non-nil provider with the default base URL.
func TestNew(t *testing.T) {
	provider := New()
	if provider == nil {
		t.Fatal("New() returned nil")
	}
	if provider.baseURL != defaultBaseURL {
		t.Errorf("expected baseURL %q, got %q", defaultBaseURL, provider.baseURL)
	}
}

// TestWithAPIKey verifies that WithAPIKey sets the apiKey field and chains correctly.
func TestWithAPIKey(t *testing.T) {
	provider := New().WithAPIKey("test-api-key").(*AnthropicProvider)
	if provider.apiKey != "test-api-key" {
		t.Errorf("expected apiKey %q, got %q", "test-api-key", provider.apiKey)
	}
}

// TestWithBaseURL verifies that WithBaseURL sets the baseURL field.
func TestWithBaseURL(t *testing.T) {
	provider := New().WithBaseURL("https://custom.anthropic.com").(*AnthropicProvider)
	if provider.baseURL != "https://custom.anthropic.com" {
		t.Errorf("expected baseURL %q, got %q", "https://custom.anthropic.com", provider.baseURL)
	}
}

// TestWithHttpClient verifies that WithHttpClient sets a custom HTTP client.
func TestWithHttpClient(t *testing.T) {
	customClient := &http.Client{}
	provider := New().WithHttpClient(customClient).(*AnthropicProvider)
	if provider.client != customClient {
		t.Error("expected custom HTTP client to be set")
	}
}

// TestWithCapabilities verifies that WithCapabilities stores the capabilities and
// GetCapabilities returns them unchanged.
func TestWithCapabilities(t *testing.T) {
	caps := Capabilities{
		ExtendedThinking: true,
		Vision:           true,
		Effort:           "high",
	}
	provider := New().WithCapabilities(caps)

	got := provider.GetCapabilities()
	if got.ExtendedThinking != caps.ExtendedThinking {
		t.Errorf("expected ExtendedThinking %v, got %v", caps.ExtendedThinking, got.ExtendedThinking)
	}
	if got.Vision != caps.Vision {
		t.Errorf("expected Vision %v, got %v", caps.Vision, got.Vision)
	}
	if got.Effort != caps.Effort {
		t.Errorf("expected Effort %q, got %q", caps.Effort, got.Effort)
	}
}

// TestSendMessage_Basic exercises the happy path: correct headers are sent,
// the request body includes messages, and the response is properly decoded.
func TestSendMessage_Basic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Anthropic authenticates via x-api-key, not Bearer token
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key 'test-key', got %q", r.Header.Get("x-api-key"))
		}

		if r.Header.Get("anthropic-version") != anthropicVersion {
			t.Errorf("expected anthropic-version %q, got %q", anthropicVersion, r.Header.Get("anthropic-version"))
		}

		// Anthropic does not use Bearer tokens; the Authorization header must be absent.
		if r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}

		// Verify the request body contains at least one message.
		var reqBody anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if len(reqBody.Messages) == 0 {
			t.Error("expected at least one message in request body")
		}

		// Return a well-formed mock response.
		resp := anthropicResponse{
			ID:   "msg_test123",
			Type: "message",
			Role: "assistant",
			Content: []responseContentBlock{
				{Type: "text", Text: "Hello! How can I help?"},
			},
			Model:      "claude-sonnet-4-20250514",
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 10, OutputTokens: 8},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*AnthropicProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if response.Content != "Hello! How can I help?" {
		t.Errorf("expected content %q, got %q", "Hello! How can I help?", response.Content)
	}

	if response.FinishReason != "stop" {
		t.Errorf("expected FinishReason %q, got %q", "stop", response.FinishReason)
	}

	if response.Usage == nil {
		t.Fatal("expected usage in response")
	}
	if response.Usage.PromptTokens != 10 {
		t.Errorf("expected PromptTokens 10, got %d", response.Usage.PromptTokens)
	}
	if response.Usage.CompletionTokens != 8 {
		t.Errorf("expected CompletionTokens 8, got %d", response.Usage.CompletionTokens)
	}
}

// TestSendMessage_WithToolCalls verifies that tool_use response blocks are
// decoded into ToolCalls and that the finish reason is mapped to "tool_calls".
func TestSendMessage_WithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:   "msg_tooltest",
			Type: "message",
			Role: "assistant",
			Content: []responseContentBlock{
				{
					Type:  "tool_use",
					ID:    "call_1",
					Name:  "get_weather",
					Input: json.RawMessage(`{"city":"London"}`),
				},
			},
			Model:      "claude-sonnet-4-20250514",
			StopReason: "tool_use",
			Usage:      anthropicUsage{InputTokens: 20, OutputTokens: 15},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*AnthropicProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "What is the weather in London?"}},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if len(response.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(response.ToolCalls))
	}

	toolCall := response.ToolCalls[0]
	if toolCall.ID != "call_1" {
		t.Errorf("expected tool call ID %q, got %q", "call_1", toolCall.ID)
	}
	if toolCall.Function.Name != "get_weather" {
		t.Errorf("expected tool name %q, got %q", "get_weather", toolCall.Function.Name)
	}
	if toolCall.Function.Arguments != `{"city":"London"}` {
		t.Errorf("expected arguments %q, got %q", `{"city":"London"}`, toolCall.Function.Arguments)
	}

	if response.FinishReason != "tool_calls" {
		t.Errorf("expected FinishReason %q, got %q", "tool_calls", response.FinishReason)
	}
}

// TestSendMessage_WithThinking verifies that thinking blocks are decoded into
// the Reasoning field and text blocks into Content.
func TestSendMessage_WithThinking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:   "msg_thinking",
			Type: "message",
			Role: "assistant",
			Content: []responseContentBlock{
				{Type: "thinking", Thinking: "Let me think..."},
				{Type: "text", Text: "The answer is 42"},
			},
			Model:      "claude-sonnet-4-20250514",
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 30, OutputTokens: 25},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*AnthropicProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "What is 6 times 7?"}},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if response.Reasoning != "Let me think..." {
		t.Errorf("expected Reasoning %q, got %q", "Let me think...", response.Reasoning)
	}

	if response.Content != "The answer is 42" {
		t.Errorf("expected Content %q, got %q", "The answer is 42", response.Content)
	}
}

// TestSendMessage_NonSuccess verifies that a non-2xx HTTP response results in
// an error containing the status code.
func TestSendMessage_NonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		if _, err := w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*AnthropicProvider)

	_, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hello"}},
	})

	if err == nil {
		t.Fatal("expected error for 429 status, got nil")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("expected error to contain %q, got: %v", "429", err)
	}
}

// TestSendMessage_NoAPIKey verifies that SendMessage returns a descriptive error
// when the API key has not been configured.
func TestSendMessage_NoAPIKey(t *testing.T) {
	provider := New().WithBaseURL("https://example.com").(*AnthropicProvider)
	// Explicitly clear any key that may have been read from the environment.
	provider.apiKey = ""

	_, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hello"}},
	})

	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY is not set") {
		t.Errorf("expected error to mention ANTHROPIC_API_KEY, got: %v", err)
	}
}

// TestSendMessage_BetaHeaders verifies that the anthropic-beta header is set
// when BetaFeatures are configured via WithCapabilities.
func TestSendMessage_BetaHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		betaHeader := r.Header.Get("anthropic-beta")
		if !strings.Contains(betaHeader, "code-execution-2025-08-25") {
			t.Errorf("expected anthropic-beta to contain %q, got %q", "code-execution-2025-08-25", betaHeader)
		}

		resp := anthropicResponse{
			ID:         "msg_beta",
			Type:       "message",
			Role:       "assistant",
			Content:    []responseContentBlock{{Type: "text", Text: "OK"}},
			Model:      "claude-sonnet-4-20250514",
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 5, OutputTokens: 2},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// WithCapabilities must be called on the concrete type before the interface-
	// returning fluent methods so the chain compiles without a cast.
	provider := New().
		WithCapabilities(Capabilities{BetaFeatures: []string{BetaCodeExecution}}).
		WithAPIKey("test-key").
		WithBaseURL(server.URL)

	_, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Run some code"}},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
}

// TestSendMessage_ExtendedThinkingBeta verifies that setting ExtendedThinking=true
// on Capabilities automatically injects the interleaved-thinking beta header.
func TestSendMessage_ExtendedThinkingBeta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		betaHeader := r.Header.Get("anthropic-beta")
		if !strings.Contains(betaHeader, BetaInterleavedThinking) {
			t.Errorf("expected anthropic-beta to contain %q, got %q", BetaInterleavedThinking, betaHeader)
		}

		resp := anthropicResponse{
			ID:         "msg_thinking_beta",
			Type:       "message",
			Role:       "assistant",
			Content:    []responseContentBlock{{Type: "text", Text: "Done thinking"}},
			Model:      "claude-sonnet-4-20250514",
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 8, OutputTokens: 4},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// WithCapabilities must be called on the concrete type before the interface-
	// returning fluent methods so the chain compiles without a cast.
	provider := New().
		WithCapabilities(Capabilities{ExtendedThinking: true}).
		WithAPIKey("test-key").
		WithBaseURL(server.URL)

	_, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Think carefully"}},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
}

// TestIsStopMessage verifies all branches of the stop-message logic with a table
// of representative inputs.
func TestIsStopMessage(t *testing.T) {
	provider := New()

	tests := []struct {
		name     string
		message  *ai.ChatResponse
		expected bool
	}{
		{
			name:     "nil message",
			message:  nil,
			expected: true,
		},
		{
			name:     "finish reason stop",
			message:  &ai.ChatResponse{Content: "Hello", FinishReason: "stop"},
			expected: true,
		},
		{
			name:     "finish reason length",
			message:  &ai.ChatResponse{Content: "Truncated", FinishReason: "length"},
			expected: true,
		},
		{
			name:     "finish reason content_filter",
			message:  &ai.ChatResponse{FinishReason: "content_filter"},
			expected: true,
		},
		{
			// Tool calls take priority: even a "stop" finish reason must be
			// treated as non-stop when tool calls are present, because some
			// providers report the wrong finish reason.
			name: "tool calls present with stop finish reason",
			message: &ai.ChatResponse{
				FinishReason: "stop",
				ToolCalls: []ai.ToolCall{
					{ID: "call_1", Type: "function", Function: ai.ToolCallFunction{Name: "some_tool"}},
				},
			},
			expected: false,
		},
		{
			name:     "empty content no media",
			message:  &ai.ChatResponse{Content: ""},
			expected: true,
		},
		{
			// A non-empty response with no finish reason should not be treated as a
			// stop — the conversation may continue with more turns.
			name:     "content present no finish reason",
			message:  &ai.ChatResponse{Content: "Some content"},
			expected: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := provider.IsStopMessage(testCase.message)
			if result != testCase.expected {
				t.Errorf("expected IsStopMessage=%v, got %v", testCase.expected, result)
			}
		})
	}
}
