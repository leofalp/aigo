package exa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
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

func TestFindSimilar_MissingURL(t *testing.T) {
	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	input := SimilarInput{
		// URL is empty — required by the Exa /findSimilar API
	}

	_, err := FindSimilar(ctx, input)
	if err == nil {
		t.Error("expected error when URL is not provided")
	}

	if !strings.Contains(err.Error(), "url is required") {
		t.Errorf("expected error about url requirement, got: %s", err.Error())
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

// TestFindSimilar_Success verifies that FindSimilar correctly parses API responses
// and builds a summary from the results.
func TestFindSimilar_Success(t *testing.T) {
	mockResponse := exaSearchAPIResponse{
		Results: []exaSearchResultItem{
			{
				ID:            "1",
				Title:         "Similar Result 1",
				URL:           "https://example.com/similar1",
				Score:         0.95,
				PublishedDate: "2024-01-15",
				Author:        "Author One",
				Text:          "This is the content of similar result 1",
			},
		},
		ResolvedSearchType: "neural",
		RequestID:          "test-request-id",
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != "POST" {
			t.Errorf("expected POST request, got %s", request.Method)
		}
		if request.URL.Path != "/findSimilar" {
			t.Errorf("expected /findSimilar path, got %s", request.URL.Path)
		}
		if request.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("expected x-api-key header, got %s", request.Header.Get("x-api-key"))
		}

		// Verify request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if reqBody["url"] != "https://example.com/source" {
			t.Errorf("expected url 'https://example.com/source', got %v", reqBody["url"])
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
	output, err := FindSimilar(ctx, SimilarInput{URL: "https://example.com/source", NumResults: 5})
	if err != nil {
		t.Fatalf("FindSimilar failed: %v", err)
	}

	if output.Source != "https://example.com/source" {
		t.Errorf("expected source 'https://example.com/source', got '%s'", output.Source)
	}

	if len(output.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(output.Results))
	}

	if output.Results[0].Title != "Similar Result 1" {
		t.Errorf("expected first result title 'Similar Result 1', got '%s'", output.Results[0].Title)
	}

	if output.Summary == "" {
		t.Error("expected non-empty summary")
	}

	if !strings.Contains(output.Summary, "Similar Result 1") {
		t.Error("expected summary to contain result title")
	}
}

// TestFindSimilar_HTTPError verifies that FindSimilar correctly handles non-200 API responses.
func TestFindSimilar_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(writer).Encode(map[string]string{"error": "internal server error"}) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	_, err := FindSimilar(ctx, SimilarInput{URL: "https://example.com"})
	if err == nil {
		t.Error("expected error for 500 response")
	}

	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("expected error to contain API error message, got: %s", err.Error())
	}
}

// TestFindSimilar_MalformedJSON verifies that FindSimilar correctly handles malformed JSON responses.
func TestFindSimilar_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte("not json")) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	_, err := FindSimilar(ctx, SimilarInput{URL: "https://example.com"})
	if err == nil {
		t.Error("expected error for malformed JSON response")
	}

	if !strings.Contains(err.Error(), "error parsing response") {
		t.Errorf("expected error about parsing response, got: %s", err.Error())
	}
}

// TestSearchAdvanced_Success verifies that SearchAdvanced correctly parses API responses.
func TestSearchAdvanced_Success(t *testing.T) {
	mockResponse := exaSearchAPIResponse{
		Results: []exaSearchResultItem{
			{
				ID:            "1",
				Title:         "Advanced Result 1",
				URL:           "https://example.com/advanced1",
				Score:         0.99,
				PublishedDate: "2024-01-15",
				Author:        "Author One",
				Text:          "This is the content of advanced result 1",
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
	output, err := SearchAdvanced(ctx, SearchInput{Query: "test advanced query"})
	if err != nil {
		t.Fatalf("SearchAdvanced failed: %v", err)
	}

	if output.Query != "test advanced query" {
		t.Errorf("expected query 'test advanced query', got '%s'", output.Query)
	}

	if len(output.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(output.Results))
	}

	if output.Results[0].Title != "Advanced Result 1" {
		t.Errorf("expected first result title 'Advanced Result 1', got '%s'", output.Results[0].Title)
	}

	if output.ResolvedSearchType != "neural" {
		t.Errorf("expected resolved search type 'neural', got '%s'", output.ResolvedSearchType)
	}

	if output.RequestID != "test-request-id" {
		t.Errorf("expected request ID 'test-request-id', got '%s'", output.RequestID)
	}
}

// TestSearchAdvanced_HTTPError verifies that SearchAdvanced correctly handles non-200 API responses.
func TestSearchAdvanced_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(writer).Encode(map[string]string{"error": "unauthorized"}) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	_, err := SearchAdvanced(ctx, SearchInput{Query: "test"})
	if err == nil {
		t.Error("expected error for 401 response")
	}

	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected error to contain API error message, got: %s", err.Error())
	}
}

