package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/leofalp/aigo/providers/tool/bravesearch"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== Brave Search API Example ===")
	fmt.Println("This example demonstrates the Brave Search tool capabilities\n")

	// Check if API key is set
	if os.Getenv("BRAVE_SEARCH_API_KEY") == "" {
		log.Fatal("BRAVE_SEARCH_API_KEY environment variable is not set")
	}

	ctx := context.Background()

	// Example 1: Simple search
	fmt.Println("📝 Example 1: Simple Web Search")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	simpleSearch(ctx)

	// Example 2: Search with localization
	fmt.Println("\n\n📍 Example 2: Localized Search")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	localizedSearch(ctx)

	// Example 3: Recent news search
	fmt.Println("\n\n📰 Example 3: Recent News Search")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	newsSearch(ctx)

	// Example 4: Advanced search with full data
	fmt.Println("\n\n🔬 Example 4: Advanced Search (Full Data)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	advancedSearch(ctx)

	// Example 5: Technical query
	fmt.Println("\n\n💻 Example 5: Technical Query")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	technicalSearch(ctx)

	fmt.Println("\n\n✅ All examples completed successfully!")
}

func simpleSearch(ctx context.Context) {
	input := bravesearch.Input{
		Query: "Go programming language",
		Count: 5,
	}

	fmt.Printf("Searching for: %q\n\n", input.Query)

	output, err := bravesearch.Search(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d results:\n\n", len(output.Results))

	for i, result := range output.Results {
		fmt.Printf("%d. %s\n", i+1, result.Title)
		fmt.Printf("   URL: %s\n", result.URL)
		fmt.Printf("   %s\n", truncateString(result.Description, 100))
		if result.Age != "" {
			fmt.Printf("   Age: %s\n", result.Age)
		}
		fmt.Println()
	}
}

func localizedSearch(ctx context.Context) {
	input := bravesearch.Input{
		Query:      "best pizza restaurants",
		Count:      5,
		Country:    "us",
		SearchLang: "en",
	}

	fmt.Printf("Searching for: %q (US, English)\n\n", input.Query)

	output, err := bravesearch.Search(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d localized results:\n\n", len(output.Results))

	for i, result := range output.Results {
		if i >= 3 { // Show top 3
			break
		}
		fmt.Printf("%d. %s\n", i+1, result.Title)
		fmt.Printf("   %s\n", result.URL)
		fmt.Println()
	}
}

func newsSearch(ctx context.Context) {
	input := bravesearch.Input{
		Query:     "artificial intelligence news",
		Count:     5,
		Freshness: "pw", // Past week
	}

	fmt.Printf("Searching for: %q (past week)\n\n", input.Query)

	output, err := bravesearch.Search(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d recent results:\n\n", len(output.Results))

	for i, result := range output.Results {
		if i >= 3 { // Show top 3
			break
		}
		fmt.Printf("%d. %s\n", i+1, result.Title)
		if result.Age != "" {
			fmt.Printf("   Published: %s\n", result.Age)
		}
		fmt.Printf("   %s\n", truncateString(result.Description, 120))
		fmt.Println()
	}
}

func advancedSearch(ctx context.Context) {
	input := bravesearch.Input{
		Query: "Albert Einstein",
		Count: 5,
	}

	fmt.Printf("Advanced search for: %q\n\n", input.Query)

	output, err := bravesearch.SearchAdvanced(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Query: %s\n", output.Query)
	fmt.Printf("Type: %s\n\n", output.Type)

	// Check for infobox
	if output.Infobox != nil {
		fmt.Println("📦 Infobox Found:")
		fmt.Printf("   Label: %s\n", output.Infobox.Label)
		if output.Infobox.Category != "" {
			fmt.Printf("   Category: %s\n", output.Infobox.Category)
		}
		if output.Infobox.ShortDesc != "" {
			fmt.Printf("   Description: %s\n", truncateString(output.Infobox.ShortDesc, 150))
		}
		if output.Infobox.Website != "" {
			fmt.Printf("   Website: %s\n", output.Infobox.Website)
		}
		fmt.Println()
	}

	// Check web results
	if output.Web != nil && len(output.Web.Results) > 0 {
		fmt.Printf("🌐 Web Results: %d\n", len(output.Web.Results))
		for i, result := range output.Web.Results {
			if i >= 2 { // Show top 2
				break
			}
			fmt.Printf("   %d. %s\n", i+1, result.Title)
			fmt.Printf("      %s\n", result.URL)
		}
		fmt.Println()
	}

	// Check news results
	if output.News != nil && len(output.News.Results) > 0 {
		fmt.Printf("📰 News Results: %d\n", len(output.News.Results))
		for i, news := range output.News.Results {
			if i >= 2 { // Show top 2
				break
			}
			fmt.Printf("   %d. %s (%s)\n", i+1, news.Title, news.Age)
		}
		fmt.Println()
	}

	// Check video results
	if output.Videos != nil && len(output.Videos.Results) > 0 {
		fmt.Printf("🎥 Video Results: %d\n", len(output.Videos.Results))
	}
}

func technicalSearch(ctx context.Context) {
	input := bravesearch.Input{
		Query: "how to implement JWT authentication in Go",
		Count: 5,
	}

	fmt.Printf("Searching for: %q\n\n", input.Query)

	output, err := bravesearch.Search(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("📚 Technical Results:\n\n")
	fmt.Println(output.Summary)
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
