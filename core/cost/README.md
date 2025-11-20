# Cost Tracking

Comprehensive cost tracking system for aigo, enabling monitoring and optimization of tool executions and model API usage.

## Overview

The cost package provides:
- **Model cost tracking** based on token usage (input/output/cached/reasoning)
- **Tool execution cost tracking** with quality metrics
- **Optimization strategies** to guide LLM tool selection
- **Automatic cost calculation** in execution overview

All costs are tracked in USD for consistency.

## Quick Start

### 1. Configure Model Costs

```go
import "github.com/leofalp/aigo/core/cost"

client, _ := client.New(
    provider,
    client.WithModelCost(cost.ModelCost{
        InputCostPerMillion:  2.50,  // $2.50 per 1M input tokens
        OutputCostPerMillion: 10.00, // $10.00 per 1M output tokens
    }),
)
```

### 2. Define Tool Costs

```go
calculatorTool := tool.NewTool(
    "calculator",
    calculatorFunc,
    tool.WithCost(cost.ToolCost{
        Amount:   0.001,  // $0.001 per execution
        Currency: "USD",
        Accuracy: 0.99,   // Optional: 99% accuracy
        Speed:    0.1,    // Optional: 0.1s avg execution time
        Quality:  0.95,   // Optional: 95% quality score
    }),
)
```

### 3. Enable LLM Optimization (Optional)

```go
client, _ := client.New(
    provider,
    client.WithTools(calculatorTool, searchTool),
    client.WithEnrichSystemPromptWithToolsCosts(cost.OptimizeForCost),
)
```

### 4. Access Cost Information

```go
overview, _ := pattern.Execute(ctx, "Calculate 42 * 17")

// Total cost
totalCost := overview.TotalCost()

// Detailed breakdown
summary := overview.CostSummary()
fmt.Printf("Tools: $%.6f\n", summary.TotalToolCost)
fmt.Printf("Model: $%.6f\n", summary.TotalModelCost)
fmt.Printf("Total: $%.6f\n", summary.TotalCost)
```

## Core Types

### ToolCost

Represents the cost and quality metrics of a tool execution.

```go
type ToolCost struct {
    Amount      float64 // Cost per execution
    Currency    string  // Currency (default: "USD")
    Description string  // Optional description
    Accuracy    float64 // Accuracy score (0.0 - 1.0)
    Speed       float64 // Execution time in seconds
    Quality     float64 // Quality score (0.0 - 1.0)
}
```

**Methods:**
- `String()` - Formatted cost representation
- `MetricsString()` - Formatted quality metrics
- `CostEffectivenessScore()` - Quality/cost ratio

### ModelCost

Pricing structure for language models (per million tokens in USD).

```go
type ModelCost struct {
    InputCostPerMillion       float64 // Input tokens
    OutputCostPerMillion      float64 // Output tokens
    CachedInputCostPerMillion float64 // Cached tokens (optional)
    ReasoningCostPerMillion   float64 // Reasoning tokens (optional)
}
```

**Methods:**
- `CalculateInputCost(tokens int) float64`
- `CalculateOutputCost(tokens int) float64`
- `CalculateCachedCost(tokens int) float64`
- `CalculateReasoningCost(tokens int) float64`
- `CalculateTotalCost(input, output, cached, reasoning int) float64`

### CostSummary

Detailed breakdown of execution costs.

```go
type CostSummary struct {
    ToolCosts          map[string]float64 // Cost per tool
    ToolExecutionCount map[string]int     // Executions per tool
    TotalToolCost      float64            // Sum of tool costs
    ModelInputCost     float64            // Input token costs
    ModelOutputCost    float64            // Output token costs
    ModelCachedCost    float64            // Cached token costs
    ModelReasoningCost float64            // Reasoning token costs
    TotalModelCost     float64            // Sum of model costs
    TotalCost          float64            // Grand total
    Currency           string             // Always "USD"
}
```

## Optimization Strategies

Guide the LLM's tool selection based on different optimization goals.

### Available Strategies

| Strategy | Priority | Use When |
|----------|----------|----------|
| `OptimizeForCost` | Minimize costs | Budget constraints |
| `OptimizeForAccuracy` | Maximize accuracy | Quality is critical |
| `OptimizeForSpeed` | Minimize execution time | Speed is critical |
| `OptimizeForQuality` | Maximize overall quality | Best results needed |
| `OptimizeBalanced` | Balance all metrics | No single priority |
| `OptimizeCostEffective` | Best quality/cost ratio | Value-conscious |

