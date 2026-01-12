package cost

import (
	"fmt"
)

// OptimizationStrategy defines the strategy for tool selection when multiple tools
// are available. This guides the LLM on which metrics to prioritize.
type OptimizationStrategy string

const (
	// OptimizeForCost prioritizes tools with lower execution costs.
	// Use when budget constraints are the primary concern.
	OptimizeForCost OptimizationStrategy = "cost"

	// OptimizeForAccuracy prioritizes tools with higher accuracy/reliability scores.
	// Use when result quality is more important than cost or speed.
	OptimizeForAccuracy OptimizationStrategy = "accuracy"

	// OptimizeForSpeed prioritizes tools with faster execution times.
	// Use when response time is critical.
	OptimizeForSpeed OptimizationStrategy = "speed"

	// OptimizeBalanced seeks a balance between cost, accuracy, and speed.
	// Use when no single metric dominates the decision criteria.
	OptimizeBalanced OptimizationStrategy = "balanced"

	// OptimizeCostEffective prioritizes the best quality-to-cost ratio.
	// Use when you want good results at reasonable prices.
	OptimizeCostEffective OptimizationStrategy = "cost_effective"
)

// String returns the string representation of the optimization strategy.
func (s OptimizationStrategy) String() string {
	return string(s)
}

// ToolMetrics represents the metrics and cost information for a single tool execution.
// It includes cost information, quality metrics, and performance data that can be used
// for optimization strategies.
//
// Example usage:
//
//	toolMetrics := cost.ToolMetrics{
//	    Amount:                   0.001,
//	    Currency:                 "USD",
//	    CostDescription:          "per API call",
//	    Accuracy:                 0.95,   // 95% accuracy
//	    AverageDurationInMillis:  1200,   // 1200ms = 1.2 seconds average
//	}
type ToolMetrics struct {
	// Amount is the cost value for executing this tool once
	Amount float64 `json:"amount" dynamodbav:"amount"`

	// Currency is the currency or unit for the cost (e.g., "USD", "EUR", "credits")
	Currency string `json:"currency,omitempty" dynamodbav:"currency,omitempty"`

	// CostDescription provides additional context about the cost
	// (e.g., "per API call", "per search query", "per MB processed")
	CostDescription string `json:"cost_description,omitempty" dynamodbav:"cost_description,omitempty"`

	// Accuracy represents the accuracy/reliability score (0.0 to 1.0)
	// Higher values indicate more accurate/reliable results
	Accuracy float64 `json:"accuracy,omitempty" dynamodbav:"accuracy,omitempty"`

	// AverageDurationInMillis represents the average execution time in milliseconds
	// Lower values indicate faster execution
	AverageDurationInMillis int64 `json:"average_duration_in_millis,omitempty" dynamodbav:"average_duration_in_millis,omitempty"`
}

// String returns a formatted string representation of the cost.
func (tm ToolMetrics) String() string {
	currency := tm.Currency
	if currency == "" {
		currency = "USD"
	}

	result := fmt.Sprintf("%.6f %s", tm.Amount, currency)

	if tm.CostDescription != "" {
		result = fmt.Sprintf("%s (%s)", result, tm.CostDescription)
	}

	return result
}

// MetricsString returns a formatted string with all quality metrics.
func (tm ToolMetrics) MetricsString() string {
	metrics := ""

	if tm.Accuracy > 0 {
		metrics += fmt.Sprintf("Accuracy: %.1f%%", tm.Accuracy*100)
	}

	if tm.AverageDurationInMillis > 0 {
		if metrics != "" {
			metrics += ", "
		}
		metrics += fmt.Sprintf("Avg Duration: %dms", tm.AverageDurationInMillis)
	}

	return metrics
}

// CostEffectivenessScore calculates a cost-effectiveness score.
// Higher scores indicate better value (accuracy per unit cost).
// Returns 0 if cost is 0 to avoid division by zero.
func (tm ToolMetrics) CostEffectivenessScore() float64 {
	if tm.Amount == 0 {
		return 0
	}

	if tm.Accuracy == 0 {
		return 0
	}

	return tm.Accuracy / tm.Amount
}

// ModelCost represents the pricing structure for a language model.
// Costs are expressed in USD per million tokens.
//
// Example usage:
//
//	modelCost := cost.ModelCost{
//	    InputCostPerMillion:       2.50,
//	    OutputCostPerMillion:      10.00,
//	    CachedInputCostPerMillion: 1.25,
//	    ReasoningCostPerMillion:   5.00,
//	}
type ModelCost struct {
	// InputCostPerMillion is the cost in USD per 1 million input tokens
	InputCostPerMillion float64 `json:"input_cost_per_million" dynamodbav:"input_cost_per_million"`

	// OutputCostPerMillion is the cost in USD per 1 million output tokens
	OutputCostPerMillion float64 `json:"output_cost_per_million" dynamodbav:"output_cost_per_million"`

	// CachedInputCostPerMillion is the cost in USD per 1 million cached input tokens
	// Some providers offer discounted rates for cached tokens (optional)
	CachedInputCostPerMillion float64 `json:"cached_input_cost_per_million,omitempty" dynamodbav:"cached_input_cost_per_million,omitempty"`

	// ReasoningCostPerMillion is the cost in USD per 1 million reasoning tokens
	// Used by models like o1/o3 that perform chain-of-thought reasoning (optional)
	ReasoningCostPerMillion float64 `json:"reasoning_cost_per_million,omitempty" dynamodbav:"reasoning_cost_per_million,omitempty"`
}

