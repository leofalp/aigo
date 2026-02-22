package graph

import (
	"time"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/tool"
)

// Option is a functional option for configuring Graph behavior.
// Options are applied during GraphBuilder construction via NewGraphBuilder.
type Option func(*graphConfig)

// NodeOption is a functional option for configuring individual node behavior.
// Node options are applied via GraphBuilder.AddNode.
type NodeOption func(*node)

// EdgeOption is a functional option for configuring individual edge behavior.
// Edge options are applied via GraphBuilder.AddEdge.
type EdgeOption func(*edge)

// --- Graph Options ---

// WithMaxConcurrency limits the number of nodes that can execute in parallel
// within the same topological level. A value of 0 (default) means unlimited
// concurrency — all ready nodes at a level execute simultaneously.
//
// Use this to control resource consumption when nodes are resource-intensive
// (e.g., each node makes expensive API calls).
//
// Example:
//
//	graph.NewGraphBuilder[Result](defaultClient,
//	    graph.WithMaxConcurrency(3), // at most 3 nodes running at once
//	)
func WithMaxConcurrency(maxConcurrency int) Option {
	return func(config *graphConfig) {
		config.maxConcurrency = maxConcurrency
	}
}

// WithExecutionTimeout sets the maximum duration for the entire graph execution.
// If the timeout is exceeded, the context is canceled and all running nodes
// receive a cancellation signal. A value of 0 (default) means no timeout.
//
// Example:
//
//	graph.NewGraphBuilder[Result](defaultClient,
//	    graph.WithExecutionTimeout(5 * time.Minute),
//	)
func WithExecutionTimeout(timeout time.Duration) Option {
	return func(config *graphConfig) {
		config.executionTimeout = timeout
	}
}

// WithErrorStrategy sets the error handling strategy for the graph.
// The default is ErrorStrategyFailFast, which cancels all running nodes
// and stops execution as soon as any node fails.
//
// Use ErrorStrategyContinueOnError to allow other nodes to finish even
// when one fails. Downstream nodes that depend on the failed node are
// automatically skipped.
//
// Example:
//
//	graph.NewGraphBuilder[Result](defaultClient,
//	    graph.WithErrorStrategy(graph.ErrorStrategyContinueOnError),
//	)
func WithErrorStrategy(strategy ErrorStrategy) Option {
	return func(config *graphConfig) {
		config.errorStrategy = strategy
	}
}

// WithOutputNode designates which node produces the final typed output T.
// By default, the last node in topological order is used as the output node.
//
// Use this when the graph has multiple terminal nodes (nodes with no outgoing
// edges) and you want to control which one provides the final result.
//
// Example:
//
//	builder.AddNode("summary", summaryExecutor)
//	builder.AddNode("metrics", metricsExecutor) // side-effect node
//	// ... edges ...
//	graph.NewGraphBuilder[Summary](defaultClient,
//	    graph.WithOutputNode("summary"),
//	)
func WithOutputNode(nodeID string) Option {
	return func(config *graphConfig) {
		config.outputNodeID = nodeID
	}
}

// WithStateProvider sets a custom StateProvider for graph state persistence.
// By default, an InMemoryStateProvider is used.
//
// Implement a custom StateProvider to persist state to a database, Redis,
// or other external storage. This enables resuming partially completed graphs
// after process restarts and distributed execution.
//
// Example:
//
//	graph.NewGraphBuilder[Result](defaultClient,
//	    graph.WithStateProvider(myPostgresStateProvider),
//	)
func WithStateProvider(provider StateProvider) Option {
	return func(config *graphConfig) {
		config.stateProvider = provider
	}
}

// --- Node Options ---

// WithNodeClient sets a node-specific LLM client that overrides the graph's
// default client. Use this when a node needs a different AI provider, model,
// system prompt, or tool configuration.
//
// Example:
//
//	analysisClient, _ := client.New(geminiProvider,
//	    client.WithSystemPrompt("You are a data analyst."),
//	    client.WithTools(chartTool),
//	)
//
//	builder.AddNode("analyze", analyzeExecutor,
//	    graph.WithNodeClient(analysisClient),
//	)
func WithNodeClient(nodeClient *client.Client) NodeOption {
	return func(nodeConfig *node) {
		nodeConfig.nodeClient = nodeClient
	}
}

// WithNodeTools registers additional tools on the node for use during execution.
// The tools are stored on the node and supplement those already registered on
// the node's LLM client (whether the graph default or a per-node override set
// via WithNodeClient).
//
// TODO: clarify — nodeTools are stored on the node struct but NodeInput currently
// exposes no Tools field; executor implementations must retrieve them via a
// node-level mechanism or custom client configuration.
//
// Example:
//
//	builder.AddNode("search", searchExecutor,
//	    graph.WithNodeTools(webSearchTool, calculatorTool),
//	)
func WithNodeTools(tools ...tool.GenericTool) NodeOption {
	return func(nodeConfig *node) {
		nodeConfig.nodeTools = append(nodeConfig.nodeTools, tools...)
	}
}

// WithNodeParams sets key-value parameters that are passed to the node
// during execution via NodeInput.Params. Use this to configure node-specific
// behavior without modifying the executor implementation.
//
// Example:
//
//	builder.AddNode("analyze_person", analyzeExecutor,
//	    graph.WithNodeParams(map[string]any{
//	        "entity_type": "person",
//	        "entity_id":   "user-123",
//	    }),
//	)
func WithNodeParams(params map[string]any) NodeOption {
	return func(nodeConfig *node) {
		nodeConfig.params = params
	}
}

// WithNodeTimeout sets the maximum duration for this node's execution.
// If the timeout is exceeded, the node's context is canceled and the node
// fails with a context deadline exceeded error.
//
// A value of 0 (default) means no node-specific timeout. The graph-level
// execution timeout (WithExecutionTimeout) still applies.
//
// Example:
//
//	builder.AddNode("slow_analysis", analysisExecutor,
//	    graph.WithNodeTimeout(30 * time.Second),
//	)
func WithNodeTimeout(timeout time.Duration) NodeOption {
	return func(nodeConfig *node) {
		nodeConfig.timeout = timeout
	}
}

// --- Edge Options ---

// WithEdgeCondition sets a condition function on an edge. The condition is
// evaluated after the source node completes, using its result and the current
// shared state. If the condition returns false, the edge is not traversed.
//
// A node is skipped if ALL of its incoming edges have conditions that evaluate
// to false. If at least one incoming edge condition is true (or has no condition),
// the node executes.
//
// Example:
//
//	builder.AddEdge("check", "premium_analysis",
//	    graph.WithEdgeCondition(func(ctx context.Context, result *graph.NodeResult, state graph.StateProvider) bool {
//	        score, _, _ := state.Get(ctx, "quality_score")
//	        return score.(float64) > 0.8
//	    }),
//	)
func WithEdgeCondition(condition EdgeCondition) EdgeOption {
	return func(edgeConfig *edge) {
		edgeConfig.condition = condition
	}
}
