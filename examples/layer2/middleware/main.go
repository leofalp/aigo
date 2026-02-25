// Package main demonstrates the middleware chain feature (Layer 2). It combines
// three built-in middlewares — Timeout, Retry, and Logging — to build a resilient
// and observable client with a single WithMiddleware option call.
//
// Execution order: Timeout → Retry → Logging → Provider.
//
// Requires the OPENAI_API_KEY environment variable.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/core/client/middleware"
	"github.com/leofalp/aigo/providers/ai/openai"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== Middleware Chain Example ===")
	fmt.Println()

	// Configure three middlewares. They are applied outermost-first, so the
	// execution order from the perspective of a request is:
	//   Timeout (30s) → Retry (up to 3 retries) → Logging → OpenAI provider
	//
	// The Timeout wraps everything, ensuring the entire call (including retries)
	// never exceeds 30 seconds. The Retry handles transient failures. The Logging
	// middleware records request/response details at Standard verbosity.
	timeoutMW := middleware.NewTimeoutMiddleware(30 * time.Second)

	retryMW := middleware.NewRetryMiddleware(middleware.RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
	})

	loggingMW := middleware.NewLoggingMiddleware(slog.Default(), middleware.LogLevelStandard)

	aiClient, err := client.New(
		openai.New(),
		client.WithSystemPrompt("You are a helpful assistant. Be concise."),
		client.WithMiddleware(timeoutMW, retryMW, loggingMW),
	)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	prompt := "What is the primary use case for Go's context package?"

	fmt.Printf("User: %s\n\n", prompt)

	resp, err := aiClient.SendMessage(ctx, prompt)
	if err != nil {
		log.Fatalf("SendMessage failed: %v", err)
	}

	fmt.Printf("Assistant: %s\n", resp.Content)

	if resp.Usage != nil {
		fmt.Println()
		fmt.Printf("--- Usage ---\n")
		fmt.Printf("  Prompt tokens:     %d\n", resp.Usage.PromptTokens)
		fmt.Printf("  Completion tokens: %d\n", resp.Usage.CompletionTokens)
		fmt.Printf("  Total tokens:      %d\n", resp.Usage.TotalTokens)
	}
}
