# Cost Tracking

Comprehensive cost tracking system for aigo, enabling monitoring and optimization of tool executions and model API usage.

## Overview

The cost package provides:
- **Model cost tracking** based on token usage (input/output/cached/reasoning)
- **Tool execution metrics** including cost and quality metrics
- **Compute/infrastructure cost tracking** based on execution time
- **Optimization strategies** to guide LLM tool selection
- **Automatic cost calculation** in execution overview
- **Environment variable fallback** for model cost configuration

All costs are tracked in USD for consistency.

## Quick Start

### 1. Configure Model Costs

**Option A: Explicit Configuration**

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

**Option B: Environment Variables (Fallback)**

If `WithModelCost` is not specified, the client will automatically load model costs from environment variables:

```bash
export AIGO_MODEL_INPUT_COST_PER_MILLION=2.50
export AIGO_MODEL_OUTPUT_COST_PER_MILLION=10.00
export AIGO_MODEL_CACHED_COST_PER_MILLION=1.25      # Optional
export AIGO_MODEL_REASONING_COST_PER_MILLION=5.00   # Optional
export AIGO_COMPUTE_COST_PER_SECOND=0.00167         # Optional: infrastructure cost
```

### 2. Configure Compute/Infrastructure Cost (Optional)

If your application runs on infrastructure with compute costs (VMs, containers, serverless), you can track those costs:

```go
client, _ := client.New(
    provider,
    client.WithComputeCost(cost.ComputeCost{
        CostPerSecond: 0.00167, // $0.00167 per second (e.g., VM cost)
    }),
)
```

Or via environment variable:
```bash
export AIGO_COMPUTE_COST_PER_SECOND=0.00167
```

### 3. Define Tool Metrics

```go
calculatorTool := tool.NewTool(
    "calculator",
    calculatorFunc,
    tool.WithMetrics(cost.ToolMetrics{
        Amount:                  0.001,  // $0.001 per execution
        Currency:                "USD",
        CostDescription:         "per calculation", // Optional: context about the cost
        Accuracy:                0.99,   // Optional: 99% accuracy
        AverageDurationInMillis: 100,    // Optional: 100ms avg execution time
    }),
)
```

**Note:** `tool.WithCost()` is deprecated but still supported. Use `tool.WithMetrics()` instead.

### 4. Enable LLM Optimization (Optional)

```go
client, _ := client.New(
    provider,
    client.WithTools(calculatorTool, searchTool),
    client.WithEnrichSystemPromptWithToolsCosts(cost.OptimizeForCost),
)
```

### 5. Access Cost Information

```go
overview, _ := pattern.Execute(ctx, "Calculate 42 * 17")

// Total cost
totalCost := overview.TotalCost()

// Detailed breakdown
summary := overview.CostSummary()
fmt.Printf("Tools:   $%.6f\n", summary.TotalToolCost)
fmt.Printf("Model:   $%.6f\n", summary.TotalModelCost)
fmt.Printf("Compute: $%.6f (%.2fs)\n", summary.ComputeCost, summary.ExecutionDurationSeconds)
fmt.Printf("Total:   $%.6f\n", summary.TotalCost)
```

## Core Types

### ToolMetrics

Represents the cost and quality metrics of a tool execution.

