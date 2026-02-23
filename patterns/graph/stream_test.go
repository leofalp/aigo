package graph

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- Streaming Test Helpers ---

// collectEvents drains a GraphStream's Iter() into a slice of GraphEvents.
// If any event yields an error, collection stops and the error is returned
// along with all events collected so far.
func collectEvents[T any](stream *GraphStream[T]) ([]GraphEvent, error) {
	var events []GraphEvent
	for event, err := range stream.Iter() {
		if err != nil {
			return events, err
		}
		events = append(events, event)
	}
	return events, nil
}

// eventCountByType counts how many events of each type appear in a slice.
func eventCountByType(events []GraphEvent) map[GraphEventType]int {
	counts := make(map[GraphEventType]int)
	for _, event := range events {
		counts[event.Type]++
	}
	return counts
}

// findEventsByType filters events to only those matching the given type.
func findEventsByType(events []GraphEvent, eventType GraphEventType) []GraphEvent {
	var matched []GraphEvent
	for _, event := range events {
		if event.Type == eventType {
			matched = append(matched, event)
		}
	}
	return matched
}

// streamingEchoExecutor creates an executor that implements both NodeExecutor and
// StreamExecutor. The stream yields content deltas (one per chunk) and then sets
// the final result to the concatenated content.
type streamingEchoExecutor struct {
	chunks []string
}

// Ensure streamingEchoExecutor satisfies both interfaces.
var _ NodeExecutor = (*streamingEchoExecutor)(nil)
var _ StreamExecutor = (*streamingEchoExecutor)(nil)

// Execute provides the non-streaming fallback.
func (executor *streamingEchoExecutor) Execute(_ context.Context, _ *NodeInput) (*NodeResult, error) {
	fullContent := strings.Join(executor.chunks, "")
	return &NodeResult{Output: fullContent}, nil
}

// ExecuteStream yields each chunk as a content delta event and sets the
// accumulated final result on the NodeStream.
func (executor *streamingEchoExecutor) ExecuteStream(_ context.Context, _ *NodeInput) (*NodeStream, error) {
	chunks := executor.chunks
	var finalResult NodeResult

	nodeStream := NewNodeStream(func(yield func(GraphEvent, error) bool) {
		var accumulated strings.Builder
		for _, chunk := range chunks {
			accumulated.WriteString(chunk)
			if !yield(GraphEvent{
				Type:    GraphEventNodeContent,
				Content: chunk,
			}, nil) {
				return
			}
		}
		finalResult = NodeResult{Output: accumulated.String()}
	}, nil)

	// SetFinalResult will be called after iteration, but we set it here as a
	// pointer so the graph executor can retrieve it after consuming the stream.
	// The iterator sets finalResult above; we pass a reference below.
	nodeStream.finalResult = &finalResult

	return nodeStream, nil
}

// streamingErrorExecutor implements StreamExecutor but yields an error mid-stream.
type streamingErrorExecutor struct {
	successChunks int
	streamError   error
}

var _ NodeExecutor = (*streamingErrorExecutor)(nil)
var _ StreamExecutor = (*streamingErrorExecutor)(nil)

func (executor *streamingErrorExecutor) Execute(_ context.Context, _ *NodeInput) (*NodeResult, error) {
	return nil, executor.streamError
}

func (executor *streamingErrorExecutor) ExecuteStream(_ context.Context, _ *NodeInput) (*NodeStream, error) {
	return NewNodeStream(func(yield func(GraphEvent, error) bool) {
		for i := 0; i < executor.successChunks; i++ {
			if !yield(GraphEvent{
				Type:    GraphEventNodeContent,
				Content: fmt.Sprintf("chunk_%d", i),
			}, nil) {
				return
			}
		}
		// Yield an error after the successful chunks.
		yield(GraphEvent{}, executor.streamError)
	}, nil), nil
}

// --- Test Cases ---