// TestSearchAdvanced_MalformedJSON verifies that SearchAdvanced correctly handles malformed JSON responses.
func TestSearchAdvanced_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte("garbage")) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	_, err := SearchAdvanced(ctx, SearchInput{Query: "test"})
	if err == nil {
		t.Error("expected error for malformed JSON response")
	}

	if !strings.Contains(err.Error(), "error parsing response") {
		t.Errorf("expected error about parsing response, got: %s", err.Error())
	}
}

// TestAnswer_EmptyResults verifies that Answer correctly handles responses with no citations.
func TestAnswer_EmptyResults(t *testing.T) {
	mockResponse := exaAnswerAPIResponse{
		Answer:    "I don't know.",
		Citations: []exaSearchResultItem{},
		Results:   []exaSearchResultItem{},
		RequestID: "answer-empty",
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
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
	output, err := Answer(ctx, AnswerInput{Query: "Unknown query"})
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}

	if output.Answer != "I don't know." {
		t.Errorf("expected answer 'I don't know.', got '%s'", output.Answer)
	}

	if len(output.Citations) != 0 {
		t.Errorf("expected 0 citations, got %d", len(output.Citations))
	}
}

// TestSearch_AllOptions verifies that all optional fields in SearchInput are correctly processed.
func TestSearch_AllOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		// Verify all fields were set
		if reqBody["type"] != "keyword" {
			t.Errorf("expected type 'keyword', got %v", reqBody["type"])
		}
		if reqBody["numResults"].(float64) != 100 { // maxResults
			t.Errorf("expected numResults 100, got %v", reqBody["numResults"])
		}
		if reqBody["startPublishedDate"] != "2024-01-01" {
			t.Errorf("expected startPublishedDate '2024-01-01', got %v", reqBody["startPublishedDate"])
		}
		if reqBody["endPublishedDate"] != "2024-12-31" {
			t.Errorf("expected endPublishedDate '2024-12-31', got %v", reqBody["endPublishedDate"])
		}
		if reqBody["startCrawlDate"] != "2024-01-01" {
			t.Errorf("expected startCrawlDate '2024-01-01', got %v", reqBody["startCrawlDate"])
		}
		if reqBody["endCrawlDate"] != "2024-12-31" {
			t.Errorf("expected endCrawlDate '2024-12-31', got %v", reqBody["endCrawlDate"])
		}
		if reqBody["category"] != "news" {
			t.Errorf("expected category 'news', got %v", reqBody["category"])
		}

		contents, ok := reqBody["contents"].(map[string]interface{})
		if !ok {
			t.Errorf("expected contents object, got %v", reqBody["contents"])
		} else {
			if contents["text"] != true {
				t.Errorf("expected contents.text true, got %v", contents["text"])
			}
			if contents["highlights"] == nil {
				t.Errorf("expected contents.highlights, got nil")
			}
		}

		writer.Header().Set("Content-Type", "application/json")
		json.NewEncoder(writer).Encode(exaSearchAPIResponse{}) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	input := SearchInput{
		Query:              "test",
		Type:               "keyword",
		NumResults:         150, // Should be capped at maxResults (100)
		IncludeDomains:     []string{"example.com"},
		ExcludeDomains:     []string{"bad.com"},
		StartPublishedDate: "2024-01-01",
		EndPublishedDate:   "2024-12-31",
		StartCrawlDate:     "2024-01-01",
		EndCrawlDate:       "2024-12-31",
		Category:           "news",
		IncludeText:        true,
		IncludeHighlights:  true,
	}

	_, err := Search(ctx, input)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
}

