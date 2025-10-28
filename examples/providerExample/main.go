package main

import (
	"aigo/cmd/jsonschema"
	"aigo/cmd/provider"
	"aigo/cmd/provider/openai"
	"aigo/cmd/tool"
	"context"
	"fmt"
	"log/slog"
	"os"
)

func main() {
	// Example Using builder pattern to configure the provider
	testProvider := openai.NewOpenAIProvider().
		WithModel("nvidia/nemotron-nano-9b-v2:free").
		WithBaseURL("https://openrouter.ai/api/v1")

	// Simple message without tools
	ctx := context.Background()

	response, err := testProvider.SendSingleMessage(ctx, provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "system", Content: "You are a helpful assistant."},
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
	tools := []tool.ToolInfo{
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

	response2, err := testProvider.SendSingleMessage(ctx, provider.ChatRequest{
		Messages: []provider.Message{
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
