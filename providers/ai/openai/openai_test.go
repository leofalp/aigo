package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
)

func TestNewOpenAIProviderWithoutEnvVariable(t *testing.T) {
	err := os.Unsetenv("OPENAI_API_KEY")
	if err != nil {
		t.Fatal("failed to set env variable: " + err.Error())
	}

	p := NewOpenAIProvider()

	if p == nil {
		t.Error("expected provider to be created even without env variable")
	}
}

func TestBuilderPatternWithAPIKey(t *testing.T) {
	p := NewOpenAIProvider().WithAPIKey("custom-key")

	if p == nil {
		t.Error("expected provider after setting API key")
	}
}

func TestBuilderPatternWithBaseURL(t *testing.T) {
	p := NewOpenAIProvider().WithBaseURL("https://custom.api.com/v1")

	if p == nil {
		t.Error("expected provider after setting base URL")
	}
}

func TestSendMessageWithValidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header 'Bearer test-key', got %s", r.Header.Get("Authorization"))
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
		}

		// Responses API style response (new models)
		response := map[string]interface{}{
			"id":         "resp_1",
			"object":     "response",
			"created_at": 1234567890,
			"model":      "gpt-test",
			"output": []map[string]interface{}{
				{
					"id":   "out_1",
					"type": "message",
					"role": "assistant",
					"content": []map[string]interface{}{
						{
							"type": "output_text",
							"text": "Paris is the capital of France.",
						},
					},
				},
			},
			"status": "completed",
		}

		w.Header().Set("Content-Type", "application/json")
		// Log the exact JSON we will send for debugging
		respBytes, _ := json.Marshal(response)
		t.Logf("server response JSON: %s", string(respBytes))
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			t.Fatal("failed to encode response: " + err.Error())
		}
	}))
	defer server.Close()

	p := NewOpenAIProvider().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: true, ToolCallMode: ToolCallModeTools})

	ctx := context.Background()
	response, err := p.SendMessage(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "user", Content: "What is the capital of France?"},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response.Content != "Paris is the capital of France." {
		t.Errorf("expected content 'Paris is the capital of France.', got %s", response.Content)
	}

	if response.FinishReason != "stop" {
		t.Errorf("expected finish reason 'stop', got %s", response.FinishReason)
	}
}

func TestSendMessageWithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			t.Fatal("failed to decode request body: " + err.Error())
		}

		if _, ok := requestBody["tools"]; !ok {
			t.Error("expected tools in request body")
		}

		// Responses API style response with a separate function_call output item
		response := map[string]interface{}{
			"id":         "resp_tool",
			"object":     "response",
			"created_at": 1234567890,
			"model":      "gpt-test",
			"output": []map[string]interface{}{
				{
					"id":      "out_1",
					"type":    "message",
					"role":    "assistant",
					"content": []map[string]interface{}{},
				},
				{
					"id":        "out_2",
					"type":      "function_call",
					"name":      "get_weather",
					"call_id":   "call_123",
					"arguments": `{"location": "Paris"}`,
				},
			},
			"status": "completed",
		}

		w.Header().Set("Content-Type", "application/json")
		respBytes, _ := json.MarshalIndent(response, "", "  ")
		t.Logf("server response JSON: %s", string(respBytes))
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			t.Fatal("failed to encode response: " + err.Error())
		}
	}))
	defer server.Close()

	p := NewOpenAIProvider().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: true, ToolCallMode: ToolCallModeTools})

	ctx := context.Background()
	response, err := p.SendMessage(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "user", Content: "What's the weather in Paris?"},
		},
		Tools: []ai.ToolDescription{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				Parameters: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"location": {Type: "string"},
					},
				},
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(response.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(response.ToolCalls))
	}

	if response.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected tool call name 'get_weather', got %s", response.ToolCalls[0].Function.Name)
	}
}

func TestSendMessageWithNon2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{"error": "Invalid API key"}`))
		if err != nil {
			t.Fatal("failed to write response: " + err.Error())
		}
	}))
	defer server.Close()

	p := NewOpenAIProvider().
		WithAPIKey("invalid-key").
		WithBaseURL(server.URL)

	ctx := context.Background()
	_, err := p.SendMessage(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if err == nil {
		t.Fatal("expected error for non-2xx status, got nil")
	}
}

func TestSendMessageWithEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return Responses API shape with empty output array
		response := map[string]interface{}{
			"id":     "resp_empty",
			"object": "response",
			"output": []map[string]interface{}{},
			"status": "completed",
		}

		w.Header().Set("Content-Type", "application/json")
		// Log the exact JSON we will send for debugging
		respBytes, _ := json.Marshal(response)
		t.Logf("server response JSON: %s", string(respBytes))
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			t.Fatal("failed to encode response: " + err.Error())
		}
	}))
	defer server.Close()

	p := NewOpenAIProvider().
		WithAPIKey("test-key").
		WithBaseURL(server.URL)

	ctx := context.Background()
	_, err := p.SendMessage(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
}

func TestWithHTTPClientSetsCustomClient(t *testing.T) {
	customClient := &http.Client{
		Timeout: 0,
	}

	p := NewOpenAIProvider().WithHttpClient(customClient)

	if p == nil {
		t.Error("expected provider after setting custom client")
	}
}

func TestBuilderPatternReturnsProviderInterface(t *testing.T) {
	var _ ai.Provider = NewOpenAIProvider()
	NewOpenAIProvider().WithAPIKey("key")
	NewOpenAIProvider().WithBaseURL("url")
}

func TestUnmarshalResponsesAPIShape(t *testing.T) {
	jsonBytes := []byte(`{
		"id":"resp_1",
		"object":"response",
		"created_at":1234567890,
		"model":"gpt-test",
		"output":[
			{
				"id":"out_1",
				"type":"message",
				"role":"assistant",
				"content":[{"type":"output_text","text":"Paris is the capital of France."}]
			}
		],
		"status":"completed"
	}`)

	var resp responseCreateResponse
	if err := json.Unmarshal(jsonBytes, &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(resp.Output) == 0 {
		t.Fatalf("expected resp.Output to have items, got 0")
	}

	if resp.Output[0].Type != "message" {
		t.Fatalf("expected output[0].type 'message', got '%s'", resp.Output[0].Type)
	}
}
