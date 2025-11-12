package main

import (
	"context"
	"fmt"
	"log"

	"github.com/leofalp/aigo/providers/tool/urlextractor"
)

func main() {
	fmt.Println("=== URLExtractor Tool Example ===")
	fmt.Println("This example demonstrates extracting URLs from websites using various methods")

	ctx := context.Background()

	// Example 1: Simple extraction with default settings
	fmt.Println("📄 Example 1: Basic URL Extraction")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	basicExtraction(ctx)

	// Example 2: Extraction with custom limits
	fmt.Println("\n\n⚙️  Example 2: Custom Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	customConfiguration(ctx)

	// Example 3: Sitemap-only extraction (no crawling)
	fmt.Println("\n\n🗺️  Example 3: Sitemap-Only Extraction")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	sitemapOnly(ctx)

	// Example 4: Deep crawling with statistics
	fmt.Println("\n\n🔍 Example 4: Deep Crawling with Statistics")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	deepCrawling(ctx)

	fmt.Println("\n\n✅ All examples completed successfully!")
}

func basicExtraction(ctx context.Context) {
	// Simple extraction with partial URL
	input := urlextractor.Input{
		URL: "neosperience.com", // Automatically becomes https://example.com
	}

	fmt.Printf("Extracting URLs from: %s\n", input.URL)

	output, err := urlextractor.Extract(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Base URL: %s\n", output.BaseURL)
	fmt.Printf("Total URLs found: %d\n", output.TotalURLs)
	fmt.Printf("Robots.txt found: %v\n", output.RobotsTxtFound)
	fmt.Printf("Sitemap found: %v\n", output.SitemapFound)

	// Show first 5 URLs
	fmt.Println("\nFirst 5 URLs:")
	for i, url := range output.URLs {
		if i >= 5 {
			break
		}
		fmt.Printf("  %d. %s\n", i+1, url)
	}

	if len(output.URLs) > 5 {
		fmt.Printf("  ... and %d more\n", len(output.URLs)-5)
	}
}

func customConfiguration(ctx context.Context) {
	// Extraction with custom limits and timeout
	input := urlextractor.Input{
		URL:                    "example.org",
		MaxURLs:                50, // Extract maximum 50 URLs
		TimeoutSeconds:         60, // 1 minute timeout
		UserAgent:              "URLExtractor-Example/1.0",
		CrawlDelayMs:           200, // 200ms delay between requests
		ForceRecursiveCrawling: true,
	}

	fmt.Printf("Extracting URLs from: %s\n", input.URL)
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Max URLs: %d\n", input.MaxURLs)
	fmt.Printf("  Timeout: %d seconds\n", input.TimeoutSeconds)
	fmt.Printf("  User-Agent: %s\n", input.UserAgent)
	fmt.Printf("  Crawl Delay: %dms\n", input.CrawlDelayMs)
	fmt.Printf("  Force Crawling: %v\n\n", input.ForceRecursiveCrawling)

	output, err := urlextractor.Extract(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("✓ Extraction completed\n")
	fmt.Printf("Total URLs: %d\n", output.TotalURLs)

	// Show source statistics
	if len(output.Sources) > 0 {
		fmt.Println("\nURL Sources:")
		for source, count := range output.Sources {
			fmt.Printf("  %s: %d URLs\n", source, count)
		}
	}
}

func sitemapOnly(ctx context.Context) {
	// Extract only from sitemap, don't force crawling
	input := urlextractor.Input{
		URL:     "example.net",
		MaxURLs: 1000,
		// ForceRecursiveCrawling defaults to false, so only sitemap will be used
	}

	fmt.Printf("Extracting URLs from: %s (sitemap only)\n", input.URL)

	output, err := urlextractor.Extract(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	if !output.SitemapFound {
		fmt.Println("⚠️  No sitemap found (crawling was disabled)")
		fmt.Printf("Total URLs: %d\n", output.TotalURLs)
		return
	}

	fmt.Printf("✓ Sitemap found and processed\n")
	fmt.Printf("Total URLs from sitemap: %d\n", output.TotalURLs)

	// Categorize URLs by path
	categorizeURLs(output.URLs)
}

func deepCrawling(ctx context.Context) {
	// Comprehensive crawling with statistics
	input := urlextractor.Input{
		URL:                    "example.com",
		MaxURLs:                500, // Allow up to 500 URLs
		ForceRecursiveCrawling: true,
		CrawlDelayMs:           150, // Polite 150ms delay
		TimeoutSeconds:         120, // 2 minute timeout
	}

	fmt.Printf("Performing comprehensive crawl of: %s\n", input.URL)
	fmt.Printf("Max URLs: %d\n", input.MaxURLs)
	fmt.Printf("Crawl Delay: %dms\n\n", input.CrawlDelayMs)

	output, err := urlextractor.Extract(ctx, input)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Extraction Summary:")
	fmt.Println("─────────────────────────────────────────────")
	fmt.Printf("Base URL:         %s\n", output.BaseURL)
	fmt.Printf("Total URLs:       %d\n", output.TotalURLs)
	fmt.Printf("Robots.txt:       %v\n", formatBool(output.RobotsTxtFound))
	fmt.Printf("Sitemap:          %v\n", formatBool(output.SitemapFound))

	fmt.Println("\nSource Breakdown:")
	fmt.Println("─────────────────────────────────────────────")
	totalFromSources := 0
	for source, count := range output.Sources {
		fmt.Printf("%-15s: %d URLs\n", source, count)
		totalFromSources += count
	}

	if totalFromSources == 0 {
		fmt.Println("(No source information available)")
	}

	// Analyze URL structure
	fmt.Println("\nURL Analysis:")
	fmt.Println("─────────────────────────────────────────────")
	analyzeURLStructure(output.URLs)
}

// Helper function to categorize URLs by their path
func categorizeURLs(urls []string) {
	if len(urls) == 0 {
		return
	}

	categories := make(map[string]int)
	for _, url := range urls {
		// Simple categorization by first path segment
		category := "root"
		if idx := len("https://"); idx < len(url) {
			remaining := url[idx:]
			if slashIdx := 0; slashIdx < len(remaining) {
				for i, ch := range remaining {
					if ch == '/' {
						slashIdx = i
						break
					}
				}
				if slashIdx > 0 && slashIdx < len(remaining)-1 {
					pathStart := slashIdx + 1
					pathEnd := pathStart
					for i := pathStart; i < len(remaining); i++ {
						if remaining[i] == '/' {
							pathEnd = i
							break
						}
					}
					if pathEnd == pathStart {
						pathEnd = len(remaining)
					}
					if pathEnd > pathStart {
						category = remaining[pathStart:pathEnd]
					}
				}
			}
		}
		categories[category]++
	}

	fmt.Println("\nURL Categories (by first path segment):")
	for category, count := range categories {
		fmt.Printf("  /%s: %d URLs\n", category, count)
	}
}

// Helper function to analyze URL structure
func analyzeURLStructure(urls []string) {
	if len(urls) == 0 {
		fmt.Println("No URLs to analyze")
		return
	}

	// Count URLs by depth (number of path segments)
	depthCount := make(map[int]int)
	maxDepth := 0

	for _, url := range urls {
		depth := 0
		inPath := false
		for i, ch := range url {
			if !inPath && i > 8 { // Skip https://
				inPath = true
			}
			if inPath && ch == '/' {
				depth++
			}
		}

		depthCount[depth]++
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	fmt.Printf("URLs by depth (path segments):\n")
	for d := 0; d <= maxDepth; d++ {
		if count, ok := depthCount[d]; ok {
			fmt.Printf("  Depth %d: %d URLs\n", d, count)
		}
	}

	// Count file extensions
	extensions := make(map[string]int)
	for _, url := range urls {
		// Simple extension detection
		lastDot := -1
		lastSlash := -1
		for i := len(url) - 1; i >= 0; i-- {
			if url[i] == '.' && lastDot == -1 {
				lastDot = i
			}
			if url[i] == '/' {
				lastSlash = i
				break
			}
		}

		ext := "no extension"
		if lastDot > lastSlash && lastDot < len(url)-1 {
			ext = url[lastDot+1:]
			// Limit extension length
			if len(ext) > 10 {
				ext = "no extension"
			}
		}
		extensions[ext]++
	}

	if len(extensions) > 1 || (len(extensions) == 1 && extensions["no extension"] != len(urls)) {
		fmt.Printf("\nFile extensions:\n")
		for ext, count := range extensions {
			fmt.Printf("  .%s: %d URLs\n", ext, count)
		}
	}
}

// Helper function to format boolean values
func formatBool(b bool) string {
	if b {
		return "✓ Found"
	}
	return "✗ Not found"
}
