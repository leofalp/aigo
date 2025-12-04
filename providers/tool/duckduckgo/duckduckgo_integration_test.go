//go:build integration

package duckduckgo

import (
	"context"
	"testing"
)

// Common test queries used across different integration test cases
var integrationTestQueries = []struct {
	name  string
	query string
}{
	{"Programming query", "Golang programming language"},
	{"Math query", "2+2"},
	{"Definition query", "serendipity"},
}

// TestSearch_Integration tests the Search function with real API calls.
// Run with: go test -tags=integration ./providers/tool/duckduckgo/...
func TestSearch_Integration(t *testing.T) {
	for _, tt := range integrationTestQueries {
		t.Run(tt.name, func(t *testing.T) {
			input := Input{Query: tt.query}
			output, err := Search(context.Background(), input)

			if err != nil {
				t.Errorf("Search() error = %v", err)
				return
			}

			if output.Query != tt.query {
				t.Errorf("Search() output.Query = %v, want %v", output.Query, tt.query)
			}
			if output.Summary == "" {
				t.Errorf("Search() output.Summary is empty")
			}

			t.Logf("Query: %s, Summary length: %d", output.Query, len(output.Summary))
		})
	}
}

// TestSearchAdvanced_Integration tests the SearchAdvanced function with real API calls.
// Run with: go test -tags=integration ./providers/tool/duckduckgo/...
func TestSearchAdvanced_Integration(t *testing.T) {
	for _, tt := range integrationTestQueries {
		t.Run(tt.name, func(t *testing.T) {
			input := Input{Query: tt.query}
			output, err := SearchAdvanced(context.Background(), input)

			if err != nil {
				t.Errorf("SearchAdvanced() error = %v", err)
				return
			}

			if output.Query != tt.query {
				t.Errorf("SearchAdvanced() output.Query = %v, want %v", output.Query, tt.query)
			}

			// Log structured output details
			t.Logf("Query: %s, Type: %s", output.Query, output.Type)
			if output.Abstract != "" {
				t.Logf("  Has Abstract from %s", output.AbstractSource)
			}
			if output.Answer != "" {
				t.Logf("  Has Answer: %s", output.AnswerType)
			}
			if len(output.RelatedTopics) > 0 {
				t.Logf("  Related Topics: %d", len(output.RelatedTopics))
			}
		})
	}
}
