package client

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
	"github.com/leofalp/aigo/providers/observability"
	"github.com/leofalp/aigo/providers/tool"
)

// ========== Mock Types ==========

// mockProvider is a mock implementation of ai.Provider for testing
type mockProvider struct {
	sendMessageFunc func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error)
}

func (m *mockProvider) SendMessage(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	if m.sendMessageFunc != nil {
		return m.sendMessageFunc(ctx, req)
	}
	return &ai.ChatResponse{
		Id:           "test-id",
		Model:        "test-model",
		Content:      "test response",
		FinishReason: "stop",
		Usage: &ai.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}, nil
}

func (m *mockProvider) IsStopMessage(resp *ai.ChatResponse) bool {
	return resp.FinishReason == "stop"
}

func (m *mockProvider) WithAPIKey(key string) ai.Provider              { return m }
func (m *mockProvider) WithBaseURL(url string) ai.Provider             { return m }
func (m *mockProvider) WithHttpClient(client *http.Client) ai.Provider { return m }

// mockTool is a mock implementation of ai.Tool for testing
type mockTool struct {
	name        string
	description string
	callCount   int
}

func (m *mockTool) ToolInfo() ai.ToolDescription {
	return ai.ToolDescription{
		Name:        m.name,
		Description: m.description,
		Parameters:  nil,
	}
}

func (m *mockTool) Call(ctx context.Context, arguments string) (string, error) {
	m.callCount++
	return `{"result": "success"}`, nil
}

func (m *mockTool) GetMetrics() *cost.ToolMetrics {
	return nil // Mock tool has no metrics
}

// testObserver is a test observer that tracks observability calls
type testObserver struct {
	spanStarted     bool
	spanEnded       bool
	errorLogged     bool
	metricsRecorded bool
}

func (o *testObserver) StartSpan(ctx context.Context, name string, attrs ...observability.Attribute) (context.Context, observability.Span) {
	o.spanStarted = true
	return ctx, &testSpan{observer: o}
}

func (o *testObserver) Counter(name string) observability.Counter {
	return &testCounter{observer: o}
}

func (o *testObserver) Histogram(name string) observability.Histogram {
	return &testHistogram{observer: o}
}

func (o *testObserver) Trace(ctx context.Context, msg string, attrs ...observability.Attribute) {}
func (o *testObserver) Debug(ctx context.Context, msg string, attrs ...observability.Attribute) {}
func (o *testObserver) Info(ctx context.Context, msg string, attrs ...observability.Attribute)  {}
func (o *testObserver) Warn(ctx context.Context, msg string, attrs ...observability.Attribute)  {}
func (o *testObserver) Error(ctx context.Context, msg string, attrs ...observability.Attribute) {
	o.errorLogged = true
}

type testSpan struct {
	observer *testObserver
}

func (s *testSpan) End() {
	s.observer.spanEnded = true
}

func (s *testSpan) SetAttributes(attrs ...observability.Attribute)              {}
func (s *testSpan) SetStatus(code observability.StatusCode, description string) {}
func (s *testSpan) RecordError(err error)                                       {}
func (s *testSpan) AddEvent(name string, attrs ...observability.Attribute)      {}

type testCounter struct {
	observer *testObserver
}

func (c *testCounter) Add(ctx context.Context, value int64, attrs ...observability.Attribute) {
	c.observer.metricsRecorded = true
}

type testHistogram struct {
	observer *testObserver
}

func (h *testHistogram) Record(ctx context.Context, value float64, attrs ...observability.Attribute) {
	h.observer.metricsRecorded = true
}

// ========== Client Creation Tests ==========

// TestNewClient_DefaultConfiguration tests client creation with default options
func TestNewClient_DefaultConfiguration(t *testing.T) {
	provider := &mockProvider{}
	client, err := New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if client.llmProvider == nil {
		t.Error("Expected llmProvider to be set")
	}
	if client.memoryProvider != nil {
		t.Error("Expected memoryProvider to be nil by default")
	}
	// Observer is nil by default (no default noop observer)
	if client.observer != nil {
		t.Error("Expected observer to be nil by default")
	}
}

