package tool

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/providers/observability"
)

// --- Test helpers ---

// testSpan is a minimal observability.Span implementation that records events
// and attributes for assertion purposes. It captures AddEvent names and
// SetAttributes calls so tests can verify that tool execution emits the
// expected observability signals.
type testSpan struct {
	events     []string
	attributes []observability.Attribute
}

// End is a no-op; the test span does not manage lifecycle state.
func (s *testSpan) End() {}

// SetAttributes appends all provided attributes to the internal slice.
func (s *testSpan) SetAttributes(attrs ...observability.Attribute) {
	s.attributes = append(s.attributes, attrs...)
}

// SetStatus is a no-op; the test span does not track status codes.
func (s *testSpan) SetStatus(code observability.StatusCode, description string) {}

// RecordError is a no-op; the test span does not capture errors directly.
func (s *testSpan) RecordError(err error) {}

// AddEvent appends the event name to the internal slice for later assertions.
func (s *testSpan) AddEvent(name string, attrs ...observability.Attribute) {
	s.events = append(s.events, name)
}

// calcInput is the input type for the test calculator tool.
type calcInput struct {
	Value int `json:"value"`
}

// calcOutput is the output type for the test calculator tool.
type calcOutput struct {
	Result int `json:"result"`
}

// --- Tests ---

// TestNewTool_DefaultNoDescription verifies that a tool created without
// WithDescription has an empty description in its ToolInfo.
func TestNewTool_DefaultNoDescription(t *testing.T) {
	handler := func(ctx context.Context, input calcInput) (calcOutput, error) {
		return calcOutput{Result: input.Value}, nil
	}

	calcTool := NewTool("calc", handler)

	toolInfo := calcTool.ToolInfo()
	if toolInfo.Description != "" {
		t.Errorf("expected empty description, got %q", toolInfo.Description)
	}
	if toolInfo.Name != "calc" {
		t.Errorf("expected name %q, got %q", "calc", toolInfo.Name)
	}
}

// TestNewTool_WithDescription verifies that WithDescription correctly sets
// the tool's description in ToolInfo.
func TestNewTool_WithDescription(t *testing.T) {
	handler := func(ctx context.Context, input calcInput) (calcOutput, error) {
		return calcOutput{Result: input.Value}, nil
	}

	calcTool := NewTool("calc", handler, WithDescription("my desc"))

	toolInfo := calcTool.ToolInfo()
	if toolInfo.Description != "my desc" {
		t.Errorf("expected description %q, got %q", "my desc", toolInfo.Description)
	}
}

// TestNewTool_WithMetrics verifies that WithMetrics correctly attaches cost
// metrics to the tool and that GetMetrics returns them unchanged.
func TestNewTool_WithMetrics(t *testing.T) {
	handler := func(ctx context.Context, input calcInput) (calcOutput, error) {
		return calcOutput{Result: input.Value}, nil
	}

	expectedMetrics := cost.ToolMetrics{Amount: 0.5, Currency: "USD"}
	calcTool := NewTool("calc", handler, WithMetrics(expectedMetrics))

	gotMetrics := calcTool.GetMetrics()
	if gotMetrics == nil {
		t.Fatal("expected non-nil metrics, got nil")
	}
	if gotMetrics.Amount != expectedMetrics.Amount {
		t.Errorf("expected Amount %f, got %f", expectedMetrics.Amount, gotMetrics.Amount)
	}
	if gotMetrics.Currency != expectedMetrics.Currency {
		t.Errorf("expected Currency %q, got %q", expectedMetrics.Currency, gotMetrics.Currency)
	}
}

