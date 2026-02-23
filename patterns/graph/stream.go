package graph

import (
	"context"
	"fmt"
	"iter"
	"sync"
	"time"

	"github.com/leofalp/aigo/core/overview"
)

// defaultStreamBufferSize is the default channel buffer size for streaming events.
// This provides a reasonable balance between memory usage and goroutine blocking
// when parallel nodes produce events faster than the consumer reads them.
const defaultStreamBufferSize = 64

// --- Event Types ---

// GraphEventType identifies what happened during graph execution.
// Each event in the stream carries exactly one type.
type GraphEventType string

const (
	// GraphEventLevelStart signals that a new execution level has begun.
	// The Level field contains the level number (0-based), and NodeIDs lists
	// the node IDs about to execute at this level.
	GraphEventLevelStart GraphEventType = "level_start"

	// GraphEventNodeStart signals that a specific node has begun executing.
	// The NodeID field identifies the node.
	GraphEventNodeStart GraphEventType = "node_start"

	// GraphEventNodeContent carries a content delta from a node's LLM call.
	// The Content field contains the text fragment.
	GraphEventNodeContent GraphEventType = "node_content"

	// GraphEventNodeReasoning carries a reasoning delta from a node's LLM call.
	// The Reasoning field contains the reasoning fragment.
	GraphEventNodeReasoning GraphEventType = "node_reasoning"

	// GraphEventNodeToolCall indicates a node's LLM decided to call a tool.
	// The ToolName and ToolInput fields are populated.
	GraphEventNodeToolCall GraphEventType = "node_tool_call"

	// GraphEventNodeToolResult indicates a tool execution completed for a node.
	// The ToolName and ToolOutput fields are populated.
	GraphEventNodeToolResult GraphEventType = "node_tool_result"

	// GraphEventNodeComplete signals that a node has finished executing.
	// The NodeResult field contains the node's final result.
	GraphEventNodeComplete GraphEventType = "node_complete"

	// GraphEventNodeError signals that a node encountered an error.
	// The Error field contains the error description.
	GraphEventNodeError GraphEventType = "node_error"

	// GraphEventLevelComplete signals that all nodes in a level have finished.
	// The Level field contains the level number.
	GraphEventLevelComplete GraphEventType = "level_complete"

	// GraphEventDone signals that the entire graph has completed execution.
	GraphEventDone GraphEventType = "done"
)

// --- Event Struct ---

// GraphEvent represents a single event from the graph execution pipeline.
// Each event carries exactly one type of payload, identified by the Type field.
// The Level and NodeID fields provide context about which part of the DAG
// produced the event.
type GraphEvent struct {
	// Type identifies what kind of event this is.
	Type GraphEventType `json:"type"`

	// Level is the topological execution level (0-based) that produced this event.
	Level int `json:"level"`

	// NodeID identifies which node produced this event.
	// Empty for level-scoped events (LevelStart, LevelComplete, Done).
	NodeID string `json:"node_id,omitempty"`

	// Content carries a text delta for GraphEventNodeContent events.
	Content string `json:"content,omitempty"`

	// Reasoning carries a reasoning delta for GraphEventNodeReasoning events.
	Reasoning string `json:"reasoning,omitempty"`

	// ToolName is the name of the tool being called or returning a result.
	// Populated for GraphEventNodeToolCall and GraphEventNodeToolResult.
	ToolName string `json:"tool_name,omitempty"`

	// ToolInput is the JSON-encoded arguments passed to the tool.
	// Populated for GraphEventNodeToolCall only.
	ToolInput string `json:"tool_input,omitempty"`

	// ToolOutput is the string result returned by the tool.
	// Populated for GraphEventNodeToolResult only.
	ToolOutput string `json:"tool_output,omitempty"`

	// NodeResult is the final result of a completed node.
	// Populated only for GraphEventNodeComplete events.
	NodeResult *NodeResult `json:"node_result,omitempty"`

	// NodeIDs lists the node IDs at a level.
	// Populated only for GraphEventLevelStart events.
	NodeIDs []string `json:"node_ids,omitempty"`

	// Error contains the error description for GraphEventNodeError events.
	Error string `json:"error,omitempty"`
}

