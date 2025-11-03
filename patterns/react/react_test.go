package react

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
	"github.com/leofalp/aigo/providers/observability"
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

func TestReactPattern_Execute_Success(t *testing.T) {
	// Setup
	memory := inmemory.New()
	mockTool := &mockTool{
		name:   "test_tool",
		result: `{"result": 42}`,
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
		client.WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := NewReactPattern[string](
		baseClient,
		WithMaxIterations(5),
		WithStopOnError(true),
	)
	if err != nil {
		t.Fatalf("Failed to create ReactPattern: %v", err)
	}

	// Execute
	ctx := context.Background()
	resp, err := reactPattern.Execute(ctx, "Test prompt")

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	if resp.Content != "Done" {
		t.Errorf("Expected 'Done', got '%s'", resp.Content)
	}

	if mockTool.callCount != 1 {
		t.Errorf("Expected tool to be called once, got: %d", mockTool.callCount)
	}

	// Check memory has messages
	messages := memory.AllMessages()
	if len(messages) == 0 {
		t.Error("Expected messages in memory after execution")
	}
}

func TestReactPattern_Execute_MaxIterations(t *testing.T) {
	// Setup
	memory := inmemory.New()
	mockTool := &mockTool{
		name:   "test_tool",
		result: `{"result": 42}`,
	}

	// Mock LLM that keeps requesting tool calls (never provides final answer)
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

	reactPattern, err := NewReactPattern[string](
		baseClient,
		WithMaxIterations(3), // Only allow 3 iterations
		WithStopOnError(true),
	)
	if err != nil {
		t.Fatalf("Failed to create ReactPattern: %v", err)
	}

	// Execute
	ctx := context.Background()
	_, err = reactPattern.Execute(ctx, "Test prompt")

	// Assert - should hit max iterations
	if err == nil {
		t.Error("Expected error for max iterations, got nil")
	}

	if mockTool.callCount != 3 { // Should stop after 3 iterations
		t.Errorf("Expected tool to be called 3 times, got: %d", mockTool.callCount)
	}
}

func TestReactPattern_Execute_ToolNotFound(t *testing.T) {
	// Setup
	memory := inmemory.New()

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

	reactPattern, err := NewReactPattern[string](
		baseClient,
		WithMaxIterations(5),
		WithStopOnError(true),
	)
	if err != nil {
		t.Fatalf("Failed to create ReactPattern: %v", err)
	}

	// Execute
	ctx := context.Background()
	_, err = reactPattern.Execute(ctx, "Test prompt")

	// Assert - should fail with tool not found
	if err == nil {
		t.Error("Expected error for tool not found, got nil")
	}
}

func TestReactPattern_Execute_ToolError_StopOnError(t *testing.T) {
	// Setup
	memory := inmemory.New()
	mockTool := &mockTool{
		name:   "failing_tool",
		result: "",
		err:    errors.New("tool execution failed"),
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

	reactPattern, err := NewReactPattern[string](
		baseClient,
		WithMaxIterations(5),
		WithStopOnError(true),
	)
	if err != nil {
		t.Fatalf("Failed to create ReactPattern: %v", err)
	}

	// Execute
	ctx := context.Background()
	_, err = reactPattern.Execute(ctx, "Test prompt")

	// Assert - should fail immediately
	if err == nil {
		t.Error("Expected error for tool failure, got nil")
	}

	if mockTool.callCount != 1 {
		t.Errorf("Expected tool to be called once, got: %d", mockTool.callCount)
	}
}

func TestReactPattern_Execute_ToolError_ContinueOnError(t *testing.T) {
	// Setup
	memory := inmemory.New()
	mockTool := &mockTool{
		name:   "failing_tool",
		result: "",
		err:    errors.New("tool execution failed"),
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

	reactPattern, err := NewReactPattern[string](
		baseClient,
		WithMaxIterations(5),
		WithStopOnError(false), // Continue on error
	)
	if err != nil {
		t.Fatalf("Failed to create ReactPattern: %v", err)
	}

	// Execute
	ctx := context.Background()
	resp, err := reactPattern.Execute(ctx, "Test prompt")

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

func TestReactPattern_Execute_WithObservability(t *testing.T) {
	// Setup
	observer := newTestObserver()
	memory := inmemory.New()
	mockTool := &mockTool{
		name:   "test_tool",
		result: `{"result": 42}`,
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

	reactPattern, err := NewReactPattern[string](
		baseClient,
		WithMaxIterations(5),
		WithStopOnError(true),
	)
	if err != nil {
		t.Fatalf("Failed to create ReactPattern: %v", err)
	}

	// Execute with observer in context
	ctx := observability.ContextWithObserver(context.Background(), observer)
	_, err = reactPattern.Execute(ctx, "Test prompt")

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

func TestNewReactPattern_NoMemory_Error(t *testing.T) {
	// Setup client without memory (stateless mode)
	mockLLM := &mockProvider{}

	baseClient, err := client.NewClient[string](
		mockLLM,
		// No WithMemory() - stateless mode
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Try to create ReactPattern without memory
	_, err = NewReactPattern[string](baseClient)

	// Assert - should fail because memory is required
	if err == nil {
		t.Error("Expected error when creating ReactPattern without memory, got nil")
	}

	if !strings.Contains(err.Error(), "memory") {
		t.Errorf("Expected error message to mention 'memory', got: %v", err)
	}
}

func TestNewReactPattern_DefaultOptions(t *testing.T) {
	// Setup
	memory := inmemory.New()
	mockLLM := &mockProvider{}

	baseClient, err := client.NewClient[string](
		mockLLM,
		client.WithMemory(memory),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create ReactPattern with defaults (no options)
	reactPattern, err := NewReactPattern[string](baseClient)
	if err != nil {
		t.Fatalf("Failed to create ReactPattern: %v", err)
	}

	// Assert defaults
	if reactPattern.maxIterations != 10 {
		t.Errorf("Expected default maxIterations to be 10, got: %d", reactPattern.maxIterations)
	}

	if reactPattern.stopOnError != true {
		t.Errorf("Expected default stopOnError to be true, got: %v", reactPattern.stopOnError)
	}
}

func TestReactPattern_CaseInsensitiveToolLookup(t *testing.T) {
	// Test with various case variations
	testCases := []struct {
		name         string
		toolCallName string
	}{
		{"lowercase", "testtool"},
		{"uppercase", "TESTTOOL"},
		{"mixed case 1", "TestTool"},
		{"mixed case 2", "testTOOL"},
		{"mixed case 3", "TESTtool"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup fresh instances for each test
			memory := inmemory.New()
			mockTool := &mockTool{
				name:   "TestTool",
				result: `{"result": "success"}`,
			}

			mockLLM := &mockProvider{
				responses: []*ai.ChatResponse{
					{
						Content:      "Using tool",
						FinishReason: "tool_calls",
						ToolCalls: []ai.ToolCall{
							{
								Type:     "function",
								Function: ai.ToolCallFunction{Name: tc.toolCallName, Arguments: "{}"},
							},
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
				client.WithTools(mockTool),
			)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			reactPattern, err := NewReactPattern[string](
				baseClient,
				WithMaxIterations(5),
				WithStopOnError(true),
			)
			if err != nil {
				t.Fatalf("Failed to create ReactPattern: %v", err)
			}

			// Execute
			ctx := context.Background()
			resp, err := reactPattern.Execute(ctx, "Test prompt")

			// Assert - should succeed regardless of case
			if err != nil {
				t.Errorf("Expected no error with tool name '%s', got: %v", tc.toolCallName, err)
			}

			if resp == nil {
				t.Fatal("Expected response, got nil")
			}

			if mockTool.callCount != 1 {
				t.Errorf("Expected tool to be called once with name '%s', got: %d", tc.toolCallName, mockTool.callCount)
			}
		})
	}
}

func TestReactPattern_CaseInsensitiveCatalogNormalization(t *testing.T) {
	// Setup
	memory := inmemory.New()
	mockTool1 := &mockTool{name: "Tool1", result: "result1"}
	mockTool2 := &mockTool{name: "TOOL2", result: "result2"}
	mockTool3 := &mockTool{name: "ToOl3", result: "result3"}

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{
				Content:      "Using tools",
				FinishReason: "tool_calls",
				ToolCalls: []ai.ToolCall{
					{Type: "function", Function: ai.ToolCallFunction{Name: "tool1", Arguments: "{}"}},
				},
			},
			{
				Content:      "Done",
				FinishReason: "stop",
			},
		},
	}

	baseClient, err := client.NewClient[string](
		mockLLM,
		client.WithMemory(memory),
		client.WithTools(mockTool1, mockTool2, mockTool3),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create ReactPattern - catalog normalization happens internally during Execute
	reactPattern, err := NewReactPattern[string](baseClient)
	if err != nil {
		t.Fatalf("Failed to create ReactPattern: %v", err)
	}

	// Execute to trigger catalog normalization
	ctx := context.Background()
	resp, err := reactPattern.Execute(ctx, "Test prompt")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	// Verify tool was called (proves case-insensitive lookup worked)
	if mockTool1.callCount != 1 {
		t.Errorf("Expected tool1 to be called once, got: %d", mockTool1.callCount)
	}
}