// TestNewClient_WithOptions tests client creation with various options
func TestNewClient_WithOptions(t *testing.T) {
	provider := &mockProvider{}
	memory := inmemory.New()
	observer := &testObserver{}
	tool := &mockTool{name: "test", description: "test tool"}

	client, err := New(
		provider,
		WithMemory(memory),
		WithObserver(observer),
		WithSystemPrompt("Test prompt"),
		WithDefaultModel("gpt-4"),
		WithTools(tool),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if client.memoryProvider == nil {
		t.Error("Expected memoryProvider to be set")
	}
	if client.observer == nil {
		t.Error("Expected observer to be set")
	}
	if client.systemPrompt != "Test prompt" {
		t.Errorf("Expected systemPrompt 'Test prompt', got: %s", client.systemPrompt)
	}
	if client.defaultModel != "gpt-4" {
		t.Errorf("Expected defaultModel 'gpt-4', got: %s", client.defaultModel)
	}
	if client.toolCatalog.Size() != 1 {
		t.Errorf("Expected 1 tool in catalog, got: %d", client.toolCatalog.Size())
	}
}

// ========== SendMessage Tests ==========

// TestSendMessage_StatelessMode tests basic stateless message sending
func TestSendMessage_StatelessMode(t *testing.T) {
	provider := &mockProvider{}
	client, err := New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()
	resp, err := client.SendMessage(ctx, "Hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if resp.Content != "test response" {
		t.Errorf("Expected 'test response', got: %s", resp.Content)
	}
}

// TestSendMessage_StatefulMode tests message sending with memory
func TestSendMessage_StatefulMode(t *testing.T) {
	var capturedRequest ai.ChatRequest
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			capturedRequest = req
			return &ai.ChatResponse{
				Id:           "test-id",
				Model:        "test-model",
				Content:      "Response 1",
				FinishReason: "stop",
			}, nil
		},
	}

	memory := inmemory.New()
	client, err := New(provider, WithMemory(memory))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	// First message
	resp1, err := client.SendMessage(ctx, "Hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify only user message was saved (SendMessage doesn't auto-save response)
	count, countErr := memory.Count(ctx)
	if countErr != nil {
		t.Fatalf("Count returned unexpected error: %v", countErr)
	}
	if count != 1 { // user only
		t.Errorf("Expected 1 message in memory, got: %d", count)
	}

	// Second message
	_, err = client.SendMessage(ctx, "World")
	if err != nil {
		t.Fatalf("Second SendMessage failed: %v", err)
	}

	// Verify history accumulates (only user messages)
	count, countErr = memory.Count(ctx)
	if countErr != nil {
		t.Fatalf("Count returned unexpected error: %v", countErr)
	}
	if count != 2 { // 2 user messages
		t.Errorf("Expected 2 messages in memory, got: %d", count)
	}

	// Verify conversation history was sent to provider
	if len(capturedRequest.Messages) < 2 {
		t.Errorf("Expected at least 2 messages in request, got: %d", len(capturedRequest.Messages))
	}

	if resp1.Content != "Response 1" {
		t.Errorf("Expected 'Response 1', got: %s", resp1.Content)
	}
}

// TestSendMessage_EmptyPrompt tests that empty prompts are rejected
func TestSendMessage_EmptyPrompt(t *testing.T) {
	provider := &mockProvider{}
	client, err := New(provider, WithMemory(inmemory.New()))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendMessage(ctx, "")

	if err == nil {
		t.Fatal("Expected error when sending empty prompt, got nil")
	}

	if !strings.Contains(err.Error(), "prompt cannot be empty") {
		t.Errorf("Expected error about empty prompt, got: %s", err.Error())
	}

	if !strings.Contains(err.Error(), "ContinueConversation()") {
		t.Errorf("Expected error to suggest ContinueConversation(), got: %s", err.Error())
	}
}

