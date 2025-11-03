package client

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
)

// TestClient_SendMessage_EmptyPrompt_ReturnsError tests that SendMessage rejects empty prompts
func TestClient_SendMessage_EmptyPrompt_ReturnsError(t *testing.T) {
	provider := &mockProvider{}
	client, err := NewClient[string](provider, WithMemory(inmemory.New()))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendMessage(ctx, "")

	if err == nil {
		t.Fatal("Expected error when sending empty prompt, got nil")
	}

	expectedMsg := "prompt cannot be empty"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain '%s', got: %s", expectedMsg, err.Error())
	}

	expectedSuggestion := "ContinueConversation()"
	if !strings.Contains(err.Error(), expectedSuggestion) {
		t.Errorf("Expected error to suggest '%s', got: %s", expectedSuggestion, err.Error())
	}
}

// TestClient_SendMessage_EmptyPrompt_StatelessMode tests empty prompt in stateless mode
func TestClient_SendMessage_EmptyPrompt_StatelessMode(t *testing.T) {
	provider := &mockProvider{}
	// Stateless client (no memory)
	client, err := NewClient[string](provider)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendMessage(ctx, "")

	if err == nil {
		t.Fatal("Expected error when sending empty prompt in stateless mode, got nil")
	}
}

// TestClient_ContinueConversation_Success tests successful conversation continuation
func TestClient_ContinueConversation_Success(t *testing.T) {
	var capturedRequest ai.ChatRequest
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			capturedRequest = req
			return &ai.ChatResponse{
				Id:           "test-id",
				Model:        "test-model",
				Content:      "Final answer based on tool results",
				FinishReason: "stop",
			}, nil
		},
	}

	memory := inmemory.New()
	client, err := NewClient[string](provider, WithMemory(memory))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	// Simulate a conversation with tool execution:
	// 1. User asks question
	memory.AppendMessage(ctx, &ai.Message{
		Role:    ai.RoleUser,
		Content: "What is 2+2?",
	})

	// 2. Simulate assistant response with tool call
	memory.AppendMessage(ctx, &ai.Message{
		Role:    ai.RoleAssistant,
		Content: "Let me calculate that",
		ToolCalls: []ai.ToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: ai.ToolCallFunction{
					Name:      "calculator",
					Arguments: `{"operation":"add","a":2,"b":2}`,
				},
			},
		},
	})

	// 3. Simulate tool result
	memory.AppendMessage(ctx, &ai.Message{
		Role:       ai.RoleTool,
		Content:    "4",
		ToolCallID: "call_123",
		Name:       "calculator",
	})

	// 4. Continue conversation to get final answer
	resp, err := client.ContinueConversation(ctx)
	if err != nil {
		t.Fatalf("ContinueConversation failed: %v", err)
	}

	// Verify response
	if resp.Content != "Final answer based on tool results" {
		t.Errorf("Expected specific content, got: %s", resp.Content)
	}

	// Verify that all messages were sent (including tool results)
	if len(capturedRequest.Messages) != 3 {
		t.Errorf("Expected 3 messages in request (user, assistant with toolcalls, tool result), got %d", len(capturedRequest.Messages))
	}

	// Verify the last message in memory before ContinueConversation was the tool result
	allMessages := memory.AllMessages()
	if len(allMessages) < 3 {
		t.Fatalf("Expected at least 3 messages in memory, got %d", len(allMessages))
	}
	lastBeforeContinue := allMessages[2] // The tool result
	if lastBeforeContinue.Role != ai.RoleTool {
		t.Errorf("Expected last message before continuation to be tool result, got: %s", lastBeforeContinue.Role)
	}
}

// TestClient_ContinueConversation_WithoutMemory_ReturnsError tests that ContinueConversation requires memory
func TestClient_ContinueConversation_WithoutMemory_ReturnsError(t *testing.T) {
	provider := &mockProvider{}
	// Stateless client (no memory)
	client, err := NewClient[string](provider)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.ContinueConversation(ctx)

	if err == nil {
		t.Fatal("Expected error when calling ContinueConversation without memory, got nil")
	}

	expectedMsg := "ContinueConversation requires a memory provider"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain '%s', got: %s", expectedMsg, err.Error())
	}

	expectedSuggestion := "WithMemory()"
	if !strings.Contains(err.Error(), expectedSuggestion) {
		t.Errorf("Expected error to suggest '%s', got: %s", expectedSuggestion, err.Error())
	}
}

// TestClient_ContinueConversation_ProviderError tests error handling
func TestClient_ContinueConversation_ProviderError(t *testing.T) {
	testError := errors.New("provider error")
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			return nil, testError
		},
	}

	memory := inmemory.New()
	client, err := NewClient[string](provider, WithMemory(memory))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Add some messages to memory
	ctx := context.Background()
	memory.AppendMessage(ctx, &ai.Message{
		Role:    ai.RoleUser,
		Content: "Hello",
	})

	_, err = client.ContinueConversation(ctx)

	if err == nil {
		t.Fatal("Expected error from provider, got nil")
	}

	if !errors.Is(err, testError) {
		t.Errorf("Expected wrapped test error, got: %v", err)
	}
}

