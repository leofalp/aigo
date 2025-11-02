package react

import (
	"aigo/core/client"
	"aigo/providers/ai"
	"aigo/providers/memory/inmemory"
	"aigo/providers/observability"
	"aigo/providers/tool"
	"context"
	"errors"
	"net/http"
	"testing"
)

// mockTool is a simple mock tool for testing
type mockTool struct {
	name      string
	callCount int
	result    string
	err       error
}

func (m *mockTool) ToolInfo() ai.ToolDescription {
	return ai.ToolDescription{
		Name:        m.name,
		Description: "Mock tool for testing",
		Parameters:  nil,
	}
}

func (m *mockTool) Call(ctx context.Context, arguments string) (string, error) {
	m.callCount++
	if m.err != nil {
		return "", m.err
	}
	return m.result, nil
}

// mockProvider is a mock LLM provider for testing
type mockProvider struct {
	responses []*ai.ChatResponse
	callIndex int
	err       error
}

func (m *mockProvider) SendMessage(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIndex >= len(m.responses) {
		return nil, errors.New("no more mock responses")
	}
	resp := m.responses[m.callIndex]
	m.callIndex++
	return resp, nil
}

func (m *mockProvider) IsStopMessage(response *ai.ChatResponse) bool {
	return len(response.ToolCalls) == 0
}

func (m *mockProvider) WithAPIKey(apiKey string) ai.Provider {
	return m
}

func (m *mockProvider) WithBaseURL(baseURL string) ai.Provider {
	return m
}

func (m *mockProvider) WithHttpClient(httpClient *http.Client) ai.Provider {
	return m
}

// testObserver implements observability.Provider for testing
type testObserver struct {
	spans   []string
	logs    []string
	errors  []error
	metrics map[string]float64
}

func newTestObserver() *testObserver {
	return &testObserver{
		spans:   make([]string, 0),
		logs:    make([]string, 0),
		errors:  make([]error, 0),
		metrics: make(map[string]float64),
	}
}

func (t *testObserver) StartSpan(ctx context.Context, name string, attrs ...observability.Attribute) (context.Context, observability.Span) {
	t.spans = append(t.spans, name)
	span := &testSpan{name: name, observer: t}
	return ctx, span
}

func (t *testObserver) Info(ctx context.Context, msg string, attrs ...observability.Attribute) {
	t.logs = append(t.logs, msg)
}
func (t *testObserver) Debug(ctx context.Context, msg string, attrs ...observability.Attribute) {
	t.logs = append(t.logs, msg)
}
func (t *testObserver) Warn(ctx context.Context, msg string, attrs ...observability.Attribute) {
	t.logs = append(t.logs, msg)
}
func (t *testObserver) Error(ctx context.Context, msg string, attrs ...observability.Attribute) {
	t.logs = append(t.logs, msg)
}
func (t *testObserver) Trace(ctx context.Context, msg string, attrs ...observability.Attribute) {
	t.logs = append(t.logs, msg)
}

func (t *testObserver) Counter(name string) observability.Counter {
	return &testCounter{name: name, observer: t}
}
func (t *testObserver) Histogram(name string) observability.Histogram {
	return &testHistogram{name: name, observer: t}
}

type testSpan struct {
	name     string
	observer *testObserver
	ended    bool
}

func (s *testSpan) End()                                                        { s.ended = true }
func (s *testSpan) SetAttributes(attrs ...observability.Attribute)              {}
func (s *testSpan) SetStatus(code observability.StatusCode, description string) {}
func (s *testSpan) RecordError(err error)                                       { s.observer.errors = append(s.observer.errors, err) }
func (s *testSpan) AddEvent(name string, attrs ...observability.Attribute)      {}

type testCounter struct {
	name     string
	observer *testObserver
}

func (c *testCounter) Add(ctx context.Context, value int64, attrs ...observability.Attribute) {
	c.observer.metrics[c.name] += float64(value)
}

type testHistogram struct {
	name     string
	observer *testObserver
}

func (h *testHistogram) Record(ctx context.Context, value float64, attrs ...observability.Attribute) {
	h.observer.metrics[h.name] = value
}

