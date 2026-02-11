package patterns

import (
	"context"
	"time"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/providers/ai"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

// overviewContextKey is the key used to store Overview in context.
const overviewContextKey contextKey = "overview"

type Overview struct {
	LastResponse  *ai.ChatResponse   `json:"last_response,omitempty"`
	Requests      []*ai.ChatRequest  `json:"requests"`
	Responses     []*ai.ChatResponse `json:"responses"`
	TotalUsage    ai.Usage           `json:"total_usage"`
	ToolCallStats map[string]int     `json:"tool_calls,omitempty"`
	// ToolCosts tracks the accumulated cost per tool
	ToolCosts map[string]float64 `json:"tool_costs,omitempty"`
	// ModelCost is the pricing configuration for the model (optional)
	ModelCost *cost.ModelCost `json:"model_cost,omitempty"`

	// ExecutionStartTime marks when the execution started
	ExecutionStartTime time.Time `json:"execution_start_time,omitempty"`
	// ExecutionEndTime marks when the execution ended
	ExecutionEndTime time.Time `json:"execution_end_time,omitempty"`
	// ComputeCost is the infrastructure/compute pricing configuration (optional)
	// Examples: AWS Lambda, VM cost, container runtime cost
	ComputeCost *cost.ComputeCost `json:"compute_cost,omitempty"`
}

// StructuredOverview extends Overview with parsed structured data from the final response.
// This is used by structured patterns (e.g., StructuredPattern[T]) to provide both
// execution statistics and the parsed final result.
type StructuredOverview[T any] struct {
	Overview
	Data *T // Parsed final response data
}

func OverviewFromContext(ctx *context.Context) *Overview {
	overviewVal := (*ctx).Value(overviewContextKey)
	if overviewVal == nil {
		overview := &Overview{
			ToolCosts: make(map[string]float64),
		}
		*ctx = overview.ToContext(*ctx)
		return overview
	}

	overview, ok := overviewVal.(*Overview)
	if !ok {
		return nil
	}
	return overview
}

func (o *Overview) ToContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, overviewContextKey, o)
}

func (o *Overview) IncludeUsage(usage *ai.Usage) {
	if usage == nil {
		return
	}
	o.TotalUsage.PromptTokens += usage.PromptTokens
	o.TotalUsage.CompletionTokens += usage.CompletionTokens
	o.TotalUsage.TotalTokens += usage.TotalTokens
	o.TotalUsage.ReasoningTokens += usage.ReasoningTokens
	o.TotalUsage.CachedTokens += usage.CachedTokens
}

func (o *Overview) AddToolCalls(tools []ai.ToolCall) {
	if o.ToolCallStats == nil {
		o.ToolCallStats = make(map[string]int)
	}

	for _, tool := range tools {
		o.ToolCallStats[tool.Function.Name]++
	}
}

func (o *Overview) AddRequest(request *ai.ChatRequest) {
	o.Requests = append(o.Requests, request)
}

func (o *Overview) AddResponse(response *ai.ChatResponse) {
	o.Responses = append(o.Responses, response)
	o.LastResponse = response
}

// AddToolExecutionCost records the cost of a tool execution.
func (o *Overview) AddToolExecutionCost(toolName string, toolMetrics *cost.ToolMetrics) {
	if o.ToolCosts == nil {
		o.ToolCosts = make(map[string]float64)
	}
	if toolMetrics != nil {
		o.ToolCosts[toolName] += toolMetrics.Amount
	}
}

// SetModelCost sets the model cost configuration for this overview.
func (o *Overview) SetModelCost(modelCost *cost.ModelCost) {
	o.ModelCost = modelCost
}

// SetComputeCost sets the compute/infrastructure cost configuration.
// This is used to calculate the cost of running the execution environment.
func (o *Overview) SetComputeCost(computeCost *cost.ComputeCost) {
	o.ComputeCost = computeCost
}

// StartExecution marks the start of execution for compute cost tracking.
func (o *Overview) StartExecution() {
	o.ExecutionStartTime = time.Now()
}

// EndExecution marks the end of execution for compute cost tracking.
func (o *Overview) EndExecution() {
	o.ExecutionEndTime = time.Now()
}

// ExecutionDuration returns the total execution duration.
// Returns 0 if execution hasn't started or ended.
func (o *Overview) ExecutionDuration() time.Duration {
	if o.ExecutionStartTime.IsZero() || o.ExecutionEndTime.IsZero() {
		return 0
	}
	return o.ExecutionEndTime.Sub(o.ExecutionStartTime)
}

// TotalCost returns the total cost of the execution (tools + model).
func (o *Overview) TotalCost() float64 {
	summary := o.CostSummary()
	return summary.TotalCost
}

// CostSummary returns a detailed breakdown of all costs.
func (o *Overview) CostSummary() cost.CostSummary {
	summary := cost.CostSummary{
		ToolCosts:          make(map[string]float64),
		ToolExecutionCount: make(map[string]int),
		Currency:           "USD",
	}

	// Calculate tool costs
	totalToolCost := 0.0
	for toolName, cost := range o.ToolCosts {
		summary.ToolCosts[toolName] = cost
		totalToolCost += cost
	}

	// Calculate tool execution counts from ToolCallStats
	for toolName, count := range o.ToolCallStats {
		summary.ToolExecutionCount[toolName] = count
	}

	summary.TotalToolCost = totalToolCost

	// Calculate model costs
	if o.ModelCost != nil {
		summary.ModelInputCost = o.ModelCost.CalculateInputCostWithTiers(o.TotalUsage.PromptTokens)
		summary.ModelOutputCost = o.ModelCost.CalculateOutputCostWithTiers(o.TotalUsage.CompletionTokens)

		summary.ModelCachedCost = o.ModelCost.CalculateCachedCost(o.TotalUsage.CachedTokens)
		summary.ModelReasoningCost = o.ModelCost.CalculateReasoningCost(o.TotalUsage.ReasoningTokens)
	}

	summary.TotalModelCost = summary.ModelInputCost + summary.ModelOutputCost +
		summary.ModelCachedCost + summary.ModelReasoningCost

	// Calculate compute/infrastructure costs
	duration := o.ExecutionDuration()
	if duration > 0 && o.ComputeCost != nil {
		summary.ExecutionDurationSeconds = duration.Seconds()
		summary.ComputeCost = o.ComputeCost.CalculateCost(duration.Seconds())
	}

	summary.TotalCost = summary.TotalToolCost + summary.TotalModelCost + summary.ComputeCost

	return summary
}
