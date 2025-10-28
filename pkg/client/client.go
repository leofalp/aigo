package client

import (
	"aigo/cmd/provider"
	"aigo/cmd/tool"
	"context"
)

type Client struct {
	llmProvider provider.Provider
	messages    []provider.Message
	tools       []tool.ToolInfo
}

func NewClient(llmProvider provider.Provider) *Client {
	return &Client{
		llmProvider: llmProvider,
	}
}

func (c *Client) AddSystemPrompt(content string) {
	c.messages = append(c.messages, provider.Message{Role: "system", Content: content}) // TODO support more roles
}

func (c *Client) AddTools(tools []tool.DocumentedTool) error {
	for _, t := range tools {
		c.tools = append(c.tools, t.ToolInfo())
	}
	return nil
}

func (c *Client) SendMessage(content string) (*provider.ChatResponse, error) {
	c.messages = append(c.messages, provider.Message{Role: "user", Content: content})
	response, err := c.llmProvider.SendSingleMessage(context.Background(), provider.ChatRequest{
		Messages: c.messages,
		Tools:    c.tools,
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}
