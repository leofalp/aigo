package tool

import (
	"aigo/internal/jsonschema"
	"aigo/providers/ai"
	"context"
	"encoding/json"
)

type Tool[I, O any] struct {
	Name        string
	Description string
	Required    bool
	Parameters  *jsonschema.Schema
	Output      *jsonschema.Schema
	Function    func(ctx context.Context, input I) (O, error)
}

type GenericTool interface {
	ToolInfo() ai.ToolDescription
	Call(inputJson string) (string, error)
}

type funcToolOptions struct {
	Description string
	Required    bool
}

func WithDescription(description string) func(tool *funcToolOptions) {
	return func(s *funcToolOptions) {
		s.Description = description
	}
}

func IsRequired() func(tool *funcToolOptions) {
	return func(s *funcToolOptions) {
		s.Required = true
	}
}

func NewTool[I, O any](name string, function func(ctx context.Context, input I) (O, error), options ...func(tool *funcToolOptions)) *Tool[I, O] {
	toolOptions := &funcToolOptions{}
	for _, o := range options {
		o(toolOptions)
	}

	tool := &Tool[I, O]{
		Name:        name,
		Required:    toolOptions.Required,
		Description: toolOptions.Description,
		Parameters:  jsonschema.GenerateJSONSchema[I](),
		Output:      jsonschema.GenerateJSONSchema[O](),
		Function:    function,
	}
	return tool
}

func (t *Tool[I, O]) ToolInfo() ai.ToolDescription {
	return ai.ToolDescription{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  t.Parameters,
	}
}

func (t *Tool[I, O]) Call(inputJson string) (string, error) {
	var parsedInput I

	err := json.Unmarshal([]byte(inputJson), &parsedInput)
	if err != nil {
		return "", err
	}

	output, err := t.Function(context.Background(), parsedInput)
	if err != nil {
		return "", err
	}

	outputBytes, err := json.Marshal(output)
	if err != nil {
		return "", err
	}

	return string(outputBytes), nil
}