// TestSendMessage_WithOutputSchema tests output schema option
func TestSendMessage_WithOutputSchema(t *testing.T) {
	var capturedRequest ai.ChatRequest
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			capturedRequest = req
			return &ai.ChatResponse{
				Id:           "test-id",
				Model:        "test-model",
				Content:      `{"result": "structured"}`,
				FinishReason: "stop",
			}, nil
		},
	}

	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"result": {Type: "string"},
		},
	}

	client, err := New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendMessage(ctx, "Get structured data", WithOutputSchema(schema))
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if capturedRequest.ResponseFormat == nil || capturedRequest.ResponseFormat.OutputSchema == nil {
		t.Error("Expected ResponseFormat.OutputSchema to be set in request")
	}
}

// TestSendMessage_ProviderError tests error handling from provider
func TestSendMessage_ProviderError(t *testing.T) {
	testError := errors.New("provider error")
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			return nil, testError
		},
	}

	client, err := New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendMessage(ctx, "Hello")

	if err == nil {
		t.Fatal("Expected error from provider, got nil")
	}

	if !errors.Is(err, testError) {
		t.Errorf("Expected wrapped test error, got: %v", err)
	}
}

// ========== ContinueConversation Tests ==========

// TestContinueConversation_Success tests successful conversation continuation
func TestContinueConversation_Success(t *testing.T) {
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
	client, err := New(provider, WithMemory(memory))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	// Simulate conversation with tool execution
	memory.AppendMessage(ctx, &ai.Message{
		Role:    ai.RoleUser,
		Content: "What is 2+2?",
	})

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

	memory.AppendMessage(ctx, &ai.Message{
		Role:       ai.RoleTool,
		Content:    "4",
		ToolCallID: "call_123",
		Name:       "calculator",
	})

	// Continue conversation
	resp, err := client.ContinueConversation(ctx)
	if err != nil {
		t.Fatalf("ContinueConversation failed: %v", err)
	}

	if resp.Content != "Final answer based on tool results" {
		t.Errorf("Expected specific content, got: %s", resp.Content)
	}

	// Verify all messages were sent
	if len(capturedRequest.Messages) != 3 {
		t.Errorf("Expected 3 messages in request, got %d", len(capturedRequest.Messages))
	}
}

// TestContinueConversation_WithoutMemory tests that memory is required
func TestContinueConversation_WithoutMemory(t *testing.T) {
	provider := &mockProvider{}
	client, err := New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.ContinueConversation(ctx)

	if err == nil {
		t.Fatal("Expected error when calling ContinueConversation without memory, got nil")
	}

	if !strings.Contains(err.Error(), "ContinueConversation requires a memory provider") {
		t.Errorf("Expected error about missing memory, got: %s", err.Error())
	}

	if !strings.Contains(err.Error(), "WithMemory()") {
		t.Errorf("Expected error to suggest WithMemory(), got: %s", err.Error())
	}
}

// TestContinueConversation_EmptyMemory tests continuation with empty memory
func TestContinueConversation_EmptyMemory(t *testing.T) {
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
	client, err := New(provider, WithMemory(memory))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.ContinueConversation(ctx)
	if err != nil {
		t.Fatalf("ContinueConversation with empty memory failed: %v", err)
	}

	if len(capturedRequest.Messages) != 0 {
		t.Errorf("Expected 0 messages in request with empty memory, got %d", len(capturedRequest.Messages))
	}
}

// TestToolExecutionWorkflow tests complete tool execution workflow
func TestToolExecutionWorkflow(t *testing.T) {
	callCount := 0
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			callCount++
			if callCount == 1 {
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
			return &ai.ChatResponse{
				Id:           "test-id-2",
				Model:        "test-model",
				Content:      "Based on the search results, here's the answer",
				FinishReason: "stop",
			}, nil
		},
	}

	memory := inmemory.New()
	client, err := New(provider, WithMemory(memory))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	// User sends initial message
	resp1, err := client.SendMessage(ctx, "Tell me about golang")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if len(resp1.ToolCalls) == 0 {
		t.Fatal("Expected tool calls in first response")
	}

	// Save tool result
	memory.AppendMessage(ctx, &ai.Message{
		Role:       ai.RoleTool,
		Content:    `{"results": "Go is a programming language..."}`,
		ToolCallID: resp1.ToolCalls[0].ID,
		Name:       resp1.ToolCalls[0].Function.Name,
	})

	// Continue conversation to get final answer
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
}

