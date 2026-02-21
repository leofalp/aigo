// Package main demonstrates streaming responses using the Gemini provider (Layer 2). It calls
// client.StreamMessage and iterates the ChatStream, printing each content delta as it arrives
// for a live typewriter effect, then prints token-usage statistics.
// Requires the GEMINI_API_KEY environment variable.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/ai/gemini"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== Streaming Responses with Gemini ===")
	fmt.Println()

	// Create the Gemini provider. The API key is loaded from GEMINI_API_KEY.
	provider := gemini.New()

	// Create the client with Gemini 2.5 Flash.
	geminiClient, err := client.New(
		provider,
		client.WithDefaultModel(gemini.Model25FlashLite),
		client.WithSystemPrompt("You are a helpful assistant. Be concise."),
	)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	prompt := "Write a short poem about the Go programming language."

	fmt.Printf("User: %s\n\n", prompt)
	fmt.Print("Assistant: ")

	// StreamMessage returns a *ai.ChatStream. Tokens arrive as they are generated,
	// so we can print each content delta immediately for a live typewriter effect.
	stream, err := geminiClient.StreamMessage(ctx, prompt)
	if err != nil {
		log.Fatalf("streaming failed: %v", err)
	}

	var finalResponse *ai.ChatResponse
	for event, iterErr := range stream.Iter() {
		if iterErr != nil {
			_, err := fmt.Fprintln(os.Stderr, "\nerror during stream:", iterErr)
			if err != nil {
				fmt.Printf("failed to write error to stderr: %v\n", err)
			}
			os.Exit(1)
		}

		switch event.Type {
		case ai.StreamEventContent:
			// Print each token as it arrives — no newline so tokens flow inline.
			fmt.Print(event.Content)

		case ai.StreamEventDone:
			// Stream finished; collect the full response for metadata inspection.
			fmt.Println() // End the output line.
			fullResponse, collectErr := stream.Collect()
			if collectErr == nil {
				finalResponse = fullResponse
			}
		}
	}

	// Print usage statistics if available.
	if finalResponse != nil && finalResponse.Usage != nil {
		fmt.Println()
		fmt.Printf("--- Usage ---\n")
		fmt.Printf("  Prompt tokens:     %d\n", finalResponse.Usage.PromptTokens)
		fmt.Printf("  Completion tokens: %d\n", finalResponse.Usage.CompletionTokens)
		fmt.Printf("  Total tokens:      %d\n", finalResponse.Usage.TotalTokens)
	}
}
