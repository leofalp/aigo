package react

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
)

// mockStreamProvider embeds mockProvider and additionally implements
// ai.StreamProvider by returning pre-built ChatStream values.
type mockStreamProvider struct {
	mockProvider                     // embedded sync provider (for fallback tests)
	streamResponses []*ai.ChatStream // pre-built streams to return in order
	streamIndex     int
	streamErr       error // optional pre-stream error
}

func (m *mockStreamProvider) StreamMessage(_ context.Context, _ ai.ChatRequest) (*ai.ChatStream, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	if m.streamIndex >= len(m.streamResponses) {
		return nil, errors.New("mockStreamProvider: no more stream responses")
	}
	stream := m.streamResponses[m.streamIndex]
	m.streamIndex++
	return stream, nil
}

// singleContentStream returns a ChatStream that yields a single content chunk
// followed by a done event with the given text.
func singleContentStream(content string) *ai.ChatStream {
	return ai.NewChatStream(func(yield func(ai.StreamEvent, error) bool) {
		if !yield(ai.StreamEvent{Type: ai.StreamEventContent, Content: content}, nil) {
			return
		}
		yield(ai.StreamEvent{Type: ai.StreamEventDone, FinishReason: "stop"}, nil)
	})
}

// multiChunkContentStream returns a ChatStream that yields content in multiple
// chunks — useful for verifying that deltas are forwarded correctly.
func multiChunkContentStream(chunks ...string) *ai.ChatStream {
	return ai.NewChatStream(func(yield func(ai.StreamEvent, error) bool) {
		for _, chunk := range chunks {
			if !yield(ai.StreamEvent{Type: ai.StreamEventContent, Content: chunk}, nil) {
				return
			}
		}
		yield(ai.StreamEvent{Type: ai.StreamEventDone, FinishReason: "stop"}, nil)
	})
}

// toolCallStream returns a ChatStream that yields a complete tool call event
// plus optional content, then a done event.
func toolCallStream(toolName, toolArgs string) *ai.ChatStream {
	return ai.NewChatStream(func(yield func(ai.StreamEvent, error) bool) {
		if !yield(ai.StreamEvent{
			Type: ai.StreamEventToolCall,
			ToolCall: &ai.ToolCallDelta{
				Index:     0,
				ID:        "call_test_1",
				Name:      toolName,
				Arguments: toolArgs,
			},
		}, nil) {
			return
		}
		yield(ai.StreamEvent{Type: ai.StreamEventDone, FinishReason: "tool_calls"}, nil)
	})
}

// midStreamErrorStream returns a ChatStream that yields one content chunk, then
// an error — simulating a mid-stream provider failure.
func midStreamErrorStream(content string, streamError error) *ai.ChatStream {
	return ai.NewChatStream(func(yield func(ai.StreamEvent, error) bool) {
		if content != "" {
			if !yield(ai.StreamEvent{Type: ai.StreamEventContent, Content: content}, nil) {
				return
			}
		}
		yield(ai.StreamEvent{}, streamError)
	})
}

// collectEvents is a test helper that drains a ReactStream and returns all
// events along with the first error encountered. The error event itself is
// included in the returned slice (it is appended before returning the error).
func collectEvents[T any](stream *ReactStream[T]) ([]ReactEvent[T], error) {
	var events []ReactEvent[T]
	for event, err := range stream.Iter() {
		events = append(events, event)
		if err != nil {
			return events, err
		}
	}
	return events, nil
}

// eventTypes extracts the list of event types from a slice of events.
func eventTypes[T any](events []ReactEvent[T]) []ReactEventType {
	types := make([]ReactEventType, len(events))
	for i, event := range events {
		types[i] = event.Type
	}
	return types
}

// TestExecuteStream_EmptyPrompt verifies that ExecuteStream returns an error
// immediately when given an empty prompt, without creating a stream.
func TestExecuteStream_EmptyPrompt(t *testing.T) {
	memProvider := inmemory.New()
	mockLLM := &mockStreamProvider{}

	baseClient, err := client.New(mockLLM, client.WithMemory(memProvider))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[string](baseClient)
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty prompt, got nil")
	}
	if stream != nil {
		t.Fatal("expected nil stream for empty prompt")
	}
}

