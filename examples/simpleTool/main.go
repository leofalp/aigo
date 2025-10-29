package main

import (
	"aigo/core/client"
	"aigo/providers/ai/openai"
	"aigo/providers/tool"
	"aigo/providers/tool/calculator"
	"fmt"
)

func main() {
	openrouter := client.NewClient[string](
		openai.NewOpenAIProvider().
			WithBaseURL("https://openrouter.ai/api/v1"),
	).
		AddTools([]tool.GenericTool{calculator.NewCalculatorTool()}).
		AddSystemPrompt("You are a helpful assistant.").
		SetMaxToolCallIterations(5) // Optional, default is 3
	//SetOutputFormat(jsonschema.GenerateJSONSchema[calculator.Output]())      // Optional free response otherwise

	resp, err := openrouter.SendMessage("3344*56")
	if err != nil {
		panic(err)
	}
	fmt.Println(resp)
}
