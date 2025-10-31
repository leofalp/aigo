package client

import (
	"aigo/internal/jsonschema"
	"aigo/providers/ai"
	"aigo/providers/memory"
	"aigo/providers/tool"
	"context"
	"encoding/json"
	"reflect"
	"strconv"
)

type Client[T any] struct {
	systemPrompt   string
	defaultModel   string
	llmProvider    ai.Provider //TODO: Enforce requirement of llmProvider
	memoryProvider memory.Provider
	// for fast accessing tool by name
	toolCatalog map[string]tool.GenericTool
	// for passing tool info to LLM without processing all tools each time
	toolDescriptions []ai.ToolDescription
	outputSchema     *jsonschema.Schema
}

type funcClientOptions struct {
	DefaultModel string
}

func WithDefaultModel(defaultModel string) func(tool *funcClientOptions) {
	return func(tool *funcClientOptions) {
		tool.DefaultModel = defaultModel
	}
}

func NewClient[T any](llmProvider ai.Provider, options ...func(tool *funcClientOptions)) *Client[T] {
	toolOptions := &funcClientOptions{}
	for _, o := range options {
		o(toolOptions)
	}

	return &Client[T]{
		defaultModel: toolOptions.DefaultModel,
		llmProvider:  llmProvider,
		toolCatalog:  map[string]tool.GenericTool{},
		outputSchema: jsonschema.GenerateJSONSchema[T](),
	}
}

func (c *Client[T]) WithLlmProvider(llmProvider ai.Provider) *Client[T] {
	c.llmProvider = llmProvider
	return c
}

func (c *Client[T]) WithMemoryProvider(memoryProvider memory.Provider) *Client[T] {
	c.memoryProvider = memoryProvider
	return c
}

func (c *Client[T]) AddSystemPrompt(content string) *Client[T] {
	c.systemPrompt += content + "\n"
	return c
}

func (c *Client[T]) AddTools(tools []tool.GenericTool) *Client[T] {
	for i, t := range tools {
		c.toolCatalog[t.ToolInfo().Name] = tools[i]
		c.toolDescriptions = append(c.toolDescriptions, t.ToolInfo())
	}

	return c
}

func (c *Client[T]) SendMessage(prompt string) (*ai.ChatResponse, error) {
	c.memoryProvider.AppendMessage(&ai.Message{Role: ai.RoleUser, Content: prompt})
	response, err := c.llmProvider.SendMessage(context.Background(), ai.ChatRequest{
		// TODO
	})
	if err != nil {
		return nil, err
	}

	return c.responseParser(response)
}

func (c *Client[T]) responseParser(response *ai.ChatResponse) (*ai.ChatResponse, error) {
	var typedVar T
	var err error

	switch reflect.TypeOf(typedVar).Kind().String() {
	case "string":
		return response, nil
	case "bool":
		_, err = strconv.ParseBool(response.Content)
	case "float32", "float64":
		_, err = strconv.ParseFloat(response.Content, 64)
	case "int", "int8", "int16", "int32", "int64":
		_, err = strconv.ParseInt(response.Content, 10, 64)
	default:
		err = json.Unmarshal([]byte(response.Content), &typedVar)
	}

	if err != nil {
		response.Content = "[Waring] Could not parse response: " + err.Error() + " --> providing raw response content as fallback.\n\n" + response.Content
	}

	return response, nil
}
