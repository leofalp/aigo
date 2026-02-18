package graph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/leofalp/aigo/core/overview"
	"github.com/leofalp/aigo/core/parse"
)

// Execute runs the graph by executing nodes in topological order, with nodes at
// the same level running in parallel (subject to maxConcurrency).
//
// The execution proceeds as follows:
//  1. Initialize state provider with initialState and set all nodes to NodePending
//  2. Create an Overview in the context for cost tracking
//  3. Start observability root span
//  4. For each topological level, launch ready nodes as goroutines
//  5. Collect results, handle errors per the configured strategy
//  6. Parse the output node's result as type T
//  7. Return StructuredOverview[T] with the parsed result and execution statistics
//
// The initialState map is loaded into the StateProvider's shared state before
// execution begins. Nodes can read and write shared state during execution.
//
// Execute is NOT safe for concurrent use on the same Graph instance. Create
// separate Graph instances for concurrent workflows.
func (graph *Graph[T]) Execute(ctx context.Context, initialState map[string]any) (*overview.StructuredOverview[T], error) {
	executionStart := time.Now()

	// Initialize the Overview for cost/usage tracking.
	executionOverview := overview.OverviewFromContext(&ctx)
	executionOverview.StartExecution()

	// Start observability.
	graph.observeGraphStart(&ctx)

	// Initialize state provider.
	stateProvider := graph.config.stateProvider
	if err := graph.initializeState(ctx, stateProvider, initialState); err != nil {
		executionOverview.EndExecution()
		graph.observeGraphFailed(ctx, err, time.Since(executionStart))
		return nil, fmt.Errorf("failed to initialize graph state: %w", err)
	}

	// Apply graph-level execution timeout if configured.
	if graph.config.executionTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, graph.config.executionTimeout)
		defer cancel()
	}

	// Execute level by level.
	executionError := graph.executeLevels(ctx, stateProvider)

	executionOverview.EndExecution()
	totalDuration := time.Since(executionStart)

	if executionError != nil {
		graph.observeGraphFailed(ctx, executionError, totalDuration)
		return nil, fmt.Errorf("graph execution failed: %w", executionError)
	}

	// Parse the output node result as T.
	parsedResult, parseError := graph.parseOutputResult(ctx, stateProvider)
	if parseError != nil {
		graph.observeGraphFailed(ctx, parseError, totalDuration)
		return nil, fmt.Errorf("failed to parse output from node %q: %w", graph.outputNodeID, parseError)
	}

	// Determine whether all nodes completed successfully.
	completedAll := graph.allNodesCompleted(ctx, stateProvider)
	graph.observeGraphCompleted(ctx, totalDuration, completedAll)

	return &overview.StructuredOverview[T]{
		Overview: *executionOverview,
		Data:     parsedResult,
	}, nil
}

// Reset clears the graph's execution state, allowing it to be re-executed.
// All node statuses are reset to NodePending and node results are cleared.
// Shared state is preserved unless a new initialState is provided.
//
// This is useful for re-running a graph with different initial state without
// rebuilding it.
func (graph *Graph[T]) Reset(ctx context.Context, initialState map[string]any) error {
	return graph.initializeState(ctx, graph.config.stateProvider, initialState)
}

// initializeState prepares the state provider for a new execution run.
// It loads initial state and sets all nodes to NodePending.
func (graph *Graph[T]) initializeState(ctx context.Context, stateProvider StateProvider, initialState map[string]any) error {
	// Load initial shared state.
	for key, value := range initialState {
		if err := stateProvider.Set(ctx, key, value); err != nil {
			return fmt.Errorf("failed to set initial state key %q: %w", key, err)
		}
	}

	// Initialize all node statuses to pending.
	nodeIDs := make([]string, 0, len(graph.nodes))
	for nodeID := range graph.nodes {
		nodeIDs = append(nodeIDs, nodeID)
	}

	// Use the InMemoryStateProvider's optimized batch method if available.
	if inMemoryProvider, isInMemory := stateProvider.(*InMemoryStateProvider); isInMemory {
		inMemoryProvider.initializeNodes(nodeIDs)
	} else {
		for _, nodeID := range nodeIDs {
			if err := stateProvider.SetNodeStatus(ctx, nodeID, NodePending); err != nil {
				return fmt.Errorf("failed to initialize node %q status: %w", nodeID, err)
			}
		}
	}

	return nil
}