// TestClient_ContinueConversation_EmptyMemory tests continuation with empty memory
func TestClient_ContinueConversation_EmptyMemory(t *testing.T) {
	var capturedRequest ai.ChatRequest
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			capturedRequest = req
			return &ai.ChatResponse{
				Id:           "test-id",
				Model:        "test-model",
				Content:      "Response",
				FinishReason: "stop",
			}, nil
		},
	}

	memory := inmemory.New()
	client, err := NewClient[string](provider, WithMemory(memory))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	// Continue with empty memory (edge case)
	_, err = client.ContinueConversation(ctx)
	if err != nil {
		t.Fatalf("ContinueConversation with empty memory failed: %v", err)
	}

	// Verify that an empty message list was sent
	if len(capturedRequest.Messages) != 0 {
		t.Errorf("Expected 0 messages in request with empty memory, got %d", len(capturedRequest.Messages))
	}
}

// TestClient_ContinueConversation_PreservesMessages tests that ContinueConversation doesn't modify memory
func TestClient_ContinueConversation_PreservesMessages(t *testing.T) {
	provider := &mockProvider{}

	memory := inmemory.New()
	client, err := NewClient[string](provider, WithMemory(memory))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	// Add initial messages
	memory.AppendMessage(ctx, &ai.Message{
		Role:    ai.RoleUser,
		Content: "Question",
	})
	memory.AppendMessage(ctx, &ai.Message{
		Role:    ai.RoleAssistant,
		Content: "Answer",
	})

	initialCount := memory.Count()

	// Continue conversation
	_, err = client.ContinueConversation(ctx)
	if err != nil {
		t.Fatalf("ContinueConversation failed: %v", err)
	}

	// Verify message count didn't change (no user message added)
	if memory.Count() != initialCount {
		t.Errorf("Expected message count to remain %d, got %d", initialCount, memory.Count())
	}
}

// TestClient_ToolExecutionWorkflow tests a complete tool execution workflow
func TestClient_ToolExecutionWorkflow(t *testing.T) {
	callCount := 0
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			callCount++
			if callCount == 1 {
				// First call: return tool call request
				return &ai.ChatResponse{
					Id:           "test-id-1",
					Model:        "test-model",
					Content:      "Let me search for that",
					FinishReason: "tool_calls",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: ai.ToolCallFunction{
								Name:      "search",
								Arguments: `{"query":"golang"}`,
							},
						},
					},
				}, nil
			}
			// Second call: return final answer
			return &ai.ChatResponse{
				Id:           "test-id-2",
				Model:        "test-model",
				Content:      "Based on the search results, here's the answer",
				FinishReason: "stop",
			}, nil
		},
	}

	memory := inmemory.New()
	client, err := NewClient[string](provider, WithMemory(memory))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	// Step 1: User sends initial message
	resp1, err := client.SendMessage(ctx, "Tell me about golang")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if len(resp1.ToolCalls) == 0 {
		t.Fatal("Expected tool calls in first response")
	}

	// Step 2: Save assistant message with tool calls
	memory.AppendMessage(ctx, &ai.Message{
		Role:      ai.RoleAssistant,
		Content:   resp1.Content,
		ToolCalls: resp1.ToolCalls,
	})

	// Step 3: Execute tool and save result
	memory.AppendMessage(ctx, &ai.Message{
		Role:       ai.RoleTool,
		Content:    `{"results": "Go is a programming language..."}`,
		ToolCallID: resp1.ToolCalls[0].ID,
		Name:       resp1.ToolCalls[0].Function.Name,
	})

	// Step 4: Continue conversation to get final answer
	resp2, err := client.ContinueConversation(ctx)
	if err != nil {
		t.Fatalf("ContinueConversation failed: %v", err)
	}

	if resp2.FinishReason != "stop" {
		t.Errorf("Expected stop finish reason, got: %s", resp2.FinishReason)
	}

	if !strings.Contains(resp2.Content, "answer") {
		t.Errorf("Expected final answer in response, got: %s", resp2.Content)
	}

	// Step 5: Manually save the final assistant response to memory
	// (ContinueConversation doesn't auto-save the response)
	memory.AppendMessage(ctx, &ai.Message{
		Role:    ai.RoleAssistant,
		Content: resp2.Content,
	})

	// Verify complete message history
	allMessages := memory.AllMessages()
	expectedMessageCount := 4 // user + assistant + tool + assistant
	if len(allMessages) != expectedMessageCount {
		t.Errorf("Expected %d messages in history, got %d", expectedMessageCount, len(allMessages))
	}

	// Verify message sequence
	if allMessages[0].Role != ai.RoleUser {
		t.Errorf("Expected first message to be user, got: %s", allMessages[0].Role)
	}
	if allMessages[1].Role != ai.RoleAssistant {
		t.Errorf("Expected second message to be assistant, got: %s", allMessages[1].Role)
	}
	if allMessages[2].Role != ai.RoleTool {
		t.Errorf("Expected third message to be tool, got: %s", allMessages[2].Role)
	}
	if allMessages[3].Role != ai.RoleAssistant {
		t.Errorf("Expected fourth message to be assistant, got: %s", allMessages[3].Role)
	}
}
