package exa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewExaSearchTool(t *testing.T) {
	exaTool := NewExaSearchTool()

	if exaTool.Name != "ExaSearch" {
		t.Errorf("expected tool name 'ExaSearch', got '%s'", exaTool.Name)
	}

	if exaTool.Description == "" {
		t.Error("expected non-empty description")
	}

	if exaTool.Metrics == nil {
		t.Error("expected metrics to be set")
	}

	if exaTool.Metrics.Amount <= 0 {
		t.Error("expected positive cost amount")
	}

	if exaTool.Metrics.Accuracy <= 0 || exaTool.Metrics.Accuracy > 1 {
		t.Errorf("expected accuracy between 0 and 1, got %f", exaTool.Metrics.Accuracy)
	}
}

func TestNewExaSearchAdvancedTool(t *testing.T) {
	advancedTool := NewExaSearchAdvancedTool()

	if advancedTool.Name != "ExaSearchAdvanced" {
		t.Errorf("expected tool name 'ExaSearchAdvanced', got '%s'", advancedTool.Name)
	}

	if advancedTool.Description == "" {
		t.Error("expected non-empty description")
	}

	if advancedTool.Metrics == nil {
		t.Error("expected metrics to be set")
	}

	// Advanced should have higher cost
	basicTool := NewExaSearchTool()
	if advancedTool.Metrics.Amount <= basicTool.Metrics.Amount {
		t.Error("expected advanced tool to have higher cost than basic")
	}
}

func TestNewExaFindSimilarTool(t *testing.T) {
	similarTool := NewExaFindSimilarTool()

	if similarTool.Name != "ExaFindSimilar" {
		t.Errorf("expected tool name 'ExaFindSimilar', got '%s'", similarTool.Name)
	}

	if similarTool.Description == "" {
		t.Error("expected non-empty description")
	}

	if similarTool.Metrics == nil {
		t.Error("expected metrics to be set")
	}
}

func TestNewExaAnswerTool(t *testing.T) {
	answerTool := NewExaAnswerTool()

	if answerTool.Name != "ExaAnswer" {
		t.Errorf("expected tool name 'ExaAnswer', got '%s'", answerTool.Name)
	}

	if answerTool.Description == "" {
		t.Error("expected non-empty description")
	}

	if answerTool.Metrics == nil {
		t.Error("expected metrics to be set")
	}

	// Answer should have higher cost due to LLM processing
	searchTool := NewExaSearchTool()
	if answerTool.Metrics.Amount <= searchTool.Metrics.Amount {
		t.Error("expected answer tool to have higher cost than search")
	}
}

func TestSearch_MissingAPIKey(t *testing.T) {
	originalKey := os.Getenv("EXA_API_KEY")
	os.Unsetenv("EXA_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("EXA_API_KEY", originalKey)
		}
	}()

	ctx := context.Background()
	input := SearchInput{
		Query: "test query",
	}

	_, err := Search(ctx, input)
	if err == nil {
		t.Error("expected error when API key is missing")
	}

	if !strings.Contains(err.Error(), "EXA_API_KEY") {
		t.Errorf("expected error message to mention EXA_API_KEY, got: %s", err.Error())
	}
}

// TestSearch_EmptyQuery verifies that Search returns an error for empty queries
// before making any API calls.
func TestSearch_EmptyQuery(t *testing.T) {
	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	input := SearchInput{Query: ""}

	_, err := Search(ctx, input)
	if err == nil {
		t.Error("expected error when query is empty")
	}

	if !strings.Contains(err.Error(), "query") {
		t.Errorf("expected error about query, got: %s", err.Error())
	}
}

// TestSearchAdvanced_EmptyQuery verifies that SearchAdvanced returns an error for empty queries.
func TestSearchAdvanced_EmptyQuery(t *testing.T) {
	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	input := SearchInput{Query: ""}

	_, err := SearchAdvanced(ctx, input)
	if err == nil {
		t.Error("expected error when query is empty")
	}

	if !strings.Contains(err.Error(), "query") {
		t.Errorf("expected error about query, got: %s", err.Error())
	}
}

func TestFindSimilar_MissingAPIKey(t *testing.T) {
	originalKey := os.Getenv("EXA_API_KEY")
	os.Unsetenv("EXA_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("EXA_API_KEY", originalKey)
		}
	}()

	ctx := context.Background()
	input := SimilarInput{
		URL: "https://example.com",
	}

	_, err := FindSimilar(ctx, input)
	if err == nil {
		t.Error("expected error when API key is missing")
	}

	if !strings.Contains(err.Error(), "EXA_API_KEY") {
		t.Errorf("expected error message to mention EXA_API_KEY, got: %s", err.Error())
	}
}

func TestFindSimilar_MissingURLAndText(t *testing.T) {
	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	input := SimilarInput{
		// Neither URL nor Text provided
	}

	_, err := FindSimilar(ctx, input)
	if err == nil {
		t.Error("expected error when neither URL nor text is provided")
	}

	if !strings.Contains(err.Error(), "url or text") {
		t.Errorf("expected error about url or text, got: %s", err.Error())
	}
}

