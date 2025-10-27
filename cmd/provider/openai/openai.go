package openai

import (
	"aigo/cmd/provider"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	defaultBaseURL          = "https://api.openai.com/v1"
	defaultModel            = "gpt-4o-mini"
	chatCompletionsEndpoint = "/chat/completions"
)

// OpenAIProvider implements the Provider interface for OpenAI API
type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAIProvider creates a new OpenAI provider instance with default values
func NewOpenAIProvider() *OpenAIProvider {
	apiKey := os.Getenv("OPENAI_API_KEY")

	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   defaultModel,
		baseURL: defaultBaseURL,
		client:  &http.Client{},
	}
}

// WithAPIKey sets the API key for the provider
func (p *OpenAIProvider) WithAPIKey(apiKey string) provider.Provider {
	p.apiKey = apiKey
	return p
}

// WithModel sets the model to use for requests
func (p *OpenAIProvider) WithModel(model string) provider.Provider {
	p.model = model
	return p
}

// WithBaseURL sets the base URL for the API
func (p *OpenAIProvider) WithBaseURL(baseURL string) provider.Provider {
	p.baseURL = baseURL
	return p
}

// WithHTTPClient sets a custom HTTP client
func (p *OpenAIProvider) WithHTTPClient(client *http.Client) *OpenAIProvider {
	p.client = client
	return p
}

// SendMessage implements the Provider interface
func (p *OpenAIProvider) SendSingleMessage(ctx context.Context, request provider.ChatRequest) (*provider.ChatResponse, error) {
	// Build the request body
	bodyMap := map[string]interface{}{
		"model":    p.model,
		"messages": request.Messages,
	}

	// Add tools if provided
	if len(request.Tools) > 0 {
		tools := make([]map[string]interface{}, 0, len(request.Tools))
		for _, tool := range request.Tools {
			tools = append(tools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"parameters":  tool.Parameters,
				},
			})
		}
		bodyMap["tools"] = tools
	}

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+chatCompletionsEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Send request
	res, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Check status code
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("non-2xx status %d: %s", res.StatusCode, string(respBody))
	}

	// Parse response
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Role      string              `json:"role"`
				Content   string              `json:"content"`
				ToolCalls []provider.ToolCall `json:"tool_calls,omitempty"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &apiResponse); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := apiResponse.Choices[0]
	return &provider.ChatResponse{
		Content:      choice.Message.Content,
		ToolCalls:    choice.Message.ToolCalls,
		FinishReason: choice.FinishReason,
	}, nil
}

// GetModelName returns the current model name
func (p *OpenAIProvider) GetModelName() string {
	return p.model
}

// SetModel sets the model to use for requests
func (p *OpenAIProvider) SetModel(model string) {
	p.model = model
}

// SetBaseURL allows changing the base URL (useful for OpenAI-compatible APIs like OpenRouter)
func (p *OpenAIProvider) SetBaseURL(baseURL string) {
	p.baseURL = baseURL
}
