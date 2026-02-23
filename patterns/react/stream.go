package react

import (
	"iter"
	"strings"

	"github.com/leofalp/aigo/core/overview"
	"github.com/leofalp/aigo/providers/ai"
)

// ReactEventType identifies the phase of the ReAct loop that produced an event.
type ReactEventType string

const (
	// ReactEventReasoning indicates the LLM is producing reasoning/thinking tokens.
	ReactEventReasoning ReactEventType = "reasoning"

	// ReactEventContent indicates a content delta from the LLM response.
	ReactEventContent ReactEventType = "content"

	// ReactEventToolCall indicates the LLM has decided to call a tool.
	// Emitted once per tool call with the complete call information after the
	// entire LLM response for that iteration has been consumed.
	ReactEventToolCall ReactEventType = "tool_call"

	// ReactEventToolResult indicates a tool has finished executing.
	// Contains the tool name and its output.
	ReactEventToolResult ReactEventType = "tool_result"

	// ReactEventIterationStart signals the beginning of a new reasoning iteration.
	ReactEventIterationStart ReactEventType = "iteration_start"

	// ReactEventFinalAnswer indicates the agent has produced a parseable final answer.
	// The Content field contains the raw response content, and Result contains the parsed T.
	ReactEventFinalAnswer ReactEventType = "final_answer"

	// ReactEventError signals an error during execution.
	// When this event is emitted, the stream is terminated immediately after.
	ReactEventError ReactEventType = "error"
)

// ReactEvent represents a single event from the ReAct agent loop.
// Each event carries exactly one type of payload, identified by the Type field.
type ReactEvent[T any] struct {
	// Type identifies what kind of event this is.
	Type ReactEventType `json:"type"`

	// Iteration is the current 1-based iteration number within the ReAct loop.
	Iteration int `json:"iteration"`

	// Content carries a text delta (ReactEventContent) or the full accumulated
	// content of the final response (ReactEventFinalAnswer).
	Content string `json:"content,omitempty"`

	// Reasoning carries a reasoning/thinking delta (ReactEventReasoning only).
	Reasoning string `json:"reasoning,omitempty"`

	// ToolName is the name of the tool being called or returning a result.
	// Populated for ReactEventToolCall and ReactEventToolResult.
	ToolName string `json:"tool_name,omitempty"`

	// ToolInput is the JSON-encoded arguments passed to the tool.
	// Populated for ReactEventToolCall only.
	ToolInput string `json:"tool_input,omitempty"`

	// ToolOutput is the string result returned by the tool.
	// Populated for ReactEventToolResult only.
	ToolOutput string `json:"tool_output,omitempty"`

	// Result is the strongly-typed parsed final answer.
	// Populated only for ReactEventFinalAnswer events.
	Result *T `json:"result,omitempty"`

	// Err holds the error for ReactEventError events.
	// It is not marshaled to JSON; callers should use the error channel of the iterator.
	Err error `json:"-"`
}

// ReactStream wraps the streaming ReAct agent execution loop.
// It yields ReactEvent values that describe each phase of the agent's work.
//
// The stream must be consumed either via Iter() or Collect() to avoid resource leaks.
// Breaking out of an Iter() range loop early is safe — the underlying iterator
// will be abandoned correctly by Go's range-over-func mechanism.
type ReactStream[T any] struct {
	iterator iter.Seq2[ReactEvent[T], error]
	// ctx is the enriched context that carries the overview; it is updated
	// after the iterator completes so Collect() can read the final overview.
	ctxPtr *contextCarrier
}

// contextCarrier holds a pointer-to-context so Collect() can read the final
// overview after the iterator has run (the context is mutated in-place by the
// ReAct loop to carry the overview).
type contextCarrier struct {
	// overview is captured at the end of ExecuteStream's iterator so Collect()
	// can build a StructuredOverview without needing to re-run the loop.
	overview *overview.Overview
}

// Iter returns the underlying iterator for range-over-func consumption.
//
// Example:
//
//	stream, _ := agent.ExecuteStream(ctx, "Research quantum computing")
//	for event, err := range stream.Iter() {
//	    if err != nil { log.Fatal(err) }
//	    switch event.Type {
//	    case react.ReactEventContent:
//	        fmt.Print(event.Content) // typewriter effect
//	    case react.ReactEventToolCall:
//	        fmt.Printf("\n[Calling %s]\n", event.ToolName)
//	    case react.ReactEventFinalAnswer:
//	        fmt.Printf("\nResult: %+v\n", event.Result)
//	    }
//	}
func (stream *ReactStream[T]) Iter() iter.Seq2[ReactEvent[T], error] {
	return stream.iterator
}

// Collect consumes the entire stream and returns the structured overview,
// equivalent to what Execute() returns but after streaming all events.
// Any mid-stream error terminates collection and returns that error.
//
// Use this when you want streaming transport (lower time-to-first-byte) but
// do not need to process intermediate events.
func (stream *ReactStream[T]) Collect() (*overview.StructuredOverview[T], error) {
	var finalResult *T

	for event, err := range stream.iterator {
		if err != nil {
			return nil, err
		}
		if event.Type == ReactEventFinalAnswer {
			finalResult = event.Result
		}
	}

	if finalResult == nil {
		return nil, nil
	}

	finalOverview := stream.ctxPtr.overview
	if finalOverview == nil {
		finalOverview = &overview.Overview{}
	}

	return &overview.StructuredOverview[T]{
		Overview: *finalOverview,
		Data:     finalResult,
	}, nil
}

// reactToolCallBuilder accumulates incremental tool call deltas from a stream
// into a complete ToolCall.
//
// NOTE: This intentionally duplicates the unexported toolCallBuilder from
// providers/ai/stream.go to keep the react package decoupled from provider
// internals (layer independence). If the streaming wire format changes, update
// both implementations. The logic is simple enough that divergence is unlikely,
// but the shared invariant is: ID and Name arrive on the first chunk for a
// given Index; subsequent chunks carry only Arguments fragments.
type reactToolCallBuilder struct {
	id        string
	name      string
	arguments strings.Builder
}

// accumulateReactToolCallDelta merges a ToolCallDelta into the running list of
// reactToolCallBuilders, growing the slice as new indices are encountered.
func accumulateReactToolCallDelta(builders []reactToolCallBuilder, delta *ai.ToolCallDelta) []reactToolCallBuilder {
	// Expand the builders slice if this is a new index
	for len(builders) <= delta.Index {
		builders = append(builders, reactToolCallBuilder{})
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

// buildToolCallsFromBuilders finalizes accumulated tool call deltas into a
// slice of complete ai.ToolCall values ready for processing.
func buildToolCallsFromBuilders(builders []reactToolCallBuilder) []ai.ToolCall {
	toolCalls := make([]ai.ToolCall, 0, len(builders))
	for _, builder := range builders {
		toolCalls = append(toolCalls, ai.ToolCall{
			ID:   builder.id,
			Type: "function",
			Function: ai.ToolCallFunction{
				Name:      builder.name,
				Arguments: builder.arguments.String(),
			},
		})
	}
	return toolCalls
}