func TestAnswer_MissingAPIKey(t *testing.T) {
	originalKey := os.Getenv("EXA_API_KEY")
	os.Unsetenv("EXA_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("EXA_API_KEY", originalKey)
		}
	}()

	ctx := context.Background()
	input := AnswerInput{
		Query: "test question",
	}

	_, err := Answer(ctx, input)
	if err == nil {
		t.Error("expected error when API key is missing")
	}

	if !strings.Contains(err.Error(), "EXA_API_KEY") {
		t.Errorf("expected error message to mention EXA_API_KEY, got: %s", err.Error())
	}
}

func TestAnswer_EmptyQuery(t *testing.T) {
	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	input := AnswerInput{
		Query: "",
	}

	_, err := Answer(ctx, input)
	if err == nil {
		t.Error("expected error when query is empty")
	}

	if !strings.Contains(err.Error(), "query") {
		t.Errorf("expected error about query, got: %s", err.Error())
	}
}

// TestSearch_Success verifies that Search correctly parses API responses
// and builds a summary from the results.
func TestSearch_Success(t *testing.T) {
	mockResponse := exaSearchAPIResponse{
		Results: []exaSearchResultItem{
			{
				ID:            "1",
				Title:         "Test Result 1",
				URL:           "https://example.com/1",
				Score:         0.95,
				PublishedDate: "2024-01-15",
				Author:        "Author One",
				Text:          "This is the content of test result 1",
			},
			{
				ID:    "2",
				Title: "Test Result 2",
				URL:   "https://example.com/2",
				Score: 0.90,
				Text:  "This is the content of test result 2",
			},
		},
		ResolvedSearchType: "neural",
		RequestID:          "test-request-id",
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != "POST" {
			t.Errorf("expected POST request, got %s", request.Method)
		}
		if request.URL.Path != "/search" {
			t.Errorf("expected /search path, got %s", request.URL.Path)
		}
		if request.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("expected x-api-key header, got %s", request.Header.Get("x-api-key"))
		}

		// Verify request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if reqBody["query"] != "test query" {
			t.Errorf("expected query 'test query', got %v", reqBody["query"])
		}

		writer.Header().Set("Content-Type", "application/json")
		json.NewEncoder(writer).Encode(mockResponse) //nolint:errcheck
	}))
	defer server.Close()

	// Override baseURL for testing
	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	output, err := Search(ctx, SearchInput{Query: "test query", NumResults: 5})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if output.Query != "test query" {
		t.Errorf("expected query 'test query', got '%s'", output.Query)
	}

	if len(output.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(output.Results))
	}

	if output.Results[0].Title != "Test Result 1" {
		t.Errorf("expected first result title 'Test Result 1', got '%s'", output.Results[0].Title)
	}

	if output.Summary == "" {
		t.Error("expected non-empty summary")
	}

	if !strings.Contains(output.Summary, "Test Result 1") {
		t.Error("expected summary to contain result title")
	}

	if !strings.Contains(output.Summary, "Author One") {
		t.Error("expected summary to contain author")
	}
}

// TestSearch_APIError verifies that Search correctly handles non-200 API responses.
func TestSearch_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(writer).Encode(map[string]string{"error": "invalid api key"}) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "bad-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	_, err := Search(ctx, SearchInput{Query: "test"})
	if err == nil {
		t.Error("expected error for 401 response")
	}

	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("expected error to contain API error message, got: %s", err.Error())
	}
}

// TestAnswer_Success verifies that Answer correctly parses API responses.
func TestAnswer_Success(t *testing.T) {
	mockResponse := exaAnswerAPIResponse{
		Answer: "Go was created by Robert Griesemer, Rob Pike, and Ken Thompson at Google.",
		Citations: []exaSearchResultItem{
			{
				Title: "Go Programming Language",
				URL:   "https://go.dev",
			},
		},
		RequestID: "answer-123",
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/answer" {
			t.Errorf("expected /answer path, got %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		json.NewEncoder(writer).Encode(mockResponse) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	output, err := Answer(ctx, AnswerInput{Query: "Who created Go?"})
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}

	if output.Answer == "" {
		t.Error("expected non-empty answer")
	}

	if len(output.Citations) != 1 {
		t.Errorf("expected 1 citation, got %d", len(output.Citations))
	}

	if output.Citations[0].URL != "https://go.dev" {
		t.Errorf("expected citation URL 'https://go.dev', got '%s'", output.Citations[0].URL)
	}
}

func TestToolMetrics_Consistency(t *testing.T) {
	searchTool := NewExaSearchTool()
	advancedTool := NewExaSearchAdvancedTool()
	similarTool := NewExaFindSimilarTool()
	answerTool := NewExaAnswerTool()

	// All tools should have metrics
	allTools := []struct {
		name     string
		accuracy float64
	}{
		{"ExaSearch", searchTool.Metrics.Accuracy},
		{"ExaSearchAdvanced", advancedTool.Metrics.Accuracy},
		{"ExaFindSimilar", similarTool.Metrics.Accuracy},
		{"ExaAnswer", answerTool.Metrics.Accuracy},
	}

	for _, toolInfo := range allTools {
		if toolInfo.accuracy <= 0 || toolInfo.accuracy > 1 {
			t.Errorf("%s: accuracy %f should be between 0 and 1", toolInfo.name, toolInfo.accuracy)
		}
	}
}
