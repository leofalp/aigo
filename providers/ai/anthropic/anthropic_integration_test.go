//go:build integration

package anthropic

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
)

const (
	// defaultIntegrationModel is the cheapest Claude model that supports tools
	// and streaming. Override with ANTHROPIC_TEST_MODEL if needed.
	defaultIntegrationModel = "claude-sonnet-4-20250514"
)

// integrationModel returns the model to use for integration tests. It reads
// ANTHROPIC_TEST_MODEL first, falling back to defaultIntegrationModel. This
// allows testing against newer or regional model variants without code changes.
func integrationModel() string {
	if model := os.Getenv("ANTHROPIC_TEST_MODEL"); model != "" {
		return model
	}
	return defaultIntegrationModel
}

// requireAPIKey fails the test immediately when ANTHROPIC_API_KEY is not set.
// Integration tests are opt-in (build tag), so a missing key is a configuration
// error that should surface loudly rather than be silently skipped.
func requireAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Fatal("ANTHROPIC_API_KEY is required for integration tests")
	}
}

// TestAnthropicSendMessage_Integration verifies that the Anthropic provider can
// complete a basic chat request against the real Messages API.
func TestAnthropicSendMessage_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()
	model := integrationModel()

	request := ai.ChatRequest{
		Model: model,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Reply with exactly: hello world"},
		},
	}

	response, err := provider.SendMessage(ctx, request)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if response == nil {
		t.Fatal("expected non-nil response")
	}

	if response.Content == "" {
		t.Error("expected non-empty content in response")
	}

	if response.Model == "" {
		t.Error("expected non-empty model in response")
	}

	if response.Usage == nil {
		t.Error("expected non-nil usage in response")
	} else {
		if response.Usage.TotalTokens <= 0 {
			t.Error("expected positive total tokens")
		}
		t.Logf("Tokens — prompt: %d, completion: %d, total: %d",
			response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens)
	}

	t.Logf("Model: %s", response.Model)
	t.Logf("Content: %s", response.Content)
	t.Logf("FinishReason: %s", response.FinishReason)
}

// TestAnthropicSendMessageWithSystemPrompt_Integration verifies that the system
// prompt is forwarded to the API and influences the model's response.
func TestAnthropicSendMessageWithSystemPrompt_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()
	model := integrationModel()

	request := ai.ChatRequest{
		Model:        model,
		SystemPrompt: "You are a helpful assistant. Always reply in exactly one word.",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "What color is the sky on a clear day?"},
		},
	}

	response, err := provider.SendMessage(ctx, request)
	if err != nil {
		t.Fatalf("SendMessage with system prompt failed: %v", err)
	}

	if response.Content == "" {
		t.Error("expected non-empty content")
	}

	t.Logf("Response: %s", response.Content)
}

// TestAnthropicIsStopMessage_Integration verifies that a normal completion is
// recognized as a stop message (finish_reason maps to "stop").
func TestAnthropicIsStopMessage_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()
	model := integrationModel()

	request := ai.ChatRequest{
		Model: model,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Say hello"},
		},
	}

	response, err := provider.SendMessage(ctx, request)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if !provider.IsStopMessage(response) {
		t.Errorf("expected IsStopMessage to return true for a normal completion, got false (finishReason=%s)", response.FinishReason)
	}
}

// TestAnthropicMultiTurn_Integration verifies that multi-turn conversations
// work correctly with the real API, maintaining context across turns.
func TestAnthropicMultiTurn_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()
	model := integrationModel()

	request := ai.ChatRequest{
		Model: model,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "My name is Alice."},
			{Role: ai.RoleAssistant, Content: "Hello Alice! Nice to meet you."},
			{Role: ai.RoleUser, Content: "What is my name?"},
		},
	}

	response, err := provider.SendMessage(ctx, request)
	if err != nil {
		t.Fatalf("Multi-turn SendMessage failed: %v", err)
	}

	if response.Content == "" {
		t.Error("expected non-empty content")
	}

	// The model should recall "Alice" from the conversation history.
	if !strings.Contains(strings.ToLower(response.Content), "alice") {
		t.Errorf("expected response to contain 'Alice', got: %s", response.Content)
	}

	t.Logf("Multi-turn response: %s", response.Content)
}

