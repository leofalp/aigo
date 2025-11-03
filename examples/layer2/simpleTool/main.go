package main

import (
	"aigo/core/client"
	"aigo/providers/ai"
	"aigo/providers/ai/openai"
	"aigo/providers/memory/inmemory"
	"aigo/providers/observability"
	"aigo/providers/tool"
	"aigo/providers/tool/calculator"
	"context"
	"encoding/json"
	"fmt"
	"log"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== Manual Tool Execution Example (Layer 2) ===")
	fmt.Println("This example demonstrates how to manually handle tool calls")
	fmt.Println("at the client layer (without using higher-level patterns).")

	// Create memory provider that we can access directly
	memory := inmemory.New()

	// Create client with memory and calculator tool
	c, err := client.NewClient[string](
		openai.NewOpenAIProvider(),
		client.WithMemory(memory),
		client.WithTools(calculator.NewCalculatorTool()),
		client.WithSystemPrompt("You are a helpful math assistant. Use the calculator tool when needed."),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create tool catalog for manual execution
	toolCatalog := tool.NewCatalog()
	toolCatalog.AddTools(calculator.NewCalculatorTool())

	ctx := context.Background()
	userPrompt := "What is 3344 multiplied by 56?"

	fmt.Printf("User: %s\n\n", userPrompt)

	// Step 1: Send initial message to LLM
	fmt.Println("--- Step 1: Sending message to LLM ---")
	resp, err := c.SendMessage(ctx, userPrompt)
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	// Step 2: Check if LLM wants to use tools
	if len(resp.ToolCalls) == 0 {
		// No tool calls - LLM responded directly
		fmt.Printf("Assistant (direct): %s\n", resp.Content)
		fmt.Println("\nNo tool calls requested. Done.")
		return
	}

	fmt.Printf("✓ LLM wants to use %d tool(s)\n\n", len(resp.ToolCalls))

	// Step 3: Add assistant's response to memory manually
	// Note: The assistant's response indicates it wants to use tools
	fmt.Println("--- Step 2: Adding assistant's response to memory ---")
	assistantMsg := fmt.Sprintf("I need to use the calculator tool to compute this. Let me calculate %s.", userPrompt)
	memory.AppendMessage(ctx, &ai.Message{
		Role:    ai.RoleAssistant,
		Content: assistantMsg,
	})

	// Step 4: Execute tools manually and add results to memory
	fmt.Println("--- Step 3: Executing tools and adding results to memory ---")

	for i, toolCall := range resp.ToolCalls {
		fmt.Printf("\nTool Call %d:\n", i+1)
		fmt.Printf("  Tool: %s\n", toolCall.Function.Name)
		fmt.Printf("  Arguments: %s\n", toolCall.Function.Arguments)

		// Find the tool in our catalog
		toolInstance, exists := toolCatalog.Get(toolCall.Function.Name)
		if !exists {
			log.Printf("  ✗ Tool '%s' not found in catalog\n", toolCall.Function.Name)
			continue
		}

		// Execute the tool
		result, err := toolInstance.Call(ctx, toolCall.Function.Arguments)
		if err != nil {
			log.Printf("  ✗ Error executing tool: %v\n", err)
			result = fmt.Sprintf(`{"error": "%s"}`, err.Error())
		} else {
			fmt.Printf("  ✓ Result: %s\n", result)
		}

		// Add tool result to memory as a user message
		// (representing the tool's output being fed back into the conversation)
		toolResultMsg := fmt.Sprintf("Tool '%s' executed successfully. Result: %s", toolCall.Function.Name, result)
		memory.AppendMessage(ctx, &ai.Message{
			Role:    ai.RoleUser,
			Content: toolResultMsg,
		})
	}

	// Step 5: Send empty message to trigger LLM to process tool results
	// The LLM will see the conversation history with tool results and respond
	fmt.Println("\n--- Step 4: Asking LLM to process tool results ---")
	fmt.Println("Sending follow-up message to get final answer...")

	finalResp, err := c.SendMessage(ctx, "Please provide the final answer based on the tool results.")
	if err != nil {
		log.Fatalf("Failed to get final response: %v", err)
	}

	// Step 6: Display final answer
	fmt.Printf("\n=== Final Answer ===\n")
	fmt.Printf("Assistant: %s\n", finalResp.Content)

	// Show conversation history
	fmt.Printf("\n=== Conversation History ===\n")
	messages := memory.AllMessages()
	for i, msg := range messages {
		fmt.Printf("%d. [%s] %s\n", i+1, msg.Role, observability.TruncateString(msg.Content, 100))
	}

	// Summary
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("1. User asked: %s\n", userPrompt)
	fmt.Printf("2. LLM requested %d tool call(s)\n", len(resp.ToolCalls))
	fmt.Printf("3. We executed the tool(s) manually\n")
	fmt.Printf("4. We added tool results to memory\n")
	fmt.Printf("5. We asked LLM to process results\n")
	fmt.Printf("6. LLM provided final answer\n")
	fmt.Printf("\nNote: This manual approach demonstrates Layer 2 client usage.\n")
	fmt.Printf("For production, use Layer 3 patterns (e.g., ReAct) that automate this loop.\n")
}

// formatToolResult formats a tool result for display
func formatToolResult(toolName string, result string) string {
	var resultObj map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resultObj); err != nil {
		return fmt.Sprintf("Tool '%s' returned: %s", toolName, result)
	}

	formatted, err := json.MarshalIndent(resultObj, "", "  ")
	if err != nil {
		return fmt.Sprintf("Tool '%s' returned: %s", toolName, result)
	}

	return fmt.Sprintf("Tool '%s' returned:\n%s", toolName, string(formatted))
}
