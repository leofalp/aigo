package provider

import (
	"aigo/pkg/tool"
	"context"
)

// Message represents a single message in a conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolCall represents a function/tool call request from the LLM
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

// ChatResponse represents the response from a chat completion
type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
}

// ChatRequest represents a request to send a chat message
type ChatRequest struct {
	Messages []Message       `json:"messages"` // TODO better a generic array on few distinct message like system prompt and  user prompt? Why called message instead of prompt?
	Tools    []tool.ToolInfo `json:"tools,omitempty"`
}

// Provider is the generic interface that all LLM providers must implement
type Provider interface {
	// SendSingleMessage sends a chat request and returns the response
	SendSingleMessage(ctx context.Context, request ChatRequest) (*ChatResponse, error)

	// GetModelName returns the name of the model being used
	GetModelName() string

	// SetModel sets the model to use for requests
	SetModel(model string)

	// Common builder methods that all providers should support
	WithAPIKey(apiKey string) Provider
	WithModel(model string) Provider
	WithBaseURL(baseURL string) Provider
}
