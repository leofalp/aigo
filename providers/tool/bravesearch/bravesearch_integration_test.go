//go:build integration

package bravesearch

import (
	"context"
	"os"
	"testing"
	"time"
)

// requireAPIKey fails the test immediately when BRAVE_SEARCH_API_KEY is not set.
// Integration tests are opt-in (build tag), so a missing key is a configuration
// error that should surface loudly rather than be silently skipped.
func requireAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv("BRAVE_SEARCH_API_KEY") == "" {
		t.Fatal("BRAVE_SEARCH_API_KEY is required for integration tests")
	}
}

// TestAPIIntegration_BasicSearch verifies the tool works with real Brave Search API.
// Run with: go test -tags=integration ./providers/tool/bravesearch/...
// Requires: BRAVE_SEARCH_API_KEY environment variable
func TestAPIIntegration_BasicSearch(t *testing.T) {
	requireAPIKey(t)

	ctx := context.Background()
	input := Input{
		Query: "Go programming language",
		Count: 5,
	}

	output, err := Search(ctx, input)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if output.Query != input.Query {
		t.Errorf("output.Query = %v, want %v", output.Query, input.Query)
	}

	if output.Summary == "" {
		t.Error("Summary is empty")
	}

	if len(output.Results) == 0 {
		t.Error("No results returned")
	}

	// Verify first result structure
	if len(output.Results) > 0 {
		first := output.Results[0]
		if first.Title == "" {
			t.Error("First result has empty title")
		}
		if first.URL == "" {
			t.Error("First result has empty URL")
		}
	}

	t.Logf("✓ Results: %d, Summary: %d chars", len(output.Results), len(output.Summary))
}

// TestAPIIntegration_AdvancedSearch verifies the advanced search with real API.
// Run with: go test -tags=integration ./providers/tool/bravesearch/...
// Requires: BRAVE_SEARCH_API_KEY environment variable
func TestAPIIntegration_AdvancedSearch(t *testing.T) {
	requireAPIKey(t)

	// Respect rate limit between tests
	time.Sleep(2 * time.Second)

	ctx := context.Background()
	input := Input{
		Query: "quantum computing",
		Count: 5,
	}

	output, err := SearchAdvanced(ctx, input)
	if err != nil {
		t.Fatalf("SearchAdvanced() error = %v", err)
	}

	if output.Query != input.Query {
		t.Errorf("output.Query = %v, want %v", output.Query, input.Query)
	}

	// Verify web results structure (tests family_friendly and thumbnail fixes)
	if output.Web != nil && len(output.Web.Results) > 0 {
		first := output.Web.Results[0]
		if first.Title == "" {
			t.Error("First web result has empty title")
		}
		if first.URL == "" {
			t.Error("First web result has empty URL")
		}
		t.Logf("✓ Web results: %d, Family friendly: %v", len(output.Web.Results), first.FamilyFriendly)
	}

	if output.News != nil && len(output.News.Results) > 0 {
		t.Logf("✓ News results: %d", len(output.News.Results))
	}

	if output.Videos != nil && len(output.Videos.Results) > 0 {
		t.Logf("✓ Video results: %d", len(output.Videos.Results))
	}
}
