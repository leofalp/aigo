package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const SEND_MESSAGE_URL = "https://openrouter.ai/api/v1/chat/completions"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	apiKey   string
	model    string
	messages []Message
	tools    []map[string]interface{}
}

func NewClient(apiKey string, model string) *Client {
	return &Client{
		apiKey: apiKey,
		model:  model,
	}
}

func (c *Client) AddSystemPrompt(content string) {
	c.messages = append(c.messages, Message{Role: "system", Content: content})
}

func (c *Client) AddTools(tools []DocumentedTool) error {
	for _, tool := range tools {
		toolInfo := tool.ToolInfo()
		parametersJsonSchema, err := json.Marshal(toolInfo.Parameters)
		if err != nil {
			return fmt.Errorf("Client.AddTools(): error parsing tool parameters: %s", err.Error())
		}

		c.tools = append(c.tools, map[string]interface{}{
			"name":        toolInfo.Name,
			"description": toolInfo.Description,
			"parameters":  parametersJsonSchema,
		})
	}

	return nil
}

func (c *Client) SendMessage(content string) error {
	c.messages = append(c.messages, Message{Role: "user", Content: content})

	bodyMap := map[string]interface{}{
		"model":    c.model,
		"messages": c.messages,
	}

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return fmt.Errorf("Client.SendMessage(): error creating request body: %s", err.Error())
	}

	req, err := http.NewRequest("POST", SEND_MESSAGE_URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("Client.SendMessage(): error creating request: %s", err.Error())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := http.Client{}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Client.SendMessage(): error send request: %s", err.Error())
	}
	defer func() { _ = res.Body.Close() }()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("Client.SendMessage(): error reading response body: %s", err.Error())
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Client.SendMessage(): non-2xx status %d: %s", res.StatusCode, string(respBody))
	}

	//TODO: append system message
	fmt.Printf("SendMessage response: %s\n", string(respBody))

	return nil
}
