package openai

import (
	"aigo/providers/ai"
	"encoding/json"
)

// apiResponse represents the JSON response returned by the OpenAI API.
// It mirrors the fields consumed by this package: id, object, created, model,
// choices, usage and system fingerprint.
type apiResponse struct {
	// Id is the response identifier.
	Id string `json:"id"`
	// Object is the type of object returned (for example "chat.completion").
	Object string `json:"object"`
	// Created is a Unix timestamp when the response was generated.
	Created int `json:"created"`
	// Model is the name of the model that produced the response.
	Model string `json:"model"`
	// Choices contains one or more generated completions/choices.
	Choices []struct {
		// Index is the choice index in the choices array.
		Index int `json:"index"`
		// Message is the message payload for this choice.
		Message struct {
			// Role indicates the message role (e.g. "assistant", "user").
			Role string `json:"role"`
			// Content holds the message content.
			Content string `json:"content"`
			// ToolCalls contains any tool invocation metadata returned with the message.
			ToolCalls []ai.ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		// LogProbs contains token-level log probability info when provided.
		LogProbs json.RawMessage `json:"logprobs"`
		// FinishReason explains why the model stopped (e.g. "stop", "length").
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	// Usage reports token usage counts for prompt/completion/total.
	Usage struct {
		// PromptTokens is the number of prompt tokens consumed.
		PromptTokens int `json:"prompt_tokens"`
		// CompletionTokens is the number of completion tokens generated.
		CompletionTokens int `json:"completion_tokens"`
		// TotalTokens is the total number of tokens for the request/response.
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	// SystemFingerprint is an optional fingerprint provided by the API.
	SystemFingerprint string `json:"system_fingerprint"`
}
