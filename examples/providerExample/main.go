package main

import (
	"aigo/cmd/provider"
	"aigo/cmd/provider/openai"
	"context"
	"fmt"
	"log"
)

func main() {
	// Example Using builder pattern to configure the provider
	testProvider := openai.NewOpenAIProvider().
		WithAPIKey("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").
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
		log.Fatalf("Error sending message: %v", err)
	}

	fmt.Printf("Response: %s\n", response.Content)
	fmt.Printf("Finish Reason: %s\n", response.FinishReason)

	// Example 3: Message with tools
	tools := []provider.ToolDefinition{
		{
			Name:        "get_weather",
			Description: "Get the current weather for a location",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "The city and state, e.g. San Francisco, CA",
					},
					"unit": map[string]interface{}{
						"type": "string",
						"enum": []string{"celsius", "fahrenheit"},
					},
				},
				"required": []string{"location"},
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
		log.Fatalf("Error sending message with tools: %v", err)
	}

	fmt.Printf("\nResponse with tools: %s\n", response2.Content)
	if len(response2.ToolCalls) > 0 {
		fmt.Printf("Tool calls requested: %d\n", len(response2.ToolCalls))
		for _, tc := range response2.ToolCalls {
			fmt.Printf("  - %s: %s\n", tc.Function.Name, tc.Function.Arguments)
		}
	}
}