// --- StreamExecutor Interface ---

// StreamExecutor is an optional interface for nodes that support streaming output.
// If a node's executor implements StreamExecutor, ExecuteStream will use it to
// obtain token-by-token events. Nodes that only implement NodeExecutor have their
// full result delivered as a single GraphEventNodeComplete event (no content deltas).
//
// The returned NodeStream must yield events with Type restricted to:
// GraphEventNodeContent, GraphEventNodeReasoning, GraphEventNodeToolCall,
// and GraphEventNodeToolResult. The graph executor handles NodeStart, NodeComplete,
// and NodeError events automatically.
//
// Example:
//
//	type StreamingAnalyzer struct{}
//
//	func (a *StreamingAnalyzer) Execute(ctx context.Context, input *NodeInput) (*NodeResult, error) {
//	    // Non-streaming fallback
//	    response, err := input.Client.SendMessage(ctx, "Analyze this")
//	    if err != nil { return nil, err }
//	    return &NodeResult{Output: response.Content}, nil
//	}
//
//	func (a *StreamingAnalyzer) ExecuteStream(ctx context.Context, input *NodeInput) (*NodeStream, error) {
//	    // Streaming implementation
//	    return NewNodeStream(func(yield func(GraphEvent, error) bool) {
//	        yield(GraphEvent{Type: GraphEventNodeContent, Content: "streaming..."}, nil)
//	    }), nil
//	}
type StreamExecutor interface {
	ExecuteStream(ctx context.Context, input *NodeInput) (*NodeStream, error)
}

// StreamNodeExecutorFunc is an adapter that allows using an ordinary function
// as a StreamExecutor — mirroring NodeExecutorFunc for the non-streaming case.
type StreamNodeExecutorFunc func(ctx context.Context, input *NodeInput) (*NodeStream, error)

// ExecuteStream calls the underlying function, satisfying the StreamExecutor interface.
func (executorFunc StreamNodeExecutorFunc) ExecuteStream(ctx context.Context, input *NodeInput) (*NodeStream, error) {
	return executorFunc(ctx, input)
}

// --- NodeStream ---

// NodeStream represents the streaming output of a single node's execution.
// It wraps an iterator that yields GraphEvent values for content deltas,
// reasoning, tool calls, and tool results produced by the node.
//
// The final NodeResult must be returned separately by the StreamExecutor
// as the accumulated output of the stream. NodeStream events should NOT
// include NodeStart, NodeComplete, or NodeError — those are managed by
// the graph executor.
type NodeStream struct {
	iterator iter.Seq2[GraphEvent, error]

	// finalResult holds the node's accumulated result after the stream completes.
	// Set by the StreamExecutor implementation once the stream is fully consumed.
	finalResult *NodeResult
}

// NewNodeStream creates a NodeStream from a raw streaming iterator and the
// final result that will be available after the stream is consumed.
// The finalResult pointer may be nil at creation time if the result is
// accumulated during streaming; in that case, set it via SetFinalResult
// before the stream ends.
func NewNodeStream(iterator iter.Seq2[GraphEvent, error], finalResult *NodeResult) *NodeStream {
	return &NodeStream{
		iterator:    iterator,
		finalResult: finalResult,
	}
}

// Iter returns the underlying iterator for range-over-func consumption.
func (stream *NodeStream) Iter() iter.Seq2[GraphEvent, error] {
	return stream.iterator
}

// FinalResult returns the accumulated result of the node after the stream
// has been fully consumed. Returns nil if the stream has not been consumed
// or if no result was set.
func (stream *NodeStream) FinalResult() *NodeResult {
	return stream.finalResult
}