// executeLevels iterates through topological levels and executes nodes at each level
// in parallel, respecting maxConcurrency and error strategy.
func (graph *Graph[T]) executeLevels(ctx context.Context, stateProvider StateProvider) error {
	for levelIndex, levelNodeIDs := range graph.levels {
		// Check context cancellation before starting a new level.
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context canceled before level %d: %w", levelIndex, err)
		}

		graph.observeLevelStart(ctx, levelIndex, levelNodeIDs)

		// Filter nodes that are ready to execute (all dependencies satisfied).
		readyNodes := graph.filterReadyNodes(ctx, levelNodeIDs, stateProvider)

		if len(readyNodes) == 0 {
			continue
		}

		// Execute all ready nodes at this level in parallel.
		if err := graph.executeLevel(ctx, readyNodes, levelIndex, stateProvider); err != nil {
			return err
		}
	}

	return nil
}

// filterReadyNodes determines which nodes at a given level should execute.
// A node is ready if all its dependencies completed successfully and all
// incoming edge conditions are satisfied (at least one unconditional or true).
func (graph *Graph[T]) filterReadyNodes(ctx context.Context, nodeIDs []string, stateProvider StateProvider) []string {
	readyNodes := make([]string, 0, len(nodeIDs))

	for _, nodeID := range nodeIDs {
		graphNode := graph.nodes[nodeID]

		// Check if all dependencies completed.
		allDependenciesMet := true
		anyDependencyFailed := false

		for _, depID := range graphNode.dependencies {
			depStatus, err := stateProvider.GetNodeStatus(ctx, depID)
			if err != nil {
				allDependenciesMet = false
				break
			}

			switch depStatus {
			case NodeCompleted:
				// Dependency satisfied.
			case NodeFailed:
				anyDependencyFailed = true
			case NodeSkipped:
				anyDependencyFailed = true
			default:
				allDependenciesMet = false
			}
		}

		if !allDependenciesMet {
			continue
		}

		// If any dependency failed, skip this node.
		if anyDependencyFailed {
			if setErr := stateProvider.SetNodeStatus(ctx, nodeID, NodeSkipped); setErr != nil {
				continue
			}
			graph.observeNodeSkipped(ctx, nodeID, "upstream dependency failed or skipped")
			continue
		}

		// Evaluate edge conditions for this node.
		if !graph.evaluateEdgeConditions(ctx, nodeID, stateProvider) {
			if setErr := stateProvider.SetNodeStatus(ctx, nodeID, NodeSkipped); setErr != nil {
				continue
			}
			graph.observeNodeSkipped(ctx, nodeID, "edge conditions not satisfied")
			continue
		}

		readyNodes = append(readyNodes, nodeID)
	}

	return readyNodes
}

// evaluateEdgeConditions checks whether a node should execute based on its
// incoming edge conditions. A node executes if at least one incoming edge
// has no condition or its condition returns true.
//
// If the node has no incoming edges (root node), it always executes.
func (graph *Graph[T]) evaluateEdgeConditions(ctx context.Context, nodeID string, stateProvider StateProvider) bool {
	// Find all incoming edges for this node.
	incomingEdges := make([]*edge, 0)
	for _, graphEdge := range graph.edges {
		if graphEdge.to == nodeID {
			incomingEdges = append(incomingEdges, graphEdge)
		}
	}

	// Root nodes (no incoming edges) always execute.
	if len(incomingEdges) == 0 {
		return true
	}

	// A node executes if at least one incoming edge is active (no condition or condition is true).
	for _, graphEdge := range incomingEdges {
		if graphEdge.condition == nil {
			return true
		}

		// Evaluate the condition with the source node's result.
		sourceResult, getErr := stateProvider.GetNodeResult(ctx, graphEdge.from)
		if getErr != nil {
			continue
		}
		if graphEdge.condition(sourceResult, stateProvider) {
			return true
		}
	}

	return false
}

