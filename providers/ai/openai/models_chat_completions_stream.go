package openai

import "encoding/json"

/*
	CHAT COMPLETIONS STREAMING API - RESPONSE TYPES

	These types model the SSE chunks returned by the /v1/chat/completions
	endpoint when stream=true. Each chunk carries incremental deltas for
	content, tool calls, and optionally usage metadata (when stream_options
	includes include_usage).
*/

// chatCompletionStreamChunk represents a single SSE chunk from the streaming
// chat completions endpoint.
type chatCompletionStreamChunk struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"` // "chat.completion.chunk"
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
	Choices           []streamChoice `json:"choices"`
	Usage             *chatUsage     `json:"usage,omitempty"` // Present only in final chunk when stream_options.include_usage is true
}

// streamChoice represents a single choice in a streaming chunk.
// Unlike the non-streaming chatChoice, it uses Delta instead of Message.
type streamChoice struct {
	Index        int          `json:"index"`
	Delta        streamDelta  `json:"delta"`
	FinishReason *string      `json:"finish_reason"` // Nullable; nil until the final chunk for this choice
	Logprobs     *interface{} `json:"logprobs,omitempty"`
}

// streamDelta carries the incremental content for a streaming chunk.
// All fields are optional — a chunk may carry only content, only tool calls,
// only a role, etc.
type streamDelta struct {
	Role      string               `json:"role,omitempty"`
	Content   *string              `json:"content,omitempty"`   // Nullable to distinguish empty string from absent
	Refusal   *string              `json:"refusal,omitempty"`   // Model refusal delta
	Reasoning *string              `json:"reasoning,omitempty"` // Reasoning/thinking delta
	ToolCalls []streamToolCallPart `json:"tool_calls,omitempty"`
}

// streamToolCallPart represents an incremental tool call delta in a streaming chunk.
// The first chunk for a tool call carries the ID and function name; subsequent chunks
// carry argument fragments.
type streamToolCallPart struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"` // Present only in the first chunk for this tool call
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

// streamOptions configures streaming behavior in the request.
type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// unmarshalStreamChunk parses a raw SSE data payload into a chatCompletionStreamChunk.
func unmarshalStreamChunk(data string) (*chatCompletionStreamChunk, error) {
	var chunk chatCompletionStreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, err
	}
	return &chunk, nil
}