// TestExecuteStream_Success verifies the happy path: one tool call, then a
// final answer. The event sequence must be:
// IterationStart → content/tool_call (iteration 1) → ToolCall → ToolResult →
// IterationStart → content → FinalAnswer.
func TestExecuteStream_Success(t *testing.T) {
	type Result struct {
		Value string `json:"value"`
	}

	memProvider := inmemory.New()
	testTool := &mockTool{name: "calculator", result: `42`}

	mockLLM := &mockStreamProvider{
		mockProvider: mockProvider{}, // not used (StreamProvider takes precedence)
		streamResponses: []*ai.ChatStream{
			toolCallStream("calculator", `{"expr":"6*7"}`),
			singleContentStream(`{"value":"42"}`),
		},
	}

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memProvider),
		client.WithTools(testTool),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[Result](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "What is 6*7?")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	events, iterErr := collectEvents(stream)
	if iterErr != nil {
		t.Fatalf("stream iteration error: %v", iterErr)
	}

	types := eventTypes(events)

	// Verify core event sequence (iteration start events, tool call/result, final answer)
	assertContainsType(t, types, ReactEventIterationStart)
	assertContainsType(t, types, ReactEventToolCall)
	assertContainsType(t, types, ReactEventToolResult)
	assertContainsType(t, types, ReactEventFinalAnswer)

	// Verify the final answer event carries the parsed result
	var finalEvent *ReactEvent[Result]
	for i := range events {
		if events[i].Type == ReactEventFinalAnswer {
			finalEvent = &events[i]
			break
		}
	}
	if finalEvent == nil {
		t.Fatal("no FinalAnswer event found")
	}
	if finalEvent.Result == nil {
		t.Fatal("FinalAnswer event has nil Result")
	}
	if finalEvent.Result.Value != "42" {
		t.Errorf("expected Result.Value=42, got %q", finalEvent.Result.Value)
	}

	// Verify tool call event has correct fields
	var toolCallEvent *ReactEvent[Result]
	for i := range events {
		if events[i].Type == ReactEventToolCall {
			toolCallEvent = &events[i]
			break
		}
	}
	if toolCallEvent == nil {
		t.Fatal("no ToolCall event found")
	}
	if toolCallEvent.ToolName != "calculator" {
		t.Errorf("expected ToolName=calculator, got %q", toolCallEvent.ToolName)
	}
}

// TestExecuteStream_Collect verifies that Collect() returns a StructuredOverview
// equivalent to what Execute() would return.
func TestExecuteStream_Collect(t *testing.T) {
	type Result struct {
		Answer int `json:"answer"`
	}

	memProvider := inmemory.New()
	testTool := &mockTool{name: "add", result: `{"sum":10}`}

	mockLLM := &mockStreamProvider{
		streamResponses: []*ai.ChatStream{
			toolCallStream("add", `{"a":4,"b":6}`),
			singleContentStream(`{"answer":10}`),
		},
	}

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memProvider),
		client.WithTools(testTool),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[Result](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "Add 4 and 6")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	result, collectErr := stream.Collect()
	if collectErr != nil {
		t.Fatalf("Collect returned unexpected error: %v", collectErr)
	}
	if result == nil {
		t.Fatal("Collect returned nil result")
	}
	if result.Data == nil {
		t.Fatal("Collect result has nil Data")
	}
	if result.Data.Answer != 10 {
		t.Errorf("expected Answer=10, got %d", result.Data.Answer)
	}
}

// TestExecuteStream_MaxIterations verifies that reaching the maximum iteration
// count terminates the stream with an error event.
func TestExecuteStream_MaxIterations(t *testing.T) {
	memProvider := inmemory.New()
	testTool := &mockTool{name: "noop", result: `{}`}

	// All LLM responses request the same tool, so we never produce a final answer
	const maxIter = 3
	streamResponses := make([]*ai.ChatStream, maxIter)
	for i := 0; i < maxIter; i++ {
		streamResponses[i] = toolCallStream("noop", `{}`)
	}

	mockLLM := &mockStreamProvider{streamResponses: streamResponses}

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memProvider),
		client.WithTools(testTool),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[string](baseClient, WithMaxIterations(maxIter))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "Loop forever")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	events, iterErr := collectEvents(stream)
	if iterErr == nil {
		t.Fatal("expected error from stream at max iterations, got nil")
	}

	// The error message should mention max iterations
	if !containsString(iterErr.Error(), "maximum iterations") {
		t.Errorf("unexpected error message: %v", iterErr)
	}

	// Last event should be an error event
	if len(events) == 0 {
		t.Fatal("expected at least one event before error")
	}
	last := events[len(events)-1]
	if last.Type != ReactEventError {
		t.Errorf("expected last event to be ReactEventError, got %v", last.Type)
	}
}

