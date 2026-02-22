package ai

import (
	"errors"
	"iter"
	"testing"
)

// makeStream is a test helper that builds a ChatStream from a hand-crafted event
// slice. If midErr is non-nil and errAtIndex is a valid index, the error is
// injected after that event instead of a normal yield.
func makeStream(events []StreamEvent, midErr error, errAtIndex int) *ChatStream {
	iteratorFunc := func(yield func(StreamEvent, error) bool) {
		for i, event := range events {
			if midErr != nil && i == errAtIndex {
				yield(event, midErr)
				return
			}
			if !yield(event, nil) {
				return
			}
		}
	}
	return NewChatStream(iter.Seq2[StreamEvent, error](iteratorFunc))
}

// ========== NewSingleEventStream ==========

// TestNewSingleEventStream_ContentOnly verifies that a response with only Content
// produces a content event followed by a done event.
func TestNewSingleEventStream_ContentOnly(t *testing.T) {
	response := &ChatResponse{Content: "hello world", FinishReason: "stop"}
	stream := NewSingleEventStream(response)

	var collected []StreamEvent
	for event, err := range stream.Iter() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		collected = append(collected, event)
	}

	if len(collected) != 2 {
		t.Fatalf("expected 2 events (content + done), got %d", len(collected))
	}
	if collected[0].Type != StreamEventContent {
		t.Errorf("expected first event type %q, got %q", StreamEventContent, collected[0].Type)
	}
	if collected[0].Content != "hello world" {
		t.Errorf("expected content %q, got %q", "hello world", collected[0].Content)
	}
	if collected[1].Type != StreamEventDone {
		t.Errorf("expected last event type %q, got %q", StreamEventDone, collected[1].Type)
	}
	if collected[1].FinishReason != "stop" {
		t.Errorf("expected FinishReason %q, got %q", "stop", collected[1].FinishReason)
	}
}

// TestNewSingleEventStream_WithReasoning verifies that a response with Reasoning
// emits a reasoning event before the done event.
func TestNewSingleEventStream_WithReasoning(t *testing.T) {
	response := &ChatResponse{Reasoning: "let me think"}
	stream := NewSingleEventStream(response)

	var collected []StreamEvent
	for event, err := range stream.Iter() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		collected = append(collected, event)
	}

	if len(collected) != 2 {
		t.Fatalf("expected 2 events (reasoning + done), got %d", len(collected))
	}
	if collected[0].Type != StreamEventReasoning {
		t.Errorf("expected reasoning event, got %q", collected[0].Type)
	}
	if collected[0].Reasoning != "let me think" {
		t.Errorf("expected reasoning %q, got %q", "let me think", collected[0].Reasoning)
	}
}

// TestNewSingleEventStream_WithToolCalls verifies that tool calls in the response
// are emitted as individual StreamEventToolCall events with the correct index.
func TestNewSingleEventStream_WithToolCalls(t *testing.T) {
	response := &ChatResponse{
		ToolCalls: []ToolCall{
			{ID: "call_1", Function: ToolCallFunction{Name: "search", Arguments: `{"q":"go"}`}},
			{ID: "call_2", Function: ToolCallFunction{Name: "calc", Arguments: `{"a":1}`}},
		},
	}
	stream := NewSingleEventStream(response)

	var toolEvents []StreamEvent
	for event, err := range stream.Iter() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if event.Type == StreamEventToolCall {
			toolEvents = append(toolEvents, event)
		}
	}

	if len(toolEvents) != 2 {
		t.Fatalf("expected 2 tool call events, got %d", len(toolEvents))
	}
	if toolEvents[0].ToolCall.Index != 0 {
		t.Errorf("expected index 0 for first tool call, got %d", toolEvents[0].ToolCall.Index)
	}
	if toolEvents[0].ToolCall.ID != "call_1" {
		t.Errorf("expected ID %q, got %q", "call_1", toolEvents[0].ToolCall.ID)
	}
	if toolEvents[1].ToolCall.Index != 1 {
		t.Errorf("expected index 1 for second tool call, got %d", toolEvents[1].ToolCall.Index)
	}
	if toolEvents[1].ToolCall.Name != "calc" {
		t.Errorf("expected name %q, got %q", "calc", toolEvents[1].ToolCall.Name)
	}
}