```go
type ToolMetrics struct {
    Amount                  float64 // Cost per execution
    Currency                string  // Currency (default: "USD")
    CostDescription         string  // Optional: context about the cost (e.g., "per API call")
    Accuracy                float64 // Accuracy score (0.0 - 1.0)
    AverageDurationInMillis int64   // Average execution time in milliseconds
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

## Environment Variables

The client supports the following environment variables for cost configuration:

### Model Cost Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `AIGO_MODEL_INPUT_COST_PER_MILLION` | Input token cost per million | Yes* |
| `AIGO_MODEL_OUTPUT_COST_PER_MILLION` | Output token cost per million | Yes* |
| `AIGO_MODEL_CACHED_COST_PER_MILLION` | Cached token cost per million | No |
| `AIGO_MODEL_REASONING_COST_PER_MILLION` | Reasoning token cost per million | No |

*Required only if you want automatic cost tracking without explicit `WithModelCost()` configuration.

### Compute Cost Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `AIGO_COMPUTE_COST_PER_SECOND` | Infrastructure cost per second | `0.00167` |

**Practical Cost Per Second Reference:**

| Infrastructure Type | Cost/Second | Monthly (730h) | Use Case |
|--------------------|-------------|----------------|----------|
| AWS Lambda (128MB) | $0.0000021 | ~$5.50 | Serverless light |
| AWS Lambda (1GB) | $0.0000167 | ~$44 | Serverless standard |
| AWS EC2 t3.nano | $0.0000014 | ~$3.80 | Testing/dev |
| AWS EC2 t3.micro | $0.0000028 | ~$7.60 | Small workloads |
| AWS EC2 t3.small | $0.0000056 | ~$15 | Development |
| AWS EC2 t3.medium | $0.0001111 | ~$30 | Small production |
| AWS EC2 c5.large | $0.0002361 | ~$64 | Compute optimized |
| AWS EC2 c5.xlarge | $0.0004722 | ~$128 | Large production |

**Conversion helper:** Divide monthly cost by 2,628,000 (seconds in 30.4 days) to get cost per second.

**Precedence:**
1. Explicit client options (`WithModelCost()`, `WithComputeCost()`) take precedence
2. Environment variables are used as fallback
3. If neither is set, that cost component is not tracked (no errors)

## Optimization Strategies

Guide the LLM's tool selection based on different optimization goals.

### Available Strategies

| Strategy | Priority | Use When |
|----------|----------|----------|
| `OptimizeForCost` | Minimize costs | Budget constraints |
| `OptimizeForAccuracy` | Maximize accuracy | Quality is critical |
| `OptimizeForSpeed` | Minimize execution time | Speed is critical |
| `OptimizeBalanced` | Balance all metrics | No single priority |
| `OptimizeCostEffective` | Best quality/cost ratio | Value-conscious |

**Note:** Tools can have zero cost (e.g., local tools) with 100% accuracy (1.0).

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
   - Cost: 0.001000 USD (per calculation)
   - Metrics: Accuracy: 99.0%, Avg Duration: 100ms

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
    // 1. Create tools with metrics
    calcTool := tool.NewTool("calculator", calcFunc,
        tool.WithMetrics(cost.ToolMetrics{
            Amount:                  0.001,
            Currency:                "USD",
            CostDescription:         "per calculation",
            Accuracy:                0.99,
            AverageDurationInMillis: 100, // 100ms
        }))
    
    searchTool := tool.NewTool("search", searchFunc,
        tool.WithMetrics(cost.ToolMetrics{
            Amount:                  0.05,
            Currency:                "USD",
            CostDescription:         "per search query",
            Accuracy:                0.85,
            AverageDurationInMillis: 2500, // 2500ms = 2.5s
        }))
    
    // 2. Create client with cost tracking
    // Model costs can be set explicitly or via environment variables
    aiClient, _ := client.New(
        provider,
        client.WithTools(calcTool, searchTool),
        client.WithModelCost(cost.ModelCost{
            InputCostPerMillion:  2.50,
            OutputCostPerMillion: 10.00,
        }),
        client.WithComputeCost(cost.ComputeCost{
            CostPerSecond: 0.00167, // Infrastructure cost
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
    
    if summary.ComputeCost > 0 {
        fmt.Printf("\nCompute Costs:\n")
        fmt.Printf("  Duration: %.2f seconds\n", summary.ExecutionDurationSeconds)
        fmt.Printf("  Cost:     $%.6f\n", summary.ComputeCost)
    }
    
    fmt.Printf("\nTotal: $%.6f USD\n", summary.TotalCost)
}
```

## Migration from ToolCost to ToolMetrics

The type `ToolCost` has been renamed to `ToolMetrics` to better reflect that it contains multiple metrics beyond just cost information.

**What changed:**
- `cost.ToolCost` → `cost.ToolMetrics`
- `tool.WithCost()` → `tool.WithMetrics()` (WithCost still works but is deprecated)
- `tool.GetCost()` → `tool.GetMetrics()` (GetCost still works but is deprecated)
- `ai.ToolDescription.Cost` → `ai.ToolDescription.Metrics`

**Migration example:**

```go
// Old (deprecated, removed)
tool.WithCost(cost.ToolCost{
    Amount:   0.001,
    Currency: "USD",
})

// New (recommended)
tool.WithMetrics(cost.ToolMetrics{
    Amount:                  0.001,
    Currency:                "USD",
    CostDescription:         "per API call",
    Accuracy:                0.99,
    AverageDurationInMillis: 100,
})
```

## How It Works

1. **Configuration**: User specifies costs via options or environment variables:
   - `ModelCost` - token pricing
   - `ComputeCostPerMinute` - infrastructure pricing
   - `ToolMetrics` - tool execution costs
2. **Execution**: 
   - Pattern calls `StartExecution()` at beginning
   - Client sets costs in Overview
   - Tools are executed with cost tracking
   - Pattern calls `EndExecution()` at end
3. **Calculation**: `CostSummary()` calculates costs on-demand from:
   - `Overview.TotalUsage` (token counts)
   - `Overview.ModelCost` (pricing)
   - `Overview.ToolCosts` (accumulated tool costs)
   - `Overview.ExecutionDuration()` and `ComputeCostPerMinute` (compute cost)
4. **Optimization** (optional): System prompt enriched with tool info and strategy guidance

## Best Practices

1. **Verify Pricing**: Always check current provider pricing before configuring
2. **Update Metrics**: Keep tool quality metrics accurate based on real usage
3. **Choose Strategy**: Select optimization strategy based on your use case
4. **Monitor Costs**: Use `CostSummary` to analyze and optimize spending
5. **Use Environment Variables**: For deployment, prefer environment variables over hardcoded costs
6. **Track Compute Costs**: Don't forget infrastructure costs - they can add up!
   - Serverless: typically $0.000017-0.00017 per second
   - VMs: $0.00083-0.00833 per second depending on size
   - Containers: varies by platform
7. **Use ToolMetrics**: Migrate from deprecated `ToolCost` to `ToolMetrics`

## API Design

- **Struct Literals**: Use direct struct initialization (no constructors)
- **Optional Metrics**: All quality metrics are optional
- **Zero Overhead**: No cost calculation if `ModelCost` not configured
- **On-Demand**: Costs calculated when `CostSummary()` is called
- **Fallback Support**: Environment variables provide flexible configuration

## See Also

- [Complete Example](../../examples/layer2/cost_tracking)
- [Client Documentation](../client)
- [Tool Documentation](../../providers/tool)