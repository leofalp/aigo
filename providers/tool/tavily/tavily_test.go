package tavily

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewTavilySearchTool(t *testing.T) {
	searchTool := NewTavilySearchTool()

	if searchTool.Name != "TavilySearch" {
		t.Errorf("expected tool name 'TavilySearch', got '%s'", searchTool.Name)
	}

	if searchTool.Description == "" {
		t.Error("expected non-empty description")
	}

	if searchTool.Metrics == nil {
		t.Error("expected metrics to be set")
	}

	if searchTool.Metrics.Amount <= 0 {
		t.Error("expected positive cost amount")
	}

	if searchTool.Metrics.Accuracy <= 0 || searchTool.Metrics.Accuracy > 1 {
		t.Errorf("expected accuracy between 0 and 1, got %f", searchTool.Metrics.Accuracy)
	}
}

func TestNewTavilySearchAdvancedTool(t *testing.T) {
	advancedTool := NewTavilySearchAdvancedTool()

	if advancedTool.Name != "TavilySearchAdvanced" {
		t.Errorf("expected tool name 'TavilySearchAdvanced', got '%s'", advancedTool.Name)
	}

	if advancedTool.Description == "" {
		t.Error("expected non-empty description")
	}

	if advancedTool.Metrics == nil {
		t.Error("expected metrics to be set")
	}

	// Advanced should have higher cost
	basicTool := NewTavilySearchTool()
	if advancedTool.Metrics.Amount <= basicTool.Metrics.Amount {
		t.Error("expected advanced tool to have higher cost than basic")
	}
}

func TestNewTavilyExtractTool(t *testing.T) {
	extractTool := NewTavilyExtractTool()

	if extractTool.Name != "TavilyExtract" {
		t.Errorf("expected tool name 'TavilyExtract', got '%s'", extractTool.Name)
	}

	if extractTool.Description == "" {
		t.Error("expected non-empty description")
	}

	if extractTool.Metrics == nil {
		t.Error("expected metrics to be set")
	}
}