// TestCall_Success verifies that Call correctly parses JSON input, invokes the
// handler, and returns JSON-encoded output with the expected fields.
func TestCall_Success(t *testing.T) {
	handler := func(ctx context.Context, input calcInput) (calcOutput, error) {
		return calcOutput{Result: input.Value * 2}, nil
	}

	calcTool := NewTool("calc", handler)
	ctx := context.Background()

	inputJSON := `{"value":42}`
	outputJSON, err := calcTool.Call(ctx, inputJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result calcOutput
	if err := json.Unmarshal([]byte(outputJSON), &result); err != nil {
		t.Fatalf("failed to unmarshal output JSON: %v", err)
	}
	if result.Result != 84 {
		t.Errorf("expected Result 84, got %d", result.Result)
	}
}

// TestCall_HandlerError verifies that Call propagates errors returned by the
// handler function and does not return any output JSON.
func TestCall_HandlerError(t *testing.T) {
	handler := func(ctx context.Context, input calcInput) (calcOutput, error) {
		return calcOutput{}, errors.New("boom")
	}

	calcTool := NewTool("calc", handler)
	ctx := context.Background()

	outputJSON, err := calcTool.Call(ctx, `{"value":1}`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if outputJSON != "" {
		t.Errorf("expected empty output on error, got %q", outputJSON)
	}
	if !errors.Is(err, err) { // Sanity check; the real assertion is the message below.
		t.Fatal("unexpected error type")
	}
	if err.Error() != "boom" {
		t.Errorf("expected error message %q, got %q", "boom", err.Error())
	}
}

// TestCall_InputParseError verifies that Call returns an error when the input
// string cannot be deserialized into the tool's input type. We use plain text
// without any JSON structure because the parser applies aggressive recovery
// (including jsonrepair) on bracket-containing strings.
func TestCall_InputParseError(t *testing.T) {
	handler := func(ctx context.Context, input calcInput) (calcOutput, error) {
		return calcOutput{Result: input.Value}, nil
	}

	calcTool := NewTool("calc", handler)
	ctx := context.Background()

	outputJSON, err := calcTool.Call(ctx, "not json at all")
	if err == nil {
		t.Fatal("expected error for invalid JSON input, got nil")
	}
	if outputJSON != "" {
		t.Errorf("expected empty output on parse error, got %q", outputJSON)
	}
}

// TestCall_WithSpan_Success verifies that when an observability span is present
// in the context, the tool records execution start and end events on it.
func TestCall_WithSpan_Success(t *testing.T) {
	handler := func(ctx context.Context, input calcInput) (calcOutput, error) {
		return calcOutput{Result: input.Value + 1}, nil
	}

	calcTool := NewTool("calc", handler)

	span := &testSpan{}
	ctx := observability.ContextWithSpan(context.Background(), span)

	outputJSON, err := calcTool.Call(ctx, `{"value":10}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the output is correct.
	var result calcOutput
	if err := json.Unmarshal([]byte(outputJSON), &result); err != nil {
		t.Fatalf("failed to unmarshal output JSON: %v", err)
	}
	if result.Result != 11 {
		t.Errorf("expected Result 11, got %d", result.Result)
	}

	// Verify that the span received both start and end events.
	if len(span.events) < 2 {
		t.Fatalf("expected at least 2 span events, got %d: %v", len(span.events), span.events)
	}

	foundStart := false
	foundEnd := false
	for _, event := range span.events {
		if event == observability.EventToolExecutionStart {
			foundStart = true
		}
		if event == observability.EventToolExecutionEnd {
			foundEnd = true
		}
	}
	if !foundStart {
		t.Errorf("expected %q event, not found in %v", observability.EventToolExecutionStart, span.events)
	}
	if !foundEnd {
		t.Errorf("expected %q event, not found in %v", observability.EventToolExecutionEnd, span.events)
	}

	// Verify that attributes were recorded (tool output and duration at minimum).
	if len(span.attributes) == 0 {
		t.Error("expected span attributes to be set, got none")
	}
}

// TestGetMetrics_NoMetrics verifies that a tool created without WithMetrics
// returns nil from GetMetrics.
func TestGetMetrics_NoMetrics(t *testing.T) {
	handler := func(ctx context.Context, input calcInput) (calcOutput, error) {
		return calcOutput{Result: input.Value}, nil
	}

	calcTool := NewTool("calc", handler)

	if metrics := calcTool.GetMetrics(); metrics != nil {
		t.Errorf("expected nil metrics, got %+v", metrics)
	}
}
