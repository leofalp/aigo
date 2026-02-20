package exa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/internal/utils" //nolint:staticcheck // used for CloseWithLog
	"github.com/leofalp/aigo/providers/tool"
)

// NewExaAnswerTool creates a new Exa Answer tool for AI-generated answers with citations.
// Returns an answer to a question backed by web sources.
func NewExaAnswerTool() *tool.Tool[AnswerInput, AnswerOutput] {
	return tool.NewTool[AnswerInput, AnswerOutput](
		"ExaAnswer",
		Answer,
		tool.WithDescription("Get an AI-generated answer to a question, grounded by citations from Exa's web search. The answer is generated based on relevant web sources and includes citations for verification. Ideal for: factual questions, research queries, getting summarized answers with sources. Requires EXA_API_KEY environment variable."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.01, // Higher cost due to LLM processing
			Currency:                "USD",
			CostDescription:         "per answer generation with citations",
			Accuracy:                0.88, // Good accuracy but depends on sources
			AverageDurationInMillis: 3000, // Slower due to search + LLM
		}),
	)
}

// Answer generates an AI answer to a question with citations from web sources
func Answer(ctx context.Context, input AnswerInput) (AnswerOutput, error) {
	if input.Query == "" {
		return AnswerOutput{}, fmt.Errorf("query is required")
	}

	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		return AnswerOutput{}, fmt.Errorf("%s environment variable is not set", envAPIKey)
	}

	// Build request body
	reqBody := map[string]interface{}{
		"query": input.Query,
	}

	// Include text content from citation sources when requested.
	// The Exa Answer API expects text inclusion inside a "contents" object.
	if input.IncludeText {
		reqBody["contents"] = map[string]interface{}{
			"text": true,
		}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return AnswerOutput{}, fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/answer", bytes.NewReader(jsonBody))
	if err != nil {
		return AnswerOutput{}, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return AnswerOutput{}, fmt.Errorf("error making request: %w", err)
	}
	defer utils.CloseWithLog(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return AnswerOutput{}, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr exaAPIError
		if err := json.Unmarshal(body, &apiErr); err == nil {
			errMsg := apiErr.ErrorMessage
			if errMsg == "" {
				errMsg = apiErr.Message
			}
			if errMsg != "" {
				return AnswerOutput{}, fmt.Errorf("exa API error (status %d): %s", resp.StatusCode, errMsg)
			}
		}
		return AnswerOutput{}, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var apiResponse exaAnswerAPIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return AnswerOutput{}, fmt.Errorf("error parsing response: %w", err)
	}

	// Convert citations from API response
	// Citations can come from either "citations" or "results" field
	var sourceCitations []exaSearchResultItem
	if len(apiResponse.Citations) > 0 {
		sourceCitations = apiResponse.Citations
	} else if len(apiResponse.Results) > 0 {
		sourceCitations = apiResponse.Results
	}

	citations := make([]Citation, 0, len(sourceCitations))
	for _, c := range sourceCitations {
		citations = append(citations, Citation{
			Title:         c.Title,
			URL:           c.URL,
			Author:        c.Author,
			PublishedDate: c.PublishedDate,
			Text:          c.Text,
		})
	}

	return AnswerOutput{
		Query:     input.Query,
		Answer:    apiResponse.Answer,
		Citations: citations,
	}, nil
}