func TestSearch_MissingAPIKey(t *testing.T) {
	originalKey := os.Getenv("TAVILY_API_KEY")
	os.Unsetenv("TAVILY_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("TAVILY_API_KEY", originalKey)
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

	if !strings.Contains(err.Error(), "TAVILY_API_KEY") {
		t.Errorf("expected error message to mention TAVILY_API_KEY, got: %s", err.Error())
	}
}

// TestSearch_Success verifies that Search correctly parses API responses
// and builds a summary from the results.
func TestSearch_Success(t *testing.T) {
	mockResponse := tavilySearchAPIResponse{
		Query:  "test query",
		Answer: "This is a test answer",
		Results: []tavilySearchResultItem{
			{
				Title:   "Test Result 1",
				URL:     "https://example.com/1",
				Content: "This is the content of test result 1",
				Score:   0.95,
			},
			{
				Title:   "Test Result 2",
				URL:     "https://example.com/2",
				Content: "This is the content of test result 2",
				Score:   0.90,
			},
		},
		ResponseTime: 0.5,
		RequestID:    "test-request-id",
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != "POST" {
			t.Errorf("expected POST request, got %s", request.Method)
		}
		if request.URL.Path != "/search" {
			t.Errorf("expected /search path, got %s", request.URL.Path)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", request.Header.Get("Content-Type"))
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept application/json, got %s", request.Header.Get("Accept"))
		}

		// Verify request body contains api_key and query
		var reqBody map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if reqBody["api_key"] == nil {
			t.Error("expected api_key in request body")
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

	os.Setenv("TAVILY_API_KEY", "test-api-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	output, err := Search(ctx, SearchInput{Query: "test query"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if output.Query != "test query" {
		t.Errorf("expected query 'test query', got '%s'", output.Query)
	}

	if output.Answer != "This is a test answer" {
		t.Errorf("expected answer 'This is a test answer', got '%s'", output.Answer)
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
}

// TestSearchAdvanced_Success verifies that SearchAdvanced returns all metadata.
func TestSearchAdvanced_Success(t *testing.T) {
	mockResponse := tavilySearchAPIResponse{
		Query: "test query",
		Results: []tavilySearchResultItem{
			{
				Title:      "Advanced Result",
				URL:        "https://example.com/1",
				Content:    "Content here",
				RawContent: "Full raw content",
				Score:      0.98,
			},
		},
		Images: []tavilyImageItem{
			{URL: "https://example.com/img.png", Description: "An image"},
		},
		ResponseTime: 1.2,
		RequestID:    "adv-123",
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		json.NewEncoder(writer).Encode(mockResponse) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("TAVILY_API_KEY", "test-api-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	output, err := SearchAdvanced(ctx, SearchInput{Query: "test query"})
	if err != nil {
		t.Fatalf("SearchAdvanced failed: %v", err)
	}

	if len(output.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(output.Results))
	}

	if output.Results[0].RawContent != "Full raw content" {
		t.Errorf("expected raw content, got '%s'", output.Results[0].RawContent)
	}

	if len(output.Images) != 1 {
		t.Errorf("expected 1 image, got %d", len(output.Images))
	}

	if output.ResponseTime != 1.2 {
		t.Errorf("expected response time 1.2, got %f", output.ResponseTime)
	}
}

// TestSearch_APIError verifies that Search correctly handles non-200 API responses.
func TestSearch_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		// Tavily returns "detail" as an object or plain string
		json.NewEncoder(writer).Encode(map[string]interface{}{
			"detail": map[string]string{"error": "invalid api key"},
		}) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("TAVILY_API_KEY", "bad-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	_, err := Search(ctx, SearchInput{Query: "test"})
	if err == nil {
		t.Error("expected error for 401 response")
	}

	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("expected error to contain API error message, got: %s", err.Error())
	}
}

// TestSearch_APIErrorPlainDetail verifies that plain string detail is handled correctly.
func TestSearch_APIErrorPlainDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		// Tavily sometimes returns "detail" as a plain string
		json.NewEncoder(writer).Encode(map[string]string{
			"detail": "access denied",
		}) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("TAVILY_API_KEY", "bad-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	_, err := Search(ctx, SearchInput{Query: "test"})
	if err == nil {
		t.Error("expected error for 403 response")
	}

	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("expected error to contain 'access denied', got: %s", err.Error())
	}
}

func TestExtract_EmptyURLs(t *testing.T) {
	os.Setenv("TAVILY_API_KEY", "test-api-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	input := ExtractInput{
		URLs: []string{},
	}

	_, err := Extract(ctx, input)
	if err == nil {
		t.Error("expected error when URLs is empty")
	}

	if !strings.Contains(err.Error(), "at least one URL") {
		t.Errorf("expected error about empty URLs, got: %s", err.Error())
	}
}

func TestExtract_TooManyURLs(t *testing.T) {
	os.Setenv("TAVILY_API_KEY", "test-api-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()

	// Create 21 URLs (exceeds limit of 20)
	urls := make([]string, 21)
	for index := range urls {
		urls[index] = "https://example.com/" + string(rune('a'+index))
	}

	input := ExtractInput{
		URLs: urls,
	}

	_, err := Extract(ctx, input)
	if err == nil {
		t.Error("expected error when too many URLs")
	}

	if !strings.Contains(err.Error(), "maximum") {
		t.Errorf("expected error about maximum URLs, got: %s", err.Error())
	}
}

func TestExtract_MissingAPIKey(t *testing.T) {
	originalKey := os.Getenv("TAVILY_API_KEY")
	os.Unsetenv("TAVILY_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("TAVILY_API_KEY", originalKey)
		}
	}()

	ctx := context.Background()
	input := ExtractInput{
		URLs: []string{"https://example.com"},
	}

	_, err := Extract(ctx, input)
	if err == nil {
		t.Error("expected error when API key is missing")
	}

	if !strings.Contains(err.Error(), "TAVILY_API_KEY") {
		t.Errorf("expected error message to mention TAVILY_API_KEY, got: %s", err.Error())
	}
}

// TestTruncateContent verifies that truncateContent preserves word boundaries.
func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		contains string
	}{
		{
			name:     "short string unchanged",
			input:    "hello world",
			maxLen:   100,
			contains: "hello world",
		},
		{
			name:     "truncated at word boundary",
			input:    "hello world this is a test",
			maxLen:   15,
			contains: "...",
		},
		{
			name:     "truncated with no space in second half",
			input:    "abcdefghijklmnopqrstuvwxyz",
			maxLen:   10,
			contains: "...",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := truncateContent(testCase.input, testCase.maxLen)
			if !strings.Contains(result, testCase.contains) {
				t.Errorf("truncateContent(%q, %d) = %q, expected to contain %q", testCase.input, testCase.maxLen, result, testCase.contains)
			}
		})
	}
}

