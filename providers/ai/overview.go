package ai

import "context"

type Overview struct {
	LastResponse *ChatResponse   `json:"last_response,omitempty"`
	Requests     []*ChatRequest  `json:"requests"`
	Responses    []*ChatResponse `json:"responses"`
	TotalUsage   Usage           `json:"total_usage"`
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

func (o *Overview) IncludeUsage(usage *Usage) {
	o.TotalUsage.PromptTokens += usage.PromptTokens
	o.TotalUsage.CompletionTokens += usage.CompletionTokens
	o.TotalUsage.TotalTokens += usage.TotalTokens
	o.TotalUsage.ReasoningTokens += usage.ReasoningTokens
	o.TotalUsage.CachedTokens += usage.CachedTokens
}

func (o *Overview) AddRequest(request *ChatRequest) {
	o.Requests = append(o.Requests, request)
}

func (o *Overview) AddResponse(response *ChatResponse) {
	o.Responses = append(o.Responses, response)
	o.LastResponse = response
}
