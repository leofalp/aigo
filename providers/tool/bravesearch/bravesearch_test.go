package bravesearch

import (
	"context"
	"os"
	"testing"

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
	defer func() { _ = os.Setenv("BRAVE_SEARCH_API_KEY", originalKey) }()

	_ = os.Unsetenv("BRAVE_SEARCH_API_KEY")

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
