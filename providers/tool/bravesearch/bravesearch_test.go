package bravesearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestFetchBraveSearchResults_Parameters(t *testing.T) {
	var lastURL *url.URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastURL = r.URL
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	_ = os.Setenv("BRAVE_SEARCH_API_KEY", "test-key")
	defer func() { _ = os.Unsetenv("BRAVE_SEARCH_API_KEY") }()

	// Test default count
	_, _ = fetchBraveSearchResults(context.Background(), Input{Query: "test"})
	if lastURL.Query().Get("count") != "10" {
		t.Errorf("Expected default count 10, got %s", lastURL.Query().Get("count"))
	}

	// Test count > 20
	_, _ = fetchBraveSearchResults(context.Background(), Input{Query: "test", Count: 25})
	if lastURL.Query().Get("count") != "20" {
		t.Errorf("Expected max count 20, got %s", lastURL.Query().Get("count"))
	}

	// Test all optional params
	_, _ = fetchBraveSearchResults(context.Background(), Input{
		Query:      "test",
		Count:      15,
		Country:    "us",
		SearchLang: "en",
		SafeSearch: "strict",
		Freshness:  "pd",
	})
	q := lastURL.Query()
	if q.Get("count") != "15" {
		t.Errorf("Expected count 15, got %s", q.Get("count"))
	}
	if q.Get("country") != "us" {
		t.Errorf("Expected country us, got %s", q.Get("country"))
	}
	if q.Get("search_lang") != "en" {
		t.Errorf("Expected search_lang en, got %s", q.Get("search_lang"))
	}
	if q.Get("safesearch") != "strict" {
		t.Errorf("Expected safesearch strict, got %s", q.Get("safesearch"))
	}
	if q.Get("freshness") != "pd" {
		t.Errorf("Expected freshness pd, got %s", q.Get("freshness"))
	}
}

func TestFetchBraveSearchResults_Errors(t *testing.T) {
	_ = os.Setenv("BRAVE_SEARCH_API_KEY", "test-key")
	defer func() { _ = os.Unsetenv("BRAVE_SEARCH_API_KEY") }()

	t.Run("Invalid URL", func(t *testing.T) {
		originalBaseURL := baseURL
		baseURL = "http://\x00invalid"
		defer func() { baseURL = originalBaseURL }()

		_, err := fetchBraveSearchResults(context.Background(), Input{Query: "test"})
		if err == nil {
			t.Error("Expected error for invalid URL")
		}
	})

	t.Run("Network Error", func(t *testing.T) {
		originalBaseURL := baseURL
		baseURL = "http://127.0.0.1:0" // Invalid port/connection refused
		defer func() { baseURL = originalBaseURL }()

		_, err := fetchBraveSearchResults(context.Background(), Input{Query: "test"})
		if err == nil {
			t.Error("Expected network error")
		}
	})

	t.Run("Malformed JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{invalid json`))
		}))
		defer server.Close()

		originalBaseURL := baseURL
		baseURL = server.URL
		defer func() { baseURL = originalBaseURL }()

		_, err := fetchBraveSearchResults(context.Background(), Input{Query: "test"})
		if err == nil || !strings.Contains(err.Error(), "error parsing response") {
			t.Errorf("Expected JSON parse error, got: %v", err)
		}
	})
}

func TestSearch_ResponseFormatting(t *testing.T) {
	_ = os.Setenv("BRAVE_SEARCH_API_KEY", "test-key")
	defer func() { _ = os.Unsetenv("BRAVE_SEARCH_API_KEY") }()

	t.Run("Empty Results", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"type": "search"}`))
		}))
		defer server.Close()

		originalBaseURL := baseURL
		baseURL = server.URL
		defer func() { baseURL = originalBaseURL }()

		output, err := Search(context.Background(), Input{Query: "test query"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !strings.Contains(output.Summary, "No results found for 'test query'") {
			t.Errorf("Expected empty results message, got: %s", output.Summary)
		}
	})

	t.Run("Rich Results", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)

			// Create 11 web results and 4 news results
			resp := BraveAPIResponse{
				Type: "search",
				Web: &WebResults{
					Results: make([]WebResult, 11),
				},
				Infobox: &Infobox{
					Label:     "Test Infobox",
					ShortDesc: "A short description",
				},
				News: &NewsResults{
					Results: make([]NewsResult, 4),
				},
			}

			for i := 0; i < 11; i++ {
				resp.Web.Results[i] = WebResult{
					Title:       fmt.Sprintf("Web %d", i),
					URL:         fmt.Sprintf("http://web%d.com", i),
					Description: fmt.Sprintf("Desc %d", i),
				}
			}
			for i := 0; i < 4; i++ {
				resp.News.Results[i] = NewsResult{
					Title: fmt.Sprintf("News %d", i),
					Age:   "1h",
				}
			}

			jsonBytes, _ := json.Marshal(resp)
			_, _ = w.Write(jsonBytes)
		}))
		defer server.Close()

		originalBaseURL := baseURL
		baseURL = server.URL
		defer func() { baseURL = originalBaseURL }()

		output, err := Search(context.Background(), Input{Query: "test"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if len(output.Results) != 10 {
			t.Errorf("Expected exactly 10 web results, got %d", len(output.Results))
		}

		if !strings.Contains(output.Summary, "Infobox: Test Infobox") {
			t.Errorf("Expected infobox label in summary")
		}
		if !strings.Contains(output.Summary, "Description: A short description") {
			t.Errorf("Expected infobox description in summary")
		}

		// Check news count in summary
		if !strings.Contains(output.Summary, "Recent news (4 articles):") {
			t.Errorf("Expected news header in summary")
		}
		if !strings.Contains(output.Summary, "- News 0") {
			t.Errorf("Expected news item in summary")
		}
		if strings.Contains(output.Summary, "- News 3") {
			t.Errorf("Expected 4th news item to be truncated from summary")
		}
	})
}

func TestSearchAdvanced_Error(t *testing.T) {
	_ = os.Setenv("BRAVE_SEARCH_API_KEY", "test-key")
	defer func() { _ = os.Unsetenv("BRAVE_SEARCH_API_KEY") }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "server error"}`))
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	_, err := SearchAdvanced(context.Background(), Input{Query: "test"})
	if err == nil {
		t.Error("Expected error from SearchAdvanced")
	}
}