### Example

```go
// Optimize for accuracy
client.WithEnrichSystemPromptWithToolsCosts(cost.OptimizeForAccuracy)

// Optimize for cost
client.WithEnrichSystemPromptWithToolsCosts(cost.OptimizeForCost)

// Optimize for best value
client.WithEnrichSystemPromptWithToolsCosts(cost.OptimizeCostEffective)
```

When enabled, the system prompt includes tool information with optimization guidance:

```
## Available Tools

You have access to the following tools. Prioritize accuracy when selecting tools:

1. **calculator**
   - Description: Performs arithmetic operations
   - Parameters: {...}
   - Cost: 0.001000 USD
   - Metrics: Accuracy: 99.0%, Speed: 0.10s, Quality: 95.0%

**Optimization Goal:** When multiple tools can accomplish the same task, 
prefer tools with higher accuracy scores.
```

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/leofalp/aigo/core/client"
    "github.com/leofalp/aigo/core/cost"
    "github.com/leofalp/aigo/patterns/react"
    "github.com/leofalp/aigo/providers/tool"
)

func main() {
    // 1. Create tools with costs
    calcTool := tool.NewTool("calculator", calcFunc,
        tool.WithCost(cost.ToolCost{
            Amount:   0.001,
            Currency: "USD",
            Accuracy: 0.99,
            Speed:    0.1,
            Quality:  0.95,
        }))
    
    searchTool := tool.NewTool("search", searchFunc,
        tool.WithCost(cost.ToolCost{
            Amount:   0.05,
            Currency: "USD",
            Accuracy: 0.85,
            Speed:    2.5,
            Quality:  0.90,
        }))
    
    // 2. Create client with cost tracking
    aiClient, _ := client.New(
        provider,
        client.WithTools(calcTool, searchTool),
        client.WithModelCost(cost.ModelCost{
            InputCostPerMillion:  2.50,
            OutputCostPerMillion: 10.00,
        }),
        client.WithEnrichSystemPromptWithToolsCosts(cost.OptimizeForCost),
    )
    
    // 3. Execute
    pattern, _ := react.New[string](aiClient)
    overview, _ := pattern.Execute(context.Background(), "Calculate 42 * 17")
    
    // 4. View costs
    summary := overview.CostSummary()
    
    fmt.Printf("Tool Costs:\n")
    for name, cost := range summary.ToolCosts {
        count := summary.ToolExecutionCount[name]
        fmt.Printf("  %s: $%.6f (%d calls)\n", name, cost, count)
    }
    
    fmt.Printf("\nModel Costs:\n")
    fmt.Printf("  Input:  $%.6f (%d tokens)\n", 
        summary.ModelInputCost, overview.TotalUsage.PromptTokens)
    fmt.Printf("  Output: $%.6f (%d tokens)\n", 
        summary.ModelOutputCost, overview.TotalUsage.CompletionTokens)
    
    fmt.Printf("\nTotal: $%.6f USD\n", summary.TotalCost)
}
```

## How It Works

1. **Configuration**: User specifies `ModelCost` and optional `ToolCost` with metrics
2. **Execution**: Client sets model cost in Overview; patterns track tool costs
3. **Calculation**: `CostSummary()` calculates costs on-demand from:
   - `Overview.TotalUsage` (token counts)
   - `Overview.ModelCost` (pricing)
   - `Overview.ToolCosts` (accumulated tool costs)
4. **Optimization** (optional): System prompt enriched with tool info and strategy guidance

## Best Practices

1. **Verify Pricing**: Always check current provider pricing before configuring
2. **Update Metrics**: Keep tool quality metrics accurate based on real usage
3. **Choose Strategy**: Select optimization strategy based on your use case
4. **Monitor Costs**: Use `CostSummary` to analyze and optimize spending

## API Design

- **Struct Literals**: Use direct struct initialization (no constructors)
- **Optional Metrics**: All quality metrics are optional
- **Zero Overhead**: No cost calculation if `ModelCost` not configured
- **On-Demand**: Costs calculated when `CostSummary()` is called

## See Also

- [Complete Example](../../examples/layer2/cost_tracking)
- [Optimization Strategies Guide](../../docs/optimization-strategies.md)
- [Cost Tracking Documentation](../../docs/cost-tracking.md)