package tool

import (
	"context"
	"encoding/json"
	"time"

	"github.com/leofalp/aigo/core/parse"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/observability"
)

type Tool[I, O any] struct {
	Name        string
	Description string
	Parameters  *jsonschema.Schema
	Output      *jsonschema.Schema
	Function    func(ctx context.Context, input I) (O, error)
	// Metrics contains optional cost and performance metrics for this tool execution
	Metrics *cost.ToolMetrics
}

type GenericTool interface {
	ToolInfo() ai.ToolDescription
	Call(ctx context.Context, inputJson string) (string, error)
}

type funcToolOptions struct {
	Description string
	Metrics     *cost.ToolMetrics
}

func WithDescription(description string) func(tool *funcToolOptions) {
	return func(s *funcToolOptions) {
		s.Description = description
	}
}

// WithMetrics sets the metrics (cost, accuracy, speed, quality) for executing this tool.
func WithMetrics(toolMetrics cost.ToolMetrics) func(tool *funcToolOptions) {
	return func(s *funcToolOptions) {
		s.Metrics = &toolMetrics
	}
}

func NewTool[I, O any](name string, function func(ctx context.Context, input I) (O, error), options ...func(tool *funcToolOptions)) *Tool[I, O] {
	toolOptions := &funcToolOptions{}
	for _, o := range options {
		o(toolOptions)
	}

	tool := &Tool[I, O]{
		Name:        name,
		Description: toolOptions.Description,
		Parameters:  jsonschema.GenerateJSONSchema[I](),
		Output:      jsonschema.GenerateJSONSchema[O](),
		Function:    function,
		Metrics:     toolOptions.Metrics,
	}
	return tool
}

func (t *Tool[I, O]) ToolInfo() ai.ToolDescription {
	toolDesc := ai.ToolDescription{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  t.Parameters,
	}

	// Attach metrics metadata if available
	if t.Metrics != nil {
		toolDesc.Metrics = t.Metrics
	}

	return toolDesc
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

	// Track cost if available (extracted from context later)
	// The cost tracking will be handled by the caller (client/pattern)

	// Flexible parse of input JSON (from llm) into the expected input type
	parsedInput, err := parse.ParseStringAs[I](inputJson)
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
		attrs := []observability.Attribute{
			observability.String(observability.AttrToolOutput, string(outputBytes)),
			observability.Duration(observability.AttrToolDuration, duration),
		}

		// Add cost and metrics information to observability if available
		if t.Metrics != nil {
			attrs = append(attrs,
				observability.Float64("tool.cost.amount", t.Metrics.Amount),
				observability.String("tool.cost.currency", t.Metrics.Currency),
			)
			if t.Metrics.CostDescription != "" {
				attrs = append(attrs, observability.String("tool.cost.description", t.Metrics.CostDescription))
			}
			if t.Metrics.Accuracy > 0 {
				attrs = append(attrs, observability.Float64("tool.metrics.accuracy", t.Metrics.Accuracy))
			}
			if t.Metrics.AverageDurationInMillis > 0 {
				attrs = append(attrs, observability.Int64("tool.metrics.avg_duration_ms", t.Metrics.AverageDurationInMillis))
			}
			if t.Metrics.Quality > 0 {
				attrs = append(attrs, observability.Float64("tool.metrics.quality", t.Metrics.Quality))
			}
		}

		span.SetAttributes(attrs...)
	}

	return string(outputBytes), nil
}

// GetMetrics returns the metrics (cost and performance data) for this tool, if any.
func (t *Tool[I, O]) GetMetrics() *cost.ToolMetrics {
	return t.Metrics
}
