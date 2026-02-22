package graph

import (
	"errors"
	"fmt"
	"sort"

	"github.com/leofalp/aigo/core/client"
)

// GraphBuilder constructs a validated Graph[T] using a fluent API.
// Nodes and edges are added incrementally, and Build() performs structural
// validation including cycle detection via Kahn's algorithm.
//
// The builder enforces the following constraints:
//   - Node IDs must be unique
//   - Edge endpoints must reference existing nodes
//   - The graph must be acyclic (DAG)
//   - If specified, the output node must exist
//
// Example:
//
//	graph, err := graph.NewGraphBuilder[FinalReport](defaultClient).
//	    AddNode("fetch", fetchExecutor).
//	    AddNode("analyze", analyzeExecutor).
//	    AddNode("summarize", summarizeExecutor).
//	    AddEdge("fetch", "analyze").
//	    AddEdge("analyze", "summarize").
//	    Build()
type GraphBuilder[T any] struct {
	// defaultClient is the LLM client used by nodes without a node-specific override.
	defaultClient *client.Client

	// config holds the graph-level configuration populated from Options.
	config *graphConfig

	// nodes stores all registered nodes keyed by their ID.
	nodes map[string]*node

	// edges stores all registered directed edges.
	edges []*edge

	// nodeOrder preserves the insertion order of nodes for deterministic output
	// when no topological constraints exist.
	nodeOrder []string

	// buildErrors accumulates validation errors encountered during AddNode/AddEdge
	// and is reported when Build() is called.
	buildErrors []error
}

// NewGraphBuilder creates a new GraphBuilder for constructing a Graph[T].
// The defaultClient is the LLM client used by all nodes that do not have
// a node-specific client override.
//
// Graph-level options (WithMaxConcurrency, WithExecutionTimeout, etc.) are
// applied here. Node and edge options are applied via AddNode and AddEdge.
//
// Example:
//
//	builder := graph.NewGraphBuilder[MyOutput](defaultClient,
//	    graph.WithMaxConcurrency(5),
//	    graph.WithExecutionTimeout(10 * time.Minute),
//	)
func NewGraphBuilder[T any](defaultClient *client.Client, opts ...Option) *GraphBuilder[T] {
	config := &graphConfig{
		errorStrategy: ErrorStrategyFailFast,
	}

	for _, opt := range opts {
		opt(config)
	}

	return &GraphBuilder[T]{
		defaultClient: defaultClient,
		config:        config,
		nodes:         make(map[string]*node),
		edges:         make([]*edge, 0),
		nodeOrder:     make([]string, 0),
		buildErrors:   make([]error, 0),
	}
}

// AddNode registers a processing node in the graph with the given unique ID
// and executor. Node options (WithNodeClient, WithNodeTools, WithNodeParams,
// WithNodeTimeout) can customize individual node behavior.
//
// Returns the builder for method chaining. If a node with the same ID already
// exists, a build error is recorded and reported at Build() time.
//
// Example:
//
//	builder.AddNode("analyze", analyzeExecutor,
//	    graph.WithNodeClient(customClient),
//	    graph.WithNodeTimeout(30 * time.Second),
//	)
func (builder *GraphBuilder[T]) AddNode(nodeID string, executor NodeExecutor, opts ...NodeOption) *GraphBuilder[T] {
	if nodeID == "" {
		builder.buildErrors = append(builder.buildErrors, fmt.Errorf("node ID must not be empty"))
		return builder
	}

	if executor == nil {
		builder.buildErrors = append(builder.buildErrors, fmt.Errorf("executor must not be nil for node %q", nodeID))
		return builder
	}

	if _, exists := builder.nodes[nodeID]; exists {
		builder.buildErrors = append(builder.buildErrors, fmt.Errorf("duplicate node ID %q", nodeID))
		return builder
	}

	graphNode := &node{
		id:       nodeID,
		executor: executor,
	}

	for _, opt := range opts {
		opt(graphNode)
	}

	builder.nodes[nodeID] = graphNode
	builder.nodeOrder = append(builder.nodeOrder, nodeID)

	return builder
}