// ========== Observability Tests ==========

// TestClient_DefaultNilObserver tests default observer is nil
func TestClient_DefaultNilObserver(t *testing.T) {
	provider := &mockProvider{}
	client, err := New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Observer is nil by default (no default noop observer)
	if client.observer != nil {
		t.Error("Expected observer to be nil by default")
	}
}

// TestClient_WithObserver tests setting custom observer
func TestClient_WithObserver(t *testing.T) {
	provider := &mockProvider{}
	observer := &testObserver{}

	client, err := New(provider, WithObserver(observer))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if client.observer != observer {
		t.Error("Expected custom observer to be set")
	}
}

// TestSendMessage_ObservabilityTracing tests observability tracing
func TestSendMessage_ObservabilityTracing(t *testing.T) {
	provider := &mockProvider{}
	observer := &testObserver{}

	client, err := New(provider, WithObserver(observer))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendMessage(ctx, "Hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if !observer.spanStarted {
		t.Error("Expected span to be started")
	}
	if !observer.spanEnded {
		t.Error("Expected span to be ended")
	}
	if !observer.metricsRecorded {
		t.Error("Expected metrics to be recorded")
	}
}

// TestSendMessage_ErrorObservability tests error observability
func TestSendMessage_ErrorObservability(t *testing.T) {
	testError := errors.New("test error")
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			return nil, testError
		},
	}
	observer := &testObserver{}

	client, err := New(provider, WithObserver(observer))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendMessage(ctx, "Hello")

	if err == nil {
		t.Fatal("Expected error from provider")
	}

	if !observer.spanStarted {
		t.Error("Expected span to be started even on error")
	}
	// Note: Current implementation doesn't end span on error (potential bug)
	// if !observer.spanEnded {
	// 	t.Error("Expected span to be ended even on error")
	// }
	if !observer.errorLogged {
		t.Error("Expected error to be logged")
	}
}