// TestSearch_HTTPError_InvalidJSON verifies handling of non-200 responses with invalid JSON.
func TestSearch_HTTPError_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		writer.Write([]byte("Bad Gateway HTML")) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	_, err := Search(ctx, SearchInput{Query: "test"})
	if err == nil {
		t.Error("expected error for 502 response")
	}

	if !strings.Contains(err.Error(), "unexpected status code 502") {
		t.Errorf("expected error to contain unexpected status code, got: %s", err.Error())
	}
}

// TestSearch_RequestError verifies handling of HTTP request execution errors.
func TestSearch_RequestError(t *testing.T) {
	originalBaseURL := baseURL
	baseURL = "http://invalid-url-that-does-not-exist.local:12345"
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	// Use a short timeout to fail fast
	ctx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()

	_, err := Search(ctx, SearchInput{Query: "test"})
	if err == nil {
		t.Error("expected error for request execution failure")
	}
	if !strings.Contains(err.Error(), "error making request") {
		t.Errorf("expected error making request, got: %s", err.Error())
	}
}

// TestAnswer_IncludeText verifies that IncludeText is correctly passed to the API.
func TestAnswer_IncludeText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		contents, ok := reqBody["contents"].(map[string]interface{})
		if !ok {
			t.Errorf("expected contents object, got %v", reqBody["contents"])
		} else if contents["text"] != true {
			t.Errorf("expected contents.text true, got %v", contents["text"])
		}

		writer.Header().Set("Content-Type", "application/json")
		json.NewEncoder(writer).Encode(exaAnswerAPIResponse{Answer: "test"}) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	_, err := Answer(ctx, AnswerInput{Query: "test", IncludeText: true})
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}
}

// TestAnswer_CitationsFromResults verifies that citations are populated from results if citations array is empty.
func TestAnswer_CitationsFromResults(t *testing.T) {
	mockResponse := exaAnswerAPIResponse{
		Answer: "Test answer",
		Results: []exaSearchResultItem{
			{
				Title: "Result Title",
				URL:   "https://example.com/result",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
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
	output, err := Answer(ctx, AnswerInput{Query: "test"})
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}

	if len(output.Citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(output.Citations))
	}
	if output.Citations[0].URL != "https://example.com/result" {
		t.Errorf("expected citation URL 'https://example.com/result', got '%s'", output.Citations[0].URL)
	}
}

// TestAnswer_HTTPErrors verifies handling of non-200 responses and invalid JSON.
func TestAnswer_HTTPErrors(t *testing.T) {
	// Test 1: Valid JSON error response
	server1 := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(map[string]string{"message": "bad request message"}) //nolint:errcheck
	}))
	defer server1.Close()

	originalBaseURL := baseURL
	baseURL = server1.URL

	os.Setenv("EXA_API_KEY", "test-api-key")
	ctx := context.Background()

	_, err := Answer(ctx, AnswerInput{Query: "test"})
	if err == nil || !strings.Contains(err.Error(), "bad request message") {
		t.Errorf("expected error with 'bad request message', got: %v", err)
	}

	// Test 2: Invalid JSON error response
	server2 := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		writer.Write([]byte("Bad Gateway HTML")) //nolint:errcheck
	}))
	defer server2.Close()

	baseURL = server2.URL
	_, err = Answer(ctx, AnswerInput{Query: "test"})
	if err == nil || !strings.Contains(err.Error(), "unexpected status code 502") {
		t.Errorf("expected error with 'unexpected status code 502', got: %v", err)
	}

	// Test 3: Invalid JSON success response
	server3 := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Write([]byte("not json")) //nolint:errcheck
	}))
	defer server3.Close()

	baseURL = server3.URL
	_, err = Answer(ctx, AnswerInput{Query: "test"})
	if err == nil || !strings.Contains(err.Error(), "error parsing response") {
		t.Errorf("expected error with 'error parsing response', got: %v", err)
	}

	// Test 4: Request execution error
	baseURL = "http://invalid-url-that-does-not-exist.local:12345"
	ctxTimeout, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()
	_, err = Answer(ctxTimeout, AnswerInput{Query: "test"})
	if err == nil || !strings.Contains(err.Error(), "error making request") {
		t.Errorf("expected error with 'error making request', got: %v", err)
	}

	baseURL = originalBaseURL
	os.Unsetenv("EXA_API_KEY")
}