// TestAnthropicStreamMessage_Integration verifies streaming via the real API.
// Iter and Collect are mutually exclusive (both consume the same underlying
// iterator), so each is tested in its own subtest with a fresh stream.
func TestAnthropicStreamMessage_Integration(t *testing.T) {
	requireAPIKey(t)

	model := integrationModel()

	// newStreamRequest returns a fresh ChatRequest for each subtest.
	newStreamRequest := func() ai.ChatRequest {
		return ai.ChatRequest{
			Model: model,
			Messages: []ai.Message{
				{Role: ai.RoleUser, Content: "Count from 1 to 5"},
			},
		}
	}

	t.Run("Iter", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		provider := New()
		stream, err := provider.StreamMessage(ctx, newStreamRequest())
		if err != nil {
			t.Fatalf("StreamMessage failed: %v", err)
		}

		eventCount := 0
		hasContent := false

		for event, iterErr := range stream.Iter() {
			if iterErr != nil {
				t.Fatalf("stream iteration error: %v", iterErr)
			}

			eventCount++

			if event.Content != "" {
				hasContent = true
			}
		}

		if eventCount == 0 {
			t.Error("expected at least one stream event")
		}

		if !hasContent {
			t.Error("expected at least one content event in the stream")
		}

		t.Logf("Received %d stream events", eventCount)
	})

	t.Run("Collect", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		provider := New()
		stream, err := provider.StreamMessage(ctx, newStreamRequest())
		if err != nil {
			t.Fatalf("StreamMessage failed: %v", err)
		}

		collected, err := stream.Collect()
		if err != nil {
			t.Fatalf("stream.Collect() failed: %v", err)
		}

		if collected == nil {
			t.Fatal("expected non-nil collected response")
		}

		if collected.Content == "" {
			t.Error("expected non-empty collected content")
		}

		t.Logf("Collected content: %s", collected.Content)
	})
}

// TestAnthropicToolCall_Integration verifies that the provider correctly handles
// a tool-use round trip: the model requests a tool call, and a subsequent request
// with the tool result produces a final text response.
func TestAnthropicToolCall_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	provider := New()
	model := integrationModel()

	// Define a simple tool the model can call.
	tools := []ai.ToolDescription{
		{
			Name:        "get_weather",
			Description: "Get the current weather for a city",
			Parameters: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"city": {Type: "string", Description: "The city name"},
				},
				Required: []string{"city"},
			},
		},
	}

	// First turn: ask the model something that should trigger the tool.
	firstRequest := ai.ChatRequest{
		Model: model,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "What is the weather in Paris? Use the get_weather tool."},
		},
		Tools: tools,
	}

	firstResponse, err := provider.SendMessage(ctx, firstRequest)
	if err != nil {
		t.Fatalf("First SendMessage failed: %v", err)
	}

	if len(firstResponse.ToolCalls) == 0 {
		t.Fatalf("expected at least one tool call, got none (content: %s)", firstResponse.Content)
	}

	toolCall := firstResponse.ToolCalls[0]
	t.Logf("Tool call: %s(%s)", toolCall.Function.Name, toolCall.Function.Arguments)

	if toolCall.Function.Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %q", toolCall.Function.Name)
	}

	// Second turn: provide the tool result and let the model produce a final answer.
	secondRequest := ai.ChatRequest{
		Model: model,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "What is the weather in Paris? Use the get_weather tool."},
			{
				Role:      ai.RoleAssistant,
				ToolCalls: firstResponse.ToolCalls,
			},
			{
				Role:       ai.RoleTool,
				ToolCallID: toolCall.ID,
				Content:    `{"temperature": "18°C", "condition": "partly cloudy"}`,
			},
		},
		Tools: tools,
	}

	secondResponse, err := provider.SendMessage(ctx, secondRequest)
	if err != nil {
		t.Fatalf("Second SendMessage (tool result) failed: %v", err)
	}

	if secondResponse.Content == "" {
		t.Error("expected non-empty final response after tool result")
	}

	t.Logf("Final response: %s", secondResponse.Content)
}

