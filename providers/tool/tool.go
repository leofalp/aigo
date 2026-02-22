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

// Tool represents a typed, callable tool that can be registered with an AI provider.
// It binds a name and description to a strongly-typed Go function, and automatically
// derives JSON schemas for both input (I) and output (O) via reflection.
// Use [NewTool] to construct a Tool; implement [GenericTool] for provider-agnostic usage.
type Tool[I, O any] struct {
	Name        string
	Description string
	Parameters  *jsonschema.Schema
	Output      *jsonschema.Schema
	Function    func(ctx context.Context, input I) (O, error)
	// Metrics contains optional cost and performance metrics for this tool execution.
	Metrics *cost.ToolMetrics
}

// GenericTool is the provider-agnostic interface for all tools.
// It abstracts over the concrete generic type parameters of [Tool] so that tools
// can be stored, dispatched, and introspected without knowing their exact input/output types.
type GenericTool interface {
	// ToolInfo returns the metadata (name, description, parameter schema) used to
	// advertise this tool to an AI provider.
	ToolInfo() ai.ToolDescription

	// Call invokes the tool with a JSON-encoded input string and returns a
	// JSON-encoded output string. Returns an error if parsing or execution fails.
	Call(ctx context.Context, inputJson string) (string, error)

	// GetMetrics returns the cost and performance metrics associated with this tool,
	// or nil if none were configured.
	GetMetrics() *cost.ToolMetrics
}

// funcToolOptions holds optional configuration for a tool created via [NewTool].
type funcToolOptions struct {
	Description string
	Metrics     *cost.ToolMetrics
}

// WithDescription sets a human-readable description for the tool.
// Providers surface this description to the language model to help it decide
// when and how to invoke the tool.
func WithDescription(description string) func(tool *funcToolOptions) {
	return func(s *funcToolOptions) {
		s.Description = description
	}
}

// WithMetrics sets the metrics (cost, accuracy, speed) for executing this tool.
func WithMetrics(toolMetrics cost.ToolMetrics) func(tool *funcToolOptions) {
	return func(s *funcToolOptions) {
		s.Metrics = &toolMetrics
	}
}

// NewTool constructs a new [Tool] with the given name and handler function.
// JSON schemas for the input type I and output type O are derived automatically
// via reflection. Optional configuration (description, metrics) can be provided
// through [WithDescription] and [WithMetrics].
//
// Example:
//
//	myTool := tool.NewTool("search", searchFunc,
//	    tool.WithDescription("Searches the web for a query."),
//	    tool.WithMetrics(cost.ToolMetrics{Amount: 0.001, Currency: "USD"}),
//	)
func NewTool[I, O any](name string, function func(ctx context.Context, input I) (O, error), options ...func(tool *funcToolOptions)) *Tool[I, O] {
	toolOptions := &funcToolOptions{}
	for _, option := range options {
		option(toolOptions)
	}

	newTool := &Tool[I, O]{
		Name:        name,
		Description: toolOptions.Description,
		Parameters:  jsonschema.GenerateJSONSchema[I](),
		Output:      jsonschema.GenerateJSONSchema[O](),
		Function:    function,
		Metrics:     toolOptions.Metrics,
	}
	return newTool
}

// ToolInfo returns the [ai.ToolDescription] used to advertise this tool to an AI provider.
// It includes the tool's name, description, parameter schema, and optional metrics metadata.
func (t *Tool[I, O]) ToolInfo() ai.ToolDescription {
	toolDesc := ai.ToolDescription{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  t.Parameters,
	}

	// Attach metrics metadata if available so providers can surface cost information.
	if t.Metrics != nil {
		toolDesc.Metrics = t.Metrics
	}

	return toolDesc
}

// Call invokes the tool's underlying function with the given JSON-encoded input.
// It deserializes inputJson into the tool's input type I, executes the function,
// and returns the result serialized as JSON. Observability span events are emitted
// at the start and end of execution when a span is present in ctx.
// Returns an error if JSON parsing, function execution, or output marshaling fails.
func (t *Tool[I, O]) Call(ctx context.Context, inputJson string) (string, error) {
	// Extract span from context for observability.
	span := observability.SpanFromContext(ctx)

	if span != nil {
		span.AddEvent(observability.EventToolExecutionStart,
			observability.String(observability.AttrToolName, t.Name),
			observability.String(observability.AttrToolInput, inputJson),
		)
		defer span.AddEvent(observability.EventToolExecutionEnd)
	}

	start := time.Now()

	// Cost tracking is handled by the caller (client/pattern) via GetMetrics.

	// Flexibly parse the LLM-supplied input JSON into the strongly-typed input type.
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

		// Add cost and metrics information to observability if available.
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
		}

		span.SetAttributes(attrs...)
	}

	return string(outputBytes), nil
}

// GetMetrics returns the metrics (cost and performance data) for this tool, if any.
func (t *Tool[I, O]) GetMetrics() *cost.ToolMetrics {
	return t.Metrics
}