// AddEdge creates a directed edge from one node to another, indicating that
// the source node must complete before the target node can execute.
//
// Edge options (WithEdgeCondition) can make the edge conditional.
//
// Returns the builder for method chaining. If either endpoint does not exist,
// a build error is recorded and reported at Build() time.
//
// Example:
//
//	builder.AddEdge("fetch", "analyze")
//	builder.AddEdge("check", "premium_path",
//	    graph.WithEdgeCondition(isPremiumUser),
//	)
func (builder *GraphBuilder[T]) AddEdge(from, to string, opts ...EdgeOption) *GraphBuilder[T] {
	if from == "" || to == "" {
		builder.buildErrors = append(builder.buildErrors, fmt.Errorf("edge endpoints must not be empty (from=%q, to=%q)", from, to))
		return builder
	}

	if from == to {
		builder.buildErrors = append(builder.buildErrors, fmt.Errorf("self-loop detected: node %q cannot have an edge to itself", from))
		return builder
	}

	graphEdge := &edge{
		from: from,
		to:   to,
	}

	for _, opt := range opts {
		opt(graphEdge)
	}

	builder.edges = append(builder.edges, graphEdge)

	return builder
}

// Build validates the graph structure and produces an executable Graph[T].
// It performs the following validations:
//
//  1. No accumulated build errors from AddNode/AddEdge
//  2. At least one node exists
//  3. All edge endpoints reference existing nodes
//  4. No duplicate edges
//  5. The graph is acyclic (validated via Kahn's algorithm)
//  6. If specified, the output node exists
//
// On success, it computes the topological ordering and level assignment.
// On failure, it returns a descriptive error.
func (builder *GraphBuilder[T]) Build() (*Graph[T], error) {
	// Report any errors accumulated during AddNode/AddEdge.
	if len(builder.buildErrors) > 0 {
		return nil, fmt.Errorf("graph build errors: %w", errors.Join(builder.buildErrors...))
	}

	if len(builder.nodes) == 0 {
		return nil, fmt.Errorf("graph must contain at least one node")
	}

	// Validate all edge endpoints reference existing nodes and check for duplicates.
	if err := builder.validateEdges(); err != nil {
		return nil, err
	}

	// Compute in-degree map and adjacency list for Kahn's algorithm.
	inDegree, adjacency := builder.buildAdjacency()

	// Run Kahn's algorithm for topological sort and cycle detection.
	topologicalOrder, levels, err := kahnTopologicalSort(inDegree, adjacency, builder.nodeOrder)
	if err != nil {
		return nil, err
	}

	// Populate node dependencies from edges.
	builder.populateDependencies()

	// Determine the output node.
	outputNodeID, err := builder.resolveOutputNode(topologicalOrder)
	if err != nil {
		return nil, err
	}

	// Use default InMemoryStateProvider if none was configured.
	if builder.config.stateProvider == nil {
		builder.config.stateProvider = NewInMemoryStateProvider(nil)
	}

	return &Graph[T]{
		defaultClient:    builder.defaultClient,
		nodes:            builder.nodes,
		edges:            builder.edges,
		levels:           levels,
		topologicalOrder: topologicalOrder,
		outputNodeID:     outputNodeID,
		config:           builder.config,
	}, nil
}

// validateEdges checks that all edge endpoints reference existing nodes
// and that there are no duplicate edges.
func (builder *GraphBuilder[T]) validateEdges() error {
	edgeSet := make(map[string]bool)

	for _, graphEdge := range builder.edges {
		if _, exists := builder.nodes[graphEdge.from]; !exists {
			return fmt.Errorf("edge references non-existent source node %q", graphEdge.from)
		}
		if _, exists := builder.nodes[graphEdge.to]; !exists {
			return fmt.Errorf("edge references non-existent target node %q", graphEdge.to)
		}

		edgeKey := graphEdge.from + "->" + graphEdge.to
		if edgeSet[edgeKey] {
			return fmt.Errorf("duplicate edge from %q to %q", graphEdge.from, graphEdge.to)
		}
		edgeSet[edgeKey] = true
	}

	return nil
}