// TestExecuteStream_ToolError_StopOnError verifies that a tool execution error
// terminates the stream when stopOnError is true.
func TestExecuteStream_ToolError_StopOnError(t *testing.T) {
	memProvider := inmemory.New()
	failingTool := &mockTool{name: "explode", err: errors.New("kaboom")}

	mockLLM := &mockStreamProvider{
		streamResponses: []*ai.ChatStream{
			toolCallStream("explode", `{}`),
		},
	}

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memProvider),
		client.WithTools(failingTool),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[string](baseClient, WithMaxIterations(5), WithStopOnError(true))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "trigger error")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	_, iterErr := collectEvents(stream)
	if iterErr == nil {
		t.Fatal("expected stream error when stopOnError=true and tool fails")
	}
}

// TestExecuteStream_ToolError_Continue verifies that when stopOnError is false
// the stream continues after a tool error and eventually produces a final answer.
func TestExecuteStream_ToolError_Continue(t *testing.T) {
	type Result struct {
		Status string `json:"status"`
	}

	memProvider := inmemory.New()
	// Tool always fails, but the agent should continue and reach the final answer
	failingTool := &mockTool{name: "fail_tool", err: errors.New("transient error")}

	mockLLM := &mockStreamProvider{
		streamResponses: []*ai.ChatStream{
			toolCallStream("fail_tool", `{}`),
			singleContentStream(`{"status":"recovered"}`),
		},
	}

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memProvider),
		client.WithTools(failingTool),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[Result](baseClient, WithMaxIterations(5), WithStopOnError(false))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "continue despite error")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	events, iterErr := collectEvents(stream)
	if iterErr != nil {
		t.Fatalf("unexpected stream error: %v", iterErr)
	}

	assertContainsType(t, eventTypes(events), ReactEventFinalAnswer)
}

