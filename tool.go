package main

import (
	"context"
	"reflect"
)

type Tool[I, O any] struct {
	Name        string
	Description string
	Parameters  *Schema
	Output      *Schema
	Function    func(ctx context.Context, input I) (O, error)
}

type DocumentedTool interface {
	ToolInfo() ToolInfo
}

type ToolInfo struct {
	Name        string
	Description string
	Parameters  *Schema
}

func NewTool[I, O any](name string, description string, function func(ctx context.Context, input I) (O, error)) *Tool[I, O] {
	var (
		intput I
		output O
	)

	parameterSchema := GenerateJSONSchema(reflect.TypeOf(intput))
	outputSchema := GenerateJSONSchema(reflect.TypeOf(output))

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
