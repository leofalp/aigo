package main

import (
	"aigo/core/client"
	"aigo/providers/ai/openai"
	"aigo/providers/memory/inmemory"
	"aigo/providers/observability/slogobs"
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== Observability Examples ===")

	// Example 1: No observability (default - zero overhead)
	fmt.Println("--- Example 1: No Observability (Nil Observer) ---")
	exampleNilObserver()

	fmt.Println("\n--- Example 2: Slog Observability (Debug Level) ---")
	exampleSlogObserverTrace()

	fmt.Println("\n--- Example 3: Slog Observability (Info Level) ---")
	exampleSlogObserverInfo()

	fmt.Println("\n--- Example 4: Environment-Based Log Level ---")
	exampleEnvBasedLogLevel()
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

func exampleSlogObserverTrace() {
	// Create a debug-level logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug - 4,
	}))

	// Create client with slog observer
	c, err := client.NewClient[string](
		openai.NewOpenAIProvider(),
		client.WithObserver(slogobs.New(logger)),
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create client with slog observer
	c, err := client.NewClient[string](
		openai.NewOpenAIProvider(),
		client.WithObserver(slogobs.New(logger)),
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
	fmt.Printf("Response: %s\n", resp.Content)
}

func exampleEnvBasedLogLevel() {
	// Get log level from environment variable AIGO_LOG_LEVEL or LOG_LEVEL
	// Supported values: DEBUG, INFO, WARN, ERROR (default: INFO)
	logLevel := slogobs.GetLogLevelFromEnv()
	fmt.Printf("Using log level from environment: %s\n", slogobs.LogLevelString(logLevel))
	fmt.Println("Set AIGO_LOG_LEVEL or LOG_LEVEL to: DEBUG, INFO, WARN, or ERROR")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	c, err := client.NewClient[string](
		openai.NewOpenAIProvider(),
		client.WithObserver(slogobs.New(logger)),
		client.WithMemory(inmemory.New()),
		client.WithSystemPrompt("You are a helpful assistant."),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	resp, err := c.SendMessage(ctx, "Say hello!")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", resp.Content)
	fmt.Println("\nTip: Run with different log levels:")
	fmt.Println("  AIGO_LOG_LEVEL=DEBUG go run main.go")
	fmt.Println("  AIGO_LOG_LEVEL=INFO go run main.go")
	fmt.Println("  AIGO_LOG_LEVEL=ERROR go run main.go")
}
