package tavily

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
	baseURL    = "https://api.tavily.com"
	envAPIKey  = "TAVILY_API_KEY"
	maxResults = 20
)

// NewTavilySearchTool creates a new Tavily Search tool for web search.
// Returns summarized results optimized for LLM consumption.
func NewTavilySearchTool() *tool.Tool[SearchInput, SearchOutput] {
	return tool.NewTool[SearchInput, SearchOutput](
		"TavilySearch",
		Search,
		tool.WithDescription("Search the web using Tavily API, optimized for LLM and RAG applications. Provides high-quality, AI-optimized web search results with optional AI-generated answers. Works well for: current events, factual information, research queries, news, and general web searches. Returns a summary of top results with titles, URLs, and content snippets. Requires TAVILY_API_KEY environment variable."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.001, // ~1 credit per basic search
			Currency:                "USD",
			CostDescription:         "per basic search query (1 API credit)",
			Accuracy:                0.92, // High accuracy - AI-optimized search
			AverageDurationInMillis: 800,
		}),
	)
}

// NewTavilySearchAdvancedTool creates a new Tavily Search tool with detailed results.
// Returns complete structured data including all metadata.
func NewTavilySearchAdvancedTool() *tool.Tool[SearchInput, SearchAdvancedOutput] {
	return tool.NewTool[SearchInput, SearchAdvancedOutput](
		"TavilySearchAdvanced",
		SearchAdvanced,
		tool.WithDescription("Advanced web search using Tavily API with complete structured results. Returns detailed information including full content, relevance scores, images, and AI-generated answers. Ideal when you need comprehensive search data with all metadata. Use search_depth='advanced' for more thorough results. Requires TAVILY_API_KEY environment variable."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.002, // ~2 credits per advanced search
			Currency:                "USD",
			CostDescription:         "per advanced search query (2 API credits)",
			Accuracy:                0.94, // Very high accuracy with advanced depth
			AverageDurationInMillis: 1200,
		}),
	)
}

// Search performs a web search and returns a summarized result optimized for LLMs
func Search(ctx context.Context, input SearchInput) (SearchOutput, error) {
	apiResponse, err := fetchTavilySearch(ctx, input)
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
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Content,
			Score:   r.Score,
		}
		results = append(results, result)

		summaryParts = append(summaryParts, fmt.Sprintf("\n%d. %s\n   URL: %s\n   %s",
			i+1, r.Title, r.URL, truncate(r.Content, 200)))
	}

	summary := strings.Join(summaryParts, "\n")
	if summary == "" {
		summary = fmt.Sprintf("No results found for '%s'. Try a different query or check your spelling.", input.Query)
	}

	return SearchOutput{
		Query:   input.Query,
		Answer:  apiResponse.Answer,
		Summary: summary,
		Results: results,
	}, nil
}

// SearchAdvanced performs a web search and returns complete structured results
func SearchAdvanced(ctx context.Context, input SearchInput) (SearchAdvancedOutput, error) {
	apiResponse, err := fetchTavilySearch(ctx, input)
	if err != nil {
		return SearchAdvancedOutput{}, err
	}

	// Convert results
	results := make([]SearchResultAdvanced, 0, len(apiResponse.Results))
	for _, r := range apiResponse.Results {
		results = append(results, SearchResultAdvanced{
			Title:      r.Title,
			URL:        r.URL,
			Content:    r.Content,
			RawContent: r.RawContent,
			Score:      r.Score,
			Images:     r.Images,
		})
	}

	// Convert images
	images := make([]ImageResult, 0, len(apiResponse.Images))
	for _, img := range apiResponse.Images {
		images = append(images, ImageResult{
			URL:         img.URL,
			Description: img.Description,
		})
	}

	return SearchAdvancedOutput{
		Query:        input.Query,
		Answer:       apiResponse.Answer,
		Results:      results,
		Images:       images,
		ResponseTime: apiResponse.ResponseTime,
		RequestID:    apiResponse.RequestID,
	}, nil
}

// fetchTavilySearch performs the API call to Tavily Search
func fetchTavilySearch(ctx context.Context, input SearchInput) (*tavilySearchAPIResponse, error) {
	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is not set", envAPIKey)
	}

	// Build request body
	reqBody := map[string]interface{}{
		"api_key": apiKey,
		"query":   input.Query,
	}

	// Set defaults and optional parameters
	if input.SearchDepth != "" {
		reqBody["search_depth"] = input.SearchDepth
	} else {
		reqBody["search_depth"] = "basic"
	}

	maxRes := input.MaxResults
	if maxRes <= 0 {
		maxRes = 10
	}
	if maxRes > maxResults {
		maxRes = maxResults
	}
	reqBody["max_results"] = maxRes

	if len(input.IncludeDomains) > 0 {
		reqBody["include_domains"] = input.IncludeDomains
	}
	if len(input.ExcludeDomains) > 0 {
		reqBody["exclude_domains"] = input.ExcludeDomains
	}
	if input.IncludeAnswer {
		reqBody["include_answer"] = true
	}
	if input.IncludeImages {
		reqBody["include_images"] = true
	}
	if input.Topic != "" {
		reqBody["topic"] = input.Topic
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
		var apiErr tavilyAPIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Detail.Error != "" {
			return nil, fmt.Errorf("tavily API error (status %d): %s", resp.StatusCode, apiErr.Detail.Error)
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var apiResponse tavilySearchAPIResponse
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