// TestNewSingleEventStream_WithUsage verifies that a non-nil Usage in the response
// is emitted as a StreamEventUsage before the done event.
func TestNewSingleEventStream_WithUsage(t *testing.T) {
	usage := &Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	response := &ChatResponse{Usage: usage}
	stream := NewSingleEventStream(response)

	var usageEvents []StreamEvent
	for event, err := range stream.Iter() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if event.Type == StreamEventUsage {
			usageEvents = append(usageEvents, event)
		}
	}

	if len(usageEvents) != 1 {
		t.Fatalf("expected 1 usage event, got %d", len(usageEvents))
	}
	if usageEvents[0].Usage.TotalTokens != 30 {
		t.Errorf("expected TotalTokens 30, got %d", usageEvents[0].Usage.TotalTokens)
	}
}

// TestNewSingleEventStream_EmptyResponse verifies that an empty ChatResponse
// produces only a single done event.
func TestNewSingleEventStream_EmptyResponse(t *testing.T) {
	stream := NewSingleEventStream(&ChatResponse{})

	var collected []StreamEvent
	for event, err := range stream.Iter() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		collected = append(collected, event)
	}

	if len(collected) != 1 {
		t.Fatalf("expected 1 event (done only), got %d", len(collected))
	}
	if collected[0].Type != StreamEventDone {
		t.Errorf("expected done event, got %q", collected[0].Type)
	}
}

// ========== ChatStream.Collect ==========

// TestCollect_Content verifies that multiple content events are concatenated into
// the final ChatResponse.Content field.
func TestCollect_Content(t *testing.T) {
	stream := makeStream([]StreamEvent{
		{Type: StreamEventContent, Content: "Hello, "},
		{Type: StreamEventContent, Content: "world!"},
		{Type: StreamEventDone, FinishReason: "stop"},
	}, nil, -1)

	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.Content != "Hello, world!" {
		t.Errorf("expected %q, got %q", "Hello, world!", response.Content)
	}
	if response.FinishReason != "stop" {
		t.Errorf("expected FinishReason %q, got %q", "stop", response.FinishReason)
	}
}

// TestCollect_ReasoningEvents verifies that reasoning deltas are concatenated
// into ChatResponse.Reasoning.
func TestCollect_ReasoningEvents(t *testing.T) {
	stream := makeStream([]StreamEvent{
		{Type: StreamEventReasoning, Reasoning: "step 1 "},
		{Type: StreamEventReasoning, Reasoning: "step 2"},
		{Type: StreamEventDone},
	}, nil, -1)

	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.Reasoning != "step 1 step 2" {
		t.Errorf("expected reasoning %q, got %q", "step 1 step 2", response.Reasoning)
	}
}

// TestCollect_ToolCalls verifies that incremental tool call deltas are assembled
// into complete ToolCall entries on the final ChatResponse.
func TestCollect_ToolCalls(t *testing.T) {
	stream := makeStream([]StreamEvent{
		{Type: StreamEventToolCall, ToolCall: &ToolCallDelta{Index: 0, ID: "call_1", Name: "search"}},
		{Type: StreamEventToolCall, ToolCall: &ToolCallDelta{Index: 0, Arguments: `{"q":`}},
		{Type: StreamEventToolCall, ToolCall: &ToolCallDelta{Index: 0, Arguments: `"go"}`}},
		{Type: StreamEventDone},
	}, nil, -1)

	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(response.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(response.ToolCalls))
	}
	toolCall := response.ToolCalls[0]
	if toolCall.ID != "call_1" {
		t.Errorf("expected ID %q, got %q", "call_1", toolCall.ID)
	}
	if toolCall.Function.Name != "search" {
		t.Errorf("expected name %q, got %q", "search", toolCall.Function.Name)
	}
	if toolCall.Function.Arguments != `{"q":"go"}` {
		t.Errorf("expected arguments %q, got %q", `{"q":"go"}`, toolCall.Function.Arguments)
	}
}

// TestCollect_MultipleToolCalls verifies that concurrent tool call streams are
// assembled into separate ToolCall entries correctly.
func TestCollect_MultipleToolCalls(t *testing.T) {
	stream := makeStream([]StreamEvent{
		{Type: StreamEventToolCall, ToolCall: &ToolCallDelta{Index: 0, ID: "a", Name: "toolA", Arguments: `{}`}},
		{Type: StreamEventToolCall, ToolCall: &ToolCallDelta{Index: 1, ID: "b", Name: "toolB", Arguments: `{}`}},
		{Type: StreamEventDone},
	}, nil, -1)

	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(response.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(response.ToolCalls))
	}
	if response.ToolCalls[0].Function.Name != "toolA" {
		t.Errorf("expected first tool name %q, got %q", "toolA", response.ToolCalls[0].Function.Name)
	}
	if response.ToolCalls[1].Function.Name != "toolB" {
		t.Errorf("expected second tool name %q, got %q", "toolB", response.ToolCalls[1].Function.Name)
	}
}

