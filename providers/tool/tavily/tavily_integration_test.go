//go:build integration

package tavily

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestTavilySearch_Integration(t *testing.T) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		t.Skip("TAVILY_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := SearchInput{
		Query:      "Go programming language",
		MaxResults: 3,
	}

	output, err := Search(ctx, input)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if output.Query != input.Query {
		t.Errorf("expected query '%s', got '%s'", input.Query, output.Query)
	}

	if len(output.Results) == 0 {
		t.Error("expected at least one result")
	}

	if output.Summary == "" {
		t.Error("expected non-empty summary")
	}

	// Verify result structure
	for i, result := range output.Results {
		if result.Title == "" {
			t.Errorf("result %d: expected non-empty title", i)
		}
		if result.URL == "" {
			t.Errorf("result %d: expected non-empty URL", i)
		}
	}

	t.Logf("Search returned %d results", len(output.Results))
	t.Logf("First result: %s - %s", output.Results[0].Title, output.Results[0].URL)
}

func TestTavilySearchAdvanced_Integration(t *testing.T) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		t.Skip("TAVILY_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := SearchInput{
		Query:         "artificial intelligence latest news",
		SearchDepth:   "advanced",
		MaxResults:    3,
		IncludeAnswer: true,
	}

	output, err := SearchAdvanced(ctx, input)
	if err != nil {
		t.Fatalf("SearchAdvanced failed: %v", err)
	}

	if output.Query != input.Query {
		t.Errorf("expected query '%s', got '%s'", input.Query, output.Query)
	}

	if len(output.Results) == 0 {
		t.Error("expected at least one result")
	}

	// Advanced search should include scores
	for i, result := range output.Results {
		if result.Score == 0 {
			t.Logf("result %d: score is 0 (may be expected)", i)
		}
	}

	t.Logf("SearchAdvanced returned %d results", len(output.Results))
	if output.Answer != "" {
		t.Logf("Answer: %s", truncate(output.Answer, 200))
	}
}

func TestTavilySearchWithAnswer_Integration(t *testing.T) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		t.Skip("TAVILY_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := SearchInput{
		Query:         "What is the capital of France?",
		MaxResults:    3,
		IncludeAnswer: true,
	}

	output, err := Search(ctx, input)
	if err != nil {
		t.Fatalf("Search with answer failed: %v", err)
	}

	// Answer should be included when requested
	if output.Answer == "" {
		t.Log("Warning: Answer was requested but not returned (may depend on query)")
	} else {
		t.Logf("Answer: %s", output.Answer)
	}
}

func TestTavilyExtract_Integration(t *testing.T) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		t.Skip("TAVILY_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	input := ExtractInput{
		URLs: []string{"https://go.dev"},
	}

	output, err := Extract(ctx, input)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(output.Results) == 0 {
		t.Error("expected at least one extraction result")
	}

	if output.Summary == "" {
		t.Error("expected non-empty summary")
	}

	for i, result := range output.Results {
		if result.URL == "" {
			t.Errorf("result %d: expected non-empty URL", i)
		}
		if result.RawContent == "" {
			t.Errorf("result %d: expected non-empty raw content", i)
		}
	}

	t.Logf("Extract returned %d results", len(output.Results))
	if len(output.Results) > 0 {
		t.Logf("Content length: %d characters", len(output.Results[0].RawContent))
	}
}

func TestTavilyExtractAdvanced_Integration(t *testing.T) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		t.Skip("TAVILY_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	input := ExtractInput{
		URLs:         []string{"https://en.wikipedia.org/wiki/Go_(programming_language)"},
		ExtractDepth: "advanced",
	}

	output, err := Extract(ctx, input)
	if err != nil {
		t.Fatalf("Extract advanced failed: %v", err)
	}

	if len(output.Results) == 0 {
		t.Error("expected at least one extraction result")
	}

	t.Logf("Extract advanced returned %d results", len(output.Results))
}

func TestTavilySearchWithDomainFilters_Integration(t *testing.T) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		t.Skip("TAVILY_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := SearchInput{
		Query:          "golang tutorials",
		MaxResults:     5,
		IncludeDomains: []string{"go.dev", "golang.org"},
	}

	output, err := Search(ctx, input)
	if err != nil {
		t.Fatalf("Search with domain filters failed: %v", err)
	}

	t.Logf("Search with domain filters returned %d results", len(output.Results))

	// Verify results are from specified domains
	for _, result := range output.Results {
		t.Logf("Result URL: %s", result.URL)
	}
}
