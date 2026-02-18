package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	_ "github.com/joho/godotenv/autoload"
	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/core/overview"
	"github.com/leofalp/aigo/patterns/react"
	"github.com/leofalp/aigo/providers/ai/openai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
	"github.com/leofalp/aigo/providers/tool"
)

// CalculatorInput defines the input parameters for the calculator tool
type CalculatorInput struct {
	Operation string  `json:"operation" jsonschema:"required,enum=add|subtract|multiply|divide"`
	A         float64 `json:"a" jsonschema:"required"`
	B         float64 `json:"b" jsonschema:"required"`
}

// CalculatorOutput defines the output of the calculator tool
type CalculatorOutput struct {
	Result float64 `json:"result" jsonschema:"required"`
}

// SearchInput defines the input for the search tool
type SearchInput struct {
	Query string `json:"query" jsonschema:"required"`
}

// SearchOutput defines the output of the search tool
type SearchOutput struct {
	Results []string `json:"results" jsonschema:"required"`
}

func main() {
	ctx := context.Background()

	// Create calculator tool with cost information
	// This tool costs $0.001 per execution
	calculatorTool := tool.NewTool(
		"calculator",
		func(ctx context.Context, input CalculatorInput) (CalculatorOutput, error) {
			var result float64
			switch input.Operation {
			case "add":
				result = input.A + input.B
			case "subtract":
				result = input.A - input.B
			case "multiply":
				result = input.A * input.B
			case "divide":
				if input.B == 0 {
					return CalculatorOutput{}, fmt.Errorf("division by zero")
				}
				result = input.A / input.B
			default:
				return CalculatorOutput{}, fmt.Errorf("unknown operation: %s", input.Operation)
			}
			return CalculatorOutput{Result: result}, nil
		},
		tool.WithDescription("Performs basic arithmetic operations: add, subtract, multiply, divide"),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.001,
			Currency:                "USD",
			CostDescription:         "per calculation",
			Accuracy:                0.99,
			AverageDurationInMillis: 100, // 100ms = 0.1s
		}),
	)

	// Create search tool with different cost
	// This tool costs $0.05 per search query
	searchTool := tool.NewTool(
		"web_search",
		func(ctx context.Context, input SearchInput) (SearchOutput, error) {
			// Mock search results
			return SearchOutput{
				Results: []string{
					fmt.Sprintf("Result 1 for '%s'", input.Query),
					fmt.Sprintf("Result 2 for '%s'", input.Query),
					fmt.Sprintf("Result 3 for '%s'", input.Query),
				},
			}, nil
		},
		tool.WithDescription("Searches the web for information"),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.05,
			Currency:                "USD",
			CostDescription:         "per search query",
			Accuracy:                0.85,
			AverageDurationInMillis: 2500, // 2500ms = 2.5s
		}),
	)

	// Create client with cost tracking enabled
	// GPT-4o pricing: $2.50 per million input tokens, $10.00 per million output tokens
	// Compute cost: $0.00167 per second (~$0.10 per minute, example: cloud VM cost)
	aiClient, err := client.New(
		openai.New(),
		client.WithSystemPrompt("You are a helpful assistant with access to tools. Consider the cost of tools when deciding which ones to use."),
		client.WithMemory(inmemory.New()),
		client.WithTools(calculatorTool, searchTool),
		client.WithModelCost(cost.ModelCost{
			InputCostPerMillion:  2.50,
			OutputCostPerMillion: 10.00,
		}),
		client.WithComputeCost(cost.ComputeCost{
			CostPerSecond: 0.00167, // Infrastructure cost: ~$0.10 per minute
		}),
		client.WithEnrichSystemPromptWithToolsDescriptions(),
		client.WithEnrichSystemPromptWithToolsCosts(cost.OptimizeForCost), // Inform LLM to optimize for cost
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create ReAct pattern (returns ReAct[string] for untyped text output)
	reactPattern, err := react.New[string](
		aiClient,
		react.WithMaxIterations(5),
	)
	if err != nil {
		log.Fatalf("Failed to create ReAct pattern: %v", err)
	}

	fmt.Println("=== Cost Tracking Example ===")
	fmt.Println("\nThis example demonstrates how to track costs for both tool executions and model API calls.")
	fmt.Println("The LLM is configured to optimize for COST (prefer cheaper tools).")
	fmt.Println("\nAvailable Strategies:")
	fmt.Println("  - cost.OptimizeForCost: Minimize costs")
	fmt.Println("  - cost.OptimizeForAccuracy: Maximize accuracy")
	fmt.Println("  - cost.OptimizeForSpeed: Minimize execution time")
	fmt.Println("  - cost.OptimizeForQuality: Maximize overall quality")
	fmt.Println("  - cost.OptimizeBalanced: Balance all metrics")
	fmt.Println("  - cost.OptimizeCostEffective: Best quality-to-cost ratio")

	// Example 1: Simple calculation (low cost)
	fmt.Println("\n--- Example 1: Simple Calculation (Optimizing for Cost) ---")
	overview1, err := reactPattern.Execute(ctx, "What is 42 multiplied by 17?")
	if err != nil {
		log.Fatalf("Failed to execute: %v", err)
	}

	fmt.Printf("Final Answer: %s\n", overview1.LastResponse.Content)
	printCostSummary(&overview1.Overview)

	// Example 2: Mixed tools (calculator is cheaper than search)
	fmt.Println("\n--- Example 2: Task Requiring Multiple Tools ---")
	ctx2 := context.Background()
	overview2, err := reactPattern.Execute(ctx2, "Calculate 100 + 50 and then search for 'artificial intelligence'")
	if err != nil {
		log.Fatalf("Failed to execute: %v", err)
	}

	fmt.Printf("Final Answer: %s\n", overview2.LastResponse.Content)
	printCostSummary(&overview2.Overview)

	// Example 3: Show detailed cost breakdown
	fmt.Println("\n--- Example 3: Detailed Cost Analysis ---")
	ctx3 := context.Background()
	overview3, err := reactPattern.Execute(ctx3, "What is (25 * 4) + (100 / 5)?")
	if err != nil {
		log.Fatalf("Failed to execute: %v", err)
	}

	fmt.Printf("Final Answer: %s\n", overview3.LastResponse.Content)
	printDetailedCostBreakdown(&overview3.Overview)

	// Example 4: Show different optimization strategy
	fmt.Println("\n--- Example 4: Different Optimization Strategy (Accuracy) ---")
	fmt.Println("Creating a new client optimized for ACCURACY instead of cost...")

	// Create a new client with accuracy optimization
	aiClientAccuracy, err := client.New(
		openai.New(),
		client.WithSystemPrompt("You are a helpful assistant. Prioritize accuracy over cost."),
		client.WithMemory(inmemory.New()),
		client.WithTools(calculatorTool, searchTool),
		client.WithModelCost(cost.ModelCost{
			InputCostPerMillion:  2.50,
			OutputCostPerMillion: 10.00,
		}),
		client.WithEnrichSystemPromptWithToolsCosts(cost.OptimizeForAccuracy), // Optimize for accuracy
	)
	if err != nil {
		log.Fatalf("Failed to create accuracy client: %v", err)
	}

	reactPatternAccuracy, err := react.New[string](aiClientAccuracy) // Returns ReAct[string]
	if err != nil {
		log.Fatalf("Failed to create accuracy ReAct pattern: %v", err)
	}

	ctx4 := context.Background()
	overview4, err := reactPatternAccuracy.Execute(ctx4, "Search for the latest AI news")
	if err != nil {
		log.Fatalf("Failed to execute: %v", err)
	}

	fmt.Printf("Final Answer: %s\n", overview4.LastResponse.Content)
	fmt.Println("\nWith OptimizeForAccuracy, the LLM is guided to prefer tools with higher accuracy scores.")
	fmt.Println("The search tool (85% accuracy) vs calculator (99% accuracy) - but search is needed for this task.")
	printCostSummary(&overview4.Overview)
}

