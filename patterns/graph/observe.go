package graph

import (
	"context"
	"time"

	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/observability"
)

// Semantic conventions for graph observability attributes.
// These extend the base aigo conventions with graph-specific attributes.
const (
	// spanGraphExecute is the span name for the entire graph execution.
	spanGraphExecute = "graph.execute"

	// spanGraphNodeExecute is the span name for individual node execution.
	spanGraphNodeExecute = "graph.node.execute"

	// attrGraphNodeID identifies the node within the graph.
	attrGraphNodeID = "graph.node.id"

	// attrGraphNodeLevel is the topological level of the node (0-based).
	attrGraphNodeLevel = "graph.node.level"

	// attrGraphNodeStatus is the execution status of a node.
	attrGraphNodeStatus = "graph.node.status"

	// attrGraphNodeDependencies lists the upstream node IDs.
	attrGraphNodeDependencies = "graph.node.dependencies"

	// attrGraphTotalNodes is the total number of nodes in the graph.
	attrGraphTotalNodes = "graph.total_nodes"

	// attrGraphTotalLevels is the total number of topological levels.
	attrGraphTotalLevels = "graph.total_levels"

	// attrGraphErrorStrategy is the configured error handling strategy.
	attrGraphErrorStrategy = "graph.error_strategy"

	// attrGraphOutputNode is the designated output node ID.
	attrGraphOutputNode = "graph.output_node"

	// metricGraphNodeDuration is the histogram for individual node execution duration.
	metricGraphNodeDuration = "aigo.graph.node.duration"

	// metricGraphNodeCount is the counter for node executions by status.
	metricGraphNodeCount = "aigo.graph.node.count"

	// metricGraphExecutionDuration is the histogram for total graph execution duration.
	metricGraphExecutionDuration = "aigo.graph.execution.duration"
)

// observerState holds the observability provider and the root span for the
// current graph execution. This is populated from the default client's observer
// at the start of Execute().
type observerState struct {
	// provider is the observability provider resolved from the default client.
	// Nil means observability is disabled (zero overhead).
	provider observability.Provider

	// rootSpan is the top-level span for the entire graph execution.
	rootSpan observability.Span
}

// observeGraphStart initializes observability for the graph execution.
// Creates the root span and logs the graph configuration.
// Returns the updated context with the span attached.
func (graph *Graph[T]) observeGraphStart(ctx *context.Context) {
	graph.observer.provider = graph.defaultClient.Observer()
	if graph.observer.provider == nil {
		// Try to get observer from context as a fallback.
		graph.observer.provider = observability.ObserverFromContext(*ctx)
	}

	if graph.observer.provider == nil {
		return
	}

	var rootSpan observability.Span
	*ctx, rootSpan = graph.observer.provider.StartSpan(*ctx, spanGraphExecute,
		observability.Int(attrGraphTotalNodes, len(graph.nodes)),
		observability.Int(attrGraphTotalLevels, len(graph.levels)),
		observability.String(attrGraphErrorStrategy, string(graph.config.errorStrategy)),
		observability.String(attrGraphOutputNode, graph.outputNodeID),
	)
	graph.observer.rootSpan = rootSpan

	// Attach span and observer to context for downstream propagation.
	*ctx = observability.ContextWithSpan(*ctx, rootSpan)
	*ctx = observability.ContextWithObserver(*ctx, graph.observer.provider)

	graph.observer.provider.Info(*ctx, "graph execution started",
		observability.Int(attrGraphTotalNodes, len(graph.nodes)),
		observability.Int(attrGraphTotalLevels, len(graph.levels)),
		observability.String(attrGraphErrorStrategy, string(graph.config.errorStrategy)),
	)
}

// observeGraphCompleted records the successful completion of the graph execution.
func (graph *Graph[T]) observeGraphCompleted(ctx context.Context, totalDuration time.Duration, completedAll bool) {
	if graph.observer.provider == nil {
		return
	}

	graph.observer.provider.Histogram(metricGraphExecutionDuration).Record(ctx, totalDuration.Seconds())

	status := "completed"
	if !completedAll {
		status = "partial"
	}

	graph.observer.provider.Info(ctx, "graph execution completed",
		observability.String(observability.AttrStatus, status),
		observability.Duration(observability.AttrDuration, totalDuration),
	)

	if graph.observer.rootSpan != nil {
		graph.observer.rootSpan.SetStatus(observability.StatusOK, "graph execution "+status)
		graph.observer.rootSpan.End()
	}
}

// observeGraphFailed records the failure of the graph execution.
func (graph *Graph[T]) observeGraphFailed(ctx context.Context, executionError error, totalDuration time.Duration) {
	if graph.observer.provider == nil {
		return
	}

	graph.observer.provider.Error(ctx, "graph execution failed",
		observability.Error(executionError),
		observability.Duration(observability.AttrDuration, totalDuration),
	)

	if graph.observer.rootSpan != nil {
		graph.observer.rootSpan.RecordError(executionError)
		graph.observer.rootSpan.SetStatus(observability.StatusError, "graph execution failed")
		graph.observer.rootSpan.End()
	}
}

