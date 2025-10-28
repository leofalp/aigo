package openai

import (
	"aigo/providers/ai"
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
	apiKey       string
	model        string
	baseURL      string
	client       *http.Client
	systemPrompt string
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
func (p *OpenAIProvider) WithAPIKey(apiKey string) ai.Provider {
	p.apiKey = apiKey
	return p
}

// WithModel sets the model to use for requests
func (p *OpenAIProvider) WithModel(model string) ai.Provider {
	p.model = model
	return p
}

// WithBaseURL sets the base URL for the API
func (p *OpenAIProvider) WithBaseURL(baseURL string) ai.Provider {
	p.baseURL = baseURL
	return p
}

// WithSystemPrompt sets the default system prompt for conversations.
func (p *OpenAIProvider) WithSystemPrompt(prompt string) ai.Provider {
	p.systemPrompt = prompt
	return p
}

// WithHttpClient sets a custom HTTP client
func (p *OpenAIProvider) WithHttpClient(httpClient *http.Client) ai.Provider {
	p.client = httpClient
	return p
}

// GetModelName returns the current model name
func (p *OpenAIProvider) GetModelName() string {
	return p.model
}

// SendMessage implements the Provider interface
func (p *OpenAIProvider) SendSingleMessage(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("API key is not set")
	}
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
				Role      string        `json:"role"`
				Content   string        `json:"content"`
				ToolCalls []ai.ToolCall `json:"tool_calls,omitempty"`
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
	return &ai.ChatResponse{
		Content:      choice.Message.Content,
		ToolCalls:    choice.Message.ToolCalls,
		FinishReason: choice.FinishReason,
	}, nil
}

// IsStopMessage reports whether the given chat response should be treated as a stop/end signal.
func (p *OpenAIProvider) IsStopMessage(message *ai.ChatResponse) bool {
	if message == nil {
		return true
	}
	// Prefer explicit finish reason from API
	if message.FinishReason == "stop" {
		return true
	}
	// If there's no content and no tool calls, treat as stop
	if message.Content == "" && len(message.ToolCalls) == 0 {
		return true
	}
	return false
}
