package tool

import (
	"aigo/cmd/jsonschema"
	"context"
	"reflect"
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

type ToolInfo struct {
	Name        string
	Description string
	Parameters  *jsonschema.Schema
}

func NewTool[I, O any](name string, description string, function func(ctx context.Context, input I) (O, error)) *Tool[I, O] {
	var (
		intput I
		output O
	)

	parameterSchema := jsonschema.GenerateJSONSchema(reflect.TypeOf(intput))
	outputSchema := jsonschema.GenerateJSONSchema(reflect.TypeOf(output))

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
