package ai

import (
	"iter"
	"strings"
)

// StreamEventType identifies the kind of delta carried by a StreamEvent.
type StreamEventType string

const (
	// StreamEventContent indicates a text content delta.
	StreamEventContent StreamEventType = "content"
	// StreamEventToolCall indicates an incremental tool call delta (name or arguments chunk).
	StreamEventToolCall StreamEventType = "tool_call"
	// StreamEventReasoning indicates a reasoning/thinking content delta.
	StreamEventReasoning StreamEventType = "reasoning"
	// StreamEventUsage carries token usage metadata (typically the final event).
	StreamEventUsage StreamEventType = "usage"
	// StreamEventDone signals that the stream has finished normally.
	StreamEventDone StreamEventType = "done"
	// StreamEventError signals an error that terminated the stream.
	StreamEventError StreamEventType = "error"
)

// ToolCallDelta represents an incremental update to a tool call being streamed.
// The Index field identifies which tool call is being updated (there may be
// multiple concurrent tool calls). ID and Name are only present on the first
// chunk for a given index; subsequent chunks carry only Arguments fragments.
type ToolCallDelta struct {
	Index     int    `json:"index"`               // Position in the tool calls list
	ID        string `json:"id,omitempty"`        // Tool call ID (first chunk only)
	Name      string `json:"name,omitempty"`      // Function name (first chunk only)
	Arguments string `json:"arguments,omitempty"` // Incremental JSON argument fragment
}

// StreamEvent represents a single delta yielded during LLM response streaming.
// Each event carries exactly one type of payload, identified by the Type field.
type StreamEvent struct {
	Type         StreamEventType `json:"type"`
	Content      string          `json:"content,omitempty"`       // Text delta (Type == StreamEventContent)
	Reasoning    string          `json:"reasoning,omitempty"`     // Reasoning delta (Type == StreamEventReasoning)
	ToolCall     *ToolCallDelta  `json:"tool_call,omitempty"`     // Tool call delta (Type == StreamEventToolCall)
	Usage        *Usage          `json:"usage,omitempty"`         // Token usage (Type == StreamEventUsage)
	FinishReason string          `json:"finish_reason,omitempty"` // Present on StreamEventDone
	Error        string          `json:"error,omitempty"`         // Error message (Type == StreamEventError)
}

// ChatStream wraps a streaming iterator and provides automatic accumulation
// of deltas into a final ChatResponse. It supports both range-based iteration
// for real-time token processing and a convenience Collect() method for callers
// who want the complete response.
//
// Important: callers must consume the stream, either by iterating with Iter()
// (including breaking out of the loop early) or by calling Collect(). The
// underlying provider may hold open resources (such as an HTTP response body)
// that are only released when the iterator completes or is abandoned via a
// loop break. Constructing a ChatStream and never iterating it will leak those
// resources.
type ChatStream struct {
	iterator iter.Seq2[StreamEvent, error]
}

// NewChatStream creates a ChatStream from a raw streaming iterator.
// The iterator is expected to yield StreamEvent values (with nil error) for
// normal deltas, and may yield a non-nil error to signal a mid-stream failure.
// The caller is responsible for consuming the returned ChatStream (see ChatStream
// documentation for resource management details).
func NewChatStream(iterator iter.Seq2[StreamEvent, error]) *ChatStream {
	return &ChatStream{iterator: iterator}
}

// NewSingleEventStream wraps a synchronous ChatResponse as a single-event stream.
// This is used as a fallback when the provider does not support streaming: the
// entire response is delivered as one content event followed by a done event.
func NewSingleEventStream(response *ChatResponse) *ChatStream {
	iteratorFunc := func(yield func(StreamEvent, error) bool) {
		// Yield content if present
		if response.Content != "" {
			if !yield(StreamEvent{Type: StreamEventContent, Content: response.Content}, nil) {
				return
			}
		}

		// Yield reasoning if present
		if response.Reasoning != "" {
			if !yield(StreamEvent{Type: StreamEventReasoning, Reasoning: response.Reasoning}, nil) {
				return
			}
		}

		// Yield tool calls if present
		for toolIndex, toolCall := range response.ToolCalls {
			if !yield(StreamEvent{
				Type: StreamEventToolCall,
				ToolCall: &ToolCallDelta{
					Index:     toolIndex,
					ID:        toolCall.ID,
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				},
			}, nil) {
				return
			}
		}

		// Yield usage if present
		if response.Usage != nil {
			if !yield(StreamEvent{Type: StreamEventUsage, Usage: response.Usage}, nil) {
				return
			}
		}

		// Yield done event
		yield(StreamEvent{Type: StreamEventDone, FinishReason: response.FinishReason}, nil)
	}

	return NewChatStream(iteratorFunc)
}

// Iter returns the underlying iterator for use with range-over-func loops.
//
// Example:
//
//	for event, err := range stream.Iter() {
//	    if err != nil { handle error }
//	    fmt.Print(event.Content)
//	}
func (stream *ChatStream) Iter() iter.Seq2[StreamEvent, error] {
	return stream.iterator
}

// Collect consumes the entire stream and returns the accumulated ChatResponse.
// This is a convenience method for callers who want the complete response but
// still benefit from streaming transport (lower time-to-first-byte).
// Any mid-stream error terminates collection and returns a partial response with the error.
func (stream *ChatStream) Collect() (*ChatResponse, error) {
	accumulated := &ChatResponse{}
	var toolCallBuilders []toolCallBuilder

	for event, err := range stream.iterator {
		if err != nil {
			return accumulated, err
		}

		switch event.Type {
		case StreamEventContent:
			accumulated.Content += event.Content

		case StreamEventReasoning:
			accumulated.Reasoning += event.Reasoning

		case StreamEventToolCall:
			if event.ToolCall != nil {
				toolCallBuilders = accumulateToolCallDelta(toolCallBuilders, event.ToolCall)
			}

		case StreamEventUsage:
			if event.Usage != nil {
				accumulated.Usage = event.Usage
			}

		case StreamEventDone:
			accumulated.FinishReason = event.FinishReason

		case StreamEventError:
			// Error events are informational; the actual error comes through the iterator's error channel
		}
	}

	// Finalize accumulated tool calls
	for _, builder := range toolCallBuilders {
		accumulated.ToolCalls = append(accumulated.ToolCalls, ToolCall{
			ID:   builder.id,
			Type: "function",
			Function: ToolCallFunction{
				Name:      builder.name,
				Arguments: builder.arguments.String(),
			},
		})
	}

	return accumulated, nil
}

// toolCallBuilder accumulates incremental tool call deltas into a complete ToolCall.
type toolCallBuilder struct {
	id        string
	name      string
	arguments strings.Builder
}

// accumulateToolCallDelta merges a ToolCallDelta into the running list of tool call builders.
// It grows the slice as needed when new tool call indices appear.
func accumulateToolCallDelta(builders []toolCallBuilder, delta *ToolCallDelta) []toolCallBuilder {
	// Expand the builders slice if this is a new index
	for len(builders) <= delta.Index {
		builders = append(builders, toolCallBuilder{})
	}

	builder := &builders[delta.Index]

	if delta.ID != "" {
		builder.id = delta.ID
	}
	if delta.Name != "" {
		builder.name = delta.Name
	}
	if delta.Arguments != "" {
		builder.arguments.WriteString(delta.Arguments)
	}

	return builders
}
