package react

import (
	"aigo/patterns"
	"aigo/providers/ai"
	"aigo/providers/memory"
	"context"
)

type ReactPattern struct {
	maxIterations int
}

func NewReactPattern(maxIterations int) *ReactPattern {
	return &ReactPattern{
		maxIterations: maxIterations,
	}
}

// TODO: not best approach to pass options every time executes, but it keeps open for flexibility between executions
func (p *ReactPattern) Execute(llmProvider ai.Provider, memoryProviderPtr *memory.Provider, options ...func(tool *patterns.PatternOptions)) (*ai.ChatResponse, error) {
	p.maxIterations = 0
	var err error
	var response *ai.ChatResponse
	memoryProvider := *memoryProviderPtr
	var opts patterns.PatternOptions
	for _, option := range options {
		option(&opts)
	}

	ctx := context.Background()
	stop := false
	for !stop {
		response, err = llmProvider.SendMessage(ctx, ai.ChatRequest{
			Model:        opts.Model,
			SystemPrompt: opts.SystemPrompt,
			Messages:     memoryProvider.AllMessages(),
			Tools:        opts.ToolDescriptions,
			ResponseFormat: &ai.ResponseFormat{
				OutputSchema: opts.OutputSchema,
			},
		})
		if err != nil {
			return nil, err
		}

		memoryProvider.AppendMessage(ctx, &ai.Message{Role: ai.RoleAssistant, Content: response.Content})

		for _, t := range response.ToolCalls {
			output, err := opts.ToolCatalog[t.Function.Name].Call(ctx, t.Function.Arguments)
			if err != nil {
				return nil, err
			}

			memoryProvider.AppendMessage(ctx, &ai.Message{Role: ai.RoleTool, Content: output})
		}

		if len(response.ToolCalls) > 0 {
			p.maxIterations++
		}
		stop = llmProvider.IsStopMessage(response) || p.maxIterations >= p.maxIterations
	}

	return response, nil
}
