package client

import (
	"aigo/cmd/provider"
	"aigo/cmd/tool"
	"context"
	"reflect"
)

type Client struct {
	llmProvider           provider.Provider
	messages              []provider.Message
	toolCatalog           map[string]tool.CallableTool
	toolDescriptions      []tool.ToolInfo
	maxToolCallIterations int
	outputFormat          reflect.Type
}

func NewClient(llmProvider provider.Provider) *Client {
	return &Client{
		llmProvider:           llmProvider,
		toolCatalog:           map[string]tool.CallableTool{},
		maxToolCallIterations: 3,
	}
}

func (c *Client) SetProvider(provider provider.Provider) {
	c.llmProvider = provider
}

func (c *Client) AddSystemPrompt(content string) *Client {
	c.appendMessage(provider.RoleSystem, content)
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

func (c *Client) SendMessage(content string) (*provider.ChatResponse, error) {
	c.appendMessage(provider.RoleUser, content)
	response := &provider.ChatResponse{}
	toolCallIterations := 0
	var err error

	stop := false
	for !stop {
		response, err = c.llmProvider.SendSingleMessage(context.Background(), provider.ChatRequest{
			Messages: c.messages,
			Tools:    c.toolDescriptions,
		})
		if err != nil {
			return nil, err
		}

		c.appendMessage(provider.RoleAssistant, response.Content)

		for _, t := range response.ToolCalls {
			output, err := c.toolCatalog[t.Function.Name].Call(t.Function.Arguments)
			if err != nil {
				return nil, err
			}

			c.appendMessage(provider.RoleTool, output)
		}

		if len(response.ToolCalls) > 0 {
			toolCallIterations++
		}
		stop = response.FinishReason == "stop" || toolCallIterations == c.maxToolCallIterations
		//TODO: too dependent on the llm provider implementation "response.FinishReason == "stop""
		//      delegate to the provider the stop check, like "c.llmProvider.IsStopMessage(response)" or similar

	}

	return response, nil
}

func (c *Client) appendMessage(role provider.MessageRole, content string) {
	c.messages = append(c.messages, provider.Message{
		Role:    role,
		Content: content,
	})
}
