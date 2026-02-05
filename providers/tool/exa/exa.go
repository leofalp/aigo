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

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/tool"
)

const (
	baseURL        = "https://api.exa.ai"
	envAPIKey      = "EXA_API_KEY"
	maxResults     = 100
	defaultResults = 10
)

// NewExaSearchTool creates a new Exa Search tool for semantic web search.
// Returns summarized results optimized for LLM consumption.
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

// NewExaSearchAdvancedTool creates a new Exa Search tool with detailed results.
// Returns complete structured data including all metadata, text content, and highlights.
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

// Search performs a semantic web search and returns a summarized result optimized for LLMs
func Search(ctx context.Context, input SearchInput) (SearchOutput, error) {
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

	for i, r := range apiResponse.Results {
		if i >= 10 { // Limit summary to top 10
			break
		}

		result := SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			PublishedDate: r.PublishedDate,
			Author:        r.Author,
			Text:          r.Text,
			Highlights:    r.Highlights,
		}
		results = append(results, result)

		// Build summary entry
		var entryParts []string
		entryParts = append(entryParts, fmt.Sprintf("\n%d. %s", i+1, r.Title))
		entryParts = append(entryParts, fmt.Sprintf("   URL: %s", r.URL))

		if r.Author != "" {
			entryParts = append(entryParts, fmt.Sprintf("   Author: %s", r.Author))
		}
		if r.PublishedDate != "" {
			entryParts = append(entryParts, fmt.Sprintf("   Published: %s", r.PublishedDate))
		}

		// Add text preview or highlights
		if len(r.Highlights) > 0 {
			entryParts = append(entryParts, fmt.Sprintf("   Highlight: %s", truncate(r.Highlights[0], 200)))
		} else if r.Text != "" {
			entryParts = append(entryParts, fmt.Sprintf("   Preview: %s", truncate(r.Text, 200)))
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

// SearchAdvanced performs a semantic web search and returns complete structured results
func SearchAdvanced(ctx context.Context, input SearchInput) (SearchAdvancedOutput, error) {
	apiResponse, err := fetchExaSearch(ctx, input)
	if err != nil {
		return SearchAdvancedOutput{}, err
	}

	// Convert results
	results := make([]SearchResultAdvanced, 0, len(apiResponse.Results))
	for _, r := range apiResponse.Results {
		results = append(results, SearchResultAdvanced{
			ID:              r.ID,
			Title:           r.Title,
			URL:             r.URL,
			Score:           r.Score,
			PublishedDate:   r.PublishedDate,
			Author:          r.Author,
			Text:            r.Text,
			Highlights:      r.Highlights,
			HighlightScores: r.HighlightScores,
			Summary:         r.Summary,
		})
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer utils.CloseWithLog(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr exaAPIError
		if err := json.Unmarshal(body, &apiErr); err == nil {
			errMsg := apiErr.Error
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

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
