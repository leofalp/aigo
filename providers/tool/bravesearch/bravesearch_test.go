package bravesearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

// braveSuccessResponse returns a minimal BraveAPIResponse JSON body with one web result.
func braveSuccessResponse() string {
	resp := BraveAPIResponse{
		Type: "search",
		Web: &WebResults{
			Type: "search",
			Results: []WebResult{
				{
					Title:       "Go Programming Language",
					URL:         "https://go.dev",
					Description: "The Go programming language.",
					Age:         "1 day ago",
				},
			},
		},
	}
	encoded, _ := json.Marshal(resp)
	return string(encoded)
}

// TestSearch_HappyPath verifies that Search correctly maps a successful API
// response to an Output with a non-empty summary and one result.
func TestSearch_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate the auth header is forwarded
		if r.Header.Get("X-Subscription-Token") == "" {
			t.Error("X-Subscription-Token header missing")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(braveSuccessResponse()))
	}))
	defer server.Close()

	// Override base URL to point at the test server.
	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	// Set a fake API key so the missing-key guard passes.
	_ = os.Setenv("BRAVE_SEARCH_API_KEY", "test-key")
	defer func() { _ = os.Unsetenv("BRAVE_SEARCH_API_KEY") }()

	output, err := Search(context.Background(), Input{Query: "golang"})
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if output.Query != "golang" {
		t.Errorf("Query = %q, want %q", output.Query, "golang")
	}
	if len(output.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(output.Results))
	}
	if output.Results[0].Title != "Go Programming Language" {
		t.Errorf("Results[0].Title = %q", output.Results[0].Title)
	}
	if !strings.Contains(output.Summary, "Go Programming Language") {
		t.Errorf("Summary does not mention result title: %q", output.Summary)
	}
}

// TestSearch_Non200Response verifies that a non-200 HTTP response causes Search
// to return a descriptive error.
func TestSearch_Non200Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	_ = os.Setenv("BRAVE_SEARCH_API_KEY", "test-key")
	defer func() { _ = os.Unsetenv("BRAVE_SEARCH_API_KEY") }()

	_, err := Search(context.Background(), Input{Query: "golang"})
	if err == nil {
		t.Fatal("Search() expected error for non-200 status, got nil")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should contain status code 429, got: %v", err)
	}
}

// TestSearchAdvanced_HappyPath verifies that SearchAdvanced correctly maps a
// successful API response to an AdvancedOutput with typed web results.
func TestSearchAdvanced_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(braveSuccessResponse()))
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	_ = os.Setenv("BRAVE_SEARCH_API_KEY", "test-key")
	defer func() { _ = os.Unsetenv("BRAVE_SEARCH_API_KEY") }()

	output, err := SearchAdvanced(context.Background(), Input{Query: "golang"})
	if err != nil {
		t.Fatalf("SearchAdvanced() unexpected error: %v", err)
	}
	if output.Query != "golang" {
		t.Errorf("Query = %q, want %q", output.Query, "golang")
	}
	if output.Web == nil || len(output.Web.Results) != 1 {
		t.Fatalf("Web.Results len = %v, want 1", output.Web)
	}
}
