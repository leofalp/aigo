package client

import (
	"aigo/internal/jsonschema"
	"aigo/providers/ai"
	"aigo/providers/tool"
	"context"
	"encoding/json"
)

type Client[T any] struct {
	llmProvider           ai.Provider
	messages              []ai.Message
	maxToolCallIterations int
	// for fast accessing tool by name
	toolCatalog map[string]tool.GenericTool
	// for passing tool info to LLM without processing all tools each time
	toolDescriptions []ai.ToolDescription
	outputSchema     *jsonschema.Schema
}

func NewClient[T any](llmProvider ai.Provider) *Client[T] {
	return &Client[T]{
		llmProvider:           llmProvider,
		toolCatalog:           map[string]tool.GenericTool{},
		maxToolCallIterations: 3,
		outputSchema:          jsonschema.GenerateJSONSchema[T](),
	}
}

func (c *Client[T]) SetProvider(provider ai.Provider) {
	c.llmProvider = provider
}

func (c *Client[T]) AddSystemPrompt(content string) *Client[T] {
	c.appendMessage(ai.RoleSystem, content)
	return c
}

func (c *Client[T]) SetMaxToolCallIterations(maxCalls int) *Client[T] {
	c.maxToolCallIterations = maxCalls
	return c
}

func (c *Client[T]) AddTools(tools []tool.GenericTool) *Client[T] {
	for i, t := range tools {
		c.toolCatalog[t.ToolInfo().Name] = tools[i]
		c.toolDescriptions = append(c.toolDescriptions, t.ToolInfo())
	}

	return c
}

func (c *Client[T]) SendMessage(content string) (*T, error) {
	c.appendMessage(ai.RoleUser, content)
	response := &ai.ChatResponse{}
	toolCallIterations := 0
	var err error

	stop := false
	for !stop {
		response, err = c.llmProvider.SendMessage(context.Background(), ai.ChatRequest{
			Messages: c.messages,
			Tools:    c.toolDescriptions,
			ResponseFormat: &ai.ResponseFormat{
				OutputSchema: c.outputSchema,
			},
		})
		if err != nil {
			return nil, err
		}

		c.appendMessage(ai.RoleAssistant, response.Content)

		for _, t := range response.ToolCalls {
			output, err := c.toolCatalog[t.Function.Name].Call(t.Function.Arguments)
			if err != nil {
				return nil, err
			}

			c.appendMessage(ai.RoleTool, output)
		}

		if len(response.ToolCalls) > 0 {
			toolCallIterations++
		}
		stop = c.llmProvider.IsStopMessage(response)

	}

	var resp T
	err = json.Unmarshal([]byte(response.Content), &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *Client[T]) appendMessage(role ai.MessageRole, content string) {
	c.messages = append(c.messages, ai.Message{
		Role:    role,
		Content: content,
	})
}