// TestExecuteStream_ParseRetry verifies that when the first response cannot be
// parsed as T, the stream transparently retries and emits a FinalAnswer event.
func TestExecuteStream_ParseRetry(t *testing.T) {
	type Result struct {
		Value int `json:"value"`
	}

	memProvider := inmemory.New()

	mockLLM := &mockStreamProvider{
		streamResponses: []*ai.ChatStream{
			// First: non-JSON content — parse will fail
			singleContentStream("Here is your answer: 42"),
			// Retry: valid JSON
			singleContentStream(`{"value":42}`),
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(memProvider))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[Result](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "give me 42")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	events, iterErr := collectEvents(stream)
	if iterErr != nil {
		t.Fatalf("unexpected stream error: %v", iterErr)
	}

	assertContainsType(t, eventTypes(events), ReactEventFinalAnswer)

	var finalEvent *ReactEvent[Result]
	for i := range events {
		if events[i].Type == ReactEventFinalAnswer {
			finalEvent = &events[i]
			break
		}
	}
	if finalEvent == nil || finalEvent.Result == nil {
		t.Fatal("FinalAnswer event has nil Result after retry")
	}
	if finalEvent.Result.Value != 42 {
		t.Errorf("expected Value=42, got %d", finalEvent.Result.Value)
	}
}

// TestExecuteStream_ParseRetryFails verifies that when both parse attempts fail
// the stream terminates with a ReactEventError and a non-nil error.
func TestExecuteStream_ParseRetryFails(t *testing.T) {
	type Result struct {
		Required string `json:"required"`
	}

	memProvider := inmemory.New()

	mockLLM := &mockStreamProvider{
		streamResponses: []*ai.ChatStream{
			// First: non-parseable content
			singleContentStream("This is not JSON at all."),
			// Retry: also non-parseable
			singleContentStream("Still not JSON."),
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(memProvider))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[Result](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "give me json")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	events, iterErr := collectEvents(stream)
	if iterErr == nil {
		t.Fatal("expected error after both parse attempts fail, got nil")
	}

	// Should have emitted a ReactEventError as the last event
	types := eventTypes(events)
	assertContainsType(t, types, ReactEventError)
}

// TestExecuteStream_MidStreamError verifies that a mid-stream LLM error
// propagates as a ReactEventError and returns from the iterator.
func TestExecuteStream_MidStreamError(t *testing.T) {
	memProvider := inmemory.New()

	mockLLM := &mockStreamProvider{
		streamResponses: []*ai.ChatStream{
			midStreamErrorStream("partial content", errors.New("connection reset")),
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(memProvider))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[string](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "test mid-stream error")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	_, iterErr := collectEvents(stream)
	if iterErr == nil {
		t.Fatal("expected mid-stream error to propagate, got nil")
	}
}

// TestExecuteStream_FallbackToSingleEvent verifies that when the provider does
// not implement ai.StreamProvider, ExecuteStream still works via the synchronous
// fallback (single-event stream wrapping).
func TestExecuteStream_FallbackToSingleEvent(t *testing.T) {
	type Result struct {
		Answer string `json:"answer"`
	}

	memProvider := inmemory.New()

	// mockProvider only implements ai.Provider, not ai.StreamProvider
	syncLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{
				Content:      `{"answer":"fallback works"}`,
				FinishReason: "stop",
				ToolCalls:    []ai.ToolCall{},
			},
		},
	}

	baseClient, err := client.New(syncLLM, client.WithMemory(memProvider))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[Result](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "test fallback")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	events, iterErr := collectEvents(stream)
	if iterErr != nil {
		t.Fatalf("unexpected stream error: %v", iterErr)
	}

	assertContainsType(t, eventTypes(events), ReactEventFinalAnswer)

	for _, event := range events {
		if event.Type == ReactEventFinalAnswer {
			if event.Result == nil {
				t.Fatal("FinalAnswer has nil Result")
			}
			if event.Result.Answer != "fallback works" {
				t.Errorf("expected Answer=%q, got %q", "fallback works", event.Result.Answer)
			}
			return
		}
	}
}

// TestExecuteStream_MultipleToolCalls verifies that two parallel tool calls in
// a single LLM response each produce separate ToolCall and ToolResult events.
func TestExecuteStream_MultipleToolCalls(t *testing.T) {
	type Result struct {
		Total int `json:"total"`
	}

	memProvider := inmemory.New()
	toolA := &mockTool{name: "tool_a", result: `{"part":10}`}
	toolB := &mockTool{name: "tool_b", result: `{"part":20}`}

	// A stream that yields two complete tool call events
	twoToolCallStream := ai.NewChatStream(func(yield func(ai.StreamEvent, error) bool) {
		if !yield(ai.StreamEvent{
			Type: ai.StreamEventToolCall,
			ToolCall: &ai.ToolCallDelta{
				Index:     0,
				ID:        "call_a",
				Name:      "tool_a",
				Arguments: `{}`,
			},
		}, nil) {
			return
		}
		if !yield(ai.StreamEvent{
			Type: ai.StreamEventToolCall,
			ToolCall: &ai.ToolCallDelta{
				Index:     1,
				ID:        "call_b",
				Name:      "tool_b",
				Arguments: `{}`,
			},
		}, nil) {
			return
		}
		yield(ai.StreamEvent{Type: ai.StreamEventDone, FinishReason: "tool_calls"}, nil)
	})

	mockLLM := &mockStreamProvider{
		streamResponses: []*ai.ChatStream{
			twoToolCallStream,
			singleContentStream(`{"total":30}`),
		},
	}

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memProvider),
		client.WithTools(toolA, toolB),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[Result](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "combine tool_a and tool_b")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	events, iterErr := collectEvents(stream)
	if iterErr != nil {
		t.Fatalf("unexpected stream error: %v", iterErr)
	}

	types := eventTypes(events)

	// Count ToolCall and ToolResult events
	toolCallCount := countType(types, ReactEventToolCall)
	toolResultCount := countType(types, ReactEventToolResult)

	if toolCallCount != 2 {
		t.Errorf("expected 2 ToolCall events, got %d", toolCallCount)
	}
	if toolResultCount != 2 {
		t.Errorf("expected 2 ToolResult events, got %d", toolResultCount)
	}

	assertContainsType(t, types, ReactEventFinalAnswer)
}

