package calculator

import (
	"context"
	"math"
	"testing"
)

// TestCalc_Add verifies the "add" and "+" operations produce the correct sum.
func TestCalc_Add(t *testing.T) {
	tests := []struct {
		name     string
		op       string
		a, b     float64
		expected float64
	}{
		{"add keyword", "add", 3, 4, 7},
		{"plus symbol", "+", 3, 4, 7},
		{"negative operands", "add", -1, -2, -3},
		{"zero operands", "add", 0, 0, 0},
		{"floating point", "+", 1.5, 2.5, 4.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := Calc(context.Background(), Input{A: tc.a, B: tc.b, Op: tc.op})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.Result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, output.Result)
			}
		})
	}
}

// TestCalc_Sub verifies the "sub" and "-" operations produce the correct difference.
func TestCalc_Sub(t *testing.T) {
	tests := []struct {
		name     string
		op       string
		a, b     float64
		expected float64
	}{
		{"sub keyword", "sub", 10, 3, 7},
		{"minus symbol", "-", 10, 3, 7},
		{"negative result", "sub", 3, 10, -7},
		{"zero result", "-", 5, 5, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := Calc(context.Background(), Input{A: tc.a, B: tc.b, Op: tc.op})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.Result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, output.Result)
			}
		})
	}
}

// TestCalc_Mul verifies the "mul" and "*" operations produce the correct product.
func TestCalc_Mul(t *testing.T) {
	tests := []struct {
		name     string
		op       string
		a, b     float64
		expected float64
	}{
		{"mul keyword", "mul", 3, 4, 12},
		{"star symbol", "*", 3, 4, 12},
		{"multiply by zero", "mul", 100, 0, 0},
		{"negative product", "*", -3, 4, -12},
		{"both negative", "mul", -3, -4, 12},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := Calc(context.Background(), Input{A: tc.a, B: tc.b, Op: tc.op})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.Result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, output.Result)
			}
		})
	}
}

// TestCalc_Div verifies the "div" and "/" operations produce the correct quotient.
func TestCalc_Div(t *testing.T) {
	tests := []struct {
		name     string
		op       string
		a, b     float64
		expected float64
	}{
		{"div keyword", "div", 10, 4, 2.5},
		{"slash symbol", "/", 10, 4, 2.5},
		{"integer division", "div", 9, 3, 3},
		{"negative divisor", "/", 10, -2, -5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := Calc(context.Background(), Input{A: tc.a, B: tc.b, Op: tc.op})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.Result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, output.Result)
			}
		})
	}
}

// TestCalc_DivByZero verifies that division by zero follows IEEE 754 semantics,
// returning ±Inf rather than an error. This is the documented behavior.
func TestCalc_DivByZero(t *testing.T) {
	tests := []struct {
		name     string
		a        float64
		expected float64
	}{
		{"positive / zero = +Inf", 1.0, math.Inf(1)},
		{"negative / zero = -Inf", -1.0, math.Inf(-1)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := Calc(context.Background(), Input{A: tc.a, B: 0, Op: "div"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output.Result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, output.Result)
			}
		})
	}
}

// TestCalc_UnknownOp verifies that an unrecognized operation silently returns 0.0
// with no error, which is the documented behaviour.
func TestCalc_UnknownOp(t *testing.T) {
	output, err := Calc(context.Background(), Input{A: 5, B: 3, Op: "pow"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Result != 0.0 {
		t.Errorf("expected 0.0 for unknown op, got %v", output.Result)
	}
}

// TestNewCalculatorTool_Name verifies the tool is registered with the correct name.
func TestNewCalculatorTool_Name(t *testing.T) {
	calculatorTool := NewCalculatorTool()
	info := calculatorTool.ToolInfo()

	if info.Name != "Calculator" {
		t.Errorf("expected tool name %q, got %q", "Calculator", info.Name)
	}
}

// TestNewCalculatorTool_Metrics verifies the tool reports zero cost and local
// metrics, consistent with in-process execution.
func TestNewCalculatorTool_Metrics(t *testing.T) {
	calculatorTool := NewCalculatorTool()
	metrics := calculatorTool.GetMetrics()

	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
	if metrics.Amount != 0.0 {
		t.Errorf("expected zero cost, got %v", metrics.Amount)
	}
	if metrics.Currency != "USD" {
		t.Errorf("expected currency %q, got %q", "USD", metrics.Currency)
	}
	if metrics.Accuracy != 1.0 {
		t.Errorf("expected accuracy 1.0, got %v", metrics.Accuracy)
	}
}
