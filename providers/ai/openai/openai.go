package openai

import (
	"aigo/internal/utils"
	"aigo/providers/ai"
	"context"
	"encoding/json"
	"fmt"
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
func (p *OpenAIProvider) SendMessage(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
	// check API key
	if p.apiKey == "" {
		return nil, fmt.Errorf("API key is not set")
	}

	// Prepend system prompt into messages, if set
	var messages []ai.Message
	if p.systemPrompt != "" {
		messages = append(messages, ai.Message{
			Role:    ai.RoleSystem,
			Content: p.systemPrompt,
		})
	}
	messages = append(messages, request.Messages...)

	// Build the request body
	bodyMap := map[string]interface{}{
		"model":    p.model,
		"messages": messages,
		"tools":    request.Tools,
	}

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %w", err)
	}

	httpResponse, resp, err := utils.DoPostSync[apiResponse](*p.client, p.baseURL+chatCompletionsEndpoint, p.apiKey, body)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response") // TODO is this an error?
	}

	choice := resp.Choices[0]
	return &ai.ChatResponse{
		HttpResponse: httpResponse,
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
	if message.FinishReason == "stop" || message.FinishReason == "length" || message.FinishReason == "content_filter" || message.FinishReason == "null" {
		return true
	}
	// If there's no content and no tool calls, treat as stop
	if message.Content == "" && len(message.ToolCalls) == 0 {
		return true
	}
	return false
}
