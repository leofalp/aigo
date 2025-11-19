package main

import (
	"context"
	"fmt"
	"log"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/patterns/react"
	"github.com/leofalp/aigo/providers/ai/openai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
	"github.com/leofalp/aigo/providers/observability/slogobs"
	"github.com/leofalp/aigo/providers/tool/calculator"
	"github.com/leofalp/aigo/providers/tool/duckduckgo"

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
	searchTool := duckduckgo.NewDuckDuckGoSearchTool()

	// Create base client with memory, tools, and observer
	baseClient, err := client.NewClient(
		openai.NewOpenAIProvider(),
		client.WithMemory(memory),
		client.WithObserver(slogobs.New()),
		client.WithTools(calcTool, searchTool),
		client.WithSystemPrompt("You are a helpful math assistant. Use tools when needed to provide accurate responses."),
		client.WithEnrichSystemPromptWithToolsDescriptions(), // Automatically adds tool descriptions and usage guidance
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create ReAct pattern with functional options
	// The ReAct pattern will use memory and tools from the base client
	reactPattern, err := react.NewReactPattern(
		baseClient,
		react.WithMaxIterations(10),
		react.WithStopOnError(true),
	)
	if err != nil {
		log.Fatalf("Failed to create ReAct pattern: %v", err)
	}

	ctx := context.Background()

	prompt := "Whats the sum of the first 3 prime numbers? And what is the capital of the country with that many letters in its name?"
	fmt.Printf("User: %s\n\n", prompt)

	respOverview, err := reactPattern.Execute(ctx, prompt)
	if err != nil {
		log.Fatalf("ReAct execution failed: %v", err)
	}

	fmt.Printf("\n✓ Assistant: %s\n", respOverview.LastResponse.Content)
	fmt.Printf("Finish Reason: %s\n", respOverview.LastResponse.FinishReason)
	fmt.Printf("Tokens Used: %d (prompt: %d, completion: %d)\n",
		respOverview.TotalUsage.TotalTokens,
		respOverview.TotalUsage.PromptTokens,
		respOverview.TotalUsage.CompletionTokens,
	)
	fmt.Printf("Tools used: %s\n", utils.ToString(respOverview.ToolCallStats))

	// Show conversation history
	fmt.Println("\n\n--- Conversation History ---")
	messages := memory.AllMessages()
	fmt.Printf("Total messages in memory: %d\n\n", len(messages))
	for i, msg := range messages {
		fmt.Printf("%d. [%s] %s\n\n", i+1, msg.Role, utils.JSONToString(msg, true))
	}
}