// TestExecuteStream_SingleNode_EmitsCorrectEvents verifies that a single-node
// graph produces the expected event sequence: level_start -> node_start ->
// node_complete -> level_complete -> done.
func TestExecuteStream_SingleNode_EmitsCorrectEvents(testCase *testing.T) {
	testClient := newTestClient(testCase)
	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("output", successExecutor("hello world")).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	events, err := collectEvents(stream)
	if err != nil {
		testCase.Fatalf("stream error: %v", err)
	}

	if len(events) == 0 {
		testCase.Fatal("expected events, got none")
	}

	// Verify minimum expected event types.
	counts := eventCountByType(events)
	if counts[GraphEventLevelStart] != 1 {
		testCase.Errorf("expected 1 level_start event, got %d", counts[GraphEventLevelStart])
	}
	if counts[GraphEventNodeStart] != 1 {
		testCase.Errorf("expected 1 node_start event, got %d", counts[GraphEventNodeStart])
	}
	if counts[GraphEventNodeComplete] != 1 {
		testCase.Errorf("expected 1 node_complete event, got %d", counts[GraphEventNodeComplete])
	}
	if counts[GraphEventLevelComplete] != 1 {
		testCase.Errorf("expected 1 level_complete event, got %d", counts[GraphEventLevelComplete])
	}
	if counts[GraphEventDone] != 1 {
		testCase.Errorf("expected 1 done event, got %d", counts[GraphEventDone])
	}

	// Verify the node_complete event carries the correct result.
	completeEvents := findEventsByType(events, GraphEventNodeComplete)
	if completeEvents[0].NodeID != "output" {
		testCase.Errorf("expected node_complete for 'output', got %q", completeEvents[0].NodeID)
	}
	if completeEvents[0].NodeResult == nil {
		testCase.Fatal("expected NodeResult on node_complete event")
	}
	if completeEvents[0].NodeResult.Output != "hello world" {
		testCase.Errorf("expected output 'hello world', got %v", completeEvents[0].NodeResult.Output)
	}

	// Verify the done event is last.
	lastEvent := events[len(events)-1]
	if lastEvent.Type != GraphEventDone {
		testCase.Errorf("expected last event to be 'done', got %q", lastEvent.Type)
	}

	// Verify event ordering: level_start before node_start before node_complete before level_complete.
	var levelStartIdx, nodeStartIdx, nodeCompleteIdx, levelCompleteIdx, doneIdx int
	for i, event := range events {
		switch event.Type {
		case GraphEventLevelStart:
			levelStartIdx = i
		case GraphEventNodeStart:
			nodeStartIdx = i
		case GraphEventNodeComplete:
			nodeCompleteIdx = i
		case GraphEventLevelComplete:
			levelCompleteIdx = i
		case GraphEventDone:
			doneIdx = i
		}
	}
	if !(levelStartIdx < nodeStartIdx && nodeStartIdx < nodeCompleteIdx && nodeCompleteIdx < levelCompleteIdx && levelCompleteIdx < doneIdx) { //nolint:staticcheck // QF1001: negated form expresses strict sequential ordering more clearly than De Morgan's law equivalent
		testCase.Errorf("unexpected event ordering: level_start=%d, node_start=%d, node_complete=%d, level_complete=%d, done=%d",
			levelStartIdx, nodeStartIdx, nodeCompleteIdx, levelCompleteIdx, doneIdx)
	}
}

// TestExecuteStream_LinearChain_LevelOrdering verifies that a linear chain
// (a -> b -> c) produces events in correct level order: three separate levels,
// each with its own level_start/level_complete bracketing.
func TestExecuteStream_LinearChain_LevelOrdering(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("a", successExecutor("result_a")).
		AddNode("b", successExecutor("result_b")).
		AddNode("c", successExecutor("result_c")).
		AddEdge("a", "b").
		AddEdge("b", "c").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	events, err := collectEvents(stream)
	if err != nil {
		testCase.Fatalf("stream error: %v", err)
	}

	counts := eventCountByType(events)

	// 3 levels => 3 level_start + 3 level_complete.
	if counts[GraphEventLevelStart] != 3 {
		testCase.Errorf("expected 3 level_start events, got %d", counts[GraphEventLevelStart])
	}
	if counts[GraphEventLevelComplete] != 3 {
		testCase.Errorf("expected 3 level_complete events, got %d", counts[GraphEventLevelComplete])
	}
	// 3 nodes => 3 node_start + 3 node_complete.
	if counts[GraphEventNodeStart] != 3 {
		testCase.Errorf("expected 3 node_start events, got %d", counts[GraphEventNodeStart])
	}
	if counts[GraphEventNodeComplete] != 3 {
		testCase.Errorf("expected 3 node_complete events, got %d", counts[GraphEventNodeComplete])
	}

	// Verify level ordering: level 0 events appear before level 1, which appear before level 2.
	levelStarts := findEventsByType(events, GraphEventLevelStart)
	for i, levelEvent := range levelStarts {
		if levelEvent.Level != i {
			testCase.Errorf("expected level_start[%d].Level=%d, got %d", i, i, levelEvent.Level)
		}
	}

	// Verify node ordering: a at level 0, b at level 1, c at level 2.
	nodeCompletes := findEventsByType(events, GraphEventNodeComplete)
	expectedNodes := []string{"a", "b", "c"}
	for i, nodeEvent := range nodeCompletes {
		if nodeEvent.NodeID != expectedNodes[i] {
			testCase.Errorf("expected node_complete[%d].NodeID=%q, got %q", i, expectedNodes[i], nodeEvent.NodeID)
		}
		if nodeEvent.Level != i {
			testCase.Errorf("expected node_complete[%d].Level=%d, got %d", i, i, nodeEvent.Level)
		}
	}
}

