package provider

import (
	"aigo/cmd/provider"
	"aigo/cmd/provider/openai"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewOpenAIProviderUsesDefaults(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "test-key-from-env")
	defer os.Unsetenv("OPENAI_API_KEY")

	p := openai.NewOpenAIProvider()

	if p.GetModelName() == "" {
		t.Error("expected non-empty model name")
	}
}

func TestNewOpenAIProviderWithoutEnvVariable(t *testing.T) {
	os.Unsetenv("OPENAI_API_KEY")

	p := openai.NewOpenAIProvider()

	if p == nil {
		t.Error("expected provider to be created even without env variable")
	}
}

func TestBuilderPatternWithAPIKey(t *testing.T) {
	p := openai.NewOpenAIProvider().WithAPIKey("custom-key")

	if p == nil {
		t.Error("expected provider after setting API key")
	}
}

func TestBuilderPatternWithModel(t *testing.T) {
	p := openai.NewOpenAIProvider().WithModel("gpt-4o")

	if p.GetModelName() != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %s", p.GetModelName())
	}
}

func TestBuilderPatternWithBaseURL(t *testing.T) {
	p := openai.NewOpenAIProvider().WithBaseURL("https://custom.api.com/v1")

	if p == nil {
		t.Error("expected provider after setting base URL")
	}
}

func TestBuilderPatternChaining(t *testing.T) {
	p := openai.NewOpenAIProvider().
		WithAPIKey("test-key").
		WithModel("gpt-4o").
		WithBaseURL("https://custom.api.com/v1")

	if p.GetModelName() != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %s", p.GetModelName())
	}
}

func TestGetModelNameReturnsCurrentModel(t *testing.T) {
	p := openai.NewOpenAIProvider().WithModel("gpt-4o-mini")

	if p.GetModelName() != "gpt-4o-mini" {
		t.Errorf("expected model name 'gpt-4o-mini', got %s", p.GetModelName())
	}
}

func TestSetModelChangesModel(t *testing.T) {
	p := openai.NewOpenAIProvider()
	p.SetModel("gpt-4")

	if p.GetModelName() != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %s", p.GetModelName())
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

		response := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Paris is the capital of France.",
					},
					"finish_reason": "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := openai.NewOpenAIProvider().
		WithAPIKey("test-key").
		WithBaseURL(server.URL)

	ctx := context.Background()
	response, err := p.SendSingleMessage(ctx, provider.ChatRequest{
		Messages: []provider.Message{
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
		json.NewDecoder(r.Body).Decode(&requestBody)

		if _, ok := requestBody["tools"]; !ok {
			t.Error("expected tools in request body")
		}

		response := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_123",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_weather",
									"arguments": `{"location": "Paris"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := openai.NewOpenAIProvider().
		WithAPIKey("test-key").
		WithBaseURL(server.URL)

	ctx := context.Background()
	response, err := p.SendSingleMessage(ctx, provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "What's the weather in Paris?"},
		},
		Tools: []provider.ToolDefinition{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]string{"type": "string"},
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
		w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	p := openai.NewOpenAIProvider().
		WithAPIKey("invalid-key").
		WithBaseURL(server.URL)

	ctx := context.Background()
	_, err := p.SendSingleMessage(ctx, provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if err == nil {
		t.Fatal("expected error for non-2xx status, got nil")
	}
}

func TestSendMessageWithEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"choices": []map[string]interface{}{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := openai.NewOpenAIProvider().
		WithAPIKey("test-key").
		WithBaseURL(server.URL)

	ctx := context.Background()
	_, err := p.SendSingleMessage(ctx, provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
}

func TestSendMessageWithContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	p := openai.NewOpenAIProvider().
		WithAPIKey("test-key").
		WithBaseURL(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.SendSingleMessage(ctx, provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestWithHTTPClientSetsCustomClient(t *testing.T) {
	customClient := &http.Client{
		Timeout: 0,
	}

	p := openai.NewOpenAIProvider().WithHTTPClient(customClient)

	if p == nil {
		t.Error("expected provider after setting custom client")
	}
}

func TestBuilderPatternReturnsProviderInterface(t *testing.T) {
	var _ provider.Provider = openai.NewOpenAIProvider()
	var _ provider.Provider = openai.NewOpenAIProvider().WithAPIKey("key")
	var _ provider.Provider = openai.NewOpenAIProvider().WithModel("model")
	var _ provider.Provider = openai.NewOpenAIProvider().WithBaseURL("url")
}
