//go:build integration

package openai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/leofalp/aigo/providers/ai"
)

const (
	// defaultTestModel is used when OPENAI_TEST_MODEL is not set.
	// gpt-4.1-nano is the cheapest/fastest OpenAI model suitable for tests.
	defaultTestModel = "gpt-4.1-nano"
)

// requireAPIKey fails the test immediately when OPENAI_API_KEY is not set.
// Integration tests are opt-in (build tag), so a missing key is a configuration
// error that should surface loudly rather than be silently skipped.
func requireAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Fatal("OPENAI_API_KEY is required for integration tests")
	}
}

// testModel returns the model to use for integration tests. It reads
// OPENAI_TEST_MODEL first, then AIGO_DEFAULT_LLM_MODEL, falling back to
// defaultTestModel. This allows running against OpenRouter or other
// OpenAI-compatible providers that may not host gpt-4.1-nano.
func testModel() string {
	if model := os.Getenv("OPENAI_TEST_MODEL"); model != "" {
		return model
	}
	if model := os.Getenv("AIGO_DEFAULT_LLM_MODEL"); model != "" {
		return model
	}
	return defaultTestModel
}

// TestOpenAISendMessage_Integration verifies that the OpenAI provider can
// complete a basic chat request against the real API. Requires OPENAI_API_KEY.
func TestOpenAISendMessage_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()
	model := testModel()

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

// TestOpenAISendMessageWithSystemPrompt_Integration verifies system prompt handling.
func TestOpenAISendMessageWithSystemPrompt_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()
	model := testModel()

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

// TestOpenAIIsStopMessage_Integration verifies that a normal completion is
// recognized as a stop message.
func TestOpenAIIsStopMessage_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()
	model := testModel()

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

// TestOpenAIStreamMessage_Integration verifies streaming via the real API.
// Iter and Collect are mutually exclusive (both consume the same underlying
// iterator), so each is tested in its own subtest with a fresh stream.
func TestOpenAIStreamMessage_Integration(t *testing.T) {
	requireAPIKey(t)

	model := testModel()

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

// TestOpenAIViaChatCompletions_Integration explicitly tests the /chat/completions
// endpoint to ensure backward-compatible providers still work.
func TestOpenAIViaChatCompletions_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()
	model := testModel()

	request := ai.ChatRequest{
		Model: model,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Reply with exactly: pong"},
		},
	}

	response, err := provider.SendMessageViaChatCompletions(ctx, request)
	if err != nil {
		t.Fatalf("SendMessageViaChatCompletions failed: %v", err)
	}

	if response.Content == "" {
		t.Error("expected non-empty content from chat completions endpoint")
	}

	t.Logf("ChatCompletions response: %s", response.Content)
}
