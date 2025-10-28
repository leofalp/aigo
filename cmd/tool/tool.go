package tool

import (
	"aigo/cmd/jsonschema"
	"context"
	"encoding/json"
)

type Tool[I, O any] struct {
	Name        string
	Description string
	Parameters  *jsonschema.Schema
	Output      *jsonschema.Schema
	Function    func(ctx context.Context, input I) (O, error)
}

type DocumentedTool interface {
	ToolInfo() ToolInfo
}

type CallableTool interface {
	Call(inputJson string) (string, error)
	DocumentedTool
}

type ToolInfo struct {
	Name        string
	Description string
	Parameters  *jsonschema.Schema
}

func NewTool[I, O any](name string, description string, function func(ctx context.Context, input I) (O, error)) *Tool[I, O] {
	parameterSchema, err := jsonschema.GenerateJSONSchema[I]()
	if err != nil {
		panic(err) // TODO handle error appropriately
	}

	outputSchema, err := jsonschema.GenerateJSONSchema[O]()
	if err != nil {
		panic(err) // TODO handle error appropriately
	}

	return &Tool[I, O]{
		Name:        name,
		Description: description,
		Parameters:  parameterSchema,
		Output:      outputSchema,
		Function:    function,
	}
}

func (t *Tool[I, O]) ToolInfo() ToolInfo {
	return ToolInfo{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  t.Parameters,
	}
}

func (t *Tool[I, O]) Call(inputJson string) (string, error) {
	var parsedInput I

	err := json.Unmarshal([]byte(inputJson), &parsedInput)
	if err != nil {
		return "", nil
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
