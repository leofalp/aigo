// Package main contrasts stateless and stateful client modes (Layer 2). The stateless example
// shows the model cannot recall earlier context; the stateful example uses an inmemory provider
// so the model remembers the user's name across turns.
// Requires the OPENAI_API_KEY environment variable.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/ai/openai"
	"github.com/leofalp/aigo/providers/memory/inmemory"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== Client Modes: Stateless vs Stateful ===")

	fmt.Println("--- Example 1: Stateless Mode (No Memory) ---")
	exampleStateless()

	fmt.Println("\n--- Example 2: Stateful Mode (With Memory) ---")
	exampleStateful()

	fmt.Println("\n--- Example 3: When to Use Each Mode ---")
	printUseCases()
}

func exampleStateless() {
	// Create client WITHOUT memory provider
	// This is perfect for single-shot completions where history isn't needed
	c, err := client.New(
		openai.New(),
		client.WithSystemPrompt("You are a helpful assistant."),
		// Note: NO WithMemory() call - defaults to stateless mode
	)
	if err != nil {
		log.Fatalf("Error creating stateless client: %v\n", err)
	}

	ctx := context.Background()

	// First message
	fmt.Println("User: My name is Alice")
	resp1, err := c.SendMessage(ctx, "My name is Alice")
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Assistant: %s\n\n", resp1.Content)

	// Second message - the LLM will NOT remember the previous message
	fmt.Println("User: What is my name?")
	resp2, err := c.SendMessage(ctx, "What is my name?")
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Assistant: %s\n", resp2.Content)
	fmt.Println("⚠️  Notice: The assistant doesn't remember 'Alice' because there's no memory!")
}

func exampleStateful() {
	// Create client WITH memory provider
	// This is perfect for conversations where context matters
	c, err := client.New(
		openai.New(),
		client.WithSystemPrompt("You are a helpful assistant."),
		client.WithMemory(inmemory.New()), // Enable stateful mode
	)
	if err != nil {
		log.Fatalf("Error creating stateful client: %v\n", err)
	}

	ctx := context.Background()

	// First message
	fmt.Println("User: My name is Bob")
	resp1, err := c.SendMessage(ctx, "My name is Bob")
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Assistant: %s\n\n", resp1.Content)

	// Second message - the LLM WILL remember the previous message
	fmt.Println("User: What is my name?")
	resp2, err := c.SendMessage(ctx, "What is my name?")
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Assistant: %s\n", resp2.Content)
	fmt.Println("✅ Notice: The assistant remembers 'Bob' because memory is enabled!")
}

func printUseCases() {
	fmt.Println("When to use STATELESS mode (no memory):")
	fmt.Println("  ✓ Single-shot completions (translation, summarization)")
	fmt.Println("  ✓ Stateless REST APIs")
	fmt.Println("  ✓ Parallel/batch processing of independent prompts")
	fmt.Println("  ✓ When you manage conversation state externally")
	fmt.Println("  ✓ Serverless functions with no persistent state")
	fmt.Println("")
	fmt.Println("When to use STATEFUL mode (with memory):")
	fmt.Println("  ✓ Multi-turn conversations")
	fmt.Println("  ✓ Chatbots and interactive assistants")
	fmt.Println("  ✓ When context from previous messages matters")
	fmt.Println("  ✓ ReAct patterns and iterative reasoning")
	fmt.Println("  ✓ Debugging and maintaining conversation flow")
}