// SetFinalResult sets the accumulated result of the node. This is intended
// for StreamExecutor implementations that build up the result incrementally
// during streaming and need to set it after the iterator completes.
func (stream *NodeStream) SetFinalResult(result *NodeResult) {
	stream.finalResult = result
}

// --- GraphStream ---

// streamContextCarrier holds the final overview and parsed output so Collect()
// can read them after the iterator has completed. This mirrors the contextCarrier
// pattern used in the ReAct streaming implementation.
type streamContextCarrier[T any] struct {
	// overview is captured at the end of ExecuteStream's iterator so Collect()
	// can build a StructuredOverview without needing to re-run the loop.
	overview *overview.Overview

	// parsedData holds the parsed output of type *T from the designated output node.
	// Set by the iterator's deferred cleanup after all levels complete successfully.
	parsedData *T

	// parseError records any error that occurred while parsing the output node result.
	// If non-nil, Collect() returns this error.
	parseError error
}

// GraphStream wraps the streaming graph execution pipeline.
// Events from parallel nodes are multiplexed onto a single stream, identified
// by their NodeID field. The type parameter T matches the Graph[T] output type,
// ensuring that Collect() returns the same type-safe result as Execute().
//
// The stream must be consumed either via Iter() or Collect() to avoid resource
// leaks. Breaking out of an Iter() range loop early is safe — the underlying
// iterator will be abandoned correctly by Go's range-over-func mechanism.
type GraphStream[T any] struct {
	iterator iter.Seq2[GraphEvent, error]

	// carrier captures the final overview and parsed output after the iterator
	// completes, allowing Collect() to build a StructuredOverview.
	carrier *streamContextCarrier[T]
}

// Iter returns the underlying iterator for range-over-func consumption.
//
// Example:
//
//	stream, _ := pipeline.ExecuteStream(ctx, initialState)
//	for event, err := range stream.Iter() {
//	    if err != nil { log.Fatal(err) }
//	    switch event.Type {
//	    case graph.GraphEventNodeContent:
//	        fmt.Printf("[%s] %s", event.NodeID, event.Content)
//	    case graph.GraphEventNodeComplete:
//	        fmt.Printf("\nNode %s complete\n", event.NodeID)
//	    case graph.GraphEventDone:
//	        fmt.Println("Pipeline finished!")
//	    }
//	}
func (stream *GraphStream[T]) Iter() iter.Seq2[GraphEvent, error] {
	return stream.iterator
}

// Collect consumes the entire stream and returns the final execution result
// as a StructuredOverview[T], equivalent to what Execute() returns.
// Any mid-stream error terminates collection and returns that error.
//
// Use this when you want streaming transport (lower time-to-first-byte) but
// do not need to process intermediate events — for example, to benefit from
// streaming backpressure semantics while still receiving a single final result.
func (stream *GraphStream[T]) Collect() (*overview.StructuredOverview[T], error) {
	// Drain the entire iterator. The iterator's deferred cleanup will populate
	// the carrier with the parsed output and overview.
	for _, err := range stream.iterator {
		if err != nil {
			return nil, err
		}
	}

	// Check if parsing the output node failed.
	if stream.carrier.parseError != nil {
		return nil, stream.carrier.parseError
	}

	finalOverview := stream.carrier.overview
	if finalOverview == nil {
		finalOverview = &overview.Overview{}
	}

	return &overview.StructuredOverview[T]{
		Overview: *finalOverview,
		Data:     stream.carrier.parsedData,
	}, nil
}

// --- Internal Streaming Types ---

// errConsumerStopped is a sentinel error returned by executeLevelStreaming and
// executeLevelsStreaming when the consumer breaks out of the range loop (yield
// returned false). Callers must NOT call yield again after receiving this error.
// This is not a real execution failure — it simply means the consumer stopped
// reading events early.
var errConsumerStopped = fmt.Errorf("stream consumer stopped iteration")

// streamEventOrError wraps a GraphEvent and an optional error for channel transport.
// This allows node goroutines to send both events and errors through the same channel.
type streamEventOrError struct {
	event GraphEvent
	err   error
}

