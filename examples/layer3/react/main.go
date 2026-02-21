// Package main demonstrates the type-safe ReAct pattern (Layer 3) with a math agent returning
// MathResult, a research agent returning ResearchResult, and an untyped string agent for
// backward compatibility. All agents run an automatic tool-execution loop.
// Requires the OPENAI_API_KEY environment variable.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/patterns/react"
	"github.com/leofalp/aigo/providers/ai/openai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
	"github.com/leofalp/aigo/providers/observability/slogobs"
	"github.com/leofalp/aigo/providers/tool/calculator"
	"github.com/leofalp/aigo/providers/tool/duckduckgo"

	_ "github.com/joho/godotenv/autoload"
)

// MathResult represents a structured math calculation result.
type MathResult struct {
	Answer      int    `json:"answer" jsonschema:"required,description=The numerical answer"`
	Explanation string `json:"explanation" jsonschema:"required,description=Step-by-step explanation"`
	Confidence  string `json:"confidence" jsonschema:"required,enum=high|medium|low,description=Confidence level"`
}

// ResearchResult represents a structured research result.
type ResearchResult struct {
	Topic       string   `json:"topic" jsonschema:"required,description=Main research topic"`
	Summary     string   `json:"summary" jsonschema:"required,description=Brief summary of findings"`
	KeyPoints   []string `json:"key_points" jsonschema:"required,description=List of key points"`
	Sources     int      `json:"sources" jsonschema:"required,description=Number of sources consulted"`
	Reliability string   `json:"reliability" jsonschema:"required,enum=high|medium|low,description=Information reliability"`
}

func main() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          Type-Safe ReAct Pattern Example (Layer 3)           ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("This example demonstrates the ReAct (Reason + Act) pattern with")
	fmt.Println("automatic tool execution loop and type-safe structured output.")
	fmt.Println()

	ctx := context.Background()

	// Example 1: Simple typed math agent
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Example 1: Type-Safe Math Agent")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	exampleMathAgent(ctx)

	fmt.Println()
	fmt.Println()

	// Example 2: Typed research agent
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Example 2: Type-Safe Research Agent")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	exampleResearchAgent(ctx)

	fmt.Println()
	fmt.Println()

	// Example 3: Untyped (backward compatibility)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Example 3: Untyped (Backward Compatibility)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	exampleUntyped(ctx)
}

// exampleMathAgent demonstrates type-safe ReAct with calculator tool.
func exampleMathAgent(ctx context.Context) {
	// Setup
	memory := inmemory.New()
	calcTool := calculator.NewCalculatorTool()

	baseClient, err := client.New(
		openai.New(),
		client.WithMemory(memory),
		client.WithObserver(slogobs.New()),
		client.WithTools(calcTool),
		client.WithSystemPrompt("You are a helpful math assistant."),
		client.WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create type-safe agent
	agent, err := react.New[MathResult](
		baseClient,
		react.WithMaxIterations(10),
		react.WithStopOnError(true),
	)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	prompt := "What is the sum of the first 5 prime numbers? Explain step by step."
	fmt.Printf("User: %s\n\n", prompt)

	// Execute - returns type-safe result
	result, err := agent.Execute(ctx, prompt)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	// Type-safe access to structured data
	fmt.Println("✓ Structured Result:")
	fmt.Printf("  Answer: %d\n", result.Data.Answer)
	fmt.Printf("  Explanation: %s\n", result.Data.Explanation)
	fmt.Printf("  Confidence: %s\n", result.Data.Confidence)
	fmt.Println()

	// Execution statistics
	fmt.Println("Execution Statistics:")
	fmt.Printf("  Total Tokens: %d (prompt: %d, completion: %d)\n",
		result.TotalUsage.TotalTokens,
		result.TotalUsage.PromptTokens,
		result.TotalUsage.CompletionTokens,
	)
	fmt.Printf("  Tools Used: %s\n", utils.ToString(result.ToolCallStats))
	if result.ModelCost != nil {
		fmt.Printf("  Total Cost: $%.4f\n", result.TotalCost())
	}
}

// exampleResearchAgent demonstrates type-safe ReAct with search tool.
func exampleResearchAgent(ctx context.Context) {
	// Setup
	memory := inmemory.New()
	searchTool := duckduckgo.NewDuckDuckGoSearchTool()

	baseClient, err := client.New(
		openai.New(),
		client.WithMemory(memory),
		client.WithObserver(slogobs.New()),
		client.WithTools(searchTool),
		client.WithSystemPrompt("You are a research assistant."),
		client.WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create type-safe agent with different type
	agent, err := react.New[ResearchResult](
		baseClient,
		react.WithMaxIterations(5),
	)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	prompt := "Research the latest developments in quantum computing"
	fmt.Printf("User: %s\n\n", prompt)

	// Execute
	result, err := agent.Execute(ctx, prompt)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	// Type-safe access
	fmt.Println("✓ Structured Research Result:")
	fmt.Printf("  Topic: %s\n", result.Data.Topic)
	fmt.Printf("  Summary: %s\n", result.Data.Summary)
	fmt.Printf("  Key Points (%d):\n", len(result.Data.KeyPoints))
	for i, point := range result.Data.KeyPoints {
		fmt.Printf("    %d. %s\n", i+1, point)
	}
	fmt.Printf("  Sources: %d\n", result.Data.Sources)
	fmt.Printf("  Reliability: %s\n", result.Data.Reliability)
	fmt.Println()

	fmt.Println("Execution Statistics:")
	fmt.Printf("  Total Tokens: %d\n", result.TotalUsage.TotalTokens)
	fmt.Printf("  Tools Used: %s\n", utils.ToString(result.ToolCallStats))
}

// exampleUntyped demonstrates backward compatibility with untyped results.
func exampleUntyped(ctx context.Context) {
	// Setup
	memory := inmemory.New()
	calcTool := calculator.NewCalculatorTool()
	searchTool := duckduckgo.NewDuckDuckGoSearchTool()

	baseClient, err := client.New(
		openai.New(),
		client.WithMemory(memory),
		client.WithObserver(slogobs.New()),
		client.WithTools(calcTool, searchTool),
		client.WithSystemPrompt("You are a helpful assistant."),
		client.WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Using string for untyped text results (default)
	agent, err := react.New[string](
		baseClient,
		react.WithMaxIterations(10),
	)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	prompt := "What's the sum of the first 3 prime numbers? Then tell me what country has that many letters in its capital city name."
	fmt.Printf("User: %s\n\n", prompt)

	result, err := agent.Execute(ctx, prompt)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	// Untyped string response
	fmt.Println("✓ Result:")
	fmt.Printf("  Response: %s\n", *result.Data)
	fmt.Println()

	fmt.Println("Execution Statistics:")
	fmt.Printf("  Tokens Used: %d\n", result.TotalUsage.TotalTokens)
	fmt.Printf("  Tools Used: %s\n", utils.ToString(result.ToolCallStats))
	fmt.Println()

	// Show conversation history
	fmt.Println("Conversation History:")
	messages := memory.AllMessages()
	fmt.Printf("  Total messages: %d\n", len(messages))
}
