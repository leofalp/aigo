package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/ai/openai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
	"github.com/leofalp/aigo/providers/observability/slogobs"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== Observability Examples ===")

	fmt.Println("\n--- Example 1: Compact Format (Default) ---")
	exampleCompactFormat()

	fmt.Println("\n--- Example 2: Pretty Format ---")
	examplePrettyFormat()

	fmt.Println("\n--- Example 3: JSON Format ---")
	exampleJSONFormat()
}

func exampleCompactFormat() {
	fmt.Println("Format: Single line with JSON attributes (default)")
	fmt.Println("Shows TRACE, DEBUG, INFO, WARN, ERROR levels")
	fmt.Println()

	// Create observer with compact format including TRACE level
	observer := slogobs.New(
		slogobs.WithFormat(slogobs.FormatCompact),
		slogobs.WithLevel(slog.LevelDebug-4), // Enable TRACE
	)

	c, err := client.NewClient[string](
		openai.NewOpenAIProvider(),
		client.WithObserver(observer),
		client.WithMemory(inmemory.New()),
		client.WithSystemPrompt("You are a helpful assistant."),
	)
	if err != nil {
		log.Printf("Error creating client: %v\n", err)
		return
	}

	resp, err := c.SendMessage(context.Background(), "What is the capital of France?")
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("\nResponse: %s\n", resp.Content)
}

func examplePrettyFormat() {
	fmt.Println("Format: Multi-line with emoji and tree-style indented attributes")
	fmt.Println("Example:")
	fmt.Println("2025-11-03 10:40:35 🔵 DEBUG  Message")
	fmt.Println("                   └─ key: value")
	fmt.Println()

	// Create observer with pretty format
	observer := slogobs.New(
		slogobs.WithFormat(slogobs.FormatPretty),
		slogobs.WithLevel(slog.LevelDebug),
	)

	c, err := client.NewClient[string](
		openai.NewOpenAIProvider(),
		client.WithObserver(observer),
		client.WithMemory(inmemory.New()),
		client.WithSystemPrompt("You are a helpful assistant."),
	)
	if err != nil {
		log.Printf("Error creating client: %v\n", err)
		return
	}

	resp, err := c.SendMessage(context.Background(), "Tell me a fun fact about Go programming language")
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("\nResponse: %s\n", resp.Content)
}

func exampleJSONFormat() {
	fmt.Println("Format: Standard JSON (for production/log aggregation)")
	fmt.Println()

	// Create observer with JSON format
	observer := slogobs.New(
		slogobs.WithFormat(slogobs.FormatJSON),
		slogobs.WithLevel(slog.LevelInfo),
	)

	c, err := client.NewClient[string](
		openai.NewOpenAIProvider(),
		client.WithObserver(observer),
		client.WithMemory(inmemory.New()),
		client.WithSystemPrompt("You are a helpful assistant."),
	)
	if err != nil {
		log.Printf("Error creating client: %v\n", err)
		return
	}

	resp, err := c.SendMessage(context.Background(), "Hello!")
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("\nResponse: %s\n", resp.Content)
}