// --- ExecuteStream ---

// ExecuteStream starts the graph execution with streaming output.
// Events from all nodes (including parallel nodes within the same level)
// are multiplexed onto a single stream, identified by NodeID.
//
// The stream must be consumed to avoid resource leaks. Respects the graph's
// maxConcurrency setting — parallel node launches within a level are throttled
// by the same semaphore mechanism used by Execute().
//
// ExecuteStream is NOT safe for concurrent use on the same Graph instance.
// Create separate Graph instances for concurrent workflows.
func (graph *Graph[T]) ExecuteStream(ctx context.Context, initialState map[string]any) (*GraphStream[T], error) {
	carrier := &streamContextCarrier[T]{}

	// Resolve the stream buffer size.
	bufferSize := graph.config.streamBufferSize
	if bufferSize <= 0 {
		bufferSize = defaultStreamBufferSize
	}

	iteratorFunc := func(yield func(GraphEvent, error) bool) {
		executionStart := time.Now()

		// Initialize the Overview for cost/usage tracking.
		executionOverview := overview.OverviewFromContext(&ctx)
		executionOverview.StartExecution()
		defer func() {
			executionOverview.EndExecution()
			// Capture the final overview so Collect() can read it.
			carrier.overview = executionOverview
		}()

		// Start observability.
		graph.observeGraphStart(&ctx)

		// Initialize state provider.
		stateProvider := graph.config.stateProvider
		if err := graph.initializeState(ctx, stateProvider, initialState); err != nil {
			graph.observeGraphFailed(ctx, err, time.Since(executionStart))
			yield(GraphEvent{}, fmt.Errorf("failed to initialize graph state: %w", err))
			return
		}

		// Apply graph-level execution timeout if configured.
		if graph.config.executionTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, graph.config.executionTimeout)
			defer cancel()
		}

		// Execute levels with streaming.
		streamError := graph.executeLevelsStreaming(ctx, stateProvider, bufferSize, yield)

		totalDuration := time.Since(executionStart)

		// If the consumer stopped iterating, we must not call yield again.
		if streamError == errConsumerStopped {
			graph.observeGraphCompleted(ctx, totalDuration, false)
			return
		}

		if streamError != nil {
			graph.observeGraphFailed(ctx, streamError, totalDuration)
			// The error was already yielded by executeLevelsStreaming.
			return
		}

		// Parse the output node result into *T for Collect().
		// This mirrors what Execute() does after executeLevels completes.
		parsedResult, parseError := graph.parseOutputResult(ctx, stateProvider)
		if parseError != nil {
			carrier.parseError = fmt.Errorf("failed to parse output from node %q: %w", graph.outputNodeID, parseError)
		} else {
			carrier.parsedData = parsedResult
		}

		// Determine whether all nodes completed successfully.
		completedAll := graph.allNodesCompleted(ctx, stateProvider)
		graph.observeGraphCompleted(ctx, totalDuration, completedAll)

		// Yield the final done event.
		yield(GraphEvent{Type: GraphEventDone}, nil)
	}

	return &GraphStream[T]{
		iterator: iteratorFunc,
		carrier:  carrier,
	}, nil
}

