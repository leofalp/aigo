package exa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/tool"
)

// baseURL is the Exa API base URL. It is a var (not const) to allow
// overriding in unit tests with httptest.NewServer.
var baseURL = "https://api.exa.ai" //nolint:gochecknoglobals // overridable for tests

const (
	envAPIKey      = "EXA_API_KEY" //nolint:gosec // Environment variable name, not a credential
	maxResults     = 100
	defaultResults = 10
	// maxSummaryResults caps the number of results included in the text summary,
	// regardless of how many results were actually returned by the API.
	maxSummaryResults = 10
	// maxBodySize is the maximum response body size (10 MB). Enforced via
	// io.LimitReader to prevent unbounded memory allocation from rogue responses.
	maxBodySize = 10 * 1024 * 1024
)

// httpClient is a shared HTTP client with a default timeout for connection reuse.
var httpClient = &http.Client{Timeout: 30 * time.Second} //nolint:gochecknoglobals // shared for connection reuse

// NewExaSearchTool returns a Tool that performs semantic web search via the
// Exa API and produces a summarized, LLM-optimized [SearchOutput]. The tool is
// registered with an estimated cost of $0.003 per query. Use
// [NewExaSearchAdvancedTool] when full structured metadata is required.
func NewExaSearchTool() *tool.Tool[SearchInput, SearchOutput] {
	return tool.NewTool[SearchInput, SearchOutput](
		"ExaSearch",
		Search,
		tool.WithDescription("Search the web using Exa's AI-native semantic search engine. Uses neural embeddings for highly relevant results. Works well for: research papers, technical content, news, company information, GitHub repos, and specific content categories. Returns a summary of top results with titles, URLs, and content. Requires EXA_API_KEY environment variable."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.003, // Estimated per search
			Currency:                "USD",
			CostDescription:         "per neural search query",
			Accuracy:                0.93, // High accuracy - neural embedding search
			AverageDurationInMillis: 1000,
		}),
	)
}

// NewExaSearchAdvancedTool returns a Tool that performs semantic web search via
// the Exa API and produces a complete [SearchAdvancedOutput] containing all
// metadata, relevance scores, and content. The tool is registered with an
// estimated cost of $0.005 per query due to content extraction. Use
// [NewExaSearchTool] when a concise summary is sufficient.
func NewExaSearchAdvancedTool() *tool.Tool[SearchInput, SearchAdvancedOutput] {
	return tool.NewTool[SearchInput, SearchAdvancedOutput](
		"ExaSearchAdvanced",
		SearchAdvanced,
		tool.WithDescription("Advanced semantic web search using Exa API with complete structured results. Returns detailed information including full text content, relevance scores, highlights, and metadata. Ideal when you need comprehensive search data with neural ranking. Supports category filtering (research paper, news, company, github, etc). Requires EXA_API_KEY environment variable."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.005, // Higher due to content extraction
			Currency:                "USD",
			CostDescription:         "per advanced search with content extraction",
			Accuracy:                0.94, // Very high accuracy with full content
			AverageDurationInMillis: 1500,
		}),
	)
}

// Search performs a semantic web search using the Exa API and returns a summarized
// result optimized for LLM consumption. Requires the EXA_API_KEY environment variable.
// If NumResults is 0 or negative, defaults to 10. Returns an error if the query is
// empty, the API key is missing, or the API returns a non-200 status.
func Search(ctx context.Context, input SearchInput) (SearchOutput, error) {
	if input.Query == "" {
		return SearchOutput{}, fmt.Errorf("query is required")
	}

	apiResponse, err := fetchExaSearch(ctx, input)
	if err != nil {
		return SearchOutput{}, err
	}

	// Convert to simplified output
	results := make([]SearchResult, 0, len(apiResponse.Results))
	var summaryParts []string

	if len(apiResponse.Results) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("Found %d results:", len(apiResponse.Results)))
	}

	for index, result := range apiResponse.Results {
		if index >= maxSummaryResults {
			break
		}

		searchResult := SearchResult{
			Title:         result.Title,
			URL:           result.URL,
			PublishedDate: result.PublishedDate,
			Author:        result.Author,
			Text:          result.Text,
			Highlights:    result.Highlights,
		}
		results = append(results, searchResult)

		// Build summary entry
		var entryParts []string
		entryParts = append(entryParts, fmt.Sprintf("\n%d. %s", index+1, result.Title))
		entryParts = append(entryParts, fmt.Sprintf("   URL: %s", result.URL))

		if result.Author != "" {
			entryParts = append(entryParts, fmt.Sprintf("   Author: %s", result.Author))
		}
		if result.PublishedDate != "" {
			entryParts = append(entryParts, fmt.Sprintf("   Published: %s", result.PublishedDate))
		}

		// Add text preview or highlights
		if len(result.Highlights) > 0 {
			entryParts = append(entryParts, fmt.Sprintf("   Highlight: %s", utils.TruncateString(result.Highlights[0], 200)))
		} else if result.Text != "" {
			entryParts = append(entryParts, fmt.Sprintf("   Preview: %s", utils.TruncateString(result.Text, 200)))
		}

		summaryParts = append(summaryParts, strings.Join(entryParts, "\n"))
	}

	summary := strings.Join(summaryParts, "\n")
	if summary == "" {
		summary = fmt.Sprintf("No results found for '%s'. Try a different query or adjust filters.", input.Query)
	}

	return SearchOutput{
		Query:   input.Query,
		Summary: summary,
		Results: results,
	}, nil
}

