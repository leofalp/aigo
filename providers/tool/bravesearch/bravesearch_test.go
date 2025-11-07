package bravesearch

import (
	"context"
	"os"
	"testing"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

// TestToolCreation tests that the tools are created correctly
func TestToolCreation(t *testing.T) {
	t.Run("Basic tool", func(t *testing.T) {
		tool := NewBraveSearchTool()
		if tool.Name != "BraveSearch" {
			t.Errorf("Tool name = %v, want BraveSearch", tool.Name)
		}
		if tool.Description == "" {
			t.Error("Tool description is empty")
		}
		if tool.Function == nil {
			t.Error("Tool function is nil")
		}
	})

	t.Run("Advanced tool", func(t *testing.T) {
		tool := NewBraveSearchAdvancedTool()
		if tool.Name != "BraveSearchAdvanced" {
			t.Errorf("Tool name = %v, want BraveSearchAdvanced", tool.Name)
		}
		if tool.Description == "" {
			t.Error("Tool description is empty")
		}
		if tool.Function == nil {
			t.Error("Tool function is nil")
		}
	})
}

// TestSearchWithoutAPIKey tests error handling when API key is missing
func TestSearchWithoutAPIKey(t *testing.T) {
	originalKey := os.Getenv("BRAVE_SEARCH_API_KEY")
	defer os.Setenv("BRAVE_SEARCH_API_KEY", originalKey)

	os.Unsetenv("BRAVE_SEARCH_API_KEY")

	input := Input{
		Query: "test query",
		Count: 5,
	}

	_, err := Search(context.Background(), input)
	if err == nil {
		t.Error("Search() should return error when API key is missing")
	}

	expectedMsg := "BRAVE_SEARCH_API_KEY environment variable is not set"
	if err != nil && err.Error() != expectedMsg {
		t.Errorf("Search() error = %v, want %v", err, expectedMsg)
	}
}

// TestCleanHTML tests HTML tag removal
func TestCleanHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<strong>hello</strong> world", "hello world"},
		{"<strong>bold</strong> and <em>italic</em>", "bold and italic"},
		{"plain text", "plain text"},
		{"", ""},
	}

	for _, tt := range tests {
		result := cleanHTML(tt.input)
		if result != tt.expected {
			t.Errorf("cleanHTML(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestTruncate tests string truncation
func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world this is long", 10, "hello worl..."},
		{"hello", 5, "hello"},
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

// TestAPIIntegration verifies the tool works with real API
// Run with: BRAVE_SEARCH_API_KEY=your_key go test -v -run TestAPIIntegration
func TestAPIIntegration(t *testing.T) {
	if os.Getenv("BRAVE_SEARCH_API_KEY") == "" {
		t.Skip("BRAVE_SEARCH_API_KEY not set, skipping integration test")
	}

	ctx := context.Background()

	// Test basic search
	t.Run("BasicSearch", func(t *testing.T) {
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
	})

	time.Sleep(2 * time.Second) // Respect rate limit

	// Test advanced search
	t.Run("AdvancedSearch", func(t *testing.T) {
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
	})
}