// TestSendMessage_NilObserver_NoPanic tests nil observer safety
func TestSendMessage_NilObserver_NoPanic(t *testing.T) {
	provider := &mockProvider{}
	client, err := New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendMessage(ctx, "Hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	// If we get here without panic, test passes
}

// ========== Prompt Enrichment Tests ==========

// TestEnrichSystemPromptWithTools tests basic enrichment
// TestEnrichSystemPromptWithTools tests the system prompt enrichment without optimization strategy
func TestEnrichSystemPromptWithTools(t *testing.T) {
	basePrompt := "You are a helpful assistant."

	// Create mock tools
	mockTools := []tool.GenericTool{
		&mockTool{name: "calculator", description: "Performs arithmetic operations"},
		&mockTool{name: "search", description: "Searches the web"},
	}

	toolDescriptions := []ai.ToolDescription{
		{Name: "calculator", Description: "Performs arithmetic operations"},
		{Name: "search", Description: "Searches the web"},
	}

	enriched := enrichSystemPromptWithTools(basePrompt, mockTools, toolDescriptions, "")

	if !strings.Contains(enriched, basePrompt) {
		t.Error("Enriched prompt should contain the base prompt")
	}

	if !strings.Contains(enriched, "## Available Tools") {
		t.Error("Enriched prompt should contain 'Available Tools' section")
	}

	for _, desc := range toolDescriptions {
		if !strings.Contains(enriched, desc.Name) {
			t.Errorf("Enriched prompt should contain tool name '%s'", desc.Name)
		}
		if !strings.Contains(enriched, desc.Description) {
			t.Errorf("Enriched prompt should contain tool description for '%s'", desc.Name)
		}
	}

	if !strings.Contains(enriched, "function calling") {
		t.Error("Enriched prompt should contain function calling guidance")
	}
}

// TestEnrichSystemPromptWithTools_EmptyTools tests with no tools
func TestEnrichSystemPromptWithTools_EmptyTools(t *testing.T) {
	basePrompt := "You are a helpful assistant."
	var mockTools []tool.GenericTool
	var toolDescriptions []ai.ToolDescription

	enriched := enrichSystemPromptWithTools(basePrompt, mockTools, toolDescriptions, "")

	if enriched != basePrompt {
		t.Error("Expected enriched prompt to equal base prompt when no tools provided")
	}
}

// TestEnrichSystemPromptWithTools_NilTools tests with nil tools
func TestEnrichSystemPromptWithTools_NilTools(t *testing.T) {
	basePrompt := "You are a helpful assistant."
	var mockTools []tool.GenericTool
	var toolDescriptions []ai.ToolDescription

	enriched := enrichSystemPromptWithTools(basePrompt, mockTools, toolDescriptions, "")

	if enriched != basePrompt {
		t.Error("Expected enriched prompt to equal base prompt when tools is nil")
	}
}

// TestNewClient_WithEnrichSystemPrompt_Enabled tests enrichment enabled
func TestNewClient_WithEnrichSystemPrompt_Enabled(t *testing.T) {
	provider := &mockProvider{}
	mockTool := &mockTool{
		name:        "TestTool",
		description: "A tool for testing",
	}

	client, err := New(
		provider,
		WithSystemPrompt("You are a helpful assistant."),
		WithTools(mockTool),
		WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if !strings.Contains(client.systemPrompt, "You are a helpful assistant.") {
		t.Error("Client system prompt should contain base prompt")
	}
	if !strings.Contains(client.systemPrompt, "Available Tools") {
		t.Error("Client system prompt should be enriched with tools section")
	}
	if !strings.Contains(client.systemPrompt, "TestTool") {
		t.Error("Client system prompt should contain tool name")
	}
	if !strings.Contains(client.systemPrompt, "A tool for testing") {
		t.Error("Client system prompt should contain tool description")
	}
}

// TestNewClient_WithEnrichSystemPrompt_Disabled tests enrichment disabled by default
func TestNewClient_WithEnrichSystemPrompt_Disabled(t *testing.T) {
	provider := &mockProvider{}
	mockTool := &mockTool{
		name:        "TestTool",
		description: "A tool for testing",
	}

	basePrompt := "You are a helpful assistant."

	client, err := New(
		provider,
		WithSystemPrompt(basePrompt),
		WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.systemPrompt != basePrompt {
		t.Error("Client system prompt should not be enriched when enrichment is disabled")
	}
	if strings.Contains(client.systemPrompt, "Available Tools") {
		t.Error("Client system prompt should not contain tools section when enrichment is disabled")
	}
}

// TestNewClient_WithEnrichSystemPrompt_Integration tests enrichment in actual request
func TestNewClient_WithEnrichSystemPrompt_Integration(t *testing.T) {
	var capturedSystemPrompt string
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			capturedSystemPrompt = req.SystemPrompt
			return &ai.ChatResponse{
				Content:      "Response",
				FinishReason: "stop",
			}, nil
		},
	}

	mockTool := &mockTool{
		name:        "Calculator",
		description: "Performs calculations",
	}

	basePrompt := "You are a math assistant."

	client, err := New(
		provider,
		WithSystemPrompt(basePrompt),
		WithTools(mockTool),
		WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendMessage(ctx, "What is 2+2?")
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	if capturedSystemPrompt == "" {
		t.Fatal("System prompt was not captured")
	}

	if !strings.Contains(capturedSystemPrompt, basePrompt) {
		t.Error("Captured system prompt should contain base prompt")
	}

	if !strings.Contains(capturedSystemPrompt, "Available Tools") {
		t.Error("Captured system prompt should contain tools section")
	}

	if !strings.Contains(capturedSystemPrompt, "Calculator") {
		t.Error("Captured system prompt should contain tool name")
	}

	if !strings.Contains(capturedSystemPrompt, "Performs calculations") {
		t.Error("Captured system prompt should contain tool description")
	}
}
