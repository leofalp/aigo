package anthropic

import (
	"encoding/json"
	"fmt"
)

/*
	ANTHROPIC SSE STREAMING - WIRE TYPES

	Anthropic streaming uses SSE with "event:" lines to identify event types,
	followed by "data:" lines containing JSON payloads. The SSEScanner only
	processes "data:" lines, so we use the "type" field inside the JSON payload
	to discriminate events.

	Event lifecycle:
	  message_start → content_block_start → content_block_delta → content_block_stop →
	  message_delta → message_stop
*/

// anthropicStreamEvent is the top-level envelope for all Anthropic SSE events.
// The Type field discriminates which optional fields are populated.
type anthropicStreamEvent struct {
	Type         string                `json:"type"`                    // Event discriminator
	Message      *anthropicResponse    `json:"message,omitempty"`       // For "message_start"
	Index        int                   `json:"index,omitempty"`         // For content_block_start/delta/stop
	ContentBlock *responseContentBlock `json:"content_block,omitempty"` // For "content_block_start"
	Delta        *streamDelta          `json:"delta,omitempty"`         // For "content_block_delta" and "message_delta"
	Usage        *anthropicUsage       `json:"usage,omitempty"`         // For "message_delta"
	Error        *anthropicError       `json:"error,omitempty"`         // For "error" events
}

// streamDelta carries incremental content within a content_block_delta or message_delta event.
// The Type field discriminates the kind of delta:
//   - "text_delta": Text field is populated
//   - "thinking_delta": Thinking field is populated
//   - "input_json_delta": PartialJSON field is populated (tool call arguments)
//   - (no type on message_delta): StopReason and StopSequence are populated
type streamDelta struct {
	Type         string `json:"type,omitempty"`          // "text_delta", "thinking_delta", "input_json_delta"
	Text         string `json:"text,omitempty"`          // For text_delta
	Thinking     string `json:"thinking,omitempty"`      // For thinking_delta
	PartialJSON  string `json:"partial_json,omitempty"`  // For input_json_delta (tool call arguments)
	StopReason   string `json:"stop_reason,omitempty"`   // For message_delta
	StopSequence string `json:"stop_sequence,omitempty"` // For message_delta
}

// anthropicError represents an error event in the Anthropic SSE stream.
type anthropicError struct {
	Type    string `json:"type"`    // Error type (e.g., "overloaded_error", "api_error")
	Message string `json:"message"` // Human-readable error description
}

// unmarshalStreamEvent parses a JSON payload string into an anthropicStreamEvent.
// Returns an error if the JSON is invalid or the type field is missing.
func unmarshalStreamEvent(payload string) (*anthropicStreamEvent, error) {
	var event anthropicStreamEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return nil, err
	}
	if event.Type == "" {
		return nil, fmt.Errorf("missing type field in stream event")
	}
	return &event, nil
}
