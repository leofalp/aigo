package client

import (
	"aigo/providers/ai"
	"aigo/providers/tool"
	"context"
	"reflect"
)

type Client struct {
	llmProvider           ai.Provider
	messages              []ai.Message
	toolCatalog           map[string]tool.CallableTool
	toolDescriptions      []tool.ToolInfo
	maxToolCallIterations int
	outputFormat          reflect.Type
}

func NewClient(llmProvider ai.Provider) *Client {
	return &Client{
		llmProvider:           llmProvider,
		toolCatalog:           map[string]tool.CallableTool{},
		maxToolCallIterations: 3,
	}
}

func (c *Client) SetProvider(provider ai.Provider) {
	c.llmProvider = provider
}

func (c *Client) AddSystemPrompt(content string) *Client {
	c.appendMessage(ai.RoleSystem, content)
	return c
}

func (c *Client) SetMaxToolCallIterations(maxCalls int) *Client {
	c.maxToolCallIterations = maxCalls
	return c
}

func (c *Client) AddTools(tools []tool.CallableTool) *Client {
	for i, t := range tools {
		c.toolCatalog[t.ToolInfo().Name] = tools[i]
		c.toolDescriptions = append(c.toolDescriptions, t.ToolInfo())
	}

	return c
}

func (c *Client) SetOutputFormat(outputObject any) *Client {
	c.outputFormat = reflect.TypeOf(outputObject) //TODO enforce structure on response
	return c
}

func (c *Client) SendMessage(content string) (*ai.ChatResponse, error) {
	c.appendMessage(ai.RoleUser, content)
	response := &ai.ChatResponse{}
	toolCallIterations := 0
	var err error

	stop := false
	for !stop {
		response, err = c.llmProvider.SendMessage(context.Background(), ai.ChatRequest{
			Messages: c.messages,
			Tools:    c.toolDescriptions,
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

	return response, nil
}

func (c *Client) appendMessage(role ai.MessageRole, content string) {
	c.messages = append(c.messages, ai.Message{
		Role:    role,
		Content: content,
	})
}
