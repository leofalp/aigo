//go:build integration

package gemini

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/leofalp/aigo/providers/ai"
)

// requireAPIKey fails the test immediately when GEMINI_API_KEY is not set.
// Integration tests are opt-in (build tag), so a missing key is a configuration
// error that should surface loudly rather than be silently skipped.
func requireAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv("GEMINI_API_KEY") == "" {
		t.Fatal("GEMINI_API_KEY is required for integration tests")
	}
}

// TestGeminiSendMessage_Integration verifies that the Gemini provider can
// complete a basic chat request against the real Google API. Requires GEMINI_API_KEY.
func TestGeminiSendMessage_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()

	request := ai.ChatRequest{
		Model: Model25FlashLite,
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

// TestGeminiSendMessageWithSystemPrompt_Integration verifies system prompt handling.
func TestGeminiSendMessageWithSystemPrompt_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()

	request := ai.ChatRequest{
		Model:        Model25FlashLite,
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

// TestGeminiIsStopMessage_Integration verifies that a normal completion is
// recognized as a stop message.
func TestGeminiIsStopMessage_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()

	request := ai.ChatRequest{
		Model: Model25FlashLite,
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

// TestGeminiStreamMessage_Integration verifies streaming via the real Gemini API.
// Iter and Collect are mutually exclusive (both consume the same underlying
// iterator), so each is tested in its own subtest with a fresh stream.
func TestGeminiStreamMessage_Integration(t *testing.T) {
	requireAPIKey(t)

	// newStreamRequest returns a fresh ChatRequest for each subtest.
	newStreamRequest := func() ai.ChatRequest {
		return ai.ChatRequest{
			Model: Model25FlashLite,
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

// TestGeminiDefaultModel_Integration verifies that the provider works with its
// default model when no model is specified in the request.
func TestGeminiDefaultModel_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()

	// Omit Model to use the provider's default
	request := ai.ChatRequest{
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Reply with exactly: pong"},
		},
	}

	response, err := provider.SendMessage(ctx, request)
	if err != nil {
		t.Fatalf("SendMessage with default model failed: %v", err)
	}

	if response.Content == "" {
		t.Error("expected non-empty content from default model")
	}

	t.Logf("Default model used: %s", response.Model)
	t.Logf("Response: %s", response.Content)
}

// TestGeminiMultiTurn_Integration verifies that multi-turn conversations work
// correctly with the real API.
func TestGeminiMultiTurn_Integration(t *testing.T) {
	requireAPIKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := New()

	request := ai.ChatRequest{
		Model: Model25FlashLite,
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

	t.Logf("Multi-turn response: %s", response.Content)
}
