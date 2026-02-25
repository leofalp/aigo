// Package graph implements a directed acyclic graph (DAG) pattern for orchestrating
// multi-step LLM workflows. Each node in the graph represents a distinct processing
// step that can use its own AI provider, tools, and configuration.
//
// The graph executes nodes in topological order, running independent nodes at the
// same level in parallel. Results flow from upstream nodes to downstream nodes
// via NodeInput.UpstreamResults, and a thread-safe SharedState allows cross-node
// data sharing.
//
// Graph[T] is generic over the final output type T: the last node (or a designated
// output node) produces the result, which is parsed into T using parse.ParseStringAs[T].
//
// The main entry points are [NewGraphBuilder] to construct a graph,
// [Graph.Execute] to run it synchronously, and [Graph.ExecuteStream] to run it
// with real-time event streaming. Use [NewInMemoryStateProvider] for in-process
// workflows, or implement [StateProvider] for persistent or distributed execution.
//
// Key features:
//   - Topological execution with automatic parallelism per level
//   - Per-node client and tool override (each node can use a different LLM provider)
//   - Conditional edges with EdgeCondition functions
//   - Configurable error strategy (fail-fast or continue-on-error)
//   - Graph-level and node-level timeouts
//   - Full observability integration (spans, counters, histograms)
//   - Pluggable state persistence via StateProvider interface
//   - Cost tracking aggregated across all nodes
//   - Streaming execution with multiplexed per-node events via [GraphStream]
//
// Example (synchronous):
//
//	type FinalReport struct {
//	    Summary string `json:"summary"`
//	    Score   int    `json:"score"`
//	}
//
//	g, err := graph.NewGraphBuilder[FinalReport](defaultClient).
//	    AddNode("analyze", analyzeExecutor).
//	    AddNode("summarize", summarizeExecutor).
//	    AddEdge("analyze", "summarize").
//	    Build()
//
//	result, err := g.Execute(ctx, map[string]any{"input": "data"})
//	fmt.Println(result.Data.Summary)
//
// Example (streaming):
//
//	stream, err := g.ExecuteStream(ctx, map[string]any{"input": "data"})
//	for event, err := range stream.Iter() {
//	    if err != nil { log.Fatal(err) }
//	    if event.Type == graph.GraphEventNodeContent {
//	        fmt.Printf("[%s] %s", event.NodeID, event.Content)
//	    }
//	}
//
// TODO: Future enhancements:
//   - Cycle support with WithAllowCycles() and max iterations per node
//   - SubGraph / nesting (nested graphs as nodes)
//   - ForEach / dynamic spawn (runtime node creation)
//   - Automatic retry per node
//   - Additional StateProvider implementations (PostgreSQL, Redis, etc.)
package graph