// TestParseTavilyError verifies both error formats are handled.
func TestParseTavilyError(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "structured error",
			body:     `{"detail": {"error": "invalid key"}}`,
			expected: "invalid key",
		},
		{
			name:     "plain string detail",
			body:     `{"detail": "not found"}`,
			expected: "not found",
		},
		{
			name:     "unparseable body",
			body:     `<html>error</html>`,
			expected: "",
		},
		{
			name:     "empty detail",
			body:     `{"detail": {}}`,
			expected: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := parseTavilyError([]byte(testCase.body))
			if result != testCase.expected {
				t.Errorf("parseTavilyError(%q) = %q, expected %q", testCase.body, result, testCase.expected)
			}
		})
	}
}

// TestExtract_Success verifies that Extract correctly parses API responses
// and builds a summary from the results.
func TestExtract_Success(t *testing.T) {
	mockResponse := tavilyExtractAPIResponse{
		Results: []tavilyExtractResultItem{
			{
				URL:        "https://example.com/1",
				RawContent: "This is the raw content of test result 1",
			},
			{
				URL:        "https://example.com/2",
				RawContent: "This is the raw content of test result 2",
			},
		},
		FailedResults: []tavilyFailedResult{
			{
				URL:   "https://example.com/3",
				Error: "Not found",
			},
		},
		ResponseTime: 0.5,
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != "POST" {
			t.Errorf("expected POST request, got %s", request.Method)
		}
		if request.URL.Path != "/extract" {
			t.Errorf("expected /extract path, got %s", request.URL.Path)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", request.Header.Get("Content-Type"))
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept application/json, got %s", request.Header.Get("Accept"))
		}

		// Verify request body contains api_key and urls
		var reqBody map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if reqBody["api_key"] == nil {
			t.Error("expected api_key in request body")
		}
		urls, ok := reqBody["urls"].([]interface{})
		if !ok || len(urls) != 2 {
			t.Errorf("expected 2 urls, got %v", reqBody["urls"])
		}

		writer.Header().Set("Content-Type", "application/json")
		json.NewEncoder(writer).Encode(mockResponse) //nolint:errcheck
	}))
	defer server.Close()

	// Override baseURL for testing
	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("TAVILY_API_KEY", "test-api-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	output, err := Extract(ctx, ExtractInput{URLs: []string{"https://example.com/1", "https://example.com/2"}})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(output.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(output.Results))
	}

	if output.Results[0].URL != "https://example.com/1" {
		t.Errorf("expected first result URL 'https://example.com/1', got '%s'", output.Results[0].URL)
	}

	if output.Summary == "" {
		t.Error("expected non-empty summary")
	}

	if !strings.Contains(output.Summary, "Failed to extract 1 URL(s)") {
		t.Errorf("expected summary to contain failed results info, got: %s", output.Summary)
	}
}

