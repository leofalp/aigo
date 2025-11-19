package patterns

import (
	"context"

	"github.com/leofalp/aigo/providers/ai"
)

type Overview struct {
	LastResponse  *ai.ChatResponse   `json:"last_response,omitempty"`
	Requests      []*ai.ChatRequest  `json:"requests"`
	Responses     []*ai.ChatResponse `json:"responses"`
	TotalUsage    ai.Usage           `json:"total_usage"`
	ToolCallStats map[string]int     `json:"tool_calls,omitempty"`
}

// StructuredOverview extends Overview with parsed structured data from the final response.
// This is used by structured patterns (e.g., StructuredPattern[T]) to provide both
// execution statistics and the parsed final result.
type StructuredOverview[T any] struct {
	Overview
	Data *T // Parsed final response data
}

func OverviewFromContext(ctx *context.Context) *Overview {
	overviewVal := (*ctx).Value("overview")
	if overviewVal == nil {
		overview := &Overview{}
		*ctx = overview.ToContext(*ctx)
		return overview
	}

	return overviewVal.(*Overview)
}

func (o *Overview) ToContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, "overview", o)
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