// executeLevel runs all ready nodes at a topological level in parallel.
// It respects the maxConcurrency limit and handles errors according to
// the configured error strategy.
func (graph *Graph[T]) executeLevel(ctx context.Context, readyNodes []string, levelIndex int, stateProvider StateProvider) error {
	var waitGroup sync.WaitGroup
	errorChannel := make(chan nodeExecutionError, len(readyNodes))

	// Create a cancellable context for fail-fast behavior.
	levelContext, cancelLevel := context.WithCancel(ctx)
	defer cancelLevel()

	// Set up concurrency semaphore if maxConcurrency is configured.
	var semaphore chan struct{}
	if graph.config.maxConcurrency > 0 {
		semaphore = make(chan struct{}, graph.config.maxConcurrency)
	}

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

			err := graph.executeNode(levelContext, executingNodeID, levelIndex, stateProvider)
			if err != nil {
				errorChannel <- nodeExecutionError{nodeID: executingNodeID, err: err}

				// For fail-fast, cancel all other nodes at this level.
				if graph.config.errorStrategy == ErrorStrategyFailFast {
					cancelLevel()
				}
			}
		}(nodeID)
	}

	// Wait for all goroutines to finish.
	waitGroup.Wait()
	close(errorChannel)

	// Collect errors.
	var executionErrors []nodeExecutionError
	for nodeError := range errorChannel {
		executionErrors = append(executionErrors, nodeError)
	}

	if len(executionErrors) == 0 {
		return nil
	}

	// For fail-fast, return the first error.
	if graph.config.errorStrategy == ErrorStrategyFailFast {
		firstError := executionErrors[0]
		return fmt.Errorf("node %q failed: %w", firstError.nodeID, firstError.err)
	}

	// For continue-on-error, errors are recorded in state but don't stop execution.
	// The graph continues to the next level; downstream nodes of failed nodes
	// will be skipped by filterReadyNodes.
	return nil
}

// nodeExecutionError pairs a node ID with its execution error for error collection.
type nodeExecutionError struct {
	nodeID string
	err    error
}

// executeNode runs a single node's executor with proper context, timeout,
// state management, and observability.
func (graph *Graph[T]) executeNode(ctx context.Context, nodeID string, levelIndex int, stateProvider StateProvider) error {
	graphNode := graph.nodes[nodeID]

	// Mark node as running.
	if err := stateProvider.SetNodeStatus(ctx, nodeID, NodeRunning); err != nil {
		return fmt.Errorf("failed to set node %q status to running: %w", nodeID, err)
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
		return fmt.Errorf("failed to assemble input for node %q: %w", nodeID, err)
	}

	// Execute the node.
	nodeStart := time.Now()
	result, execError := graphNode.executor.Execute(nodeContext, nodeInput)
	executionDuration := time.Since(nodeStart)

	if execError != nil {
		markNodeFailed(nodeContext, stateProvider, nodeID, execError, executionDuration)
		graph.observeNodeFailed(nodeContext, nodeID, execError, executionDuration)
		return fmt.Errorf("node %q execution failed: %w", nodeID, execError)
	}

	// Ensure result is not nil.
	if result == nil {
		result = &NodeResult{}
	}

	result.Duration = executionDuration

	// Store result and mark completed.
	if err := stateProvider.SetNodeResult(nodeContext, nodeID, result); err != nil {
		return fmt.Errorf("failed to store result for node %q: %w", nodeID, err)
	}

	if err := stateProvider.SetNodeStatus(nodeContext, nodeID, NodeCompleted); err != nil {
		return fmt.Errorf("failed to set node %q status to completed: %w", nodeID, err)
	}

	graph.observeNodeCompleted(nodeContext, nodeID, result)

	return nil
}

