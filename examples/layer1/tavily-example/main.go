// Package main demonstrates the Tavily search and content-extraction tool (Layer 1) with
// five scenarios: simple web search, AI-generated answer, news topic search, advanced
// search with images, and URL content extraction.
// Requires the TAVILY_API_KEY environment variable to be set.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/leofalp/aigo/providers/tool/tavily"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== Tavily Search API Example ===")
	fmt.Println("This example demonstrates the Tavily Search tool capabilities")

	// Check if API key is set
	if os.Getenv("TAVILY_API_KEY") == "" {
		log.Fatal("TAVILY_API_KEY environment variable is not set")
	}

	ctx := context.Background()

	// Example 1: Simple search
	fmt.Println("\n1. Simple Web Search")
	fmt.Println("----------------------------------------------------")
	simpleSearch(ctx)

	// Example 2: Search with AI-generated answer
	fmt.Println("\n\n2. Search with AI-Generated Answer")
	fmt.Println("----------------------------------------------------")
	searchWithAnswer(ctx)

	// Example 3: News search
	fmt.Println("\n\n3. News Topic Search")
	fmt.Println("----------------------------------------------------")
	newsSearch(ctx)

	// Example 4: Advanced search
	fmt.Println("\n\n4. Advanced Search (Detailed Results)")
	fmt.Println("----------------------------------------------------")
	advancedSearch(ctx)

	// Example 5: URL extraction
	fmt.Println("\n\n5. Extract Content from URLs")
	fmt.Println("----------------------------------------------------")
	extractContent(ctx)

	fmt.Println("\n\nAll examples completed successfully!")
}

func simpleSearch(ctx context.Context) {
	input := tavily.SearchInput{
		Query:      "Go programming language",
		MaxResults: 5,
	}

	fmt.Printf("Searching for: %q\n\n", input.Query)

	output, err := tavily.Search(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d results:\n\n", len(output.Results))

	for i, result := range output.Results {
		fmt.Printf("%d. %s\n", i+1, result.Title)
		fmt.Printf("   URL: %s\n", result.URL)
		fmt.Printf("   Score: %.2f\n", result.Score)
		fmt.Printf("   %s\n", truncateString(result.Content, 100))
		fmt.Println()
	}
}

func searchWithAnswer(ctx context.Context) {
	input := tavily.SearchInput{
		Query:         "What is the capital of France and its population?",
		MaxResults:    5,
		IncludeAnswer: true,
	}

	fmt.Printf("Searching for: %q (with AI answer)\n\n", input.Query)

	output, err := tavily.Search(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	if output.Answer != "" {
		fmt.Printf("AI-Generated Answer:\n%s\n\n", output.Answer)
	}

	fmt.Printf("Top %d supporting sources:\n", len(output.Results))
	for i, result := range output.Results {
		if i >= 3 { // Show top 3
			break
		}
		fmt.Printf("   %d. %s\n", i+1, result.Title)
		fmt.Printf("      %s\n", result.URL)
	}
}

func newsSearch(ctx context.Context) {
	input := tavily.SearchInput{
		Query:      "artificial intelligence latest developments",
		MaxResults: 5,
		Topic:      "news",
	}

	fmt.Printf("Searching for: %q (topic: news)\n\n", input.Query)

	output, err := tavily.Search(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d news results:\n\n", len(output.Results))

	for i, result := range output.Results {
		if i >= 3 { // Show top 3
			break
		}
		fmt.Printf("%d. %s\n", i+1, result.Title)
		fmt.Printf("   %s\n", truncateString(result.Content, 120))
		fmt.Println()
	}
}

func advancedSearch(ctx context.Context) {
	input := tavily.SearchInput{
		Query:         "Albert Einstein contributions to physics",
		MaxResults:    5,
		SearchDepth:   "advanced",
		IncludeAnswer: true,
		IncludeImages: true,
	}

	fmt.Printf("Advanced search for: %q\n\n", input.Query)

	output, err := tavily.SearchAdvanced(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Query: %s\n", output.Query)
	fmt.Printf("Response Time: %.2f seconds\n\n", output.ResponseTime)

	if output.Answer != "" {
		fmt.Printf("AI Answer:\n%s\n\n", truncateString(output.Answer, 300))
	}

	fmt.Printf("Results (%d):\n", len(output.Results))
	for i, result := range output.Results {
		if i >= 3 { // Show top 3
			break
		}
		fmt.Printf("   %d. %s (score: %.2f)\n", i+1, result.Title, result.Score)
		fmt.Printf("      URL: %s\n", result.URL)
	}

	if len(output.Images) > 0 {
		fmt.Printf("\nImages (%d):\n", len(output.Images))
		for i, img := range output.Images {
			if i >= 2 { // Show top 2
				break
			}
			fmt.Printf("   - %s\n", img.URL)
		}
	}
}

func extractContent(ctx context.Context) {
	input := tavily.ExtractInput{
		URLs: []string{
			"https://go.dev/doc/",
		},
		ExtractDepth: "basic",
	}

	fmt.Printf("Extracting content from %d URL(s)\n\n", len(input.URLs))

	output, err := tavily.Extract(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Extracted %d page(s):\n\n", len(output.Results))

	for _, result := range output.Results {
		fmt.Printf("URL: %s\n", result.URL)
		fmt.Printf("Content preview:\n%s\n", truncateString(result.RawContent, 300))
		fmt.Println()
	}
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