// executeLevelsStreaming iterates through topological levels and streams events
// from each level's parallel node executions. Returns nil on success,
// errConsumerStopped if the consumer broke out of the range loop, or a real
// error if graph execution fails (e.g., fail-fast).
func (graph *Graph[T]) executeLevelsStreaming(
	ctx context.Context,
	stateProvider StateProvider,
	bufferSize int,
	yield func(GraphEvent, error) bool,
) error {
	for levelIndex, levelNodeIDs := range graph.levels {
		// Check context cancellation before starting a new level.
		if err := ctx.Err(); err != nil {
			wrappedErr := fmt.Errorf("context canceled before level %d: %w", levelIndex, err)
			yield(GraphEvent{}, wrappedErr)
			return wrappedErr
		}

		graph.observeLevelStart(ctx, levelIndex, levelNodeIDs)

		// Filter nodes that are ready to execute (all dependencies satisfied).
		readyNodes := graph.filterReadyNodes(ctx, levelNodeIDs, stateProvider)

		if len(readyNodes) == 0 {
			continue
		}

		// Yield level start event.
		if !yield(GraphEvent{
			Type:    GraphEventLevelStart,
			Level:   levelIndex,
			NodeIDs: readyNodes,
		}, nil) {
			return errConsumerStopped
		}

		// Execute all ready nodes at this level with streaming.
		levelError := graph.executeLevelStreaming(ctx, readyNodes, levelIndex, stateProvider, bufferSize, yield)
		if levelError == errConsumerStopped {
			return errConsumerStopped
		}
		if levelError != nil {
			return levelError
		}

		// Yield level complete event.
		if !yield(GraphEvent{
			Type:  GraphEventLevelComplete,
			Level: levelIndex,
		}, nil) {
			return errConsumerStopped
		}
	}

	return nil
}

// executeLevelStreaming runs all ready nodes at a topological level in parallel,
// streaming events from each node through a shared buffered channel. Events are
// multiplexed onto the yield function as they arrive.
//
// Returns nil on success, errConsumerStopped if the consumer broke out of the
// range loop, or a real error if fail-fast detected a node failure.
func (graph *Graph[T]) executeLevelStreaming(
	ctx context.Context,
	readyNodes []string,
	levelIndex int,
	stateProvider StateProvider,
	bufferSize int,
	yield func(GraphEvent, error) bool,
) error {
	var waitGroup sync.WaitGroup
	eventChannel := make(chan streamEventOrError, bufferSize)

	// Create a cancellable context for fail-fast behavior.
	levelContext, cancelLevel := context.WithCancel(ctx)
	defer cancelLevel()

	// Set up concurrency semaphore if maxConcurrency is configured.
	var semaphore chan struct{}
	if graph.config.maxConcurrency > 0 {
		semaphore = make(chan struct{}, graph.config.maxConcurrency)
	}

	// Launch goroutines for each node.
	for _, nodeID := range readyNodes {
		waitGroup.Add(1)

		go func(executingNodeID string) {
			defer waitGroup.Done()

			// Acquire semaphore slot if concurrency is limited.
			if semaphore != nil {
				select {
				case semaphore <- struct{}{}:
					defer func() { <-semaphore }()
				case <-levelContext.Done():
					return
				}
			}

			// Check if level context was canceled (fail-fast from another node).
			if levelContext.Err() != nil {
				return
			}

			err := graph.executeNodeStreaming(levelContext, executingNodeID, levelIndex, stateProvider, eventChannel)
			if err != nil {
				// For fail-fast, cancel all other nodes at this level.
				if graph.config.errorStrategy == ErrorStrategyFailFast {
					cancelLevel()
				}
			}
		}(nodeID)
	}

	// Close the event channel once all node goroutines complete.
	go func() {
		waitGroup.Wait()
		close(eventChannel)
	}()

	// Drain the event channel and yield events to the consumer.
	// If yield returns false (consumer stopped), we must continue draining
	// the channel to allow goroutines to finish and the channel to close,
	// but we must NOT call yield again.
	var firstError error
	consumerStopped := false

	for eventOrErr := range eventChannel {
		// If the consumer already stopped, just drain without yielding.
		if consumerStopped {
			continue
		}

		if eventOrErr.err != nil {
			// Yield the error event to the consumer.
			if !yield(eventOrErr.event, eventOrErr.err) {
				consumerStopped = true
				// Cancel remaining nodes since consumer stopped.
				cancelLevel()
				// Continue draining (don't break — let the channel close).
				continue
			}
			if firstError == nil {
				firstError = eventOrErr.err
			}
			continue
		}

		if !yield(eventOrErr.event, nil) {
			consumerStopped = true
			cancelLevel()
			continue
		}
	}

	if consumerStopped {
		return errConsumerStopped
	}

	// For fail-fast, return the first error to stop level iteration.
	if firstError != nil && graph.config.errorStrategy == ErrorStrategyFailFast {
		return firstError
	}

	return nil
}

