package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"strings"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/patterns/react"
	"github.com/leofalp/aigo/providers/ai/openai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
	"github.com/leofalp/aigo/providers/observability/slogobs"
	"github.com/leofalp/aigo/providers/tool/bravesearch"
	"github.com/leofalp/aigo/providers/tool/calculator"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== ReAct Pattern with Brave Search Example ===")
	fmt.Println("This example demonstrates how an AI agent uses Brave Search")
	fmt.Println("to find real-time information from the web.")

	// Check required API keys
	if os.Getenv("BRAVE_SEARCH_API_KEY") == "" {
		log.Fatal("BRAVE_SEARCH_API_KEY environment variable is not set")
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("OPENAI_API_KEY environment variable is not set")
	}

	// Create memory provider
	memory := inmemory.New()

	// Create tools
	searchTool := bravesearch.NewBraveSearchTool()
	calcTool := calculator.NewCalculatorTool()

	// Create base client with tools
	baseClient, err := client.NewClient(
		openai.NewOpenAIProvider(),
		client.WithMemory(memory),
		client.WithObserver(slogobs.New()),
		client.WithTools(searchTool, calcTool),
		client.WithSystemPrompt("You are a helpful research assistant with access to web search. Use the search tool to find current, accurate information when needed."),
		client.WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create ReAct pattern
	reactPattern, err := react.NewReactPattern(
		baseClient,
		react.WithMaxIterations(5),
		react.WithStopOnError(false),
	)
	if err != nil {
		log.Fatalf("Failed to create ReAct pattern: %v", err)
	}

	ctx := context.Background()

	// Example queries that require web search
	queries := []string{
		"What are the latest developments in quantum computing this week?",
		"Who won the most recent Nobel Prize in Physics and what was it for?",
		"Find information about the Go programming language version 1.22 features",
	}

	for i, prompt := range queries {
		fmt.Printf("\n%s\n", strings.Repeat("━", 80))
		fmt.Printf("Query %d: %s\n", i+1, prompt)
		fmt.Printf("%s\n\n", strings.Repeat("━", 80))

		respOverview, err := reactPattern.Execute(ctx, prompt)
		if err != nil {
			log.Printf("ReAct execution failed: %v\n", err)
			continue
		}

		fmt.Printf("\n✓ Assistant Response:\n%s\n", respOverview.LastResponse.Content)
		fmt.Printf("\nFinish Reason: %s\n", respOverview.LastResponse.FinishReason)
		fmt.Printf("Total Requests: %d\n", len(respOverview.Requests))
		fmt.Printf("Total Responses: %d\n", len(respOverview.Responses))
		fmt.Printf("Tokens Used: %d (prompt: %d, completion: %d)\n",
			respOverview.TotalUsage.TotalTokens,
			respOverview.TotalUsage.PromptTokens,
			respOverview.TotalUsage.CompletionTokens,
		)
	}

	// Show full conversation history for last query
	fmt.Printf("\n\n%s\n", strings.Repeat("━", 80))
	fmt.Println("Conversation History (Last Query)")
	fmt.Printf("%s\n", strings.Repeat("━", 80))
	messages := memory.AllMessages()
	fmt.Printf("Total messages in memory: %d\n\n", len(messages))

	// Show last few messages to see tool interaction
	start := len(messages) - 6
	if start < 0 {
		start = 0
	}

	for i := start; i < len(messages); i++ {
		msg := messages[i]
		fmt.Printf("%d. [%s]\n", i+1, msg.Role)

		switch msg.Role {
		case "user":
			fmt.Printf("   Content: %s\n", msg.Content)
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				fmt.Printf("   Tool Calls: %d\n", len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					fmt.Printf("   - %s: %s\n", tc.Function.Name, tc.Function.Arguments)
				}
			} else {
				preview := msg.Content
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				fmt.Printf("   Content: %s\n", preview)
			}
		case "tool":
			fmt.Printf("   Tool: %s\n", msg.Name)
			preview := msg.Content
			if len(preview) > 150 {
				preview = preview[:150] + "..."
			}
			fmt.Printf("   Result: %s\n", preview)
		}
		fmt.Println()
	}

	fmt.Println("\n✅ All examples completed!")
}
