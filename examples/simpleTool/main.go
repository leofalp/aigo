package main

import (
	"aigo/cmd/provider/openai"
	"aigo/cmd/tool"
	"aigo/pkg/client"
	"context"
	"fmt"
)

func main() {

	calculatorTool := tool.NewTool[calculatorInput, calculatorOutput](
		"Calculator",
		"A simple calculator to perform basic arithmetic operations like addition, subtraction, multiplication, and division.",
		calculator,
	)

	openrouter := client.NewClient(
		openai.NewOpenAIProvider().
			WithBaseURL("https://openrouter.ai/api/v1").
			WithModel("openrouter/andromeda-alpha"),
	)
	err := openrouter.AddTools([]tool.DocumentedTool{calculatorTool})
	if err != nil {
		panic(err)
	}

	openrouter.AddSystemPrompt("You are a helpful assistant.")

	resp, err := openrouter.SendMessage("Hello, how can you assist me today?")
	if err != nil {
		panic(err)
	}
	fmt.Println(resp)
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
