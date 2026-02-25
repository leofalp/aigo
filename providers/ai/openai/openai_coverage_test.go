package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/observability"
	"github.com/leofalp/aigo/providers/observability/slogobs"
)

func TestSendMessage_Routing(t *testing.T) {
	// Test routing to Responses API
	serverResponses := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("expected path /responses, got %s", r.URL.Path)
		}
		response := map[string]interface{}{
			"id":     "resp_1",
			"object": "response",
			"output": []map[string]interface{}{
				{
					"type": "message",
					"content": []map[string]interface{}{
						{"type": "output_text", "text": "Hello"},
					},
				},
			},
			"status": "completed",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer serverResponses.Close()

	p1 := New().WithAPIKey("test-key").WithBaseURL(serverResponses.URL).(*OpenAIProvider)
	p1 = p1.WithCapabilities(Capabilities{SupportsResponses: true})

	ctx := context.Background()
	_, err := p1.SendMessage(ctx, ai.ChatRequest{Messages: []ai.Message{{Role: "user", Content: "Hi"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test routing to Chat Completions API
	serverChat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
		}
		response := map[string]interface{}{
			"id":     "chat_1",
			"object": "chat.completion",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello",
					},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer serverChat.Close()

	p2 := New().WithAPIKey("test-key").WithBaseURL(serverChat.URL).(*OpenAIProvider)
	p2 = p2.WithCapabilities(Capabilities{SupportsResponses: false})

	_, err = p2.SendMessage(ctx, ai.ChatRequest{Messages: []ai.Message{{Role: "user", Content: "Hi"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendMessage_Observability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":     "chat_1",
			"object": "chat.completion",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"total_tokens": 10,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := New().WithAPIKey("test-key").WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: false})

	// Create a context with observability
	observer := slogobs.New()
	ctx := observability.ContextWithObserver(context.Background(), observer)

	// We can't easily assert the span/trace calls without a mock observer,
	// but this ensures the code paths are executed and don't panic.
	_, err := p.SendMessage(ctx, ai.ChatRequest{Messages: []ai.Message{{Role: "user", Content: "Hi"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendMessageViaResponses_Errors(t *testing.T) {
	// Test HTTP error
	serverError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer serverError.Close()

	p1 := New().WithAPIKey("test-key").WithBaseURL(serverError.URL).(*OpenAIProvider)
	_, err := p1.SendMessageViaResponses(context.Background(), ai.ChatRequest{})
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}

	// Test empty output
	serverEmpty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":     "resp_1",
			"object": "response",
			"output": []map[string]interface{}{},
			"status": "completed",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer serverEmpty.Close()

	p2 := New().WithAPIKey("test-key").WithBaseURL(serverEmpty.URL).(*OpenAIProvider)
	_, err = p2.SendMessageViaResponses(context.Background(), ai.ChatRequest{})
	if err == nil {
		t.Fatal("expected error for empty output, got nil")
	}
}

func TestSendMessageViaChatCompletions_Errors(t *testing.T) {
	// Test HTTP error
	serverError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer serverError.Close()

	p1 := New().WithAPIKey("test-key").WithBaseURL(serverError.URL).(*OpenAIProvider)
	_, err := p1.SendMessageViaChatCompletions(context.Background(), ai.ChatRequest{})
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}

	// Test empty choices
	serverEmpty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":      "chat_1",
			"object":  "chat.completion",
			"choices": []map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer serverEmpty.Close()

	p2 := New().WithAPIKey("test-key").WithBaseURL(serverEmpty.URL).(*OpenAIProvider)
	_, err = p2.SendMessageViaChatCompletions(context.Background(), ai.ChatRequest{})
	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
}
