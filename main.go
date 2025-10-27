package main

import (
	"context"
)

func main() {

	calculatorTool := NewTool[calculatorInput, calculatorOutput](
		"Calculator",
		"A simple calculator to perform basic arithmetic operations like addition, subtraction, multiplication, and division.",
		calculator,
	)

	client := NewClient("your_api_key", "gpt-4o-mini")
	err := client.AddTools([]DocumentedTool{calculatorTool})
	if err != nil {
		panic(err)
	}

	client.AddSystemPrompt("You are a helpful assistant.")
	err = client.SendMessage("Hello, how can you assist me today?")
	if err != nil {
		panic(err)
	}
}

func calculator(ctx context.Context, req calculatorInput) (calculatorOutput, error) {
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
	return calculatorOutput{Result: result}, nil
}

type calculatorInput struct {
	A  float64 `json:"A"  jsonschema:"description=First integer operand,required"`
	B  float64 `json:"B"  jsonschema:"description=Second integer operand,required"`
	Op string  `json:"Op" jsonschema:"description=Operation type,enum=add,enum=sub,enum=mul,enum=div,required"`
}

type calculatorOutput struct {
	Result float64 `json:"result"`
}
