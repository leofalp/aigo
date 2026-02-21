package calculator

import (
	"context"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/providers/tool"
)

// NewCalculatorTool returns a [tool.Tool] configured for basic arithmetic.
// It registers [Calc] as its execution function and annotates the tool with
// zero-cost local metrics, since the computation runs in-process with no
// external API calls. Use [tool.WithDescription] or [tool.WithMetrics] on
// your own instance if you need to override these defaults.
func NewCalculatorTool() *tool.Tool[Input, Output] {
	return tool.NewTool[Input, Output](
		"Calculator",
		Calc,
		tool.WithDescription("A simple calculator to perform basic arithmetic operations like addition, subtraction, multiplication, and division."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.0, // Free - local execution
			Currency:                "USD",
			CostDescription:         "local computation",
			Accuracy:                1.0, // 100% accuracy for mathematical operations
			AverageDurationInMillis: 2,   // Very fast - local operation
		}),
	)
}

// Calc performs the arithmetic operation specified by req.Op on the operands
// req.A and req.B. Supported operations are "add"/"+", "sub"/"-",
// "mul"/"*", and "div"/"/". Division by zero returns positive or negative
// infinity consistent with IEEE 754 floating-point semantics; no explicit
// error is returned for that case. An unrecognised Op value silently returns
// a result of 0.0.
//
// Example:
//
//	result, err := Calc(ctx, calculator.Input{A: 10, B: 4, Op: "div"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Result) // 2.5
func Calc(ctx context.Context, req Input) (Output, error) {
	var result float64
	switch req.Op {
	case "add", "+":
		result = req.A + req.B
	case "sub", "-":
		result = req.A - req.B
	case "mul", "*":
		result = req.A * req.B
	case "div", "/":
		result = req.A / req.B
	}
	return Output{Result: result}, nil
}

// Input holds the two operands and the operation to be applied by [Calc].
// Field names follow the JSON conventions expected by the LLM tool-call schema
// generated from the jsonschema tags.
type Input struct {
	A  float64 `json:"A"  jsonschema:"description=First integer operand,required"`
	B  float64 `json:"B"  jsonschema:"description=Second integer operand,required"`
	Op string  `json:"Op" jsonschema:"description=Operation type,enum=add,enum=sub,enum=mul,enum=div,required"`
}

// Output carries the single floating-point result produced by [Calc].
type Output struct {
	Result float64 `json:"result"  jsonschema:"description=The result of the calculation"`
}
