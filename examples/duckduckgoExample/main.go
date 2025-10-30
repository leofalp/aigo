package main

import (
	"aigo/core/client"
	"aigo/providers/ai/openai"
	"aigo/providers/tool"
	"aigo/providers/tool/duckduckgo"
	"context"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"log"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file: %s", err)
	}

	fmt.Println("=== DuckDuckGo Search Tool - Usage Examples ===\n")

	// Case 1: Direct use - Base Version (Summary)
	fmt.Println("--- Case 1: Base Search (Summary) ---")
	exampleDirectBase()

	// Case 2: Direct use - Advanced Version (Structured)
	fmt.Println("\n--- Case 2: Advanced Search (Structured) ---")
	exampleDirectAdvanced()

	// Case 3: AI Integration - Base Version
	fmt.Println("\n--- Case 3: AI with Base Tool ---")
	exampleAIBase()

	// Case 4: AI Integration - Advanced Version
	fmt.Println("\n--- Case 4: AI with Advanced Tool ---")
	exampleAIAdvanced()
}

func exampleDirectBase() {
	input := duckduckgo.Input{Query: "Go programming language"}
	output, err := duckduckgo.Search(context.Background(), input)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Query: %s\n", output.Query)
	fmt.Printf("Summary:\n%s\n", output.Summary)
}

func exampleDirectAdvanced() {
	input := duckduckgo.Input{Query: "Albert Einstein"}
	output, err := duckduckgo.SearchAdvanced(context.Background(), input)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Show structured output
	fmt.Printf("Query: %s\n", output.Query)
	if output.Abstract != "" {
		fmt.Printf("Abstract: %s\n", truncate(output.Abstract, 150))
		fmt.Printf("Source: %s (%s)\n", output.AbstractSource, output.AbstractURL)
	}
	if output.Image != "" {
		fmt.Printf("Image: %s (%sx%s)\n", output.Image, output.ImageWidth, output.ImageHeight)
	}
	fmt.Printf("Related Topics: %d\n", len(output.RelatedTopics))

	// Also show complete JSON (optional)
	if jsonBytes, err := json.MarshalIndent(output, "", "  "); err == nil {
		fmt.Printf("\nComplete JSON (first 500 chars):\n%s...\n", truncate(string(jsonBytes), 500))
	}
}

func exampleAIBase() {
	c := client.NewClient[string](
		openai.NewOpenAIProvider().
			WithModels([]string{"nvidia/nemotron-nano-9b-v2:free"}).
			WithModel("nvidia/nemotron-nano-9b-v2:free"),
	).
		AddTools([]tool.GenericTool{duckduckgo.NewDuckDuckGoSearchTool()}).
		AddSystemPrompt("You are a helpful assistant. Use the search tool to find information.").
		SetMaxToolCallIterations(3)

	resp, err := c.SendMessage("What is Go programming language?")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(resp.Content)
}

func exampleAIAdvanced() {
	c := client.NewClient[string](
		openai.NewOpenAIProvider().
			WithModels([]string{"nvidia/nemotron-nano-9b-v2:free"}).
			WithModel("nvidia/nemotron-nano-9b-v2:free"),
	).
		AddTools([]tool.GenericTool{duckduckgo.NewDuckDuckGoSearchAdvancedTool()}).
		AddSystemPrompt("You are a helpful assistant. Use the advanced search to get detailed structured data with sources.").
		SetMaxToolCallIterations(3)

	resp, err := c.SendMessage("Tell me about Albert Einstein with sources")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(resp.Content)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