// printCostSummary prints a summary of the costs for an execution
func printCostSummary(overview *overview.Overview) {
	summary := overview.CostSummary()

	fmt.Println("\n💰 Cost Summary:")
	fmt.Printf("  Total Cost:       $%.6f USD\n", summary.TotalCost)
	fmt.Printf("  - Tools:          $%.6f USD\n", summary.TotalToolCost)
	fmt.Printf("  - Model API:      $%.6f USD\n", summary.TotalModelCost)
	if summary.ComputeCost > 0 {
		fmt.Printf("  - Compute:        $%.6f USD (%.2f seconds)\n", summary.ComputeCost, summary.ExecutionDurationSeconds)
	}
	fmt.Printf("\n  Token Usage:\n")
	fmt.Printf("    Input:          %d tokens\n", overview.TotalUsage.PromptTokens)
	fmt.Printf("    Output:         %d tokens\n", overview.TotalUsage.CompletionTokens)
	fmt.Printf("    Total:          %d tokens\n", overview.TotalUsage.TotalTokens)
}

// printDetailedCostBreakdown prints a detailed breakdown of all costs
func printDetailedCostBreakdown(overview *overview.Overview) {
	summary := overview.CostSummary()

	fmt.Println("\n💰 Detailed Cost Breakdown:")
	fmt.Println("\n📋 Tool Costs:")
	if len(summary.ToolCosts) == 0 {
		fmt.Println("  No tools used")
	} else {
		for toolName, toolCost := range summary.ToolCosts {
			execCount := summary.ToolExecutionCount[toolName]
			fmt.Printf("  - %-15s: $%.6f USD (%d executions)\n", toolName, toolCost, execCount)
		}
		fmt.Printf("  Total Tool Cost:   $%.6f USD\n", summary.TotalToolCost)
	}

	fmt.Println("\n🤖 Model API Costs:")
	fmt.Printf("  - Input tokens:    $%.6f USD (%d tokens)\n", summary.ModelInputCost, overview.TotalUsage.PromptTokens)
	fmt.Printf("  - Output tokens:   $%.6f USD (%d tokens)\n", summary.ModelOutputCost, overview.TotalUsage.CompletionTokens)
	if summary.ModelCachedCost > 0 {
		fmt.Printf("  - Cached tokens:   $%.6f USD (%d tokens)\n", summary.ModelCachedCost, overview.TotalUsage.CachedTokens)
	}
	if summary.ModelReasoningCost > 0 {
		fmt.Printf("  - Reasoning:       $%.6f USD (%d tokens)\n", summary.ModelReasoningCost, overview.TotalUsage.ReasoningTokens)
	}
	fmt.Printf("  Total Model Cost:  $%.6f USD\n", summary.TotalModelCost)

	if summary.ComputeCost > 0 {
		fmt.Println("\n⚙️  Compute/Infrastructure Costs:")
		fmt.Printf("  - Execution time:  %.2f seconds (%.2f minutes)\n", summary.ExecutionDurationSeconds, summary.ExecutionDurationSeconds/60)
		fmt.Printf("  - Compute cost:    $%.6f USD\n", summary.ComputeCost)
	}

	fmt.Println("\n💵 Grand Total:")
	fmt.Printf("  Total Execution Cost: $%.6f %s\n", summary.TotalCost, summary.Currency)

	// Print as JSON for programmatic use
	fmt.Println("\n📊 JSON Summary:")
	jsonData, err := json.MarshalIndent(summary, "  ", "  ")
	if err != nil {
		fmt.Printf("Error marshaling summary: %v\n", err)
	} else {
		fmt.Println(string(jsonData))
	}
}
