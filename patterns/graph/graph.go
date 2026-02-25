package graph

import (
	"context"
	"time"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/tool"
)

// NodeStatus represents the lifecycle status of a node during graph execution.
type NodeStatus string

const (
	// NodePending indicates the node has not started execution yet.
	NodePending NodeStatus = "pending"

	// NodeRunning indicates the node is currently executing.
	NodeRunning NodeStatus = "running"

	// NodeCompleted indicates the node has finished execution successfully.
	NodeCompleted NodeStatus = "completed"

	// NodeFailed indicates the node encountered an error during execution.
	NodeFailed NodeStatus = "failed"

	// NodeSkipped indicates the node was skipped because a dependency failed
	// or an edge condition evaluated to false.
	NodeSkipped NodeStatus = "skipped"
)

// ErrorStrategy defines how the graph handles errors when nodes fail during
// parallel execution within the same level.
type ErrorStrategy string

const (
	// ErrorStrategyFailFast cancels all running nodes and stops graph execution
	// as soon as any node fails. This is the default strategy.
	ErrorStrategyFailFast ErrorStrategy = "fail_fast"

	// ErrorStrategyContinueOnError allows other nodes to continue executing
	// when one fails. Downstream nodes that depend on the failed node are
	// automatically skipped.
	ErrorStrategyContinueOnError ErrorStrategy = "continue_on_error"
)

// NodeResult contains the output produced by a node after execution.
// The Output field must be JSON-serializable when using an external StateProvider
// for persistence.
type NodeResult struct {
	// Output is the data produced by the node. It can be any type, but must be
	// JSON-serializable when using external state persistence.
	Output any

	// Error records the execution error, if the node failed.
	Error error

	// Duration is the wall-clock time the node took to execute.
	Duration time.Duration

	// Metadata contains arbitrary key-value pairs for additional information
	// such as token counts, model used, cost breakdown, etc.
	Metadata map[string]any
}

// NodeInput contains all the data available to a node during execution.
// It provides access to upstream results, shared state, node-specific parameters,
// and the LLM client configured for this node.
type NodeInput struct {
	// UpstreamResults maps each upstream node ID to its execution result.
	// Only completed upstream nodes appear in this map.
	UpstreamResults map[string]*NodeResult

	// SharedState provides thread-safe access to state shared across all nodes.
	SharedState StateProvider

	// Params contains node-specific parameters set at construction time
	// via WithNodeParams.
	Params map[string]any

	// Client is the LLM client for this node. It is either the node-specific
	// client set via WithNodeClient, or the graph's default client.
	Client *client.Client
}

// NodeExecutor is the interface that every graph node must implement.
// It defines the processing logic for a single step in the workflow.
//
// Implementations should:
//   - Use input.Client for LLM interactions
//   - Read upstream results from input.UpstreamResults
//   - Use input.SharedState for cross-node data sharing
//   - Return a NodeResult with the Output field populated on success
//   - Return an error if the execution fails
//
// Example:
//
//	type AnalyzeExecutor struct{}
//
//	func (e *AnalyzeExecutor) Execute(ctx context.Context, input *NodeInput) (*NodeResult, error) {
//	    response, err := input.Client.SendMessage(ctx, "Analyze this data")
//	    if err != nil {
//	        return nil, fmt.Errorf("failed to analyze: %w", err)
//	    }
//	    return &NodeResult{Output: response.Content}, nil
//	}
type NodeExecutor interface {
	Execute(ctx context.Context, input *NodeInput) (*NodeResult, error)
}

// NodeExecutorFunc is an adapter that allows using an ordinary function as a
// NodeExecutor. If f is a function with the appropriate signature,
// NodeExecutorFunc(f) is a NodeExecutor that calls f.
type NodeExecutorFunc func(ctx context.Context, input *NodeInput) (*NodeResult, error)