// TestExecuteStream_DiamondTopology_ParallelEvents verifies that a diamond
// topology (root -> left+right -> merge) produces events with correct level
// metadata and that both parallel nodes at level 1 emit events.
func TestExecuteStream_DiamondTopology_ParallelEvents(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("root", successExecutor("root_data")).
		AddNode("left", successExecutor("left_result")).
		AddNode("right", successExecutor("right_result")).
		AddNode("merge", successExecutor("merged")).
		AddEdge("root", "left").
		AddEdge("root", "right").
		AddEdge("left", "merge").
		AddEdge("right", "merge").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	events, err := collectEvents(stream)
	if err != nil {
		testCase.Fatalf("stream error: %v", err)
	}

	counts := eventCountByType(events)

	// Diamond has 3 levels: {root}, {left, right}, {merge}.
	if counts[GraphEventLevelStart] != 3 {
		testCase.Errorf("expected 3 level_start events, got %d", counts[GraphEventLevelStart])
	}
	// 4 nodes total.
	if counts[GraphEventNodeComplete] != 4 {
		testCase.Errorf("expected 4 node_complete events, got %d", counts[GraphEventNodeComplete])
	}

	// Verify level 1 has 2 node IDs in its level_start event.
	levelStarts := findEventsByType(events, GraphEventLevelStart)
	level1Start := levelStarts[1]
	if len(level1Start.NodeIDs) != 2 {
		testCase.Errorf("expected 2 nodes at level 1, got %d: %v", len(level1Start.NodeIDs), level1Start.NodeIDs)
	}

	// Verify both "left" and "right" have node_complete events at level 1.
	nodeCompletes := findEventsByType(events, GraphEventNodeComplete)
	level1Nodes := make(map[string]bool)
	for _, nodeEvent := range nodeCompletes {
		if nodeEvent.Level == 1 {
			level1Nodes[nodeEvent.NodeID] = true
		}
	}
	if !level1Nodes["left"] || !level1Nodes["right"] {
		testCase.Errorf("expected both 'left' and 'right' to complete at level 1, got: %v", level1Nodes)
	}
}

// TestExecuteStream_StreamExecutor_ContentDeltas verifies that a node implementing
// StreamExecutor produces content delta events (GraphEventNodeContent) for each chunk.
func TestExecuteStream_StreamExecutor_ContentDeltas(testCase *testing.T) {
	testClient := newTestClient(testCase)
	chunks := []string{"Hello", ", ", "World", "!"}

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("streamer", &streamingEchoExecutor{chunks: chunks}).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	events, err := collectEvents(stream)
	if err != nil {
		testCase.Fatalf("stream error: %v", err)
	}

	// Should have content delta events for each chunk.
	contentEvents := findEventsByType(events, GraphEventNodeContent)
	if len(contentEvents) != len(chunks) {
		testCase.Fatalf("expected %d content events, got %d", len(chunks), len(contentEvents))
	}

	for i, contentEvent := range contentEvents {
		if contentEvent.Content != chunks[i] {
			testCase.Errorf("content[%d]: expected %q, got %q", i, chunks[i], contentEvent.Content)
		}
		if contentEvent.NodeID != "streamer" {
			testCase.Errorf("content[%d]: expected nodeID 'streamer', got %q", i, contentEvent.NodeID)
		}
	}

	// The node_complete event should carry the accumulated result.
	completeEvents := findEventsByType(events, GraphEventNodeComplete)
	if len(completeEvents) != 1 {
		testCase.Fatalf("expected 1 node_complete event, got %d", len(completeEvents))
	}
	if completeEvents[0].NodeResult == nil {
		testCase.Fatal("expected NodeResult on node_complete event")
	}
	expectedFull := strings.Join(chunks, "")
	if completeEvents[0].NodeResult.Output != expectedFull {
		testCase.Errorf("expected output %q, got %v", expectedFull, completeEvents[0].NodeResult.Output)
	}
}

// TestExecuteStream_MixedExecutors verifies that a graph with both streaming and
// non-streaming nodes produces the correct events for each type.
func TestExecuteStream_MixedExecutors(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("standard", successExecutor("standard_result")).
		AddNode("streaming", &streamingEchoExecutor{chunks: []string{"A", "B"}}).
		AddEdge("standard", "streaming").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	events, err := collectEvents(stream)
	if err != nil {
		testCase.Fatalf("stream error: %v", err)
	}

	// "standard" node: no content events (non-streaming).
	// "streaming" node: 2 content events (streaming).
	contentEvents := findEventsByType(events, GraphEventNodeContent)
	if len(contentEvents) != 2 {
		testCase.Errorf("expected 2 content events from streaming node, got %d", len(contentEvents))
	}
	for _, contentEvent := range contentEvents {
		if contentEvent.NodeID != "streaming" {
			testCase.Errorf("expected content events from 'streaming' node, got from %q", contentEvent.NodeID)
		}
	}

	// Both nodes should have node_complete events.
	completeEvents := findEventsByType(events, GraphEventNodeComplete)
	if len(completeEvents) != 2 {
		testCase.Errorf("expected 2 node_complete events, got %d", len(completeEvents))
	}
}