// executeNodeStreaming runs a single node's executor with streaming support.
// It sends events to the shared event channel. If the node's executor implements
// StreamExecutor, it streams content deltas; otherwise, it falls back to the
// standard Execute method and emits a single NodeComplete event.
func (graph *Graph[T]) executeNodeStreaming(
	ctx context.Context,
	nodeID string,
	levelIndex int,
	stateProvider StateProvider,
	eventChannel chan<- streamEventOrError,
) error {
	graphNode := graph.nodes[nodeID]

	// Mark node as running.
	if err := stateProvider.SetNodeStatus(ctx, nodeID, NodeRunning); err != nil {
		return graph.sendNodeError(eventChannel, nodeID, levelIndex, err)
	}

	// Start observability span for this node.
	nodeContext := ctx
	graph.observeNodeStart(&nodeContext, nodeID, levelIndex, graphNode.dependencies)

	// Apply node-level timeout if configured.
	if graphNode.timeout > 0 {
		var cancel context.CancelFunc
		nodeContext, cancel = context.WithTimeout(nodeContext, graphNode.timeout)
		defer cancel()
	}

	// Assemble the NodeInput.
	nodeInput, err := graph.assembleNodeInput(nodeContext, graphNode, stateProvider)
	if err != nil {
		failDuration := time.Duration(0)
		markNodeFailed(nodeContext, stateProvider, nodeID, err, failDuration)
		graph.observeNodeFailed(nodeContext, nodeID, err, failDuration)
		return graph.sendNodeError(eventChannel, nodeID, levelIndex, fmt.Errorf("failed to assemble input for node %q: %w", nodeID, err))
	}

	// Send node start event.
	eventChannel <- streamEventOrError{
		event: GraphEvent{
			Type:   GraphEventNodeStart,
			Level:  levelIndex,
			NodeID: nodeID,
		},
	}

	// Check if the executor supports streaming.
	streamExecutor, supportsStreaming := graphNode.executor.(StreamExecutor)

	nodeStart := time.Now()

	if supportsStreaming {
		return graph.executeStreamingNode(nodeContext, nodeID, levelIndex, stateProvider, eventChannel, streamExecutor, nodeInput, nodeStart)
	}

	return graph.executeNonStreamingNode(nodeContext, nodeID, levelIndex, stateProvider, eventChannel, graphNode.executor, nodeInput, nodeStart)
}

