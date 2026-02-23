// Package main demonstrates the ReAct pattern with the Brave Search tool (Layer 3). An agent
// autonomously decides when to call search or calculator tools across three prompts requiring
// real-time web information, and prints the full conversation history for the last query.
// Requires the BRAVE_SEARCH_API_KEY and OPENAI_API_KEY environment variables.
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
	fmt.Println("=== ReAct with Brave Search Example ===")
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
	baseClient, err := client.New(
		openai.New(),
		client.WithMemory(memory),
		client.WithObserver(slogobs.New()),
		client.WithTools(searchTool, calcTool),
		client.WithSystemPrompt("You are a helpful research assistant with access to web search. Use the search tool to find current, accurate information when needed."),
		client.WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create ReAct agent (using string for untyped text output)
	agent, err := react.New[string](
		baseClient,
		react.WithMaxIterations(5),
		react.WithStopOnError(false),
	)
	if err != nil {
		log.Fatalf("Failed to create ReAct agent: %v", err)
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

		result, err := agent.Execute(ctx, prompt)
		if err != nil {
			log.Printf("ReAct execution failed: %v\n", err)
			continue
		}

		fmt.Printf("\n✓ Assistant Response:\n%s\n", result.LastResponse.Content)
		fmt.Printf("\nFinish Reason: %s\n", result.LastResponse.FinishReason)
		fmt.Printf("Total Requests: %d\n", len(result.Requests))
		fmt.Printf("Total Responses: %d\n", len(result.Responses))
		fmt.Printf("Tokens Used: %d (prompt: %d, completion: %d)\n",
			result.TotalUsage.TotalTokens,
			result.TotalUsage.PromptTokens,
			result.TotalUsage.CompletionTokens,
		)
	}

	// Show full conversation history for last query
	fmt.Printf("\n\n%s\n", strings.Repeat("━", 80))
	fmt.Println("Conversation History (Last Query)")
	fmt.Printf("%s\n", strings.Repeat("━", 80))
	messages, err := memory.AllMessages(ctx)
	if err != nil {
		log.Fatalf("Failed to retrieve messages from memory: %v", err)
	}
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