// SearchAdvanced performs a semantic web search using the Exa API and returns
// complete structured results with all metadata fields populated. Requires the
// EXA_API_KEY environment variable. Returns an error if Query is empty, the
// API key is missing, or the API returns a non-200 status.
func SearchAdvanced(ctx context.Context, input SearchInput) (SearchAdvancedOutput, error) {
	if input.Query == "" {
		return SearchAdvancedOutput{}, fmt.Errorf("query is required")
	}

	apiResponse, err := fetchExaSearch(ctx, input)
	if err != nil {
		return SearchAdvancedOutput{}, err
	}

	// Convert results
	results := make([]SearchResultAdvanced, 0, len(apiResponse.Results))
	for _, r := range apiResponse.Results {
		results = append(results, SearchResultAdvanced(r))
	}

	return SearchAdvancedOutput{
		Query:              input.Query,
		Results:            results,
		ResolvedSearchType: apiResponse.ResolvedSearchType,
		RequestID:          apiResponse.RequestID,
	}, nil
}

// fetchExaSearch performs the API call to Exa Search
func fetchExaSearch(ctx context.Context, input SearchInput) (*exaSearchAPIResponse, error) {
	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is not set", envAPIKey)
	}

	// Build request body
	reqBody := map[string]interface{}{
		"query": input.Query,
	}

	// Set search type
	if input.Type != "" {
		reqBody["type"] = input.Type
	} else {
		reqBody["type"] = "auto"
	}

	// Set number of results
	numRes := input.NumResults
	if numRes <= 0 {
		numRes = defaultResults
	}
	if numRes > maxResults {
		numRes = maxResults
	}
	reqBody["numResults"] = numRes

	// Optional filters
	if len(input.IncludeDomains) > 0 {
		reqBody["includeDomains"] = input.IncludeDomains
	}
	if len(input.ExcludeDomains) > 0 {
		reqBody["excludeDomains"] = input.ExcludeDomains
	}
	if input.StartPublishedDate != "" {
		reqBody["startPublishedDate"] = input.StartPublishedDate
	}
	if input.EndPublishedDate != "" {
		reqBody["endPublishedDate"] = input.EndPublishedDate
	}
	if input.StartCrawlDate != "" {
		reqBody["startCrawlDate"] = input.StartCrawlDate
	}
	if input.EndCrawlDate != "" {
		reqBody["endCrawlDate"] = input.EndCrawlDate
	}
	if input.Category != "" {
		reqBody["category"] = input.Category
	}

	// Content options
	if input.IncludeText || input.IncludeHighlights {
		contents := make(map[string]interface{})
		if input.IncludeText {
			contents["text"] = true
		}
		if input.IncludeHighlights {
			contents["highlights"] = map[string]interface{}{
				"numSentences":     3,
				"highlightsPerUrl": 3,
			}
		}
		reqBody["contents"] = contents
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/search", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer utils.CloseWithLog(resp.Body)

	// Cap body reads to maxBodySize to prevent unbounded memory allocation.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr exaAPIError
		if err := json.Unmarshal(body, &apiErr); err == nil {
			errMsg := apiErr.ErrorMessage
			if errMsg == "" {
				errMsg = apiErr.Message
			}
			if errMsg != "" {
				return nil, fmt.Errorf("exa API error (status %d): %s", resp.StatusCode, errMsg)
			}
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var apiResponse exaSearchAPIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return &apiResponse, nil
}