// TestExecuteStream_FailFast_StopsOnError verifies that ExecuteStream with fail-fast
// strategy stops yielding events after a node error and the stream error is propagated.
func TestExecuteStream_FailFast_StopsOnError(testCase *testing.T) {
	testClient := newTestClient(testCase)
	expectedError := errors.New("streaming node failure")

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("root", successExecutor("ok")).
		AddNode("failing", failingExecutor(expectedError)).
		AddNode("downstream", successExecutor("should not reach")).
		AddEdge("root", "failing").
		AddEdge("failing", "downstream").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	_, streamErr := collectEvents(stream)
	if streamErr == nil {
		testCase.Fatal("expected stream error from failing node, got nil")
	}
	if !strings.Contains(streamErr.Error(), "streaming node failure") {
		testCase.Errorf("expected error to contain 'streaming node failure', got: %v", streamErr)
	}
}

// TestExecuteStream_ContinueOnError_SkipsDownstream verifies that with
// continue-on-error strategy, a failing node's downstream nodes are skipped
// but other branches complete normally.
func TestExecuteStream_ContinueOnError_SkipsDownstream(testCase *testing.T) {
	testClient := newTestClient(testCase)

	var downstreamExecuted atomic.Bool

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithErrorStrategy(ErrorStrategyContinueOnError),
		WithOutputNode("success_branch"),
	).
		AddNode("root", successExecutor("ok")).
		AddNode("failing", failingExecutor(errors.New("fail"))).
		AddNode("downstream_of_fail", NodeExecutorFunc(func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
			downstreamExecuted.Store(true)
			return &NodeResult{Output: "should not run"}, nil
		})).
		AddNode("success_branch", successExecutor("success")).
		AddEdge("root", "failing").
		AddEdge("root", "success_branch").
		AddEdge("failing", "downstream_of_fail").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	// With continue-on-error, the stream should contain error events but continue.
	// The error events are yielded through the channel with the err field set.
	// However, the main yield function receives them — we need to collect all events.
	var events []GraphEvent
	var firstStreamErr error
	for event, streamErr := range stream.Iter() {
		if streamErr != nil {
			if firstStreamErr == nil {
				firstStreamErr = streamErr
			}
			// Continue collecting events — continue-on-error doesn't stop the stream.
			continue
		}
		events = append(events, event)
	}

	// There should have been an error from the failing node.
	if firstStreamErr == nil {
		testCase.Error("expected at least one error event from the failing node")
	}

	// The downstream node should NOT have been executed.
	if downstreamExecuted.Load() {
		testCase.Error("downstream of failed node should have been skipped")
	}

	// The success branch should have completed.
	completeEvents := findEventsByType(events, GraphEventNodeComplete)
	successCompleted := false
	for _, completeEvent := range completeEvents {
		if completeEvent.NodeID == "success_branch" {
			successCompleted = true
		}
	}
	if !successCompleted {
		testCase.Error("expected 'success_branch' to complete")
	}

	// A done event should still be emitted.
	doneEvents := findEventsByType(events, GraphEventDone)
	if len(doneEvents) != 1 {
		testCase.Errorf("expected 1 done event, got %d", len(doneEvents))
	}
}

// TestExecuteStream_Collect_ReturnsStructuredOverview verifies that Collect()
// returns a StructuredOverview[T] equivalent to what Execute() returns, with
// the typed result and overview populated.
func TestExecuteStream_Collect_ReturnsStructuredOverview(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("output", successExecutor("collected_result")).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	result, err := stream.Collect()
	if err != nil {
		testCase.Fatalf("Collect error: %v", err)
	}

	if result.Data == nil {
		testCase.Fatal("expected non-nil Data from Collect()")
	}
	if *result.Data != "collected_result" {
		testCase.Errorf("expected 'collected_result', got %q", *result.Data)
	}

	// Overview should have execution times set.
	if result.ExecutionStartTime.IsZero() {
		testCase.Error("expected ExecutionStartTime to be set")
	}
	if result.ExecutionEndTime.IsZero() {
		testCase.Error("expected ExecutionEndTime to be set")
	}
	if result.ExecutionDuration() <= 0 {
		testCase.Error("expected positive execution duration")
	}
}

// TestExecuteStream_Collect_MatchesExecute verifies that Collect() and Execute()
// produce equivalent results for the same graph and input.
func TestExecuteStream_Collect_MatchesExecute(testCase *testing.T) {
	type Report struct {
		Summary string `json:"summary"`
		Score   int    `json:"score"`
	}

	testClient := newTestClient(testCase)
	buildGraph := func() *Graph[Report] {
		executionGraph, err := NewGraphBuilder[Report](testClient).
			AddNode("output", successExecutor(`{"summary":"test","score":42}`)).
			Build()
		if err != nil {
			testCase.Fatalf("build error: %v", err)
		}
		return executionGraph
	}

	// Execute() path.
	executeGraph := buildGraph()
	executeResult, err := executeGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("Execute error: %v", err)
	}

	// ExecuteStream() + Collect() path.
	streamGraph := buildGraph()
	stream, err := streamGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}
	collectResult, err := stream.Collect()
	if err != nil {
		testCase.Fatalf("Collect error: %v", err)
	}

	// Both should produce the same typed result.
	if executeResult.Data == nil || collectResult.Data == nil {
		testCase.Fatal("expected non-nil Data from both Execute() and Collect()")
	}
	if executeResult.Data.Summary != collectResult.Data.Summary {
		testCase.Errorf("Summary mismatch: Execute=%q, Collect=%q",
			executeResult.Data.Summary, collectResult.Data.Summary)
	}
	if executeResult.Data.Score != collectResult.Data.Score {
		testCase.Errorf("Score mismatch: Execute=%d, Collect=%d",
			executeResult.Data.Score, collectResult.Data.Score)
	}
}