// Execute calls the underlying function, satisfying the NodeExecutor interface.
func (executorFunc NodeExecutorFunc) Execute(ctx context.Context, input *NodeInput) (*NodeResult, error) {
	return executorFunc(ctx, input)
}

// EdgeCondition is a function that determines whether an edge should be traversed
// during execution. It receives the execution context, the result of the source node,
// and the current shared state. If the condition returns false, the target node may
// be skipped (if all its incoming edges have false conditions).
//
// The context carries cancellation signals and deadlines from the graph execution,
// and should be passed to any StateProvider calls within the condition.
//
// A nil EdgeCondition means the edge is always traversed.
type EdgeCondition func(ctx context.Context, result *NodeResult, state StateProvider) bool

// node represents a single processing step in the graph.
// It is created internally by the GraphBuilder and is not directly instantiated by users.
type node struct {
	// id is the unique identifier for this node within the graph.
	id string

	// executor contains the processing logic for this node.
	executor NodeExecutor

	// nodeClient overrides the graph's default client for this node.
	// If nil, the graph's default client is used.
	nodeClient *client.Client

	// nodeTools are additional tools available to this node's client.
	nodeTools []tool.GenericTool

	// params contains node-specific parameters accessible via NodeInput.Params.
	params map[string]any

	// timeout is the maximum duration allowed for this node's execution.
	// Zero means no timeout (uses the graph-level timeout if set).
	timeout time.Duration

	// dependencies lists the IDs of nodes that must complete before this node
	// can execute. Populated during Build() from the graph edges.
	dependencies []string
}

// edge represents a directed connection between two nodes in the graph.
type edge struct {
	// from is the ID of the source node.
	from string

	// to is the ID of the target node.
	to string

	// condition is an optional function that determines whether this edge
	// should be traversed. If nil, the edge is always traversed.
	condition EdgeCondition
}

// graphConfig holds the configuration for a Graph, populated by Options.
type graphConfig struct {
	// maxConcurrency limits the number of nodes that can execute in parallel.
	// Zero means unlimited concurrency.
	maxConcurrency int

	// executionTimeout is the maximum duration for the entire graph execution.
	// Zero means no timeout.
	executionTimeout time.Duration

	// errorStrategy determines how the graph handles node failures.
	// Defaults to ErrorStrategyFailFast.
	errorStrategy ErrorStrategy

	// outputNodeID designates which node produces the final output.
	// If empty, the last node in topological order is used.
	outputNodeID string

	// stateProvider is the storage backend for graph state.
	// If nil, InMemoryStateProvider is used.
	stateProvider StateProvider

	// streamBufferSize is the internal channel buffer size for streaming events.
	// Used by ExecuteStream to control backpressure between node goroutines
	// and the consumer. Zero means use the default (defaultStreamBufferSize).
	streamBufferSize int
}

// Graph represents a validated, executable directed acyclic graph of LLM processing steps.
// It is generic over T, the type of the final output produced by the designated output node.
//
// A Graph is created via GraphBuilder[T].Build(), which validates the graph structure
// (cycle detection, edge validation) and computes the topological ordering.
//
// The Graph is safe for sequential use but not for concurrent Execute() calls on the
// same instance, because node statuses are mutated during execution. Create separate
// Graph instances for concurrent workflows.
type Graph[T any] struct {
	// defaultClient is the LLM client used by nodes that don't have a node-specific override.
	defaultClient *client.Client

	// nodes maps node IDs to their definitions.
	nodes map[string]*node

	// edges contains all directed edges in the graph.
	edges []*edge

	// levels contains node IDs grouped by topological level.
	// Level 0 nodes have no dependencies; level N nodes depend only on nodes in levels < N.
	levels [][]string

	// topologicalOrder contains all node IDs in topological sort order.
	topologicalOrder []string

	// outputNodeID is the node whose result is parsed as the final output T.
	outputNodeID string

	// config holds the graph's execution configuration.
	config *graphConfig

	// observer is resolved from the default client for observability.
	observer observerState
}