// observeNodeStart creates a child span for a node execution and logs the start event.
// Returns the updated context with the node span attached.
func (graph *Graph[T]) observeNodeStart(ctx *context.Context, nodeID string, level int, dependencies []string) {
	if graph.observer.provider == nil {
		return
	}

	var nodeSpan observability.Span
	*ctx, nodeSpan = graph.observer.provider.StartSpan(*ctx, spanGraphNodeExecute,
		observability.String(attrGraphNodeID, nodeID),
		observability.Int(attrGraphNodeLevel, level),
		observability.StringSlice(attrGraphNodeDependencies, dependencies),
	)

	// Attach the node span to context for downstream propagation.
	*ctx = observability.ContextWithSpan(*ctx, nodeSpan)

	graph.observer.provider.Debug(*ctx, "node execution started",
		observability.String(attrGraphNodeID, nodeID),
		observability.Int(attrGraphNodeLevel, level),
	)
}

// observeNodeCompleted records the successful completion of a node and closes its span.
func (graph *Graph[T]) observeNodeCompleted(ctx context.Context, nodeID string, result *NodeResult) {
	if graph.observer.provider == nil {
		return
	}

	graph.observer.provider.Histogram(metricGraphNodeDuration).Record(ctx, result.Duration.Seconds(),
		observability.String(attrGraphNodeID, nodeID),
	)

	graph.observer.provider.Counter(metricGraphNodeCount).Add(ctx, 1,
		observability.String(attrGraphNodeStatus, string(NodeCompleted)),
		observability.String(attrGraphNodeID, nodeID),
	)

	logAttrs := []observability.Attribute{
		observability.String(attrGraphNodeID, nodeID),
		observability.String(attrGraphNodeStatus, string(NodeCompleted)),
		observability.Duration(observability.AttrDuration, result.Duration),
	}

	// Include output preview if it's a string.
	if outputStr, isString := result.Output.(string); isString {
		logAttrs = append(logAttrs,
			observability.String("graph.node.output", utils.TruncateString(outputStr, 100)),
		)
	}

	graph.observer.provider.Info(ctx, "node execution completed", logAttrs...)

	nodeSpan := observability.SpanFromContext(ctx)
	if nodeSpan != nil {
		nodeSpan.SetAttributes(
			observability.String(attrGraphNodeStatus, string(NodeCompleted)),
			observability.Duration(observability.AttrDuration, result.Duration),
		)
		nodeSpan.SetStatus(observability.StatusOK, "node completed")
		nodeSpan.End()
	}
}

// observeNodeFailed records the failure of a node and closes its span.
func (graph *Graph[T]) observeNodeFailed(ctx context.Context, nodeID string, nodeError error, duration time.Duration) {
	if graph.observer.provider == nil {
		return
	}

	graph.observer.provider.Histogram(metricGraphNodeDuration).Record(ctx, duration.Seconds(),
		observability.String(attrGraphNodeID, nodeID),
	)

	graph.observer.provider.Counter(metricGraphNodeCount).Add(ctx, 1,
		observability.String(attrGraphNodeStatus, string(NodeFailed)),
		observability.String(attrGraphNodeID, nodeID),
	)

	graph.observer.provider.Error(ctx, "node execution failed",
		observability.String(attrGraphNodeID, nodeID),
		observability.Error(nodeError),
		observability.Duration(observability.AttrDuration, duration),
	)

	nodeSpan := observability.SpanFromContext(ctx)
	if nodeSpan != nil {
		nodeSpan.RecordError(nodeError)
		nodeSpan.SetAttributes(
			observability.String(attrGraphNodeStatus, string(NodeFailed)),
			observability.Duration(observability.AttrDuration, duration),
		)
		nodeSpan.SetStatus(observability.StatusError, "node failed")
		nodeSpan.End()
	}
}

// observeNodeSkipped records that a node was skipped (due to failed dependencies
// or unsatisfied edge conditions) and increments the skip counter.
func (graph *Graph[T]) observeNodeSkipped(ctx context.Context, nodeID string, reason string) {
	if graph.observer.provider == nil {
		return
	}

	graph.observer.provider.Counter(metricGraphNodeCount).Add(ctx, 1,
		observability.String(attrGraphNodeStatus, string(NodeSkipped)),
		observability.String(attrGraphNodeID, nodeID),
	)

	graph.observer.provider.Info(ctx, "node skipped",
		observability.String(attrGraphNodeID, nodeID),
		observability.String("graph.node.skip_reason", reason),
	)
}

// observeLevelStart logs the beginning of a topological level execution,
// including the number of nodes that will execute at this level.
func (graph *Graph[T]) observeLevelStart(ctx context.Context, level int, nodeIDs []string) {
	if graph.observer.provider == nil {
		return
	}

	graph.observer.provider.Debug(ctx, "level execution started",
		observability.Int(attrGraphNodeLevel, level),
		observability.Int("graph.level.node_count", len(nodeIDs)),
		observability.StringSlice("graph.level.nodes", nodeIDs),
	)
}