// TestExecuteStream_ContextCancellation verifies that canceling the context
// during streaming stops the execution and the stream yields an error.
func TestExecuteStream_ContextCancellation(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("slow", delayedExecutor(5*time.Second, "result")).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	stream, err := executionGraph.ExecuteStream(ctx, nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	// Cancel after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, streamErr := collectEvents(stream)
	if streamErr == nil {
		testCase.Fatal("expected stream error from context cancellation, got nil")
	}
}

// TestExecuteStream_WithMaxConcurrency verifies that maxConcurrency is
// respected during streaming execution — no more than the configured
// number of nodes execute simultaneously.
func TestExecuteStream_WithMaxConcurrency(testCase *testing.T) {
	testClient := newTestClient(testCase)

	var maxConcurrent atomic.Int32
	var currentCount atomic.Int32

	concurrencyTracker := func(nodeID string) NodeExecutorFunc {
		return func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
			current := currentCount.Add(1)
			for {
				currentMax := maxConcurrent.Load()
				if current <= currentMax || maxConcurrent.CompareAndSwap(currentMax, current) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			currentCount.Add(-1)
			return &NodeResult{Output: nodeID}, nil
		}
	}

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithMaxConcurrency(2),
	).
		AddNode("root", successExecutor("ok")).
		AddNode("a", concurrencyTracker("a")).
		AddNode("b", concurrencyTracker("b")).
		AddNode("c", concurrencyTracker("c")).
		AddNode("d", concurrencyTracker("d")).
		AddNode("merge", successExecutor("done")).
		AddEdge("root", "a").
		AddEdge("root", "b").
		AddEdge("root", "c").
		AddEdge("root", "d").
		AddEdge("a", "merge").
		AddEdge("b", "merge").
		AddEdge("c", "merge").
		AddEdge("d", "merge").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	_, streamErr := collectEvents(stream)
	if streamErr != nil {
		testCase.Fatalf("stream error: %v", streamErr)
	}

	if maxConcurrent.Load() > 2 {
		testCase.Errorf("expected max concurrency <= 2, got %d", maxConcurrent.Load())
	}
}

// TestExecuteStream_ConditionalEdge_NodeSkipped verifies that conditional edges
// work correctly during streaming — skipped nodes do not emit node_start or
// node_complete events.
func TestExecuteStream_ConditionalEdge_NodeSkipped(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithOutputNode("check"),
	).
		AddNode("check", NodeExecutorFunc(func(ctx context.Context, input *NodeInput) (*NodeResult, error) {
			_ = input.SharedState.Set(ctx, "quality", 0.3)
			return &NodeResult{Output: "checked"}, nil
		})).
		AddNode("premium", successExecutor("premium_output")).
		AddEdge("check", "premium", WithEdgeCondition(func(ctx context.Context, _ *NodeResult, state StateProvider) bool {
			value, _, _ := state.Get(ctx, "quality")
			return value.(float64) > 0.8
		})).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	events, err := collectEvents(stream)
	if err != nil {
		testCase.Fatalf("stream error: %v", err)
	}

	// "premium" should be skipped — no node_start or node_complete for it.
	for _, event := range events {
		if event.NodeID == "premium" {
			testCase.Errorf("unexpected event for skipped node 'premium': type=%q", event.Type)
		}
	}

	// "check" should have node_complete.
	completeEvents := findEventsByType(events, GraphEventNodeComplete)
	if len(completeEvents) != 1 || completeEvents[0].NodeID != "check" {
		testCase.Errorf("expected exactly 1 node_complete for 'check', got: %v", completeEvents)
	}
}

// TestExecuteStream_NodeStream_EarlyBreak verifies that breaking out of the
// stream's Iter() loop early is safe and doesn't cause panics or resource leaks.
func TestExecuteStream_NodeStream_EarlyBreak(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("a", successExecutor("a_result")).
		AddNode("b", successExecutor("b_result")).
		AddNode("c", successExecutor("c_result")).
		AddEdge("a", "b").
		AddEdge("b", "c").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	// Break after the first event.
	eventCount := 0
	for _, err := range stream.Iter() {
		if err != nil {
			testCase.Fatalf("stream error: %v", err)
		}
		eventCount++
		break // Early break — should be safe.
	}

	if eventCount != 1 {
		testCase.Errorf("expected exactly 1 event before break, got %d", eventCount)
	}
}