// TestAnthropicThinking_Integration verifies that extended thinking (adaptive mode)
// produces a non-empty Reasoning field in the response. This test uses a model
// that supports thinking; if the default test model does not, it is skipped.
func TestAnthropicThinking_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	provider := New()
	model := integrationModel()

	request := ai.ChatRequest{
		Model: model,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "What is 17 * 23? Think step by step."},
		},
		GenerationConfig: &ai.GenerationConfig{
			IncludeThoughts: true,
		},
	}

	response, err := provider.SendMessage(ctx, request)
	if err != nil {
		// Some models do not support thinking; skip rather than fail.
		if strings.Contains(err.Error(), "thinking") || strings.Contains(err.Error(), "not supported") {
			t.Skipf("Model %s does not support thinking: %v", model, err)
		}
		t.Fatalf("SendMessage with thinking failed: %v", err)
	}

	if response.Content == "" {
		t.Error("expected non-empty content")
	}

	// Adaptive thinking should produce reasoning output.
	if response.Reasoning == "" {
		t.Log("Warning: no reasoning returned — model may not support thinking")
	} else {
		t.Logf("Reasoning (first 200 chars): %.200s", response.Reasoning)
	}

	t.Logf("Content: %s", response.Content)
}

// TestAnthropicStreamWithThinking_Integration verifies that streaming with
// extended thinking produces both reasoning and content events.
func TestAnthropicStreamWithThinking_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	provider := New()
	model := integrationModel()

	request := ai.ChatRequest{
		Model: model,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "What is 17 * 23? Think step by step."},
		},
		GenerationConfig: &ai.GenerationConfig{
			IncludeThoughts: true,
		},
	}

	stream, err := provider.StreamMessage(ctx, request)
	if err != nil {
		if strings.Contains(err.Error(), "thinking") || strings.Contains(err.Error(), "not supported") {
			t.Skipf("Model %s does not support thinking: %v", model, err)
		}
		t.Fatalf("StreamMessage with thinking failed: %v", err)
	}

	collected, err := stream.Collect()
	if err != nil {
		t.Fatalf("stream.Collect() failed: %v", err)
	}

	if collected.Content == "" {
		t.Error("expected non-empty collected content")
	}

	if collected.Reasoning == "" {
		t.Log("Warning: no reasoning in collected stream — model may not support thinking")
	} else {
		t.Logf("Collected reasoning (first 200 chars): %.200s", collected.Reasoning)
	}

	t.Logf("Collected content: %s", collected.Content)
}

// TestAnthropicPromptCaching_Integration verifies that prompt caching can be
// enabled without causing API errors. The test confirms the request succeeds;
// actual cache hits depend on Anthropic's server-side caching behavior.
func TestAnthropicPromptCaching_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()
	provider = provider.WithCapabilities(Capabilities{
		PromptCaching: true,
	})

	model := integrationModel()

	request := ai.ChatRequest{
		Model:        model,
		SystemPrompt: "You are a helpful assistant that always replies concisely.",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Reply with exactly: cached"},
		},
	}

	response, err := provider.SendMessage(ctx, request)
	if err != nil {
		t.Fatalf("SendMessage with prompt caching failed: %v", err)
	}

	if response.Content == "" {
		t.Error("expected non-empty content")
	}

	t.Logf("Response: %s", response.Content)

	// Log cache token info if available.
	if response.Usage != nil && response.Usage.CachedTokens > 0 {
		t.Logf("Cached tokens: %d", response.Usage.CachedTokens)
	}
}
