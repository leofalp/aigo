package duckduckgo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestToolCreation tests that the tools are created correctly (unit test - no external calls)
func TestToolCreation(t *testing.T) {
	t.Run("Base tool", func(t *testing.T) {
		tool := NewDuckDuckGoSearchTool()
		if tool.Name != "DuckDuckGoSearch" {
			t.Errorf("Tool name = %v, want DuckDuckGoSearch", tool.Name)
		}
		if tool.Description == "" {
			t.Error("Tool description is empty")
		}
		if tool.Function == nil {
			t.Error("Tool function is nil")
		}
	})

	t.Run("Advanced tool", func(t *testing.T) {
		tool := NewDuckDuckGoSearchAdvancedTool()
		if tool.Name != "DuckDuckGoSearchAdvanced" {
			t.Errorf("Tool name = %v, want DuckDuckGoSearchAdvanced", tool.Name)
		}
		if tool.Description == "" {
			t.Error("Tool description is empty")
		}
		if tool.Function == nil {
			t.Error("Tool function is nil")
		}
	})
}

// TestInputStructure tests the Input struct fields (unit test - no external calls)
func TestInputStructure(t *testing.T) {
	input := Input{
		Query: "test query",
	}

	if input.Query != "test query" {
		t.Errorf("Input.Query = %v, want 'test query'", input.Query)
	}
}

// TestOutputStructure tests the Output struct fields (unit test - no external calls)
func TestOutputStructure(t *testing.T) {
	output := Output{
		Query:   "test",
		Summary: "test summary",
	}

	if output.Query != "test" {
		t.Errorf("Output.Query = %v, want 'test'", output.Query)
	}
	if output.Summary != "test summary" {
		t.Errorf("Output.Summary = %v, want 'test summary'", output.Summary)
	}
}

// TestAdvancedOutputStructure tests the AdvancedOutput struct fields (unit test - no external calls)
func TestAdvancedOutputStructure(t *testing.T) {
	output := AdvancedOutput{
		Query:    "test",
		Type:     "A",
		Abstract: "test abstract",
	}

	if output.Query != "test" {
		t.Errorf("AdvancedOutput.Query = %v, want 'test'", output.Query)
	}
	if output.Type != "A" {
		t.Errorf("AdvancedOutput.Type = %v, want 'A'", output.Type)
	}
	if output.Abstract != "test abstract" {
		t.Errorf("AdvancedOutput.Abstract = %v, want 'test abstract'", output.Abstract)
	}
}

// ddgSuccessResponse returns a minimal DDGResponse JSON body with an abstract
// and one related topic, suitable for httptest mocking.
func ddgSuccessResponse() string {
	raw := DDGResponse{
		AbstractText:   "Go is an open-source programming language.",
		AbstractSource: "Wikipedia",
		AbstractURL:    "https://en.wikipedia.org/wiki/Go_(programming_language)",
		Heading:        "Go",
		Type:           "A",
		RelatedTopics: []relatedTopicResponse{
			{Text: "Go concurrency", FirstURL: "https://en.wikipedia.org/wiki/Go_(programming_language)"},
		},
	}
	encoded, _ := json.Marshal(raw)
	return string(encoded)
}

// TestSearch_HappyPath verifies that Search maps a successful API response to
// an Output with a non-empty summary containing the abstract.
func TestSearch_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ddgSuccessResponse()))
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL + "/"
	defer func() { baseURL = originalBaseURL }()

	output, err := Search(context.Background(), Input{Query: "golang"})
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if output.Query != "golang" {
		t.Errorf("Query = %q, want %q", output.Query, "golang")
	}
	if !strings.Contains(output.Summary, "Go is an open-source programming language.") {
		t.Errorf("Summary missing abstract text: %q", output.Summary)
	}
}

// TestSearch_Non200Response verifies that a non-200 HTTP status causes Search
// to return an error.
func TestSearch_Non200Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL + "/"
	defer func() { baseURL = originalBaseURL }()

	_, err := Search(context.Background(), Input{Query: "golang"})
	if err == nil {
		t.Fatal("Search() expected error for non-200 status, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500, got: %v", err)
	}
}

// TestSearchAdvanced_HappyPath verifies that SearchAdvanced maps a successful
// API response to an AdvancedOutput with abstract, heading, and related topics.
func TestSearchAdvanced_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ddgSuccessResponse()))
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL + "/"
	defer func() { baseURL = originalBaseURL }()

	output, err := SearchAdvanced(context.Background(), Input{Query: "golang"})
	if err != nil {
		t.Fatalf("SearchAdvanced() unexpected error: %v", err)
	}
	if output.Query != "golang" {
		t.Errorf("Query = %q, want %q", output.Query, "golang")
	}
	if output.Abstract != "Go is an open-source programming language." {
		t.Errorf("Abstract = %q", output.Abstract)
	}
	if output.Heading != "Go" {
		t.Errorf("Heading = %q, want %q", output.Heading, "Go")
	}
	if len(output.RelatedTopics) != 1 {
		t.Errorf("len(RelatedTopics) = %d, want 1", len(output.RelatedTopics))
	}
}

// TestSearch_EmptyResponse verifies that Search returns a "no results" summary
// when the API response contains no abstract, answer, or related topics.
func TestSearch_EmptyResponse(t *testing.T) {
	emptyBody, _ := json.Marshal(DDGResponse{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(emptyBody)
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL + "/"
	defer func() { baseURL = originalBaseURL }()

	output, err := Search(context.Background(), Input{Query: "xyznotfound"})
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if !strings.Contains(output.Summary, "No results") {
		t.Errorf("expected 'No results' fallback, got: %q", output.Summary)
	}
}
