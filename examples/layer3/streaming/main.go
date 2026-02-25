// Package main demonstrates the streaming ReAct pattern (Layer 3) with ExecuteStream().
// It runs a math agent that streams intermediate reasoning steps, tool calls, and
// the final answer token-by-token — giving real-time visibility into the agent's work.
// Requires the OPENAI_API_KEY environment variable.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/patterns/react"
	"github.com/leofalp/aigo/providers/ai/openai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
	"github.com/leofalp/aigo/providers/tool/calculator"

	_ "github.com/joho/godotenv/autoload"
)

// MathResult is the strongly-typed output the agent must produce.
type MathResult struct {
	Answer      int    `json:"answer"      jsonschema:"required,description=The numerical answer"`
	Explanation string `json:"explanation" jsonschema:"required,description=Step-by-step explanation"`
}

func main() {
	fmt.Println("=== Streaming ReAct Pattern (Layer 3) ===")
	fmt.Println()

	ctx := context.Background()

	// Set up a memory-backed client with a calculator tool.
	// ExecuteStream requires memory just like Execute.
	baseClient, err := client.New(
		openai.New(),
		client.WithMemory(inmemory.New()),
		client.WithTools(calculator.NewCalculatorTool()),
		client.WithSystemPrompt("You are a helpful math assistant."),
		client.WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	// Create a type-safe ReAct agent.
	agent, err := react.New[MathResult](
		baseClient,
		react.WithMaxIterations(5),
	)
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}

	prompt := "What is the sum of the squares of the first 4 prime numbers? Show your working."
	fmt.Printf("User: %s\n\n", prompt)

	// ExecuteStream returns immediately. The iterator drives the loop.
	stream, err := agent.ExecuteStream(ctx, prompt)
	if err != nil {
		log.Fatalf("ExecuteStream failed: %v", err)
	}

	var finalResult *MathResult

	// Iterate over the stream. Each event describes one phase of the agent's work.
	for event, iterErr := range stream.Iter() {
		if iterErr != nil {
			fmt.Fprintf(os.Stderr, "\nstream error: %v\n", iterErr)
			os.Exit(1)
		}

		switch event.Type {
		case react.ReactEventIterationStart:
			fmt.Printf("\n--- Step %d ---\n", event.Iteration)

		case react.ReactEventReasoning:
			// Reasoning deltas arrive token-by-token from models that expose chain-of-thought.
			fmt.Print(event.Reasoning)

		case react.ReactEventContent:
			// Content deltas produce a typewriter effect.
			fmt.Print(event.Content)

		case react.ReactEventToolCall:
			fmt.Printf("\n[Calling tool: %s  args: %s]\n", event.ToolName, event.ToolInput)

		case react.ReactEventToolResult:
			fmt.Printf("[Tool result: %s]\n", event.ToolOutput)

		case react.ReactEventFinalAnswer:
			// FinalAnswer carries the fully parsed, strongly-typed result.
			finalResult = event.Result
		}
	}

	// Print the structured final answer.
	if finalResult != nil {
		fmt.Println()
		fmt.Println()
		fmt.Println("--- Final Answer ---")
		fmt.Printf("  Answer:      %d\n", finalResult.Answer)
		fmt.Printf("  Explanation: %s\n", finalResult.Explanation)
	}
}
