package main

import (
	"context"
	"fmt"
	"log"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/ai/openai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
	"github.com/leofalp/aigo/providers/tool"
	"github.com/leofalp/aigo/providers/tool/calculator"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("=== Manual Tool Calling with Proper Message Linking ===")
	fmt.Println("This example demonstrates:")
	fmt.Println("- How to manually execute tool calls from the LLM")
	fmt.Println("- Using ContinueConversation() to process tool results")
	fmt.Println("- Proper message linking with ToolCallID and Name fields")
	fmt.Println("- Structured ToolResult for consistent error/success reporting")
	fmt.Println()
	fmt.Println("Key Pattern:")
	fmt.Println("1. Send user message with SendMessage()")
	fmt.Println("2. LLM responds with tool calls")
	fmt.Println("3. Execute tools and append results to memory")
	fmt.Println("4. Use ContinueConversation() to let LLM process results")
	fmt.Println()

	// Create memory provider
	memory := inmemory.New()

	// Create client with memory and tools
	c, err := client.New(
		openai.New(),
		client.WithMemory(memory),
		client.WithTools(calculator.NewCalculatorTool()),
		client.WithSystemPrompt("You are a helpful math assistant. Use the calculator tool when needed."),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	userPrompt := "What is 42 multiplied by 17?"
	fmt.Printf("User: %s\n\n", userPrompt)

	// Step 1: Send message to LLM
	// The LLM will analyze the question and decide if it needs to use tools
	fmt.Println("--- Step 1: LLM Response ---")
	resp, err := c.SendMessage(ctx, userPrompt)
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	// Step 2: Check for tool calls
	if len(resp.ToolCalls) == 0 {
		fmt.Printf("Assistant: %s\n", resp.Content)
		fmt.Println("\nNo tool calls - done!")
		return
	}

	fmt.Printf("LLM requested %d tool call(s)\n\n", len(resp.ToolCalls))

	// Step 3: Save assistant message WITH tool calls (NEW!)
	fmt.Println("--- Step 2: Saving Assistant Message with ToolCalls ---")
	// Important: Save the assistant's response including the tool calls
	// This maintains the complete conversation history for proper context
	assistantMsg := &ai.Message{
		Role:      ai.RoleAssistant,
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls, // Include tool call requests
	}
	memory.AppendMessage(ctx, assistantMsg)
	fmt.Printf("✓ Saved assistant message with %d tool call(s)\n\n", len(assistantMsg.ToolCalls))

	// Step 4: Execute tools and add results with proper linking
	fmt.Println("--- Step 3: Executing Tools with Proper Linking ---")
	// For each tool call, execute it and save the result with:
	// - ToolCallID: Links this result to the specific tool call
	// - Name: Identifies which tool generated this result

	toolCatalog := tool.NewCatalogWithTools(calculator.NewCalculatorTool())

	for i, toolCall := range resp.ToolCalls {
		fmt.Printf("Tool Call #%d:\n", i+1)
		fmt.Printf("  ID:        %s\n", toolCall.ID)
		fmt.Printf("  Tool:      %s\n", toolCall.Function.Name)
		fmt.Printf("  Arguments: %s\n", toolCall.Function.Arguments)

		// Find and execute tool
		toolInstance, exists := toolCatalog.Get(toolCall.Function.Name)
		if !exists {
			// Tool not found - return structured error using ToolResult
			fmt.Printf("  ✗ Tool not found!\n\n")

			toolResult := ai.NewToolResultError(
				"tool_not_found",
				fmt.Sprintf("Tool '%s' not found in catalog", toolCall.Function.Name),
			)
			resultJSON, err := toolResult.ToJSON()
			if err != nil {
				resultJSON = fmt.Sprintf(`{"error":"failed to serialize tool result: %s"}`, err.Error())
			}

			// Add error to memory with proper linking
			memory.AppendMessage(ctx, &ai.Message{
				Role:       ai.RoleTool,
				Content:    resultJSON,
				ToolCallID: toolCall.ID,            // Links this response to the tool call request
				Name:       toolCall.Function.Name, // Tool that generated this response
			})
			continue
		}

		// Execute tool
		result, err := toolInstance.Call(ctx, toolCall.Function.Arguments)
		if err != nil {
			// Execution error - return structured error using ToolResult
			fmt.Printf("  ✗ Execution failed: %v\n\n", err)

			toolResult := ai.NewToolResultError("tool_execution_failed", err.Error())
			resultJSON, err := toolResult.ToJSON()
			if err != nil {
				resultJSON = fmt.Sprintf(`{"error":"failed to serialize tool result: %s"}`, err.Error())
			}

			// Add error to memory with proper linking
			memory.AppendMessage(ctx, &ai.Message{
				Role:       ai.RoleTool,
				Content:    resultJSON,
				ToolCallID: toolCall.ID,            // Links this response to the tool call request
				Name:       toolCall.Function.Name, // Tool that generated this response
			})
			continue
		}

		fmt.Printf("  ✓ Result:  %s\n\n", result)

		// Add success result to memory with proper linking
		memory.AppendMessage(ctx, &ai.Message{
			Role:       ai.RoleTool,
			Content:    result,
			ToolCallID: toolCall.ID,            // Links this response to the tool call request
			Name:       toolCall.Function.Name, // Tool that generated this response
		})
	}

	// Step 5: Continue conversation to get final answer
	fmt.Println("--- Step 4: Getting Final Answer ---")
	fmt.Println("(Continuing conversation to process tool results)")
	// ContinueConversation() sends all messages in memory to the LLM
	// WITHOUT adding a new user message. This allows the LLM to process
	// the tool results and generate a final answer.
	//
	// Key difference from SendMessage(""):
	// - SendMessage("") would return an error
	// - ContinueConversation() is explicit and validated
	// - Only works with memory provider (stateful mode)
	finalResp, err := c.ContinueConversation(ctx)
	if err != nil {
		log.Fatalf("Failed to get final response: %v", err)
	}

	fmt.Printf("\n✓ Assistant: %s\n", finalResp.Content)

	if finalResp.Reasoning != "" {
		fmt.Printf("\n[Reasoning]: %s\n", truncate(finalResp.Reasoning, 500))
	}

	// Step 6: Show complete conversation with proper structure
	fmt.Println("\n\n--- Complete Conversation Structure ---")
	messages := memory.AllMessages()

	for i, msg := range messages {
		fmt.Printf("\nMessage #%d:\n", i+1)
		fmt.Printf("  Role:    %s\n", msg.Role)

		// Show content
		if msg.Content != "" {
			fmt.Printf("  Content: %s\n", truncate(msg.Content, 500))
		} else {
			fmt.Printf("  Content: (empty - continuing conversation)\n")
		}

		// Show reasoning if present
		if msg.Reasoning != "" {
			fmt.Printf("  Reasoning: %s\n", truncate(msg.Reasoning, 500))
		}

		// Show refusal if present
		if msg.Refusal != "" {
			fmt.Printf("  Refusal: %s\n", msg.Refusal)
		}

		// Show tool-related fields
		if len(msg.ToolCalls) > 0 {
			fmt.Printf("  ToolCalls:\n")
			for _, tc := range msg.ToolCalls {
				fmt.Printf("    - ID: %s, Tool: %s\n", tc.ID, tc.Function.Name)
			}
		}

		if msg.ToolCallID != "" {
			fmt.Printf("  ToolCallID: %s (links to request)\n", msg.ToolCallID)
		}

		if msg.Name != "" {
			fmt.Printf("  Name: %s (tool that generated this)\n", msg.Name)
		}
	}

	// Summary
	fmt.Println("=== Summary ===")
	fmt.Println("✓ Assistant message saved with ToolCalls field")
	fmt.Println("✓ Tool responses linked via ToolCallID")
	fmt.Println("✓ Tool names specified via Name field")
	fmt.Println("✓ Errors returned as structured ToolResult")
	fmt.Println("✓ Reasoning extracted from <think> tags (if present)")
	fmt.Println("✓ ContinueConversation() explicitly continues without new user message")
	fmt.Println("✓ SendMessage(\"\") now returns clear error suggesting ContinueConversation()")
	fmt.Println("✓ Complete request-response traceability")
	fmt.Println("\nThis enables:")
	fmt.Println("- Proper conversation history replay")
	fmt.Println("- Better LLM understanding of tool results")
	fmt.Println("- Chain-of-thought reasoning visibility")
	fmt.Println("- Full observability and debugging")
	fmt.Println("- Support for parallel tool calls")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
