package openai

import (
	"aigo/internal/utils"
	"aigo/providers/ai"
	"context"
	"fmt"
	"net/http"
	"os"
)

const (
	defaultBaseURL          = "https://api.openai.com/v1"
	chatCompletionsEndpoint = "/responses"
)

// OpenAIProvider implements the Provider interface for OpenAI API
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAIProvider creates a new OpenAI provider instance with default values
func NewOpenAIProvider() *OpenAIProvider {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_API_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

// WithAPIKey sets the API key for the provider
func (p *OpenAIProvider) WithAPIKey(apiKey string) ai.Provider {
	p.apiKey = apiKey
	return p
}

// WithBaseURL sets the base URL for the API
func (p *OpenAIProvider) WithBaseURL(baseURL string) ai.Provider {
	p.baseURL = baseURL
	return p
}

// WithHttpClient sets a custom HTTP client
func (p *OpenAIProvider) WithHttpClient(httpClient *http.Client) ai.Provider {
	p.client = httpClient
	return p
}

// SendMessage implements the Provider interface
func (p *OpenAIProvider) SendMessage(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
	// check API key
	if p.apiKey == "" {
		return nil, fmt.Errorf("API key is not set")
	}

	httpResponse, resp, err := utils.DoPostSync[response](*p.client, p.baseURL+chatCompletionsEndpoint, p.apiKey, requestFromGeneric(request))
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, fmt.Errorf("empty response from OpenAI API: %s", httpResponse.Status)
	}

	if len(resp.Output) == 0 {
		return nil, fmt.Errorf("no choices in response") // TODO is this an error?
	}

	return responseToGeneric(*resp), nil
}

// IsStopMessage reports whether the given chat response should be treated as a stop/end signal.
func (p *OpenAIProvider) IsStopMessage(message *ai.ChatResponse) bool {
	if message == nil {
		return true
	}
	// Prefer explicit finish reason from API
	if message.FinishReason == "stop" || message.FinishReason == "length" || message.FinishReason == "content_filter" {
		return true
	}
	// If there's no content and no tool calls, treat as stop
	if message.Content == "" && len(message.ToolCalls) == 0 {
		return true
	}
	return false
}