// TestFindSimilar_AllOptions verifies that all optional fields in SimilarInput are correctly processed.
func TestFindSimilar_AllOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		if reqBody["numResults"].(float64) != 100 { // maxResults
			t.Errorf("expected numResults 100, got %v", reqBody["numResults"])
		}

		contents, ok := reqBody["contents"].(map[string]interface{})
		if !ok {
			t.Errorf("expected contents object, got %v", reqBody["contents"])
		} else {
			if contents["text"] != true {
				t.Errorf("expected contents.text true, got %v", contents["text"])
			}
			if contents["highlights"] == nil {
				t.Errorf("expected contents.highlights, got nil")
			}
		}

		writer.Header().Set("Content-Type", "application/json")
		json.NewEncoder(writer).Encode(exaSearchAPIResponse{}) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	input := SimilarInput{
		URL:               "https://example.com",
		NumResults:        150, // Should be capped at maxResults (100)
		IncludeDomains:    []string{"example.com"},
		ExcludeDomains:    []string{"bad.com"},
		IncludeText:       true,
		IncludeHighlights: true,
	}

	_, err := FindSimilar(ctx, input)
	if err != nil {
		t.Fatalf("FindSimilar failed: %v", err)
	}
}

// TestFindSimilar_EmptyResults verifies handling of empty results.
func TestFindSimilar_EmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		json.NewEncoder(writer).Encode(exaSearchAPIResponse{Results: []exaSearchResultItem{}}) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	output, err := FindSimilar(ctx, SimilarInput{URL: "https://example.com"})
	if err != nil {
		t.Fatalf("FindSimilar failed: %v", err)
	}

	if len(output.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(output.Results))
	}
	if !strings.Contains(output.Summary, "No similar content found") {
		t.Errorf("expected summary to indicate no content, got: %s", output.Summary)
	}
}

// TestFindSimilar_HTTPErrors verifies handling of non-200 responses and invalid JSON.
func TestFindSimilar_HTTPErrors(t *testing.T) {
	// Test 1: Valid JSON error response
	server1 := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(map[string]string{"message": "bad request message"}) //nolint:errcheck
	}))
	defer server1.Close()

	originalBaseURL := baseURL
	baseURL = server1.URL

	os.Setenv("EXA_API_KEY", "test-api-key")
	ctx := context.Background()

	_, err := FindSimilar(ctx, SimilarInput{URL: "https://example.com"})
	if err == nil || !strings.Contains(err.Error(), "bad request message") {
		t.Errorf("expected error with 'bad request message', got: %v", err)
	}

	// Test 2: Invalid JSON error response
	server2 := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		writer.Write([]byte("Bad Gateway HTML")) //nolint:errcheck
	}))
	defer server2.Close()

	baseURL = server2.URL
	_, err = FindSimilar(ctx, SimilarInput{URL: "https://example.com"})
	if err == nil || !strings.Contains(err.Error(), "unexpected status code 502") {
		t.Errorf("expected error with 'unexpected status code 502', got: %v", err)
	}

	// Test 3: Request execution error
	baseURL = "http://invalid-url-that-does-not-exist.local:12345"
	ctxTimeout, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()
	_, err = FindSimilar(ctxTimeout, SimilarInput{URL: "https://example.com"})
	if err == nil || !strings.Contains(err.Error(), "error making request") {
		t.Errorf("expected error with 'error making request', got: %v", err)
	}

	baseURL = originalBaseURL
	os.Unsetenv("EXA_API_KEY")
}
