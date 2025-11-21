# Cost Tracking Example

This example demonstrates how to track costs for both tool executions and model API calls in aigo.

## Overview

The cost tracking system allows you to:

1. **Track model API costs** based on token usage (input/output tokens)
2. **Track tool execution costs** for each tool call
3. **Inform the LLM about tool costs** so it can make cost-aware decisions
4. **Get detailed cost breakdowns** after execution

## Features

### Model Cost Tracking

Enable cost tracking by specifying the price per million tokens when creating the client:

```go
client, err := client.New(
    llmProvider,
    client.WithModelCost(cost.ModelCost{
        InputCostPerMillion:  2.50,
        OutputCostPerMillion: 10.00,
    }),
)
```

The client will automatically track costs for each API call based on token usage.

### Tool Cost Tracking

Add cost information to your tools:

```go
calculatorTool := tool.NewTool(
    "calculator",
    calculatorFunc,
    tool.WithDescription("Performs arithmetic operations"),
    tool.WithMetrics(cost.ToolMetrics{
        Amount:                  0.001,
        Currency:                "USD",
        CostDescription:         "per calculation",
        Accuracy:                0.99,
        AverageDurationInMillis: 100, // 100ms
        Quality:                 0.95,
    }),
)
```

To inform the LLM about tool costs, use `WithEnrichSystemPromptWithToolsCosts()` when creating the client.

### Advanced Model Costs

For models with cached or reasoning tokens:

```go
client, err := client.New(
    llmProvider,
    client.WithModelCost(cost.ModelCost{
        InputCostPerMillion:       2.50,
        OutputCostPerMillion:      10.00,
        CachedInputCostPerMillion: 1.25,
        ReasoningCostPerMillion:   5.00,
    }),
)
```

### Enable Tool Optimization

To inform the LLM about tool capabilities and guide its selection based on an optimization strategy:

```go
client, err := client.New(
    llmProvider,
    client.WithTools(calcTool, searchTool),
    client.WithEnrichSystemPromptWithToolsCosts(cost.OptimizeForCost),
)
```

This will:
1. Add tool descriptions and parameters to the system prompt
2. Include cost/quality metrics for each tool
3. Provide optimization guidance based on the chosen strategy

## Usage

### Basic Cost Tracking

```go
// Create client with cost tracking
aiClient, err := client.New(
    llmProvider,
    client.WithModelCost(cost.ModelCost{
        InputCostPerMillion:  2.50,
        OutputCostPerMillion: 10.00,
    }),
    client.WithTools(myTool),
)

// Execute with ReAct pattern
reactPattern, err := react.New[string](aiClient)
overview, err := reactPattern.Execute(ctx, "What is 42 * 17?")

// Get total cost
totalCost := overview.TotalCost()
fmt.Printf("Total cost: $%.6f\n", totalCost)
```

### Detailed Cost Breakdown

```go
summary := overview.CostSummary()

// Tool costs
for toolName, cost := range summary.ToolCosts {
    execCount := summary.ToolExecutionCount[toolName]
    fmt.Printf("%s: $%.6f (%d executions)\n", toolName, cost, execCount)
}

// Model costs
fmt.Printf("Model input cost: $%.6f\n", summary.ModelInputCost)
fmt.Printf("Model output cost: $%.6f\n", summary.ModelOutputCost)
fmt.Printf("Total cost: $%.6f\n", summary.TotalCost)
```

## Optimization-Aware LLM Decisions

When you enable `WithEnrichSystemPromptWithToolsCosts(strategy)` on the client, the system prompt will include comprehensive tool information:

```
## Available Tools

You have access to the following tools. Each tool has an associated cost. Minimize costs when selecting tools:

1. **calculator**
   - Description: Performs basic arithmetic operations
   - Parameters: {...}
   - Cost: 0.001000 USD (per calculation)
   - Metrics: Accuracy: 99.0%, Avg Duration: 100ms, Quality: 95.0%

2. **web_search**
   - Description: Searches the web for information
   - Parameters: {...}
   - Cost: 0.050000 USD (per search query)
   - Metrics: Accuracy: 85.0%, Avg Duration: 2500ms, Quality: 90.0%

**Optimization Goal:** When multiple tools can accomplish the same task, prefer lower-cost options.
Only use expensive tools when their unique capabilities are necessary.
```

This allows the LLM to:
- See complete tool descriptions and parameters
- View cost and quality metrics for each tool
- Make optimization-aware decisions based on the chosen strategy
- Balance between cost, accuracy, speed, and quality

## Running the Example

```bash
export OPENAI_API_KEY=your_api_key_here
cd examples/layer2/cost_tracking
go run main.go
```

## Example Output

```
=== Cost Tracking Example ===

--- Example 1: Simple Calculation ---
Final Answer: 42 multiplied by 17 equals 714.

💰 Cost Summary:
  Total Cost:       $0.000234 USD
  - Tools:          $0.001000 USD
  - Model API:      $0.000221 USD

  Token Usage:
    Input:          45 tokens
    Output:         12 tokens
    Total:          57 tokens
```

## Key Concepts

1. **Optional Cost Tracking**: Cost tracking is only enabled when you use `WithModelCost()`
2. **Tool Cost Declaration**: Tool costs are optional metadata - you can use tools without specifying costs
3. **USD Standard**: All costs are tracked in USD for consistency
4. **Per-Million Pricing**: Model costs are specified per million tokens to match provider pricing
5. **Unified Tool Enrichment**: Tool descriptions and cost/quality metrics are shown together in one section
6. **Optimization Strategies**: Choose from 6 strategies to guide the LLM's tool selection (cost, accuracy, speed, quality, balanced, cost-effective)

## Notes

- Model pricing can change, so always verify current rates with your provider
- Cost tracking adds minimal overhead to execution
- The `Overview` object contains all cost information for post-execution analysis
- Tool costs are tracked per execution, not per token or time
- Tools can have zero cost (e.g., local tools) with 100% accuracy (1.0)
- Use environment variables to configure model costs: `AIGO_MODEL_INPUT_COST_PER_MILLION` and `AIGO_MODEL_OUTPUT_COST_PER_MILLION`