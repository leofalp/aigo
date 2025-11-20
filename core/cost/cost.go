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

	// OptimizeForQuality prioritizes tools with higher overall quality scores.
	// Quality can be a combination of accuracy, reliability, and result richness.
	OptimizeForQuality OptimizationStrategy = "quality"

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

// ToolCost represents the cost information for a single tool execution.
// The cost can be expressed as a fixed amount per call or as a custom unit.
// It also includes optional quality metrics for optimization strategies.
//
// Example usage:
//
//	toolCost := cost.ToolCost{
//	    Amount:      0.001,
//	    Currency:    "USD",
//	    Description: "per API call",
//	    Accuracy:    0.95,  // 95% accuracy
//	    Speed:       1.2,   // 1.2 seconds average
//	}
type ToolCost struct {
	// Amount is the cost value for executing this tool once
	Amount float64 `json:"amount"`

	// Currency is the currency or unit for the cost (e.g., "USD", "EUR", "credits")
	Currency string `json:"currency,omitempty"`

	// Description provides additional context about the cost
	// (e.g., "per API call", "per search query")
	Description string `json:"description,omitempty"`

	// Accuracy represents the accuracy/reliability score (0.0 to 1.0)
	// Higher values indicate more accurate/reliable results
	Accuracy float64 `json:"accuracy,omitempty"`

	// Speed represents the average execution time in seconds
	// Lower values indicate faster execution
	Speed float64 `json:"speed,omitempty"`

	// Quality represents an overall quality score (0.0 to 1.0)
	// This can be a composite metric of various factors
	Quality float64 `json:"quality,omitempty"`
}

// String returns a formatted string representation of the cost.
func (tc ToolCost) String() string {
	currency := tc.Currency
	if currency == "" {
		currency = "USD"
	}

	result := fmt.Sprintf("%.6f %s", tc.Amount, currency)

	if tc.Description != "" {
		result = fmt.Sprintf("%s (%s)", result, tc.Description)
	}

	return result
}

// MetricsString returns a formatted string with all quality metrics.
func (tc ToolCost) MetricsString() string {
	metrics := ""

	if tc.Accuracy > 0 {
		metrics += fmt.Sprintf("Accuracy: %.1f%%", tc.Accuracy*100)
	}

	if tc.Speed > 0 {
		if metrics != "" {
			metrics += ", "
		}
		metrics += fmt.Sprintf("Speed: %.2fs", tc.Speed)
	}

	if tc.Quality > 0 {
		if metrics != "" {
			metrics += ", "
		}
		metrics += fmt.Sprintf("Quality: %.1f%%", tc.Quality*100)
	}

	return metrics
}

// CostEffectivenessScore calculates a cost-effectiveness score.
// Higher scores indicate better value (quality per unit cost).
// Returns 0 if cost is 0 to avoid division by zero.
func (tc ToolCost) CostEffectivenessScore() float64 {
	if tc.Amount == 0 {
		return 0
	}

	qualityScore := tc.Quality
	if qualityScore == 0 && tc.Accuracy > 0 {
		// Use accuracy as a fallback if quality is not set
		qualityScore = tc.Accuracy
	}

	if qualityScore == 0 {
		return 0
	}

	return qualityScore / tc.Amount
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
	InputCostPerMillion float64 `json:"input_cost_per_million"`

	// OutputCostPerMillion is the cost in USD per 1 million output tokens
	OutputCostPerMillion float64 `json:"output_cost_per_million"`

	// CachedInputCostPerMillion is the cost in USD per 1 million cached input tokens
	// Some providers offer discounted rates for cached tokens (optional)
	CachedInputCostPerMillion float64 `json:"cached_input_cost_per_million,omitempty"`

	// ReasoningCostPerMillion is the cost in USD per 1 million reasoning tokens
	// Used by models like o1/o3 that perform chain-of-thought reasoning (optional)
	ReasoningCostPerMillion float64 `json:"reasoning_cost_per_million,omitempty"`
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

// CostSummary provides a detailed breakdown of all costs during an execution.
type CostSummary struct {
	// ToolCosts maps tool names to their accumulated execution costs
	ToolCosts map[string]float64 `json:"tool_costs,omitempty"`

	// ToolExecutionCount tracks how many times each tool was called
	ToolExecutionCount map[string]int `json:"tool_execution_count,omitempty"`

	// TotalToolCost is the sum of all tool execution costs
	TotalToolCost float64 `json:"total_tool_cost"`

	// ModelInputCost is the cost from input tokens
	ModelInputCost float64 `json:"model_input_cost"`

	// ModelOutputCost is the cost from output tokens
	ModelOutputCost float64 `json:"model_output_cost"`

	// ModelCachedCost is the cost from cached tokens
	ModelCachedCost float64 `json:"model_cached_cost"`

	// ModelReasoningCost is the cost from reasoning tokens
	ModelReasoningCost float64 `json:"model_reasoning_cost"`

	// TotalModelCost is the sum of all model costs
	TotalModelCost float64 `json:"total_model_cost"`

	// TotalCost is the grand total (tools + model)
	TotalCost float64 `json:"total_cost"`

	// Currency is always "USD" for consistency
	Currency string `json:"currency"`
}
