package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/ai/openai"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	// Example Using builder pattern to configure the provider
	testProvider := openai.New()

	// Simple message without tools
	ctx := context.Background()

	response, err := testProvider.SendMessage(ctx, ai.ChatRequest{
		Model:        os.Getenv("AIGO_DEFAULT_LLM_MODEL"),
		SystemPrompt: "You are a helpful assistant.",
		Messages: []ai.Message{
			{Role: "user", Content: "What is the capital of France?"},
		},
	})

	if err != nil {
		slog.Error("Error sending message", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Response: %s\n", response.Content)
	fmt.Printf("Finish Reason: %s\n", response.FinishReason)

	// Example 3: Message with tools
	tools := []ai.ToolDescription{
		{
			Name:        "get_weather",
			Description: "Get the current weather for a location",
			Parameters: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"location": {
						Type:        "string",
						Description: "The city and state, e.g. San Francisco, CA",
					},
					"unit": {
						Type: "string",
						Enum: []any{"celsius", "fahrenheit"},
					},
				},
				Required: []string{"location"},
			},
		},
	}

	response2, err := testProvider.SendMessage(ctx, ai.ChatRequest{
		Model:        os.Getenv("AIGO_DEFAULT_LLM_MODEL"),
		SystemPrompt: "You are a helpful assistant.",
		Messages: []ai.Message{
			{Role: "user", Content: "What's the weather like in Paris?"},
		},
		Tools: tools,
	})

	if err != nil {
		slog.Error("Error sending message with tools", "error", err)
		os.Exit(1)
	}

	fmt.Printf("\nResponse with tools: %s\n", response2.Content)
	if len(response2.ToolCalls) > 0 {
		fmt.Printf("Tool calls requested: %d\n", len(response2.ToolCalls))
		for _, tc := range response2.ToolCalls {
			fmt.Printf("  - %s: %s\n", tc.Function.Name, tc.Function.Arguments)
		}
	}
}
