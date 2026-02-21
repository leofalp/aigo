package overview

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

// Overview aggregates execution statistics, token usage, cost tracking,
// and request/response history for a single execution lifecycle.
// It is the primary carrier of observability data produced by the AI client
// and is stored in a [context.Context] via [Overview.ToContext] so that all
// layers of a call-stack can contribute to the same shared instance.
// Use [OverviewFromContext] to retrieve or lazily create an Overview from a context.
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
	// Data holds the strongly-typed value parsed from the final AI response.
	// It is populated by structured execution patterns (e.g. StructuredPattern[T])
	// and is nil when the response could not be parsed into T.
	Data *T `json:"data,omitempty"`
}

// OverviewFromContext retrieves the Overview from the context, creating one if
// it does not already exist. The context pointer is updated in-place when a new
// Overview is created so callers see the enriched context.
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

// ToContext stores the Overview in the given context under a private key and
// returns the enriched context. The private key prevents collisions with other
// packages that also use context values. Callers should retrieve the stored
// value via [OverviewFromContext] rather than accessing the context directly,
// so that nil-context and type-assertion edge cases are handled uniformly.
// If ctx is nil, context.Background() is used as the base.
func (overview *Overview) ToContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, overviewContextKey, overview)
}

// IncludeUsage accumulates token usage from an AI response into the overview totals.
func (overview *Overview) IncludeUsage(usage *ai.Usage) {
	if usage == nil {
		return
	}
	overview.TotalUsage.PromptTokens += usage.PromptTokens
	overview.TotalUsage.CompletionTokens += usage.CompletionTokens
	overview.TotalUsage.TotalTokens += usage.TotalTokens
	overview.TotalUsage.ReasoningTokens += usage.ReasoningTokens
	overview.TotalUsage.CachedTokens += usage.CachedTokens
}

// AddToolCalls records tool call invocations in the overview statistics.
func (overview *Overview) AddToolCalls(tools []ai.ToolCall) {
	if overview.ToolCallStats == nil {
		overview.ToolCallStats = make(map[string]int)
	}

	for _, tool := range tools {
		overview.ToolCallStats[tool.Function.Name]++
	}
}

// AddRequest appends a chat request to the overview's request history.
func (overview *Overview) AddRequest(request *ai.ChatRequest) {
	overview.Requests = append(overview.Requests, request)
}

// AddResponse appends a chat response to the overview's response history and
// updates the last response reference.
func (overview *Overview) AddResponse(response *ai.ChatResponse) {
	overview.Responses = append(overview.Responses, response)
	overview.LastResponse = response
}

// AddToolExecutionCost records the cost of a tool execution.
func (overview *Overview) AddToolExecutionCost(toolName string, toolMetrics *cost.ToolMetrics) {
	if overview.ToolCosts == nil {
		overview.ToolCosts = make(map[string]float64)
	}
	if toolMetrics != nil {
		overview.ToolCosts[toolName] += toolMetrics.Amount
	}
}

// SetModelCost attaches a model pricing configuration so that [Overview.CostSummary]
// and [Overview.TotalCost] can calculate per-token input, output, cached, and
// reasoning costs. Calling this is optional; omitting it leaves all model cost
// fields in the returned [cost.CostSummary] as zero.
func (overview *Overview) SetModelCost(modelCost *cost.ModelCost) {
	overview.ModelCost = modelCost
}

// SetComputeCost attaches an infrastructure pricing configuration so that
// [Overview.CostSummary] can calculate the cost of the execution environment
// (e.g. AWS Lambda, VM, or container runtime) proportional to the duration
// measured between [Overview.StartExecution] and [Overview.EndExecution].
// Calling this is optional; omitting it leaves compute costs as zero.
func (overview *Overview) SetComputeCost(computeCost *cost.ComputeCost) {
	overview.ComputeCost = computeCost
}

// StartExecution marks the start of execution for compute cost tracking.
func (overview *Overview) StartExecution() {
	overview.ExecutionStartTime = time.Now()
}

// EndExecution marks the end of execution for compute cost tracking.
func (overview *Overview) EndExecution() {
	overview.ExecutionEndTime = time.Now()
}

// ExecutionDuration returns the total execution duration.
// Returns 0 if execution hasn't started or ended.
func (overview *Overview) ExecutionDuration() time.Duration {
	if overview.ExecutionStartTime.IsZero() || overview.ExecutionEndTime.IsZero() {
		return 0
	}
	return overview.ExecutionEndTime.Sub(overview.ExecutionStartTime)
}

// TotalCost returns the total cost of the execution (tools + model).
func (overview *Overview) TotalCost() float64 {
	summary := overview.CostSummary()
	return summary.TotalCost
}

// CostSummary returns a detailed breakdown of all costs accumulated during the
// execution. The returned [cost.CostSummary] contains per-tool execution costs
// and invocation counts, model input/output/cached/reasoning costs derived from
// token usage and the configured [cost.ModelCost], and compute/infrastructure
// costs derived from the measured execution duration and the configured
// [cost.ComputeCost]. Currency is always "USD". Call [Overview.TotalCost] when
// only the scalar total is needed.
func (overview *Overview) CostSummary() cost.CostSummary {
	summary := cost.CostSummary{
		ToolCosts:          make(map[string]float64),
		ToolExecutionCount: make(map[string]int),
		Currency:           "USD",
	}

	// Calculate tool costs
	totalToolCost := 0.0
	for toolName, cost := range overview.ToolCosts {
		summary.ToolCosts[toolName] = cost
		totalToolCost += cost
	}

	// Calculate tool execution counts from ToolCallStats
	for toolName, count := range overview.ToolCallStats {
		summary.ToolExecutionCount[toolName] = count
	}

	summary.TotalToolCost = totalToolCost

	// Calculate model costs
	if overview.ModelCost != nil {
		summary.ModelInputCost = overview.ModelCost.CalculateInputCostWithTiers(overview.TotalUsage.PromptTokens)
		summary.ModelOutputCost = overview.ModelCost.CalculateOutputCostWithTiers(overview.TotalUsage.CompletionTokens)

		summary.ModelCachedCost = overview.ModelCost.CalculateCachedCost(overview.TotalUsage.CachedTokens)
		summary.ModelReasoningCost = overview.ModelCost.CalculateReasoningCost(overview.TotalUsage.ReasoningTokens)
	}

	summary.TotalModelCost = summary.ModelInputCost + summary.ModelOutputCost +
		summary.ModelCachedCost + summary.ModelReasoningCost

	// Calculate compute/infrastructure costs
	duration := overview.ExecutionDuration()
	if duration > 0 && overview.ComputeCost != nil {
		summary.ExecutionDurationSeconds = duration.Seconds()
		summary.ComputeCost = overview.ComputeCost.CalculateCost(duration.Seconds())
	}

	summary.TotalCost = summary.TotalToolCost + summary.TotalModelCost + summary.ComputeCost

	return summary
}
