package openai

import (
	"aigo/internal/jsonschema"
	"aigo/providers/ai"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCapabilitiesDetectionFromBaseURL(t *testing.T) {
	cases := []struct {
		baseURL   string
		expectRes bool
		expectTCM ToolCallMode
	}{
		{"https://api.openai.com/v1", true, ToolCallModeTools},
		{"https://my-instance.openai.azure.com/openai/deployments", false, ToolCallModeTools},
		{"http://localhost:11434/v1", false, ToolCallModeBoth},
		{"http://localhost:1234/v1", false, ToolCallModeTools},
	}

	for _, tc := range cases {
		p := NewOpenAIProvider().WithBaseURL(tc.baseURL).(*OpenAIProvider)
		cap := p.GetCapabilities()
		if cap.SupportsResponses != tc.expectRes {
			t.Errorf("%s: expected SupportsResponses=%v, got %v", tc.baseURL, tc.expectRes, cap.SupportsResponses)
		}
		if cap.ToolCallMode != tc.expectTCM {
			t.Errorf("%s: expected ToolCallMode=%s, got %s", tc.baseURL, tc.expectTCM, cap.ToolCallMode)
		}
	}
}

func TestSendMessageUsesResponsesEndpointWhenSupported(t *testing.T) {
	var seenPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Fatalf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "resp_x",
			"object":     "response",
			"created_at": 1,
			"model":      "gpt-x",
			"output": []map[string]any{{
				"id": "o1", "type": "message", "role": "assistant",
				"content": []map[string]any{{"type": "output_text", "text": "hi"}},
			}},
			"status": "completed",
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider().WithAPIKey("key").WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: true, ToolCallMode: ToolCallModeTools})

	resp, err := p.SendMessage(context.Background(), ai.ChatRequest{Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenPath != "/responses" {
		t.Fatalf("expected /responses path, got %s", seenPath)
	}
	if resp.Content != "hi" || resp.FinishReason != "stop" {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

func TestSendMessageUsesChatCompletionsWhenResponsesNotSupported(t *testing.T) {
	var seenPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chat_x",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-x",
			"choices": []map[string]any{{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": "pong"},
				"finish_reason": "stop",
			}},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: false, ToolCallMode: ToolCallModeTools})

	resp, err := p.SendMessage(context.Background(), ai.ChatRequest{Messages: []ai.Message{{Role: ai.RoleUser, Content: "ping"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenPath != "/chat/completions" {
		t.Fatalf("expected /chat/completions path, got %s", seenPath)
	}
	if resp.Content != "pong" || resp.FinishReason != "stop" {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

func TestChatCompletionsMapsToolCallsNewFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chat_tools",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-x",
			"choices": []map[string]any{{
				"index": 0,
				"message": map[string]any{
					"role": "assistant",
					"tool_calls": []map[string]any{{
						"id":       "call1",
						"type":     "function",
						"function": map[string]any{"name": "lookup", "arguments": "{\"q\":\"Paris\"}"},
					}},
				},
				"finish_reason": "tool_calls",
			}},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: false, ToolCallMode: ToolCallModeTools})

	resp, err := p.SendMessage(context.Background(), ai.ChatRequest{Messages: []ai.Message{{Role: ai.RoleUser, Content: "use tool"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Function.Name != "lookup" {
		t.Fatalf("unexpected tool calls: %+v", resp.ToolCalls)
	}
	if resp.FinishReason != "tool_calls" {
		t.Fatalf("unexpected finish reason: %s", resp.FinishReason)
	}
}

func TestChatCompletionsErrorsOnEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "x",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-x",
			"choices": []any{},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: false, ToolCallMode: ToolCallModeTools})
	_, err := p.SendMessage(context.Background(), ai.ChatRequest{Messages: []ai.Message{{Role: ai.RoleUser, Content: "x"}}})
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestResponsesRequestIncludesSystemPromptAsDeveloper(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		input, ok := body["input"].([]any)
		if !ok || len(input) != 2 {
			t.Fatalf("unexpected input: %#v", body["input"])
		}
		first := input[0].(map[string]any)
		second := input[1].(map[string]any)
		if first["role"] != "developer" || first["content"] != "sys" {
			t.Fatalf("unexpected first message: %#v", first)
		}
		if second["role"] != "user" || second["content"] != "hi" {
			t.Fatalf("unexpected second message: %#v", second)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "r", "object": "response", "created_at": 1, "model": "m", "output": []map[string]any{{"id": "o", "type": "message", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": "ok"}}}}, "status": "completed"})
	}))
	defer server.Close()

	p := NewOpenAIProvider().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: true, ToolCallMode: ToolCallModeTools})

	_, err := p.SendMessage(context.Background(), ai.ChatRequest{SystemPrompt: "sys", Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChatCompletionsRequestMapsGenerationConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["temperature"]; !ok {
			t.Fatalf("missing temperature in request")
		}
		if _, ok := body["top_p"]; !ok {
			t.Fatalf("missing top_p in request")
		}
		if _, ok := body["max_completion_tokens"]; !ok {
			t.Fatalf("missing max_completion_tokens in request")
		}
		if _, hasMaxTokens := body["max_tokens"]; hasMaxTokens {
			t.Fatalf("unexpected max_tokens present when max_completion_tokens set")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "c",
			"object":  "chat.completion",
			"created": 1,
			"model":   "m",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: false, ToolCallMode: ToolCallModeTools})

	_, err := p.SendMessage(context.Background(), ai.ChatRequest{
		Messages:         []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
		GenerationConfig: &ai.GenerationConfig{Temperature: 0.7, TopP: 0.9, MaxOutputTokens: 128, FrequencyPenalty: 0.5, PresencePenalty: -0.5},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResponsesRequestHonorsForcedToolChoiceNone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["tool_choice"] != "none" {
			t.Fatalf("expected tool_choice=none, got %#v", body["tool_choice"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "r", "object": "response", "created_at": 1, "model": "m", "output": []map[string]any{{"id": "o", "type": "message", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": "ok"}}}}, "status": "completed"})
	}))
	defer server.Close()

	schema := &jsonschema.Schema{Type: "object"}
	p := NewOpenAIProvider().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: true, ToolCallMode: ToolCallModeTools})

	_, err := p.SendMessage(context.Background(), ai.ChatRequest{
		Messages:         []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
		Tools:            []ai.ToolDescription{{Name: "f", Description: "d", Parameters: schema}},
		ToolChoiceForced: "none",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChatCompletionsUsesLegacyFunctionsWhenConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, hasFunctions := body["functions"]; !hasFunctions {
			t.Fatalf("expected legacy functions present")
		}
		if _, hasFunctionCall := body["function_call"]; !hasFunctionCall {
			t.Fatalf("expected function_call present")
		}
		if _, hasTools := body["tools"]; hasTools {
			t.Fatalf("did not expect tools in legacy mode")
		}
		if _, hasToolChoice := body["tool_choice"]; hasToolChoice {
			t.Fatalf("did not expect tool_choice in legacy mode")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "c",
			"object":  "chat.completion",
			"created": 1,
			"model":   "m",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
		})
	}))
	defer server.Close()

	schema := &jsonschema.Schema{Type: "object"}
	p := NewOpenAIProvider().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: false, ToolCallMode: ToolCallModeFunctions})

	_, err := p.SendMessage(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
		Tools:    []ai.ToolDescription{{Name: "f", Parameters: schema}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsStopMessageBehavior(t *testing.T) {
	p := NewOpenAIProvider()
	if !p.IsStopMessage(nil) {
		t.Fatal("nil response should be stop")
	}
	if !p.IsStopMessage(&ai.ChatResponse{FinishReason: "stop"}) {
		t.Fatal("finish reason stop should be stop")
	}
	if !p.IsStopMessage(&ai.ChatResponse{FinishReason: "length"}) {
		t.Fatal("finish reason length should be stop")
	}
	if !p.IsStopMessage(&ai.ChatResponse{FinishReason: "content_filter"}) {
		t.Fatal("finish reason content_filter should be stop")
	}
	if !p.IsStopMessage(&ai.ChatResponse{Content: "", ToolCalls: nil}) {
		t.Fatal("empty content and no tool calls should be stop")
	}
	if p.IsStopMessage(&ai.ChatResponse{Content: "ok"}) {
		t.Fatal("content present should not be stop")
	}
	if p.IsStopMessage(&ai.ChatResponse{ToolCalls: []ai.ToolCall{{Type: "function", Function: ai.ToolCallFunction{Name: "x", Arguments: "{}"}}}}) {
		t.Fatal("tool calls present should not be stop")
	}
	// CRITICAL: Tool calls take priority over finish_reason
	// This is the case where some providers (e.g., OpenRouter) return finish_reason="stop" even with tool calls
	if p.IsStopMessage(&ai.ChatResponse{
		FinishReason: "stop",
		Content:      "some content",
		ToolCalls:    []ai.ToolCall{{Type: "function", Function: ai.ToolCallFunction{Name: "calculator", Arguments: "{}"}}},
	}) {
		t.Fatal("tool calls present should not be stop, even if finish_reason is 'stop'")
	}
}
