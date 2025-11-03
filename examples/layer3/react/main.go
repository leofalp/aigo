package main

import (
	"aigo/core/client"
	"aigo/patterns/react"
	"aigo/providers/ai/openai"
	"aigo/providers/memory/inmemory"
	"aigo/providers/observability/slogobs"
	"aigo/providers/tool/calculator"
	"context"
	"fmt"
	"log"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== ReAct Pattern Example (Layer 3) ===")
	fmt.Println("This example demonstrates the ReAct (Reason + Act) pattern")
	fmt.Println("which automatically handles the tool execution loop.")

	// Create memory provider
	memory := inmemory.New()

	// Create calculator tool
	calcTool := calculator.NewCalculatorTool()

	// Create base client with memory, tools, and observer
	baseClient, err := client.NewClient[string](
		openai.NewOpenAIProvider(),
		client.WithMemory(memory),
		client.WithObserver(slogobs.New()),
		client.WithTools(calcTool),
		client.WithSystemPrompt("You are a helpful math assistant. Use tools when needed to provide accurate calculations."),
		client.WithEnrichSystemPrompt(), // Automatically adds tool descriptions and usage guidance
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create ReAct pattern with functional options
	// The ReAct pattern will use memory and tools from the base client
	reactPattern, err := react.NewReactPattern[string](
		baseClient,
		react.WithMaxIterations(5),
		react.WithStopOnError(true),
	)
	if err != nil {
		log.Fatalf("Failed to create ReAct pattern: %v", err)
	}

	ctx := context.Background()

	// Test Case 1: Simple calculation
	fmt.Println("--- Test Case 1: Simple Calculation ---")
	prompt1 := "What is 1234 multiplied by 567?"
	fmt.Printf("User: %s\n\n", prompt1)

	resp1, err := reactPattern.Execute(ctx, prompt1)
	if err != nil {
		log.Fatalf("ReAct execution failed: %v", err)
	}

	fmt.Printf("\n✓ Assistant: %s\n", resp1.Content)
	fmt.Printf("Finish Reason: %s\n", resp1.FinishReason)
	if resp1.Usage != nil {
		fmt.Printf("Tokens Used: %d (prompt: %d, completion: %d)\n",
			resp1.Usage.TotalTokens,
			resp1.Usage.PromptTokens,
			resp1.Usage.CompletionTokens,
		)
	}

	// Test Case 2: Multi-step reasoning
	fmt.Println("\n\n--- Test Case 2: Multi-Step Calculation ---")
	prompt2 := "Calculate (150 + 250) * 3, then subtract 100"
	fmt.Printf("User: %s\n\n", prompt2)

	resp2, err := reactPattern.Execute(ctx, prompt2)
	if err != nil {
		log.Fatalf("ReAct execution failed: %v", err)
	}

	fmt.Printf("\n✓ Assistant: %s\n", resp2.Content)
	fmt.Printf("Finish Reason: %s\n", resp2.FinishReason)

	// Show conversation history
	fmt.Println("\n\n--- Conversation History ---")
	messages := memory.AllMessages()
	fmt.Printf("Total messages in memory: %d\n\n", len(messages))
	for i, msg := range messages {
		content := msg.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		fmt.Printf("%d. [%s] %s\n", i+1, msg.Role, content)
	}

	// Summary
	fmt.Println("\n\n=== Summary ===")
	fmt.Println("The ReAct pattern automatically:")
	fmt.Println("1. Sends the user prompt to the LLM")
	fmt.Println("2. Detects when the LLM wants to use tools")
	fmt.Println("3. Executes the tools and adds results to memory")
	fmt.Println("4. Continues the loop until a final answer is reached")
	fmt.Println("5. Provides full observability (spans, logs, metrics)")
	fmt.Println("\nNew improvements:")
	fmt.Println("- Uses ReactPattern (not ReactClient) for clarity")
	fmt.Println("- Functional options pattern for configuration")
	fmt.Println("- No manual tool catalog needed (uses client tools)")
	fmt.Println("- Memory accessed directly from client")
	fmt.Println("- Case-insensitive tool lookup")
	fmt.Println("\nBenefits over manual tool execution (Layer 2):")
	fmt.Println("- 90% less boilerplate code")
	fmt.Println("- Automatic iteration management")
	fmt.Println("- Built-in observability")
	fmt.Println("- Error handling and max iteration protection")
	fmt.Println("- Reusable across different use cases")
}
