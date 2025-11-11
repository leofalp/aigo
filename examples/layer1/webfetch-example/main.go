package main

import (
	"context"
	"fmt"
	"log"

	"github.com/leofalp/aigo/providers/tool/webfetch"
)

func main() {
	fmt.Println("=== WebFetch Tool Example ===")
	fmt.Println("This example demonstrates fetching web pages and converting them to Markdown\n")

	ctx := context.Background()

	// Example 1: Simple fetch with partial URL
	fmt.Println("📄 Example 1: Fetch with Partial URL")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	simpleFetch(ctx)

	// Example 2: Fetch with redirect detection
	fmt.Println("\n\n🔄 Example 2: Fetch with Redirect")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fetchWithRedirect(ctx)

	// Example 3: Fetch with custom timeout
	fmt.Println("\n\n⏱️  Example 3: Fetch with Custom Timeout")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fetchWithTimeout(ctx)

	fmt.Println("\n\n✅ All examples completed successfully!")
}

func simpleFetch(ctx context.Context) {
	// Partial URLs are automatically normalized to https://
	input := webfetch.Input{
		URL: "example.com",
	}

	fmt.Printf("Fetching: %s\n", input.URL)

	output, err := webfetch.Fetch(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Final URL: %s\n", output.URL)
	fmt.Printf("Markdown length: %d characters\n\n", len(output.Markdown))

	// Show first 200 characters of markdown
	preview := output.Markdown
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	fmt.Printf("Content preview:\n%s\n", preview)
}

func fetchWithRedirect(ctx context.Context) {
	// This URL will redirect to www.google.com
	input := webfetch.Input{
		URL: "google.it",
	}

	fmt.Printf("Fetching: %s\n", input.URL)

	output, err := webfetch.Fetch(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	// Check if redirect happened
	normalizedInput := "https://" + input.URL
	if output.URL != normalizedInput {
		fmt.Printf("✓ Redirect detected!\n")
		fmt.Printf("  Original:  %s\n", normalizedInput)
		fmt.Printf("  Final URL: %s\n", output.URL)
	}

	fmt.Printf("\nMarkdown length: %d characters\n", len(output.Markdown))
}

func fetchWithTimeout(ctx context.Context) {
	input := webfetch.Input{
		URL:            "example.org",
		TimeoutSeconds: 10,
		UserAgent:      "WebFetch-Example/1.0",
	}

	fmt.Printf("Fetching: %s\n", input.URL)
	fmt.Printf("Timeout: %d seconds\n", input.TimeoutSeconds)
	fmt.Printf("User-Agent: %s\n\n", input.UserAgent)

	output, err := webfetch.Fetch(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Final URL: %s\n", output.URL)
	fmt.Printf("Successfully fetched %d characters\n", len(output.Markdown))
}