func TestReactClient_Execute_Success(t *testing.T) {
	// Setup
	memory := inmemory.New()
	mockTool := &mockTool{
		name:   "test_tool",
		result: `{"result": 42}`,
	}
	toolCatalog := map[string]tool.GenericTool{
		"test_tool": mockTool,
	}

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{
				Content:      "I need to use the tool",
				FinishReason: "tool_calls",
				ToolCalls: []ai.ToolCall{
					{
						Type:     "function",
						Function: ai.ToolCallFunction{Name: "test_tool", Arguments: `{"input": "test"}`},
					},
				},
			},
			{
				Content:      "The answer is 42",
				FinishReason: "stop",
				ToolCalls:    []ai.ToolCall{},
			},
		},
	}

	baseClient, err := client.NewClient[string](
		mockLLM,
		client.WithMemory(memory),
		client.WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactClient := NewReactClient[string](
		baseClient,
		memory,
		toolCatalog,
		Config{MaxIterations: 5, StopOnError: true},
	)

	// Execute
	ctx := context.Background()
	resp, err := reactClient.Execute(ctx, "Test prompt")

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	if resp.Content != "The answer is 42" {
		t.Errorf("Expected 'The answer is 42', got: %s", resp.Content)
	}

	if mockTool.callCount != 1 {
		t.Errorf("Expected tool to be called once, got: %d", mockTool.callCount)
	}

	// Check memory has messages
	messages := memory.AllMessages()
	if len(messages) < 3 { // user + assistant + tool result + assistant
		t.Errorf("Expected at least 3 messages in memory, got: %d", len(messages))
	}
}

func TestReactClient_Execute_MaxIterations(t *testing.T) {
	// Setup
	memory := inmemory.New()
	mockTool := &mockTool{
		name:   "test_tool",
		result: `{"result": "ok"}`,
	}
	toolCatalog := map[string]tool.GenericTool{
		"test_tool": mockTool,
	}

	// Mock LLM that always wants to use tools (infinite loop scenario)
	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{
				Content:      "Using tool",
				FinishReason: "tool_calls",
				ToolCalls: []ai.ToolCall{
					{Type: "function", Function: ai.ToolCallFunction{Name: "test_tool", Arguments: "{}"}},
				},
			},
			{
				Content:      "Using tool again",
				FinishReason: "tool_calls",
				ToolCalls: []ai.ToolCall{
					{Type: "function", Function: ai.ToolCallFunction{Name: "test_tool", Arguments: "{}"}},
				},
			},
			{
				Content:      "Using tool again",
				FinishReason: "tool_calls",
				ToolCalls: []ai.ToolCall{
					{Type: "function", Function: ai.ToolCallFunction{Name: "test_tool", Arguments: "{}"}},
				},
			},
			{
				Content:      "Using tool again",
				FinishReason: "tool_calls",
				ToolCalls: []ai.ToolCall{
					{Type: "function", Function: ai.ToolCallFunction{Name: "test_tool", Arguments: "{}"}},
				},
			},
		},
	}

	baseClient, err := client.NewClient[string](
		mockLLM,
		client.WithMemory(memory),
		client.WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactClient := NewReactClient[string](
		baseClient,
		memory,
		toolCatalog,
		Config{MaxIterations: 3, StopOnError: true}, // Only allow 3 iterations
	)

	// Execute
	ctx := context.Background()
	_, err = reactClient.Execute(ctx, "Test prompt")

	// Assert - should hit max iterations
	if err == nil {
		t.Error("Expected error for max iterations, got nil")
	}

	if mockTool.callCount != 3 { // Should stop after 3 iterations
		t.Errorf("Expected tool to be called 3 times, got: %d", mockTool.callCount)
	}
}

func TestReactClient_Execute_ToolNotFound(t *testing.T) {
	// Setup
	memory := inmemory.New()
	toolCatalog := map[string]tool.GenericTool{} // Empty catalog

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{
				Content:      "Using unknown tool",
				FinishReason: "tool_calls",
				ToolCalls: []ai.ToolCall{
					{
						Type:     "function",
						Function: ai.ToolCallFunction{Name: "unknown_tool", Arguments: "{}"},
					},
				},
			},
		},
	}

	baseClient, err := client.NewClient[string](
		mockLLM,
		client.WithMemory(memory),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactClient := NewReactClient[string](
		baseClient,
		memory,
		toolCatalog,
		Config{MaxIterations: 5, StopOnError: true},
	)

	// Execute
	ctx := context.Background()
	_, err = reactClient.Execute(ctx, "Test prompt")

	// Assert - should fail with tool not found
	if err == nil {
		t.Error("Expected error for tool not found, got nil")
	}
}