// assembleNodeInput creates the NodeInput struct for a node, gathering upstream
// results from the state provider and selecting the appropriate client.
func (graph *Graph[T]) assembleNodeInput(ctx context.Context, graphNode *node, stateProvider StateProvider) (*NodeInput, error) {
	// Gather upstream results.
	upstreamResults := make(map[string]*NodeResult)
	for _, depID := range graphNode.dependencies {
		result, err := stateProvider.GetNodeResult(ctx, depID)
		if err != nil {
			return nil, fmt.Errorf("failed to get result for upstream node %q: %w", depID, err)
		}
		if result != nil {
			upstreamResults[depID] = result
		}
	}

	// Select client: node-specific or graph default.
	nodeClient := graph.defaultClient
	if graphNode.nodeClient != nil {
		nodeClient = graphNode.nodeClient
	}

	return &NodeInput{
		UpstreamResults: upstreamResults,
		SharedState:     stateProvider,
		Params:          graphNode.params,
		Client:          nodeClient,
	}, nil
}

// parseOutputResult extracts the output node's result and parses it as type T.
// It first attempts a direct type assertion, then falls back to
// parse.ParseStringAs[T] for string outputs containing JSON.
func (graph *Graph[T]) parseOutputResult(ctx context.Context, stateProvider StateProvider) (*T, error) {
	outputResult, err := stateProvider.GetNodeResult(ctx, graph.outputNodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get output node result: %w", err)
	}

	if outputResult == nil {
		return nil, fmt.Errorf("output node %q has no result", graph.outputNodeID)
	}

	if outputResult.Error != nil {
		return nil, fmt.Errorf("output node %q failed: %w", graph.outputNodeID, outputResult.Error)
	}

	// Try direct type assertion first.
	if typedResult, isTargetType := outputResult.Output.(*T); isTargetType {
		return typedResult, nil
	}

	if typedResult, isTargetType := outputResult.Output.(T); isTargetType {
		return &typedResult, nil
	}

	// Fall back to string-based parsing via parse.ParseStringAs[T].
	outputString, isString := outputResult.Output.(string)
	if !isString {
		return nil, fmt.Errorf("output node %q produced non-string, non-%T output of type %T", graph.outputNodeID, *new(T), outputResult.Output)
	}

	parsedResult, parseError := parse.ParseStringAs[T](outputString)
	if parseError != nil {
		return nil, fmt.Errorf("failed to parse output as %T: %w", *new(T), parseError)
	}

	return &parsedResult, nil
}

// allNodesCompleted checks whether every node in the graph has status NodeCompleted.
func (graph *Graph[T]) allNodesCompleted(ctx context.Context, stateProvider StateProvider) bool {
	for nodeID := range graph.nodes {
		status, err := stateProvider.GetNodeStatus(ctx, nodeID)
		if err != nil || status != NodeCompleted {
			return false
		}
	}
	return true
}

// markNodeFailed is a best-effort helper that sets a node's status to NodeFailed
// and stores the failure result. Errors from the state provider are intentionally
// ignored because the primary execution error takes precedence.
func markNodeFailed(ctx context.Context, stateProvider StateProvider, nodeID string, nodeError error, duration time.Duration) {
	// Best-effort: errors here are secondary to the primary execution error.
	_ = stateProvider.SetNodeStatus(ctx, nodeID, NodeFailed)  //nolint:errcheck
	_ = stateProvider.SetNodeResult(ctx, nodeID, &NodeResult{ //nolint:errcheck
		Error:    nodeError,
		Duration: duration,
	})
}