// TestExecuteStream_ContentDeltas verifies that multi-chunk streaming responses
// produce individual ReactEventContent events for each chunk, and that the
// accumulated content in the FinalAnswer event is the concatenation of all chunks.
func TestExecuteStream_ContentDeltas(t *testing.T) {
	type Result struct {
		Msg string `json:"msg"`
	}

	memProvider := inmemory.New()

	mockLLM := &mockStreamProvider{
		streamResponses: []*ai.ChatStream{
			// Simulate streaming a JSON response in three chunks
			multiChunkContentStream(`{"m`, `sg":"hel`, `lo"}`),
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(memProvider))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[Result](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "say hello")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	events, iterErr := collectEvents(stream)
	if iterErr != nil {
		t.Fatalf("unexpected stream error: %v", iterErr)
	}

	// Count content deltas
	deltaCount := countType(eventTypes(events), ReactEventContent)
	if deltaCount != 3 {
		t.Errorf("expected 3 ReactEventContent deltas, got %d", deltaCount)
	}

	// FinalAnswer should have the correct parsed result
	for _, event := range events {
		if event.Type == ReactEventFinalAnswer {
			if event.Result == nil {
				t.Fatal("FinalAnswer has nil Result")
			}
			if event.Result.Msg != "hello" {
				t.Errorf("expected Msg=%q, got %q", "hello", event.Result.Msg)
			}
			return
		}
	}
	t.Fatal("no FinalAnswer event found")
}

// TestExecuteStream_ContextCancellation verifies that cancelling the context
// propagates as a stream error.
func TestExecuteStream_ContextCancellation(t *testing.T) {
	memProvider := inmemory.New()

	ctx, cancel := context.WithCancel(context.Background())

	// A stream that blocks until the context is cancelled
	cancellingStream := ai.NewChatStream(func(yield func(ai.StreamEvent, error) bool) {
		// Cancel before we yield anything
		cancel()
		// Propagate context error
		yield(ai.StreamEvent{}, ctx.Err())
	})

	mockLLM := &mockStreamProvider{
		streamResponses: []*ai.ChatStream{cancellingStream},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(memProvider))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[string](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(ctx, "test cancellation")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	_, iterErr := collectEvents(stream)
	if iterErr == nil {
		t.Fatal("expected error from context cancellation, got nil")
	}
	if !errors.Is(iterErr, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", iterErr)
	}
}

// TestExecuteStream_PreStreamError verifies that a provider error before the
// stream starts is surfaced through the iterator as a ReactEventError.
func TestExecuteStream_PreStreamError(t *testing.T) {
	memProvider := inmemory.New()

	mockLLM := &mockStreamProvider{
		streamErr: fmt.Errorf("auth failed: 401 Unauthorized"),
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(memProvider))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent, err := New[string](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("failed to create ReAct: %v", err)
	}

	stream, err := agent.ExecuteStream(context.Background(), "test pre-stream error")
	if err != nil {
		t.Fatalf("ExecuteStream returned unexpected error: %v", err)
	}

	events, iterErr := collectEvents(stream)
	if iterErr == nil {
		t.Fatal("expected error from pre-stream failure, got nil")
	}

	types := eventTypes(events)
	assertContainsType(t, types, ReactEventError)
}

// --------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------

// assertContainsType fails the test if want is not present in the types slice.
func assertContainsType(t *testing.T, types []ReactEventType, want ReactEventType) {
	t.Helper()
	for _, eventType := range types {
		if eventType == want {
			return
		}
	}
	t.Errorf("expected event type %q in sequence %v", want, types)
}

// countType returns the number of occurrences of target in types.
func countType(types []ReactEventType, target ReactEventType) int {
	count := 0
	for _, eventType := range types {
		if eventType == target {
			count++
		}
	}
	return count
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

// Ensure mockStreamProvider satisfies ai.StreamProvider at compile time.
var _ ai.StreamProvider = (*mockStreamProvider)(nil)

// Ensure mockProvider satisfies ai.Provider (not StreamProvider) at compile time.
var _ ai.Provider = (*mockProvider)(nil)
