//go:build integration

package exa

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leofalp/aigo/internal/utils"
)

func TestExaSearch_Integration(t *testing.T) {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		t.Skip("EXA_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := SearchInput{
		Query:      "Go programming language",
		NumResults: 3,
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

func TestExaSearchAdvanced_Integration(t *testing.T) {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		t.Skip("EXA_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := SearchInput{
		Query:             "machine learning research papers",
		Type:              "neural",
		NumResults:        3,
		Category:          "research paper",
		IncludeText:       true,
		IncludeHighlights: true,
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

	t.Logf("SearchAdvanced returned %d results", len(output.Results))
	t.Logf("Resolved search type: %s", output.ResolvedSearchType)

	for i, result := range output.Results {
		t.Logf("Result %d: %s (score: %.4f)", i+1, result.Title, result.Score)
		if result.Text != "" {
			t.Logf("  Text length: %d chars", len(result.Text))
		}
		if len(result.Highlights) > 0 {
			t.Logf("  Highlights: %d", len(result.Highlights))
		}
	}
}

func TestExaSearchWithCategory_Integration(t *testing.T) {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		t.Skip("EXA_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := SearchInput{
		Query:      "OpenAI",
		NumResults: 3,
		Category:   "company",
	}

	output, err := Search(ctx, input)
	if err != nil {
		t.Fatalf("Search with category failed: %v", err)
	}

	t.Logf("Search with category 'company' returned %d results", len(output.Results))
	for _, result := range output.Results {
		t.Logf("  %s - %s", result.Title, result.URL)
	}
}

func TestExaFindSimilar_Integration(t *testing.T) {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		t.Skip("EXA_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := SimilarInput{
		URL:        "https://go.dev",
		NumResults: 3,
	}

	output, err := FindSimilar(ctx, input)
	if err != nil {
		t.Fatalf("FindSimilar failed: %v", err)
	}

	if len(output.Results) == 0 {
		t.Error("expected at least one similar result")
	}

	if output.Summary == "" {
		t.Error("expected non-empty summary")
	}

	t.Logf("FindSimilar returned %d results", len(output.Results))
	for i, result := range output.Results {
		t.Logf("Similar %d: %s - %s", i+1, result.Title, result.URL)
	}
}

func TestExaFindSimilarByText_Integration(t *testing.T) {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		t.Skip("EXA_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// The Exa /findSimilar API requires a URL — text-only similarity is not
	// supported. This test verifies that FindSimilar correctly rejects
	// text-only input before hitting the API.
	input := SimilarInput{
		NumResults: 3,
	}

	_, err := FindSimilar(ctx, input)
	if err == nil {
		t.Fatal("expected error when URL is empty, but got nil")
	}

	if !strings.Contains(err.Error(), "url is required") {
		t.Errorf("expected error about url requirement, got: %s", err.Error())
	}

	t.Log("Correctly rejected empty URL input")
}

func TestExaAnswer_Integration(t *testing.T) {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		t.Skip("EXA_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	input := AnswerInput{
		Query: "What is the Go programming language and who created it?",
	}

	output, err := Answer(ctx, input)
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}

	if output.Query != input.Query {
		t.Errorf("expected query '%s', got '%s'", input.Query, output.Query)
	}

	if output.Answer == "" {
		t.Error("expected non-empty answer")
	}

	t.Logf("Answer: %s", utils.TruncateString(output.Answer, 500))
	t.Logf("Citations: %d", len(output.Citations))

	for i, citation := range output.Citations {
		t.Logf("  [%d] %s - %s", i+1, citation.Title, citation.URL)
	}
}

func TestExaAnswerWithText_Integration(t *testing.T) {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		t.Skip("EXA_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	input := AnswerInput{
		Query:       "What are the main features of Kubernetes?",
		IncludeText: true,
	}

	output, err := Answer(ctx, input)
	if err != nil {
		t.Fatalf("Answer with text failed: %v", err)
	}

	if output.Answer == "" {
		t.Error("expected non-empty answer")
	}

	// When IncludeText is true, citations should have text
	hasTextInCitations := false
	for _, citation := range output.Citations {
		if citation.Text != "" {
			hasTextInCitations = true
			break
		}
	}

	if !hasTextInCitations && len(output.Citations) > 0 {
		t.Log("Warning: Expected text in citations when IncludeText=true, but none found")
	}

	t.Logf("Answer length: %d chars", len(output.Answer))
	t.Logf("Citations with text: %d", len(output.Citations))
}

func TestExaSearchWithDomainFilters_Integration(t *testing.T) {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		t.Skip("EXA_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := SearchInput{
		Query:          "golang concurrency",
		NumResults:     5,
		IncludeDomains: []string{"go.dev", "golang.org", "github.com"},
	}

	output, err := Search(ctx, input)
	if err != nil {
		t.Fatalf("Search with domain filters failed: %v", err)
	}

	t.Logf("Search with domain filters returned %d results", len(output.Results))
	for _, result := range output.Results {
		t.Logf("  %s", result.URL)
	}
}

func TestExaSearchWithDateFilters_Integration(t *testing.T) {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		t.Skip("EXA_API_KEY not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := SearchInput{
		Query:              "AI news",
		NumResults:         3,
		StartPublishedDate: "2024-01-01",
	}

	output, err := Search(ctx, input)
	if err != nil {
		t.Fatalf("Search with date filters failed: %v", err)
	}

	t.Logf("Search with date filter returned %d results", len(output.Results))
	for _, result := range output.Results {
		t.Logf("  %s (published: %s)", result.Title, result.PublishedDate)
	}
}
