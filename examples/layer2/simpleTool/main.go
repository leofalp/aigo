package main

import (
	"aigo/core/client"
	"aigo/providers/ai/openai"
	"aigo/providers/tool"
	"aigo/providers/tool/calculator"
	"fmt"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	client1 := client.NewClient[string](openai.NewOpenAIProvider(), client.WithDefaultModel("nvidia/nemotron-nano-9b-v2:free")).
		AddTools([]tool.GenericTool{calculator.NewCalculatorTool()}).
		AddSystemPrompt("You are a helpful assistant.")
	//SetOutputFormat(jsonschema.GenerateJSONSchema[calculator.Output]())      // Optional free response otherwise

	resp, err := client1.SendMessage("3344*56")
	if err != nil {
		panic(err)
	}
	fmt.Println(resp)
}
