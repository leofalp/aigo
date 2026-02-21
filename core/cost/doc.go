// Package cost defines pricing structures and optimization strategies used
// across the aigo framework to track and calculate the monetary cost of
// model inference, tool execution, and infrastructure compute.
//
// The main types are [ModelCost] for per-token LLM pricing (including tiered
// and cached-token rates), [ComputeCost] for infrastructure execution time,
// [ToolMetrics] for per-call tool cost and quality metadata, and [CostSummary]
// for an aggregated breakdown of all costs produced during a single execution.
// [OptimizationStrategy] constants guide the LLM when selecting among multiple
// available tools that differ in cost, accuracy, or speed.
package cost