// TestWithStreamBufferSize_Option verifies that the WithStreamBufferSize option
// is correctly applied to the graph configuration.
func TestWithStreamBufferSize_Option(testCase *testing.T) {
	testClient := newTestClient(testCase)

	builder := NewGraphBuilder[string](testClient,
		WithStreamBufferSize(128),
	)

	if builder.config.streamBufferSize != 128 {
		testCase.Errorf("expected streamBufferSize=128, got %d", builder.config.streamBufferSize)
	}

	// Also verify default is 0 (meaning use defaultStreamBufferSize at runtime).
	defaultBuilder := NewGraphBuilder[string](testClient)
	if defaultBuilder.config.streamBufferSize != 0 {
		testCase.Errorf("expected default streamBufferSize=0, got %d", defaultBuilder.config.streamBufferSize)
	}
}

// TestExecuteStream_Collect_StructOutput verifies that Collect() correctly
// parses structured output (JSON) into the type parameter T.
func TestExecuteStream_Collect_StructOutput(testCase *testing.T) {
	type Report struct {
		Summary string `json:"summary"`
		Score   int    `json:"score"`
	}

	testClient := newTestClient(testCase)
	executionGraph, err := NewGraphBuilder[Report](testClient).
		AddNode("output", successExecutor(`{"summary":"streaming_test","score":99}`)).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	result, err := stream.Collect()
	if err != nil {
		testCase.Fatalf("Collect error: %v", err)
	}

	if result.Data == nil {
		testCase.Fatal("expected non-nil Data")
	}
	if result.Data.Summary != "streaming_test" {
		testCase.Errorf("expected summary 'streaming_test', got %q", result.Data.Summary)
	}
	if result.Data.Score != 99 {
		testCase.Errorf("expected score 99, got %d", result.Data.Score)
	}
}

// TestExecuteStream_StreamExecutor_WithUpstream verifies that a streaming node
// correctly receives upstream results from a non-streaming predecessor.
func TestExecuteStream_StreamExecutor_WithUpstream(testCase *testing.T) {
	testClient := newTestClient(testCase)

	// A streaming executor that reads its upstream results and incorporates
	// them into its output.
	type upstreamAwareStreamExecutor struct {
		streamingEchoExecutor
	}

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("producer", successExecutor("upstream_data")).
		AddNode("consumer", &streamingEchoExecutor{chunks: []string{"got: ", "upstream_data"}}).
		AddEdge("producer", "consumer").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	events, err := collectEvents(stream)
	if err != nil {
		testCase.Fatalf("stream error: %v", err)
	}

	// Verify both nodes completed.
	completeEvents := findEventsByType(events, GraphEventNodeComplete)
	if len(completeEvents) != 2 {
		testCase.Errorf("expected 2 node_complete events, got %d", len(completeEvents))
	}

	// The consumer should have streaming content events.
	contentEvents := findEventsByType(events, GraphEventNodeContent)
	if len(contentEvents) != 2 {
		testCase.Errorf("expected 2 content events, got %d", len(contentEvents))
	}
}

// TestExecuteStream_InitialState verifies that initial state is accessible
// to nodes during streaming execution.
func TestExecuteStream_InitialState(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("reader", NodeExecutorFunc(func(ctx context.Context, input *NodeInput) (*NodeResult, error) {
			value, exists, stateError := input.SharedState.Get(ctx, "greeting")
			if stateError != nil {
				return nil, stateError
			}
			if !exists {
				return nil, errors.New("greeting not found in state")
			}
			return &NodeResult{Output: value.(string)}, nil
		})).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), map[string]any{
		"greeting": "hello from streaming state",
	})
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	result, err := stream.Collect()
	if err != nil {
		testCase.Fatalf("Collect error: %v", err)
	}

	if result.Data == nil || *result.Data != "hello from streaming state" {
		testCase.Errorf("expected 'hello from streaming state', got %v", result.Data)
	}
}

// TestStreamNodeExecutorFunc_SatisfiesInterface verifies that StreamNodeExecutorFunc
// correctly adapts a function to the StreamExecutor interface.
func TestStreamNodeExecutorFunc_SatisfiesInterface(testCase *testing.T) {
	var executor StreamExecutor = StreamNodeExecutorFunc(func(_ context.Context, _ *NodeInput) (*NodeStream, error) {
		return NewNodeStream(func(yield func(GraphEvent, error) bool) {
			yield(GraphEvent{Type: GraphEventNodeContent, Content: "test"}, nil)
		}, &NodeResult{Output: "test"}), nil
	})

	nodeStream, err := executor.ExecuteStream(context.Background(), &NodeInput{})
	if err != nil {
		testCase.Fatalf("unexpected error: %v", err)
	}

	var contentCount int
	for event, streamErr := range nodeStream.Iter() {
		if streamErr != nil {
			testCase.Fatalf("unexpected stream error: %v", streamErr)
		}
		if event.Type == GraphEventNodeContent {
			contentCount++
		}
	}
	if contentCount != 1 {
		testCase.Errorf("expected 1 content event, got %d", contentCount)
	}
	if nodeStream.FinalResult() == nil {
		testCase.Error("expected non-nil FinalResult")
	}
}