// executeStreamingNode handles execution for nodes that implement StreamExecutor.
// It consumes the NodeStream, forwarding events to the shared channel, and
// finalizes the node result in the state provider.
func (graph *Graph[T]) executeStreamingNode(
	ctx context.Context,
	nodeID string,
	levelIndex int,
	stateProvider StateProvider,
	eventChannel chan<- streamEventOrError,
	executor StreamExecutor,
	nodeInput *NodeInput,
	nodeStart time.Time,
) error {
	nodeStream, streamErr := executor.ExecuteStream(ctx, nodeInput)
	if streamErr != nil {
		executionDuration := time.Since(nodeStart)
		markNodeFailed(ctx, stateProvider, nodeID, streamErr, executionDuration)
		graph.observeNodeFailed(ctx, nodeID, streamErr, executionDuration)
		return graph.sendNodeError(eventChannel, nodeID, levelIndex, fmt.Errorf("node %q streaming execution failed: %w", nodeID, streamErr))
	}

	// Consume the node's stream and forward events to the shared channel.
	var streamConsumeError error
	for event, err := range nodeStream.Iter() {
		if err != nil {
			streamConsumeError = err
			break
		}

		// Tag the event with node metadata.
		event.NodeID = nodeID
		event.Level = levelIndex

		eventChannel <- streamEventOrError{event: event}
	}

	executionDuration := time.Since(nodeStart)

	if streamConsumeError != nil {
		markNodeFailed(ctx, stateProvider, nodeID, streamConsumeError, executionDuration)
		graph.observeNodeFailed(ctx, nodeID, streamConsumeError, executionDuration)
		return graph.sendNodeError(eventChannel, nodeID, levelIndex, fmt.Errorf("node %q stream consumption failed: %w", nodeID, streamConsumeError))
	}

	// Get the final result from the stream.
	result := nodeStream.FinalResult()
	if result == nil {
		result = &NodeResult{}
	}
	result.Duration = executionDuration

	// Store result and mark completed.
	if err := stateProvider.SetNodeResult(ctx, nodeID, result); err != nil {
		return graph.sendNodeError(eventChannel, nodeID, levelIndex, fmt.Errorf("failed to store result for node %q: %w", nodeID, err))
	}

	if err := stateProvider.SetNodeStatus(ctx, nodeID, NodeCompleted); err != nil {
		return graph.sendNodeError(eventChannel, nodeID, levelIndex, fmt.Errorf("failed to set node %q status to completed: %w", nodeID, err))
	}

	graph.observeNodeCompleted(ctx, nodeID, result)

	// Send node complete event.
	eventChannel <- streamEventOrError{
		event: GraphEvent{
			Type:       GraphEventNodeComplete,
			Level:      levelIndex,
			NodeID:     nodeID,
			NodeResult: result,
		},
	}

	return nil
}

// executeNonStreamingNode handles execution for nodes that only implement
// NodeExecutor (no streaming support). It calls Execute and emits a single
// NodeComplete event with the full result.
func (graph *Graph[T]) executeNonStreamingNode(
	ctx context.Context,
	nodeID string,
	levelIndex int,
	stateProvider StateProvider,
	eventChannel chan<- streamEventOrError,
	executor NodeExecutor,
	nodeInput *NodeInput,
	nodeStart time.Time,
) error {
	result, execError := executor.Execute(ctx, nodeInput)
	executionDuration := time.Since(nodeStart)

	if execError != nil {
		markNodeFailed(ctx, stateProvider, nodeID, execError, executionDuration)
		graph.observeNodeFailed(ctx, nodeID, execError, executionDuration)
		return graph.sendNodeError(eventChannel, nodeID, levelIndex, fmt.Errorf("node %q execution failed: %w", nodeID, execError))
	}

	// Ensure result is not nil.
	if result == nil {
		result = &NodeResult{}
	}
	result.Duration = executionDuration

	// Store result and mark completed.
	if err := stateProvider.SetNodeResult(ctx, nodeID, result); err != nil {
		return graph.sendNodeError(eventChannel, nodeID, levelIndex, fmt.Errorf("failed to store result for node %q: %w", nodeID, err))
	}

	if err := stateProvider.SetNodeStatus(ctx, nodeID, NodeCompleted); err != nil {
		return graph.sendNodeError(eventChannel, nodeID, levelIndex, fmt.Errorf("failed to set node %q status to completed: %w", nodeID, err))
	}

	graph.observeNodeCompleted(ctx, nodeID, result)

	// Send node complete event with the full result.
	eventChannel <- streamEventOrError{
		event: GraphEvent{
			Type:       GraphEventNodeComplete,
			Level:      levelIndex,
			NodeID:     nodeID,
			NodeResult: result,
		},
	}

	return nil
}

// sendNodeError sends a NodeError event to the event channel and returns the error.
// This is a convenience helper to avoid repetition in error paths.
func (graph *Graph[T]) sendNodeError(
	eventChannel chan<- streamEventOrError,
	nodeID string,
	levelIndex int,
	nodeError error,
) error {
	eventChannel <- streamEventOrError{
		event: GraphEvent{
			Type:   GraphEventNodeError,
			Level:  levelIndex,
			NodeID: nodeID,
			Error:  nodeError.Error(),
		},
		err: nodeError,
	}
	return nodeError
}
