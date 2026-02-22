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
	maxURLs = 20
)

// NewTavilyExtractTool returns a [tool.Tool] that extracts and parses web page
// content into clean markdown via the Tavily Extract API.
// Supports up to 20 URLs per request. Use [tool.WithDescription] and
// [tool.WithMetrics] to customize the tool after construction if needed.
func NewTavilyExtractTool() *tool.Tool[ExtractInput, ExtractOutput] {
	return tool.NewTool[ExtractInput, ExtractOutput](
		"TavilyExtract",
		Extract,
		tool.WithDescription("Extract and parse content from web pages using Tavily Extract API. Optimized for LLM and RAG applications, returns clean markdown content from specified URLs. Ideal for: reading article content, extracting documentation, parsing web pages for analysis. Supports up to 20 URLs per request. Requires TAVILY_API_KEY environment variable."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.0002, // ~1 credit per 5 URLs (basic)
			Currency:                "USD",
			CostDescription:         "per URL extracted (basic depth)",
			Accuracy:                0.90,
			AverageDurationInMillis: 1500,
		}),
	)
}

// Extract retrieves and parses content from the URLs listed in [ExtractInput],
// returning an [ExtractOutput] with per-URL markdown content and a combined
// summary. It validates that at least one URL is provided and that the total
// does not exceed 20. URLs that the API fails to process are reported in the
// summary but do not cause the function to return an error.
// Returns an error if the TAVILY_API_KEY environment variable is not set, the
// HTTP request fails, or the response cannot be decoded.
func Extract(ctx context.Context, input ExtractInput) (ExtractOutput, error) {
	if len(input.URLs) == 0 {
		return ExtractOutput{}, fmt.Errorf("at least one URL is required")
	}
	if len(input.URLs) > maxURLs {
		return ExtractOutput{}, fmt.Errorf("maximum %d URLs allowed, got %d", maxURLs, len(input.URLs))
	}

	apiResponse, err := fetchTavilyExtract(ctx, input)
	if err != nil {
		return ExtractOutput{}, err
	}

	// Convert to output format
	results := make([]ExtractResult, 0, len(apiResponse.Results))
	var summaryParts []string

	for _, r := range apiResponse.Results {
		results = append(results, ExtractResult{
			URL:        r.URL,
			RawContent: r.RawContent,
			Favicon:    r.Favicon,
		})

		// Build summary with truncated content
		content := truncateContent(r.RawContent, 500)
		summaryParts = append(summaryParts, fmt.Sprintf("## %s\n%s", r.URL, content))
	}

	// Add failed results info to summary if any
	if len(apiResponse.FailedResults) > 0 {
		var failedURLs []string
		for _, f := range apiResponse.FailedResults {
			failedURLs = append(failedURLs, f.URL)
		}
		summaryParts = append(summaryParts, fmt.Sprintf("\n---\nFailed to extract %d URL(s): %s",
			len(apiResponse.FailedResults), strings.Join(failedURLs, ", ")))
	}

	summary := strings.Join(summaryParts, "\n\n")
	if len(results) == 0 {
		summary = "No content could be extracted from the provided URLs."
	}

	return ExtractOutput{
		Results: results,
		Summary: summary,
	}, nil
}

// fetchTavilyExtract performs the API call to Tavily Extract
func fetchTavilyExtract(ctx context.Context, input ExtractInput) (*tavilyExtractAPIResponse, error) {
	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is not set", envAPIKey)
	}

	// Build request body
	reqBody := map[string]interface{}{
		"api_key": apiKey,
		"urls":    input.URLs,
	}

	// Set extraction depth
	if input.ExtractDepth != "" {
		reqBody["extract_depth"] = input.ExtractDepth
	} else {
		reqBody["extract_depth"] = "basic"
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/extract", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

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
		errMsg := parseTavilyError(body)
		if errMsg != "" {
			return nil, fmt.Errorf("tavily API error (status %d): %s", resp.StatusCode, errMsg)
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var apiResponse tavilyExtractAPIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return &apiResponse, nil
}

// truncateContent truncates content preserving word boundaries
func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	// Find last space before maxLen
	truncated := s[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen/2 {
		truncated = truncated[:lastSpace]
	}

	return truncated + "..."
}