// buildAdjacency constructs the in-degree map and adjacency list from the
// registered nodes and edges. Every node starts with in-degree 0.
func (builder *GraphBuilder[T]) buildAdjacency() (map[string]int, map[string][]string) {
	inDegree := make(map[string]int, len(builder.nodes))
	adjacency := make(map[string][]string, len(builder.nodes))

	// Initialize all nodes with in-degree 0.
	for nodeID := range builder.nodes {
		inDegree[nodeID] = 0
		adjacency[nodeID] = make([]string, 0)
	}

	// Populate from edges.
	for _, graphEdge := range builder.edges {
		adjacency[graphEdge.from] = append(adjacency[graphEdge.from], graphEdge.to)
		inDegree[graphEdge.to]++
	}

	return inDegree, adjacency
}

// populateDependencies populates each node's dependencies list from the edges.
// A node's dependencies are all nodes that have an edge pointing to it.
func (builder *GraphBuilder[T]) populateDependencies() {
	for _, graphEdge := range builder.edges {
		targetNode := builder.nodes[graphEdge.to]
		targetNode.dependencies = append(targetNode.dependencies, graphEdge.from)
	}
}

// resolveOutputNode determines which node produces the final output T.
// If WithOutputNode was used, validates that the specified node exists.
// Otherwise, uses the last node in topological order.
func (builder *GraphBuilder[T]) resolveOutputNode(topologicalOrder []string) (string, error) {
	if builder.config.outputNodeID != "" {
		if _, exists := builder.nodes[builder.config.outputNodeID]; !exists {
			return "", fmt.Errorf("output node %q does not exist in the graph", builder.config.outputNodeID)
		}
		return builder.config.outputNodeID, nil
	}

	// Default: last node in topological order.
	return topologicalOrder[len(topologicalOrder)-1], nil
}

// kahnTopologicalSort performs Kahn's algorithm for topological sorting.
// It simultaneously detects cycles and computes topological levels.
//
// Returns:
//   - topologicalOrder: all node IDs sorted in topological order
//   - levels: node IDs grouped by their topological level (level 0 = roots)
//   - error: if a cycle is detected
//
// Within each level, nodes are sorted by their insertion order (using nodeOrder)
// to ensure deterministic output.
func kahnTopologicalSort(inDegree map[string]int, adjacency map[string][]string, nodeOrder []string) ([]string, [][]string, error) {
	// Build a position map for deterministic ordering within levels.
	nodePosition := make(map[string]int, len(nodeOrder))
	for index, nodeID := range nodeOrder {
		nodePosition[nodeID] = index
	}

	// Find all root nodes (in-degree 0) as the initial frontier.
	currentLevel := make([]string, 0)
	for nodeID, degree := range inDegree {
		if degree == 0 {
			currentLevel = append(currentLevel, nodeID)
		}
	}

	// Sort root nodes by insertion order for determinism.
	sort.Slice(currentLevel, func(nodeIndexA, nodeIndexB int) bool {
		return nodePosition[currentLevel[nodeIndexA]] < nodePosition[currentLevel[nodeIndexB]]
	})

	topologicalOrder := make([]string, 0, len(inDegree))
	levels := make([][]string, 0)
	processedCount := 0

	// Process level by level.
	for len(currentLevel) > 0 {
		levels = append(levels, currentLevel)
		topologicalOrder = append(topologicalOrder, currentLevel...)
		processedCount += len(currentLevel)

		nextLevel := make([]string, 0)

		for _, nodeID := range currentLevel {
			for _, neighbor := range adjacency[nodeID] {
				inDegree[neighbor]--
				if inDegree[neighbor] == 0 {
					nextLevel = append(nextLevel, neighbor)
				}
			}
		}

		// Sort next level by insertion order for determinism.
		sort.Slice(nextLevel, func(nodeIndexA, nodeIndexB int) bool {
			return nodePosition[nextLevel[nodeIndexA]] < nodePosition[nextLevel[nodeIndexB]]
		})

		currentLevel = nextLevel
	}

	// If we didn't process all nodes, there is a cycle.
	if processedCount != len(inDegree) {
		// Identify the nodes involved in the cycle for a helpful error message.
		cycleNodes := make([]string, 0)
		for nodeID, degree := range inDegree {
			if degree > 0 {
				cycleNodes = append(cycleNodes, nodeID)
			}
		}
		sort.Strings(cycleNodes)
		return nil, nil, fmt.Errorf("cycle detected in graph involving nodes: %v", cycleNodes)
	}

	return topologicalOrder, levels, nil
}
