// Package main demonstrates the Exa AI-native search tool (Layer 1) with five scenarios:
// semantic web search, research-paper category search, finding similar pages, AI-generated
// answers with citations, and advanced search with highlights.
// Requires the EXA_API_KEY environment variable to be set.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/leofalp/aigo/providers/tool/exa"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== Exa Search API Example ===")
	fmt.Println("This example demonstrates the Exa AI-native search capabilities")

	// Check if API key is set
	if os.Getenv("EXA_API_KEY") == "" {
		log.Fatal("EXA_API_KEY environment variable is not set")
	}

	ctx := context.Background()

	// Example 1: Simple semantic search
	fmt.Println("\n1. Semantic Web Search")
	fmt.Println("----------------------------------------------------")
	simpleSearch(ctx)

	// Example 2: Research paper search with category filter
	fmt.Println("\n\n2. Research Paper Search")
	fmt.Println("----------------------------------------------------")
	researchSearch(ctx)

	// Example 3: Find similar content
	fmt.Println("\n\n3. Find Similar Content")
	fmt.Println("----------------------------------------------------")
	findSimilar(ctx)

	// Example 4: AI-generated answer with citations
	fmt.Println("\n\n4. AI Answer with Citations")
	fmt.Println("----------------------------------------------------")
	getAnswer(ctx)

	// Example 5: Advanced search with highlights
	fmt.Println("\n\n5. Advanced Search with Highlights")
	fmt.Println("----------------------------------------------------")
	advancedSearch(ctx)

	fmt.Println("\n\nAll examples completed successfully!")
}

func simpleSearch(ctx context.Context) {
	input := exa.SearchInput{
		Query:      "Go programming language concurrency patterns",
		NumResults: 5,
	}

	fmt.Printf("Searching for: %q\n\n", input.Query)

	output, err := exa.Search(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d results:\n\n", len(output.Results))

	for i, result := range output.Results {
		fmt.Printf("%d. %s\n", i+1, result.Title)
		fmt.Printf("   URL: %s\n", result.URL)
		if result.Author != "" {
			fmt.Printf("   Author: %s\n", result.Author)
		}
		if result.PublishedDate != "" {
			fmt.Printf("   Published: %s\n", result.PublishedDate)
		}
		fmt.Println()
	}
}

func researchSearch(ctx context.Context) {
	input := exa.SearchInput{
		Query:      "transformer architecture attention mechanism",
		NumResults: 5,
		Category:   "research paper",
	}

	fmt.Printf("Searching for: %q (category: research paper)\n\n", input.Query)

	output, err := exa.Search(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d research papers:\n\n", len(output.Results))

	for i, result := range output.Results {
		if i >= 3 { // Show top 3
			break
		}
		fmt.Printf("%d. %s\n", i+1, result.Title)
		fmt.Printf("   URL: %s\n", result.URL)
		if result.Author != "" {
			fmt.Printf("   Author: %s\n", result.Author)
		}
		fmt.Println()
	}
}

func findSimilar(ctx context.Context) {
	input := exa.SimilarInput{
		URL:        "https://go.dev/blog/",
		NumResults: 5,
	}

	fmt.Printf("Finding content similar to: %s\n\n", input.URL)

	output, err := exa.FindSimilar(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d similar pages:\n\n", len(output.Results))

	for i, result := range output.Results {
		if i >= 3 { // Show top 3
			break
		}
		fmt.Printf("%d. %s\n", i+1, result.Title)
		fmt.Printf("   URL: %s\n", result.URL)
		fmt.Println()
	}
}

func getAnswer(ctx context.Context) {
	input := exa.AnswerInput{
		Query:       "What are the main advantages of using Go for backend development?",
		IncludeText: true,
	}

	fmt.Printf("Question: %s\n\n", input.Query)

	output, err := exa.Answer(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Answer:\n%s\n\n", truncateString(output.Answer, 500))

	if len(output.Citations) > 0 {
		fmt.Printf("Citations (%d sources):\n", len(output.Citations))
		for i, citation := range output.Citations {
			if i >= 3 { // Show top 3
				break
			}
			fmt.Printf("   %d. %s\n", i+1, citation.Title)
			fmt.Printf("      %s\n", citation.URL)
		}
	}
}

func advancedSearch(ctx context.Context) {
	input := exa.SearchInput{
		Query:             "kubernetes deployment best practices",
		NumResults:        5,
		Type:              "neural",
		IncludeText:       true,
		IncludeHighlights: true,
	}

	fmt.Printf("Advanced search for: %q\n\n", input.Query)

	output, err := exa.SearchAdvanced(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Query: %s\n", output.Query)
	fmt.Printf("Search Type: %s\n\n", output.ResolvedSearchType)

	fmt.Printf("Results (%d):\n", len(output.Results))
	for i, result := range output.Results {
		if i >= 3 { // Show top 3
			break
		}
		fmt.Printf("\n%d. %s\n", i+1, result.Title)
		fmt.Printf("   URL: %s\n", result.URL)
		if result.Score > 0 {
			fmt.Printf("   Score: %.4f\n", result.Score)
		}

		if len(result.Highlights) > 0 {
			fmt.Printf("   Highlight: %s\n", truncateString(result.Highlights[0], 150))
		}
	}
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