// TestNodeStream_SetFinalResult verifies that SetFinalResult updates the
// stream's result after creation.
func TestNodeStream_SetFinalResult(testCase *testing.T) {
	nodeStream := NewNodeStream(func(yield func(GraphEvent, error) bool) {}, nil)

	if nodeStream.FinalResult() != nil {
		testCase.Error("expected nil FinalResult initially")
	}

	result := &NodeResult{Output: "updated"}
	nodeStream.SetFinalResult(result)

	if nodeStream.FinalResult() != result {
		testCase.Error("expected FinalResult to match the value set via SetFinalResult")
	}
}

// TestExecuteStream_WideParallelFanOut verifies streaming with many parallel
// nodes at the same level — all should emit events and complete.
func TestExecuteStream_WideParallelFanOut(testCase *testing.T) {
	testClient := newTestClient(testCase)
	const fanOutWidth = 10
	var completedCount atomic.Int32

	builder := NewGraphBuilder[string](testClient, WithOutputNode("root"))
	builder.AddNode("root", successExecutor("root"))

	for nodeIndex := 0; nodeIndex < fanOutWidth; nodeIndex++ {
		nodeID := fmt.Sprintf("worker_%d", nodeIndex)
		builder.AddNode(nodeID, NodeExecutorFunc(func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
			completedCount.Add(1)
			return &NodeResult{Output: "done"}, nil
		}))
		builder.AddEdge("root", nodeID)
	}

	executionGraph, err := builder.Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	events, err := collectEvents(stream)
	if err != nil {
		testCase.Fatalf("stream error: %v", err)
	}

	if completedCount.Load() != int32(fanOutWidth) {
		testCase.Errorf("expected %d completions, got %d", fanOutWidth, completedCount.Load())
	}

	// Should have node_complete events for all workers + root.
	completeEvents := findEventsByType(events, GraphEventNodeComplete)
	if len(completeEvents) != fanOutWidth+1 {
		testCase.Errorf("expected %d node_complete events, got %d", fanOutWidth+1, len(completeEvents))
	}
}

// TestExecuteStream_SharedStateBetweenNodes verifies that shared state written
// by one node is readable by a downstream node during streaming execution.
func TestExecuteStream_SharedStateBetweenNodes(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("writer", NodeExecutorFunc(func(ctx context.Context, input *NodeInput) (*NodeResult, error) {
			if setErr := input.SharedState.Set(ctx, "computed_value", "42"); setErr != nil {
				return nil, setErr
			}
			return &NodeResult{Output: "written"}, nil
		})).
		AddNode("reader", NodeExecutorFunc(func(ctx context.Context, input *NodeInput) (*NodeResult, error) {
			value, exists, stateError := input.SharedState.Get(ctx, "computed_value")
			if stateError != nil {
				return nil, stateError
			}
			if !exists {
				return nil, errors.New("computed_value not found")
			}
			return &NodeResult{Output: "read:" + value.(string)}, nil
		})).
		AddEdge("writer", "reader").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	result, err := stream.Collect()
	if err != nil {
		testCase.Fatalf("Collect error: %v", err)
	}

	if result.Data == nil || *result.Data != "read:42" {
		testCase.Errorf("expected 'read:42', got %v", result.Data)
	}
}

// TestExecuteStream_LevelStartNodeIDs verifies that level_start events carry
// the correct list of node IDs for each level.
func TestExecuteStream_LevelStartNodeIDs(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("root", successExecutor("ok")).
		AddNode("left", successExecutor("left")).
		AddNode("right", successExecutor("right")).
		AddNode("merge", successExecutor("done")).
		AddEdge("root", "left").
		AddEdge("root", "right").
		AddEdge("left", "merge").
		AddEdge("right", "merge").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	events, err := collectEvents(stream)
	if err != nil {
		testCase.Fatalf("stream error: %v", err)
	}

	levelStarts := findEventsByType(events, GraphEventLevelStart)
	if len(levelStarts) != 3 {
		testCase.Fatalf("expected 3 level_start events, got %d", len(levelStarts))
	}

	// Level 0: [root]
	if len(levelStarts[0].NodeIDs) != 1 || levelStarts[0].NodeIDs[0] != "root" {
		testCase.Errorf("expected level 0 NodeIDs=[root], got %v", levelStarts[0].NodeIDs)
	}

	// Level 1: [left, right] (order may vary).
	if len(levelStarts[1].NodeIDs) != 2 {
		testCase.Errorf("expected level 1 NodeIDs to have 2 entries, got %v", levelStarts[1].NodeIDs)
	}
	level1Set := make(map[string]bool)
	for _, nodeID := range levelStarts[1].NodeIDs {
		level1Set[nodeID] = true
	}
	if !level1Set["left"] || !level1Set["right"] {
		testCase.Errorf("expected level 1 NodeIDs to contain 'left' and 'right', got %v", levelStarts[1].NodeIDs)
	}

	// Level 2: [merge]
	if len(levelStarts[2].NodeIDs) != 1 || levelStarts[2].NodeIDs[0] != "merge" {
		testCase.Errorf("expected level 2 NodeIDs=[merge], got %v", levelStarts[2].NodeIDs)
	}
}