// TestCollect_UsageEvent verifies that usage metadata from the stream is captured
// on the final ChatResponse.
func TestCollect_UsageEvent(t *testing.T) {
	usage := &Usage{PromptTokens: 5, CompletionTokens: 10, TotalTokens: 15}
	stream := makeStream([]StreamEvent{
		{Type: StreamEventContent, Content: "hi"},
		{Type: StreamEventUsage, Usage: usage},
		{Type: StreamEventDone},
	}, nil, -1)

	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.Usage == nil {
		t.Fatal("expected Usage to be set, got nil")
	}
	if response.Usage.TotalTokens != 15 {
		t.Errorf("expected TotalTokens 15, got %d", response.Usage.TotalTokens)
	}
}

// TestCollect_MidStreamError verifies that a mid-stream error causes Collect to
// return the partial response accumulated so far, alongside the error.
func TestCollect_MidStreamError(t *testing.T) {
	sentinelErr := errors.New("network interrupted")
	// Event 0 succeeds, event 1 triggers the error.
	stream := makeStream([]StreamEvent{
		{Type: StreamEventContent, Content: "partial "},
		{Type: StreamEventContent, Content: "content"},
	}, sentinelErr, 1)

	response, err := stream.Collect()
	if !errors.Is(err, sentinelErr) {
		t.Errorf("expected sentinel error, got %v", err)
	}
	// The partial content from event 0 should be preserved.
	if response.Content != "partial " {
		t.Errorf("expected partial content %q, got %q", "partial ", response.Content)
	}
}

// TestCollect_EmptyStream verifies that an empty stream returns a zero-value
// ChatResponse with no error.
func TestCollect_EmptyStream(t *testing.T) {
	stream := makeStream(nil, nil, -1)

	response, err := stream.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.Content != "" {
		t.Errorf("expected empty content, got %q", response.Content)
	}
}

// ========== accumulateToolCallDelta ==========

// TestAccumulateToolCallDelta_NewIndex verifies that the builders slice grows when
// a delta with a previously unseen index arrives.
func TestAccumulateToolCallDelta_NewIndex(t *testing.T) {
	var builders []toolCallBuilder

	builders = accumulateToolCallDelta(builders, &ToolCallDelta{Index: 0, ID: "id1", Name: "fn1"})
	if len(builders) != 1 {
		t.Fatalf("expected 1 builder after index 0, got %d", len(builders))
	}

	builders = accumulateToolCallDelta(builders, &ToolCallDelta{Index: 2, ID: "id3", Name: "fn3"})
	if len(builders) != 3 {
		t.Fatalf("expected 3 builders after index 2, got %d", len(builders))
	}
}

// TestAccumulateToolCallDelta_IncrementalArgs verifies that argument fragments
// are concatenated in order on the correct builder.
func TestAccumulateToolCallDelta_IncrementalArgs(t *testing.T) {
	var builders []toolCallBuilder

	builders = accumulateToolCallDelta(builders, &ToolCallDelta{Index: 0, ID: "id1", Name: "fn1", Arguments: `{"x`})
	builders = accumulateToolCallDelta(builders, &ToolCallDelta{Index: 0, Arguments: `":1}`})

	if builders[0].arguments.String() != `{"x":1}` {
		t.Errorf("expected arguments %q, got %q", `{"x":1}`, builders[0].arguments.String())
	}
}

// TestAccumulateToolCallDelta_IDAndNameNotOverwritten verifies that once an ID and
// name have been set by the first delta for an index, subsequent deltas that omit
// those fields do not clear them.
func TestAccumulateToolCallDelta_IDAndNameNotOverwritten(t *testing.T) {
	var builders []toolCallBuilder

	// First chunk carries ID and Name.
	builders = accumulateToolCallDelta(builders, &ToolCallDelta{Index: 0, ID: "id1", Name: "fn1"})
	// Second chunk carries only Arguments (empty ID and Name).
	builders = accumulateToolCallDelta(builders, &ToolCallDelta{Index: 0, Arguments: `{}`})

	if builders[0].id != "id1" {
		t.Errorf("expected id %q, got %q", "id1", builders[0].id)
	}
	if builders[0].name != "fn1" {
		t.Errorf("expected name %q, got %q", "fn1", builders[0].name)
	}
}
