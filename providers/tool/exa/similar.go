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

// NewExaFindSimilarTool creates a new Exa FindSimilar tool for finding similar content.
// Returns content similar to a given URL.
func NewExaFindSimilarTool() *tool.Tool[SimilarInput, SimilarOutput] {
	return tool.NewTool[SimilarInput, SimilarOutput](
		"ExaFindSimilar",
		FindSimilar,
		tool.WithDescription("Find web pages similar to a given URL using Exa's semantic similarity search. Useful for: finding related articles, discovering similar research papers, exploring content clusters, finding alternatives to a given resource. Requires a URL as input. Requires EXA_API_KEY environment variable."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.003, // Estimated per search
			Currency:                "USD",
			CostDescription:         "per similarity search",
			Accuracy:                0.91, // High accuracy for semantic similarity
			AverageDurationInMillis: 900,
		}),
	)
}

// FindSimilar calls the Exa findSimilar endpoint and returns web pages
// semantically similar to the provided URL. The Exa API requires a URL for
// this endpoint — text-only similarity is not supported. Returns an error if
// URL is empty, EXA_API_KEY is not set, or the API returns a non-200 status.
func FindSimilar(ctx context.Context, input SimilarInput) (SimilarOutput, error) {
	// The Exa /findSimilar API requires a URL
	if input.URL == "" {
		return SimilarOutput{}, fmt.Errorf("url is required for similarity search")
	}

	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		return SimilarOutput{}, fmt.Errorf("%s environment variable is not set", envAPIKey)
	}

	// Build request body — url is the only required field
	reqBody := map[string]interface{}{
		"url": input.URL,
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
		return SimilarOutput{}, fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/findSimilar", bytes.NewReader(jsonBody))
	if err != nil {
		return SimilarOutput{}, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return SimilarOutput{}, fmt.Errorf("error making request: %w", err)
	}
	defer utils.CloseWithLog(resp.Body)

	// Cap body reads to maxBodySize to prevent unbounded memory allocation.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return SimilarOutput{}, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr exaAPIError
		if err := json.Unmarshal(body, &apiErr); err == nil {
			errMsg := apiErr.ErrorMessage
			if errMsg == "" {
				errMsg = apiErr.Message
			}
			if errMsg != "" {
				return SimilarOutput{}, fmt.Errorf("exa API error (status %d): %s", resp.StatusCode, errMsg)
			}
		}
		return SimilarOutput{}, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var apiResponse exaSearchAPIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return SimilarOutput{}, fmt.Errorf("error parsing response: %w", err)
	}

	// Convert to output format
	results := make([]SearchResult, 0, len(apiResponse.Results))
	var summaryParts []string

	if len(apiResponse.Results) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("Found %d pages similar to %s:", len(apiResponse.Results), input.URL))
	}

	for index, apiResult := range apiResponse.Results {
		if index >= maxSummaryResults {
			break
		}

		searchResult := SearchResult{
			Title:         apiResult.Title,
			URL:           apiResult.URL,
			PublishedDate: apiResult.PublishedDate,
			Author:        apiResult.Author,
			Text:          apiResult.Text,
			Highlights:    apiResult.Highlights,
		}
		results = append(results, searchResult)

		// Build summary entry
		var entryParts []string
		entryParts = append(entryParts, fmt.Sprintf("\n%d. %s", index+1, apiResult.Title))
		entryParts = append(entryParts, fmt.Sprintf("   URL: %s", apiResult.URL))

		if apiResult.Author != "" {
			entryParts = append(entryParts, fmt.Sprintf("   Author: %s", apiResult.Author))
		}

		// Add text preview or highlights
		if len(apiResult.Highlights) > 0 {
			entryParts = append(entryParts, fmt.Sprintf("   Highlight: %s", utils.TruncateString(apiResult.Highlights[0], 200)))
		} else if apiResult.Text != "" {
			entryParts = append(entryParts, fmt.Sprintf("   Preview: %s", utils.TruncateString(apiResult.Text, 200)))
		}

		summaryParts = append(summaryParts, strings.Join(entryParts, "\n"))
	}

	summary := strings.Join(summaryParts, "\n")
	if summary == "" {
		summary = fmt.Sprintf("No similar content found for %s.", input.URL)
	}

	return SimilarOutput{
		Source:  input.URL,
		Summary: summary,
		Results: results,
	}, nil
}
