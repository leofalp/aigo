package tool

import (
	"context"
	"encoding/json"
	"time"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/observability"
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
	Call(ctx context.Context, inputJson string) (string, error)
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

func (t *Tool[I, O]) Call(ctx context.Context, inputJson string) (string, error) {
	// Extract span from context for observability
	span := observability.SpanFromContext(ctx)

	if span != nil {
		span.AddEvent(observability.EventToolExecutionStart,
			observability.String(observability.AttrToolName, t.Name),
			observability.String(observability.AttrToolInput, inputJson),
		)
		defer span.AddEvent(observability.EventToolExecutionEnd)
	}

	start := time.Now()
	var parsedInput I

	err := json.Unmarshal([]byte(inputJson), &parsedInput)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetAttributes(
				observability.String(observability.AttrToolError, err.Error()),
			)
		}
		return "", err
	}

	output, err := t.Function(ctx, parsedInput)
	duration := time.Since(start)

	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetAttributes(
				observability.String(observability.AttrToolError, err.Error()),
				observability.Duration(observability.AttrToolDuration, duration),
			)
		}
		return "", err
	}

	outputBytes, err := json.Marshal(output)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return "", err
	}

	if span != nil {
		span.SetAttributes(
			observability.String(observability.AttrToolOutput, string(outputBytes)),
			observability.Duration(observability.AttrToolDuration, duration),
		)
	}

	return string(outputBytes), nil
}