// TestExtract_HTTPError verifies that Extract correctly handles non-200 API responses.
func TestExtract_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(writer).Encode(map[string]interface{}{
			"detail": map[string]string{"error": "invalid api key"},
		}) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("TAVILY_API_KEY", "bad-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	_, err := Extract(ctx, ExtractInput{URLs: []string{"https://example.com"}})
	if err == nil {
		t.Error("expected error for 401 response")
	}

	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("expected error to contain API error message, got: %s", err.Error())
	}
}

// TestExtract_MalformedJSON verifies that Extract correctly handles malformed JSON responses.
func TestExtract_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(`{malformed json`))
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("TAVILY_API_KEY", "test-api-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	_, err := Extract(ctx, ExtractInput{URLs: []string{"https://example.com"}})
	if err == nil {
		t.Error("expected error for malformed JSON response")
	}

	if !strings.Contains(err.Error(), "error parsing response") {
		t.Errorf("expected error about parsing response, got: %s", err.Error())
	}
}

// TestFetchTavilyExtract_Success verifies that fetchTavilyExtract correctly parses API responses.
func TestFetchTavilyExtract_Success(t *testing.T) {
	mockResponse := tavilyExtractAPIResponse{
		Results: []tavilyExtractResultItem{
			{
				URL:        "https://example.com/1",
				RawContent: "Content 1",
			},
		},
		ResponseTime: 0.5,
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		json.NewEncoder(writer).Encode(mockResponse) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("TAVILY_API_KEY", "test-api-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	resp, err := fetchTavilyExtract(ctx, ExtractInput{URLs: []string{"https://example.com/1"}})
	if err != nil {
		t.Fatalf("fetchTavilyExtract failed: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].RawContent != "Content 1" {
		t.Errorf("expected 'Content 1', got '%s'", resp.Results[0].RawContent)
	}
}

// TestFetchTavilyExtract_HTTPError verifies that fetchTavilyExtract correctly handles 500 errors.
func TestFetchTavilyExtract_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		writer.Write([]byte(`Internal Server Error`))
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("TAVILY_API_KEY", "test-api-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	_, err := fetchTavilyExtract(ctx, ExtractInput{URLs: []string{"https://example.com"}})
	if err == nil {
		t.Error("expected error for 500 response")
	}

	if !strings.Contains(err.Error(), "unexpected status code 500") {
		t.Errorf("expected error about status code 500, got: %s", err.Error())
	}
}

// TestFetchTavilyExtract_MalformedJSON verifies that fetchTavilyExtract correctly handles malformed JSON.
func TestFetchTavilyExtract_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(`[not an object]`))
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("TAVILY_API_KEY", "test-api-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	_, err := fetchTavilyExtract(ctx, ExtractInput{URLs: []string{"https://example.com"}})
	if err == nil {
		t.Error("expected error for malformed JSON response")
	}

	if !strings.Contains(err.Error(), "error parsing response") {
		t.Errorf("expected error about parsing response, got: %s", err.Error())
	}
}

// TestFetchTavilyExtract_EmptyURLs verifies that fetchTavilyExtract handles empty URLs list.
// Note: Extract() validates this before calling fetchTavilyExtract, but we test the inner function anyway.
func TestFetchTavilyExtract_EmptyURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// Verify request body contains empty urls array or null
		var reqBody map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		// API might return an error for empty URLs, let's simulate a 400 Bad Request
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(map[string]interface{}{
			"detail": "urls cannot be empty",
		}) //nolint:errcheck
	}))
	defer server.Close()

	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	os.Setenv("TAVILY_API_KEY", "test-api-key")
	defer os.Unsetenv("TAVILY_API_KEY")

	ctx := context.Background()
	_, err := fetchTavilyExtract(ctx, ExtractInput{URLs: []string{}})
	if err == nil {
		t.Error("expected error for empty URLs")
	}

	if !strings.Contains(err.Error(), "urls cannot be empty") {
		t.Errorf("expected error about empty urls, got: %s", err.Error())
	}
}
