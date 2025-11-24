package calculator

import (
	"context"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/providers/tool"
)

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

type Input struct {
	A  float64 `json:"A"  jsonschema:"description=First integer operand,required"`
	B  float64 `json:"B"  jsonschema:"description=Second integer operand,required"`
	Op string  `json:"Op" jsonschema:"description=Operation type,enum=add,enum=sub,enum=mul,enum=div,required"`
}

type Output struct {
	Result float64 `json:"result"  jsonschema:"description=The result of the calculation"`
}