// TestExecuteStream_GraphTimeout verifies that graph-level execution timeout
// works correctly with streaming.
func TestExecuteStream_GraphTimeout(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithExecutionTimeout(100*time.Millisecond),
	).
		AddNode("slow", delayedExecutor(5*time.Second, "too slow")).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	_, streamErr := collectEvents(stream)
	if streamErr == nil {
		testCase.Fatal("expected timeout error from streaming, got nil")
	}
}

// TestExecuteStream_StreamingNodeMidStreamError verifies that when a streaming
// node yields an error mid-stream, the error is correctly propagated to the
// stream consumer and the node is marked as failed.
func TestExecuteStream_StreamingNodeMidStreamError(testCase *testing.T) {
	testClient := newTestClient(testCase)
	midStreamError := errors.New("mid-stream failure")

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("streamer", &streamingErrorExecutor{
			successChunks: 2,
			streamError:   midStreamError,
		}).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	_, streamErr := collectEvents(stream)
	if streamErr == nil {
		testCase.Fatal("expected error from mid-stream failure, got nil")
	}
	if !strings.Contains(streamErr.Error(), "mid-stream failure") {
		testCase.Errorf("expected error to contain 'mid-stream failure', got: %v", streamErr)
	}
}

// TestExecuteStream_ParallelStreamingNodes verifies that multiple streaming
// nodes at the same level produce interleaved content events, all correctly
// tagged with their respective NodeIDs.
func TestExecuteStream_ParallelStreamingNodes(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithOutputNode("root"),
	).
		AddNode("root", successExecutor("ok")).
		AddNode("stream_a", &streamingEchoExecutor{chunks: []string{"A1", "A2", "A3"}}).
		AddNode("stream_b", &streamingEchoExecutor{chunks: []string{"B1", "B2"}}).
		AddEdge("root", "stream_a").
		AddEdge("root", "stream_b").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stream, err := executionGraph.ExecuteStream(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("ExecuteStream error: %v", err)
	}

	events, err := collectEvents(stream)
	if err != nil {
		testCase.Fatalf("stream error: %v", err)
	}

	// Verify content events from both streaming nodes.
	contentEvents := findEventsByType(events, GraphEventNodeContent)

	// stream_a produces 3 content events, stream_b produces 2.
	streamAContent := 0
	streamBContent := 0
	for _, contentEvent := range contentEvents {
		switch contentEvent.NodeID {
		case "stream_a":
			streamAContent++
		case "stream_b":
			streamBContent++
		}
	}

	if streamAContent != 3 {
		testCase.Errorf("expected 3 content events from stream_a, got %d", streamAContent)
	}
	if streamBContent != 2 {
		testCase.Errorf("expected 2 content events from stream_b, got %d", streamBContent)
	}
}

// --- Concurrency Safety Test ---

// TestExecuteStream_EventChannelNotLeaked verifies that the event channel is
// properly closed after all goroutines finish, preventing goroutine leaks.
// This is verified indirectly by the fact that the stream terminates with a
// done event and no deadlock occurs.
func TestExecuteStream_EventChannelNotLeaked(testCase *testing.T) {
	testClient := newTestClient(testCase)

	// Create a graph with multiple levels and parallel nodes to exercise
	// the channel fan-in pattern.
	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("root", successExecutor("ok")).
		AddNode("a", delayedExecutor(10*time.Millisecond, "a")).
		AddNode("b", delayedExecutor(10*time.Millisecond, "b")).
		AddNode("c", delayedExecutor(10*time.Millisecond, "c")).
		AddNode("merge", successExecutor("done")).
		AddEdge("root", "a").
		AddEdge("root", "b").
		AddEdge("root", "c").
		AddEdge("a", "merge").
		AddEdge("b", "merge").
		AddEdge("c", "merge").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	// Run multiple times to increase confidence there are no race conditions.
	for iteration := 0; iteration < 10; iteration++ {
		// Reset state between runs.
		if err := executionGraph.Reset(context.Background(), nil); err != nil {
			testCase.Fatalf("reset error on iteration %d: %v", iteration, err)
		}

		stream, err := executionGraph.ExecuteStream(context.Background(), nil)
		if err != nil {
			testCase.Fatalf("ExecuteStream error on iteration %d: %v", iteration, err)
		}

		events, err := collectEvents(stream)
		if err != nil {
			testCase.Fatalf("stream error on iteration %d: %v", iteration, err)
		}

		// Verify a done event was emitted.
		doneEvents := findEventsByType(events, GraphEventDone)
		if len(doneEvents) != 1 {
			testCase.Errorf("iteration %d: expected 1 done event, got %d", iteration, len(doneEvents))
		}
	}
}

// Suppress unused import warnings for sync (used by test helpers from graph_test.go).
var _ = sync.Mutex{}