func TestReactClient_Execute_ToolError_StopOnError(t *testing.T) {
	// Setup
	memory := inmemory.New()
	mockTool := &mockTool{
		name:   "failing_tool",
		result: "",
		err:    errors.New("tool execution failed"),
	}
	toolCatalog := map[string]tool.GenericTool{
		"failing_tool": mockTool,
	}

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{
				Content:      "Using tool",
				FinishReason: "tool_calls",
				ToolCalls: []ai.ToolCall{
					{
						Type:     "function",
						Function: ai.ToolCallFunction{Name: "failing_tool", Arguments: "{}"},
					},
				},
			},
		},
	}

	baseClient, err := client.NewClient[string](
		mockLLM,
		client.WithMemory(memory),
		client.WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactClient := NewReactClient[string](
		baseClient,
		memory,
		toolCatalog,
		Config{MaxIterations: 5, StopOnError: true},
	)

	// Execute
	ctx := context.Background()
	_, err = reactClient.Execute(ctx, "Test prompt")

	// Assert - should fail immediately
	if err == nil {
		t.Error("Expected error for tool failure, got nil")
	}

	if mockTool.callCount != 1 {
		t.Errorf("Expected tool to be called once, got: %d", mockTool.callCount)
	}
}

func TestReactClient_Execute_ToolError_ContinueOnError(t *testing.T) {
	// Setup
	memory := inmemory.New()
	mockTool := &mockTool{
		name:   "failing_tool",
		result: "",
		err:    errors.New("tool execution failed"),
	}
	toolCatalog := map[string]tool.GenericTool{
		"failing_tool": mockTool,
	}

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{
				Content:      "Using tool",
				FinishReason: "tool_calls",
				ToolCalls: []ai.ToolCall{
					{Type: "function", Function: ai.ToolCallFunction{Name: "failing_tool", Arguments: "{}"}},
				},
			},
			{
				Content:      "Final answer despite error",
				FinishReason: "stop",
				ToolCalls:    []ai.ToolCall{},
			},
		},
	}

	baseClient, err := client.NewClient[string](
		mockLLM,
		client.WithMemory(memory),
		client.WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactClient := NewReactClient[string](
		baseClient,
		memory,
		toolCatalog,
		Config{MaxIterations: 5, StopOnError: false}, // Continue on error
	)

	// Execute
	ctx := context.Background()
	resp, err := reactClient.Execute(ctx, "Test prompt")

	// Assert - should complete despite tool error
	if err != nil {
		t.Errorf("Expected no error with StopOnError=false, got: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	if resp.Content != "Final answer despite error" {
		t.Errorf("Expected final answer, got: %s", resp.Content)
	}
}

func TestReactClient_Execute_WithObservability(t *testing.T) {
	// Setup
	observer := newTestObserver()
	memory := inmemory.New()
	mockTool := &mockTool{
		name:   "test_tool",
		result: `{"result": 42}`,
	}
	toolCatalog := map[string]tool.GenericTool{
		"test_tool": mockTool,
	}

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{
				Content:      "Using tool",
				FinishReason: "tool_calls",
				ToolCalls: []ai.ToolCall{
					{Type: "function", Function: ai.ToolCallFunction{Name: "test_tool", Arguments: "{}"}},
				},
			},
			{
				Content:      "Done",
				FinishReason: "stop",
				ToolCalls:    []ai.ToolCall{},
			},
		},
	}

	baseClient, err := client.NewClient[string](
		mockLLM,
		client.WithMemory(memory),
		client.WithObserver(observer),
		client.WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactClient := NewReactClient[string](
		baseClient,
		memory,
		toolCatalog,
		Config{MaxIterations: 5, StopOnError: true},
	)

	// Execute with observer in context
	ctx := observability.ContextWithObserver(context.Background(), observer)
	_, err = reactClient.Execute(ctx, "Test prompt")

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check observability data
	if len(observer.spans) == 0 {
		t.Error("Expected spans to be recorded")
	}

	if len(observer.logs) == 0 {
		t.Error("Expected logs to be recorded")
	}

	// Check for react-specific spans
	hasReactSpan := false
	for _, span := range observer.spans {
		if span == "react.execute" {
			hasReactSpan = true
			break
		}
	}
	if !hasReactSpan {
		t.Error("Expected 'react.execute' span to be recorded")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxIterations != 10 {
		t.Errorf("Expected MaxIterations to be 10, got: %d", cfg.MaxIterations)
	}

	if cfg.StopOnError != true {
		t.Errorf("Expected StopOnError to be true, got: %v", cfg.StopOnError)
	}
}
