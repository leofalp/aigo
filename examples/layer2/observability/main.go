package main

import (
	"aigo/core/client"
	"aigo/providers/ai/openai"
	"aigo/providers/memory/inmemory"
	"aigo/providers/observability/slog"
	"context"
	"fmt"
	"log"
	logslog "log/slog"
	"os"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== Observability Examples ===")

	// Example 1: No observability (default - zero overhead)
	fmt.Println("--- Example 1: No Observability (Nil Observer) ---")
	exampleNilObserver()

	fmt.Println("\n--- Example 2: Slog Observability (Debug Level) ---")
	exampleSlogObserverDebug()

	fmt.Println("\n--- Example 3: Slog Observability (Info Level) ---")
	exampleSlogObserverInfo()
}

func exampleNilObserver() {
	// Default behavior - nil observer (zero overhead, no observability)
	c, err := client.NewClient[string](
		openai.NewOpenAIProvider(),
		client.WithSystemPrompt("You are a helpful assistant."),
	)
	if err != nil {
		log.Printf("Error creating client: %v\n", err)
		return
	}

	resp, err := c.SendMessage(context.Background(), "What is 2+2?")
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Response: %s\n", resp.Content)
	fmt.Println("(No observability output - observer is nil)")
}

func exampleSlogObserverDebug() {
	// Create a debug-level logger
	logger := logslog.New(logslog.NewTextHandler(os.Stdout, &logslog.HandlerOptions{
		Level: logslog.LevelDebug,
	}))

	// Create client with slog observer
	c, err := client.NewClient[string](
		openai.NewOpenAIProvider(),
		client.WithObserver(slog.New(logger)),
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

func exampleSlogObserverInfo() {
	// Create an info-level logger (no debug traces)
	logger := logslog.New(logslog.NewJSONHandler(os.Stdout, &logslog.HandlerOptions{
		Level: logslog.LevelInfo,
	}))

	// Create client with slog observer
	c, err := client.NewClient[string](
		openai.NewOpenAIProvider(),
		client.WithObserver(slog.New(logger)),
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