// CalculateInputCost calculates the cost for the given number of input tokens.
func (mc ModelCost) CalculateInputCost(tokens int) float64 {
	return (float64(tokens) / 1_000_000.0) * mc.InputCostPerMillion
}

// CalculateOutputCost calculates the cost for the given number of output tokens.
func (mc ModelCost) CalculateOutputCost(tokens int) float64 {
	return (float64(tokens) / 1_000_000.0) * mc.OutputCostPerMillion
}

// CalculateCachedCost calculates the cost for the given number of cached tokens.
func (mc ModelCost) CalculateCachedCost(tokens int) float64 {
	return (float64(tokens) / 1_000_000.0) * mc.CachedInputCostPerMillion
}

// CalculateReasoningCost calculates the cost for the given number of reasoning tokens.
func (mc ModelCost) CalculateReasoningCost(tokens int) float64 {
	return (float64(tokens) / 1_000_000.0) * mc.ReasoningCostPerMillion
}

// CalculateTotalCost calculates the total cost for all token types.
func (mc ModelCost) CalculateTotalCost(inputTokens, outputTokens, cachedTokens, reasoningTokens int) float64 {
	total := mc.CalculateInputCost(inputTokens)
	total += mc.CalculateOutputCost(outputTokens)

	if mc.CachedInputCostPerMillion > 0 && cachedTokens > 0 {
		total += mc.CalculateCachedCost(cachedTokens)
	}

	if mc.ReasoningCostPerMillion > 0 && reasoningTokens > 0 {
		total += mc.CalculateReasoningCost(reasoningTokens)
	}

	return total
}

// String returns a formatted string representation of the model costs.
func (mc ModelCost) String() string {
	return fmt.Sprintf("Input: $%.6f/M, Output: $%.6f/M",
		mc.InputCostPerMillion, mc.OutputCostPerMillion)
}

// ComputeCost represents the pricing for infrastructure/compute resources.
// Cost is expressed in USD per second of execution time.
//
// Example usage:
//
//	computeCost := cost.ComputeCost{
//	    CostPerSecond: 0.00167, // $0.10 per minute = $0.00167 per second
//	}
type ComputeCost struct {
	// CostPerSecond is the infrastructure cost in USD per second of execution
	// Examples: VM cost, container cost, serverless cost
	CostPerSecond float64 `json:"cost_per_second" dynamodbav:"cost_per_second"`
}

// CalculateCost calculates the total cost for the given execution duration in seconds.
func (cc ComputeCost) CalculateCost(durationSeconds float64) float64 {
	return durationSeconds * cc.CostPerSecond
}

// String returns a formatted string representation of the compute costs.
func (cc ComputeCost) String() string {
	return fmt.Sprintf("$%.6f/sec", cc.CostPerSecond)
}

// CostSummary provides a detailed breakdown of all costs during an execution.
type CostSummary struct {
	// ToolCosts maps tool names to their accumulated execution costs
	ToolCosts map[string]float64 `json:"tool_costs,omitempty" dynamodbav:"tool_costs,omitempty"`

	// ToolExecutionCount tracks how many times each tool was called
	ToolExecutionCount map[string]int `json:"tool_execution_count,omitempty" dynamodbav:"tool_execution_count,omitempty"`

	// TotalToolCost is the sum of all tool execution costs
	TotalToolCost float64 `json:"total_tool_cost" dynamodbav:"total_tool_cost"`

	// ModelInputCost is the cost from input tokens
	ModelInputCost float64 `json:"model_input_cost" dynamodbav:"model_input_cost"`

	// ModelOutputCost is the cost from output tokens
	ModelOutputCost float64 `json:"model_output_cost" dynamodbav:"model_output_cost"`

	// ModelCachedCost is the cost from cached tokens
	ModelCachedCost float64 `json:"model_cached_cost" dynamodbav:"model_cached_cost"`

	// ModelReasoningCost is the cost from reasoning tokens
	ModelReasoningCost float64 `json:"model_reasoning_cost" dynamodbav:"model_reasoning_cost"`

	// TotalModelCost is the sum of all model costs
	TotalModelCost float64 `json:"total_model_cost" dynamodbav:"total_model_cost"`

	// TotalCost is the grand total (tools + model + compute)
	TotalCost float64 `json:"total_cost" dynamodbav:"total_cost"`

	// ExecutionDurationSeconds is the total execution time in seconds
	ExecutionDurationSeconds float64 `json:"execution_duration_seconds,omitempty" dynamodbav:"execution_duration_seconds,omitempty"`

	// ComputeCost is the infrastructure/compute cost based on execution time
	// Calculated as: (execution_duration_minutes * compute_cost_per_minute)
	ComputeCost float64 `json:"compute_cost,omitempty" dynamodbav:"compute_cost,omitempty"`

	// Currency is always "USD" for consistency
	Currency string `json:"currency" dynamodbav:"currency"`
}
