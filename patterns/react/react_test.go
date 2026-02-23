package react

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/core/cost"
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

func (m *mockTool) GetMetrics() *cost.ToolMetrics {
	return nil // Mock tool has no metrics
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
				Content:      `{"result": "structured response"}`,
				FinishReason: "stop",
				ToolCalls:    []ai.ToolCall{},
			},
		},
	}

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memory),
		client.WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[string](
		baseClient,
		WithMaxIterations(5),
	)
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	// Execute
	ctx := context.Background()
	overview, err := reactPattern.Execute(ctx, "Test prompt")

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if overview == nil {
		t.Fatal("Expected overview, got nil")
	}

	if overview.LastResponse == nil {
		t.Fatal("Expected last response, got nil")
	}

	if overview.LastResponse.Content != `{"result": "structured response"}` {
		t.Errorf("Expected structured response, got '%s'", overview.LastResponse.Content)
	}

	if mockTool.callCount != 1 {
		t.Errorf("Expected tool to be called once, got: %d", mockTool.callCount)
	}

	// Check memory has messages
	messages, memErr := memory.AllMessages(ctx)
	if memErr != nil {
		t.Fatalf("AllMessages returned unexpected error: %v", memErr)
	}
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

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memory),
		client.WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[string](
		baseClient,
		WithMaxIterations(3), // Only allow 3 iterations
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

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memory),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[string](
		baseClient,
		WithMaxIterations(5),
	)
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
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

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memory),
		client.WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[string](
		baseClient,
		WithMaxIterations(5),
		WithStopOnError(true), // Stop on first tool error
	)
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
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
					{Type: "function", Function: ai.ToolCallFunction{Name: "test_tool", Arguments: "{}"}},
				},
			},
			{
				Content:      `{"result": "structured response"}`,
				FinishReason: "stop",
				ToolCalls:    []ai.ToolCall{},
			},
		},
	}

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memory),
		client.WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[string](
		baseClient,
		WithMaxIterations(5),
		WithStopOnError(false), // Continue on tool errors
	)
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	// Execute
	ctx := context.Background()
	overview, err := reactPattern.Execute(ctx, "Test prompt")

	// Assert - should complete despite tool error
	if err != nil {
		t.Errorf("Expected no error with StopOnError=false, got: %v", err)
	}

	if overview == nil {
		t.Fatal("Expected overview, got nil")
	}

	if overview.LastResponse == nil {
		t.Fatal("Expected last response, got nil")
	}

	if overview.LastResponse.Content != `{"result": "structured response"}` {
		t.Errorf("Expected structured response, got: %s", overview.LastResponse.Content)
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
				Content:      `{"result": "structured response"}`,
				FinishReason: "stop",
				ToolCalls:    []ai.ToolCall{},
			},
		},
	}

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memory),
		client.WithObserver(observer),
		client.WithTools(mockTool),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[string](
		baseClient,
		WithMaxIterations(5),
	)
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
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

	baseClient, err := client.New(
		mockLLM,
		// No WithMemory() - stateless mode
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Try to create ReAct without memory - should fail
	_, err = New[string](baseClient)

	// Assert - should fail because memory is required
	if err == nil {
		t.Error("Expected error when creating ReAct without memory, got nil")
	}

	if !strings.Contains(err.Error(), "memory") {
		t.Errorf("Expected error message to mention 'memory', got: %v", err)
	}
}

func TestNewReactPattern_DefaultOptions(t *testing.T) {
	// Setup
	memory := inmemory.New()
	mockLLM := &mockProvider{}

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memory),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create ReactPattern with defaults (no options)
	reactPattern, err := New[string](baseClient)
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	// Assert defaults
	if reactPattern.maxIterations != 10 {
		t.Errorf("Expected default maxIterations to be 10, got: %d", reactPattern.maxIterations)
	}

	if reactPattern.stopOnError != false {
		t.Errorf("Expected default stopOnError to be false, got: %v", reactPattern.stopOnError)
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
						Content:      `{"result": "structured response"}`,
						FinishReason: "stop",
						ToolCalls:    []ai.ToolCall{},
					},
				},
			}

			baseClient, err := client.New(
				mockLLM,
				client.WithMemory(memory),
				client.WithTools(mockTool),
			)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			reactPattern, err := New[string](
				baseClient,
				WithMaxIterations(5),
				WithStopOnError(true),
			)
			if err != nil {
				t.Fatalf("Failed to create ReAct: %v", err)
			}

			// Execute
			ctx := context.Background()
			overview, err := reactPattern.Execute(ctx, "Test prompt")

			// Assert - should succeed regardless of case
			if err != nil {
				t.Errorf("Expected no error with tool name '%s', got: %v", tc.toolCallName, err)
			}

			if overview == nil {
				t.Fatal("Expected overview, got nil")
			}

			if overview.LastResponse == nil {
				t.Fatal("Expected last response, got nil")
			}

			if mockTool.callCount != 1 {
				t.Errorf("Expected tool to be called once with name '%s', got: %d", tc.toolCallName, mockTool.callCount)
			}
		})
	}
}

// testAnswer is a struct used for tests that need parse failure scenarios.
// ParseStringAs[string] always succeeds (returns content as-is), so a struct
// type is necessary to exercise the JSON parsing + retry code paths.
type testAnswer struct {
	Answer string `json:"answer"`
	Reason string `json:"reason"`
}

// --- Observability coverage tests ---
// These tests exercise the internal observe* functions indirectly through
// Execute, by creating scenarios that trigger each code path.

func TestExecute_MaxIterationsReached(t *testing.T) {
	// Provider always returns tool calls, so the loop never reaches a final
	// answer and must hit the max-iteration cap.
	mem := inmemory.New()
	testTool := &mockTool{name: "loop_tool", result: `{"ok": true}`}

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{Content: "thinking", FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{
				{Type: "function", Function: ai.ToolCallFunction{Name: "loop_tool", Arguments: "{}"}},
			}},
			{Content: "still thinking", FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{
				{Type: "function", Function: ai.ToolCallFunction{Name: "loop_tool", Arguments: "{}"}},
			}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem), client.WithTools(testTool))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(2))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	_, err = reactPattern.Execute(context.Background(), "Loop forever")

	if err == nil {
		t.Fatal("Expected error for max iterations, got nil")
	}
	if !strings.Contains(err.Error(), "maximum iterations") {
		t.Errorf("Expected 'maximum iterations' in error, got: %v", err)
	}
	if testTool.callCount != 2 {
		t.Errorf("Expected tool called 2 times, got: %d", testTool.callCount)
	}
}

func TestExecute_MaxIterationsReached_WithObserver(t *testing.T) {
	// Same scenario as above but with an observer attached, so
	// observeMaxIteration's internal branches (span, timer, counter) are hit.
	observer := newTestObserver()
	mem := inmemory.New()
	testTool := &mockTool{name: "loop_tool", result: `{"ok": true}`}

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{Content: "thinking", FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{
				{Type: "function", Function: ai.ToolCallFunction{Name: "loop_tool", Arguments: "{}"}},
			}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem), client.WithObserver(observer), client.WithTools(testTool))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(1))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	_, err = reactPattern.Execute(context.Background(), "Loop forever")

	if err == nil {
		t.Fatal("Expected error for max iterations, got nil")
	}
	if !strings.Contains(err.Error(), "maximum iterations") {
		t.Errorf("Expected 'maximum iterations' in error, got: %v", err)
	}

	// Verify observer captured the max-iteration warning
	hasMaxIterLog := false
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "max iterations") {
			hasMaxIterLog = true
			break
		}
	}
	if !hasMaxIterLog {
		t.Errorf("Expected observer to log about max iterations, got logs: %v", observer.logs)
	}

	// Verify metrics counter was incremented
	if val, ok := observer.metrics["react.executions.total"]; !ok || val < 1 {
		t.Errorf("Expected react.executions.total counter ≥ 1, got: %v (ok=%v)", val, ok)
	}
}

func TestExecute_ToolNotFound_StopOnError(t *testing.T) {
	// Provider requests a tool that does not exist in the catalog. With only
	// one mock response, the loop continues to iteration 2 which fails
	// because the mock has no more responses. This verifies the tool-not-found
	// error message is added to memory and the loop eventually terminates.
	mem := inmemory.New()

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{Content: "Using unknown tool", FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{
				{Type: "function", Function: ai.ToolCallFunction{Name: "unknown_tool", Arguments: "{}"}},
			}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(5), WithStopOnError(true))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	_, err = reactPattern.Execute(context.Background(), "Use the unknown tool")

	// Should error — either because stopOnError triggers or the mock runs out
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestExecute_ToolNotFound_StopOnError_WithObserver(t *testing.T) {
	// With stopOnError=true, a tool-not-found error must immediately stop the
	// ReAct loop and return an error. The observer should capture logs from
	// both executeToolCall ("Tool call failed - not found"), observeToolError
	// ("Tool execution failed, stopping ReAct loop"), and observeStopOnError
	// ("ReAct pattern terminated due to tool error").
	observer := newTestObserver()
	mem := inmemory.New()

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{Content: "Using unknown tool", FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{
				{Type: "function", Function: ai.ToolCallFunction{Name: "unknown_tool", Arguments: "{}"}},
			}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem), client.WithObserver(observer))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(5), WithStopOnError(true))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	_, err = reactPattern.Execute(context.Background(), "Use the unknown tool")

	// stopOnError=true must propagate the tool-not-found error
	if err == nil {
		t.Fatal("Expected error from stopOnError, got nil")
	}
	if !strings.Contains(err.Error(), "tool execution failed at iteration") {
		t.Errorf("Expected 'tool execution failed at iteration' in error, got: %v", err)
	}

	// Verify observer logged the tool-not-found error inside executeToolCall
	hasToolNotFoundLog := false
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "Tool call failed - not found") {
			hasToolNotFoundLog = true
			break
		}
	}
	if !hasToolNotFoundLog {
		t.Errorf("Expected observer to log 'Tool call failed - not found', got logs: %v", observer.logs)
	}

	// Verify observeToolError logged "Tool execution failed, stopping ReAct loop"
	hasStoppingLog := false
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "Tool execution failed, stopping ReAct loop") {
			hasStoppingLog = true
			break
		}
	}
	if !hasStoppingLog {
		t.Errorf("Expected observer to log 'Tool execution failed, stopping ReAct loop', got logs: %v", observer.logs)
	}

	// Verify observeStopOnError logged "ReAct pattern terminated due to tool error"
	hasTerminatedLog := false
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "ReAct pattern terminated due to tool error") {
			hasTerminatedLog = true
			break
		}
	}
	if !hasTerminatedLog {
		t.Errorf("Expected observer to log 'ReAct pattern terminated due to tool error', got logs: %v", observer.logs)
	}

	// Verify error status metric from observeStopOnError
	if val, ok := observer.metrics["react.executions.total"]; !ok || val < 1 {
		t.Errorf("Expected react.executions.total ≥ 1 (error status), got: %v", val)
	}
}

func TestExecute_ToolNotFound_ContinueOnError(t *testing.T) {
	// Provider requests an unknown tool, but StopOnError is false. The loop
	// should continue. The provider then returns a valid final answer.
	mem := inmemory.New()

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			// Iteration 1: requests an unknown tool
			{Content: "Using unknown tool", FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{
				{Type: "function", Function: ai.ToolCallFunction{Name: "unknown_tool", Arguments: "{}"}},
			}},
			// Iteration 2: provides the final answer
			{Content: `{"answer":"42","reason":"computed"}`, FinishReason: "stop", ToolCalls: []ai.ToolCall{}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(5), WithStopOnError(false))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	result, err := reactPattern.Execute(context.Background(), "Use unknown tool then answer")

	if err != nil {
		t.Fatalf("Expected no error with ContinueOnError, got: %v", err)
	}
	if result == nil || result.Data == nil {
		t.Fatal("Expected structured result, got nil")
	}
	if result.Data.Answer != "42" {
		t.Errorf("Expected answer '42', got: %s", result.Data.Answer)
	}
}

func TestExecute_ToolNotFound_ContinueOnError_WithObserver(t *testing.T) {
	// Tool-not-found with observer and StopOnError(false). The observer
	// inside executeToolCall should log "Tool call failed - not found" and
	// the loop should continue to produce a final answer.
	observer := newTestObserver()
	mem := inmemory.New()

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{Content: "Using unknown tool", FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{
				{Type: "function", Function: ai.ToolCallFunction{Name: "unknown_tool", Arguments: "{}"}},
			}},
			{Content: `{"answer":"ok","reason":"recovered"}`, FinishReason: "stop", ToolCalls: []ai.ToolCall{}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem), client.WithObserver(observer))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(5), WithStopOnError(false))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	result, err := reactPattern.Execute(context.Background(), "Continue past error")

	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if result == nil || result.Data == nil {
		t.Fatal("Expected structured result, got nil")
	}

	// The observer inside executeToolCall should have logged the not-found error
	hasToolNotFoundLog := false
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "Tool call failed - not found") {
			hasToolNotFoundLog = true
			break
		}
	}
	if !hasToolNotFoundLog {
		t.Errorf("Expected observer to log 'Tool call failed - not found', got logs: %v", observer.logs)
	}
}

func TestExecute_ToolExecutionError_StopOnError(t *testing.T) {
	// Register a tool that always returns an error. With stopOnError=true,
	// the ReAct loop must stop immediately and return the tool error wrapped
	// in a "tool execution failed at iteration" message.
	mem := inmemory.New()
	failingTool := &mockTool{
		name: "explode",
		err:  errors.New("kaboom: internal tool failure"),
	}

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{Content: "Running tool", FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{
				{Type: "function", Function: ai.ToolCallFunction{Name: "explode", Arguments: `{"force":true}`}},
			}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem), client.WithTools(failingTool))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(5), WithStopOnError(true))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	_, err = reactPattern.Execute(context.Background(), "Use the exploding tool")

	// stopOnError=true must propagate the tool execution error
	if err == nil {
		t.Fatal("Expected error from stopOnError, got nil")
	}
	if !strings.Contains(err.Error(), "tool execution failed at iteration") {
		t.Errorf("Expected 'tool execution failed at iteration' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Errorf("Expected original error 'kaboom' in wrapped error, got: %v", err)
	}
	if failingTool.callCount != 1 {
		t.Errorf("Expected tool called once, got: %d", failingTool.callCount)
	}
}

func TestExecute_ToolExecutionError_StopOnError_WithObserver(t *testing.T) {
	// Tool execution error with observer and stopOnError=true. The observer
	// should capture logs from executeToolCall ("Tool call failed"),
	// observeToolError ("Tool execution failed, stopping ReAct loop"), and
	// observeStopOnError ("ReAct pattern terminated due to tool error").
	observer := newTestObserver()
	mem := inmemory.New()
	failingTool := &mockTool{
		name: "explode",
		err:  errors.New("kaboom"),
	}

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{Content: "Running tool", FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{
				{Type: "function", Function: ai.ToolCallFunction{Name: "explode", Arguments: `{"x":1}`}},
			}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem), client.WithObserver(observer), client.WithTools(failingTool))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(5), WithStopOnError(true))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	_, err = reactPattern.Execute(context.Background(), "Explode")

	// stopOnError=true must propagate the tool execution error
	if err == nil {
		t.Fatal("Expected error from stopOnError, got nil")
	}
	if !strings.Contains(err.Error(), "tool execution failed at iteration") {
		t.Errorf("Expected 'tool execution failed at iteration' in error, got: %v", err)
	}

	// Verify observer logged the tool execution failure from inside executeToolCall
	hasToolFailedLog := false
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "Tool call failed") {
			hasToolFailedLog = true
			break
		}
	}
	if !hasToolFailedLog {
		t.Errorf("Expected observer to log 'Tool call failed', got logs: %v", observer.logs)
	}

	// Verify observeToolError logged "Tool execution failed, stopping ReAct loop"
	hasStoppingLog := false
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "Tool execution failed, stopping ReAct loop") {
			hasStoppingLog = true
			break
		}
	}
	if !hasStoppingLog {
		t.Errorf("Expected observer to log 'Tool execution failed, stopping ReAct loop', got logs: %v", observer.logs)
	}

	// Verify observeStopOnError logged "ReAct pattern terminated due to tool error"
	hasTerminatedLog := false
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "ReAct pattern terminated due to tool error") {
			hasTerminatedLog = true
			break
		}
	}
	if !hasTerminatedLog {
		t.Errorf("Expected observer to log 'ReAct pattern terminated due to tool error', got logs: %v", observer.logs)
	}

	// Verify error status metric from observeStopOnError
	if val, ok := observer.metrics["react.executions.total"]; !ok || val < 1 {
		t.Errorf("Expected react.executions.total ≥ 1 (error status), got: %v", val)
	}
}

func TestExecute_ParseRetry_Succeeds(t *testing.T) {
	// First provider response has no tool calls but invalid JSON for
	// testAnswer. After the parse-retry prompt, the provider returns valid
	// JSON. This exercises observeParseError and
	// observeRequestingStructuredFinalAnswer.
	mem := inmemory.New()

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			// Iteration 1: final answer but not valid JSON for testAnswer
			{Content: "This is plain text, not JSON at all", FinishReason: "stop", ToolCalls: []ai.ToolCall{}},
			// Retry: valid JSON
			{Content: `{"answer":"42","reason":"because math"}`, FinishReason: "stop", ToolCalls: []ai.ToolCall{}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	result, err := reactPattern.Execute(context.Background(), "Give me an answer")

	if err != nil {
		t.Fatalf("Expected success after retry, got: %v", err)
	}
	if result == nil || result.Data == nil {
		t.Fatal("Expected structured result, got nil")
	}
	if result.Data.Answer != "42" {
		t.Errorf("Expected answer '42', got: %s", result.Data.Answer)
	}
	if result.Data.Reason != "because math" {
		t.Errorf("Expected reason 'because math', got: %s", result.Data.Reason)
	}
}

func TestExecute_ParseRetry_Succeeds_WithObserver(t *testing.T) {
	// Same as above with observer to cover observeParseError and
	// observeRequestingStructuredFinalAnswer logging branches.
	observer := newTestObserver()
	mem := inmemory.New()

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{Content: "not json!!!", FinishReason: "stop", ToolCalls: []ai.ToolCall{}},
			{Content: `{"answer":"yes","reason":"confirmed"}`, FinishReason: "stop", ToolCalls: []ai.ToolCall{}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem), client.WithObserver(observer))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	result, err := reactPattern.Execute(context.Background(), "Give me JSON")

	if err != nil {
		t.Fatalf("Expected success after retry, got: %v", err)
	}
	if result == nil || result.Data == nil {
		t.Fatal("Expected structured result, got nil")
	}

	// Verify parse error was logged
	hasParseErrorLog := false
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "Failed to parse final answer") {
			hasParseErrorLog = true
			break
		}
	}
	if !hasParseErrorLog {
		t.Errorf("Expected observer to log parse error, got logs: %v", observer.logs)
	}

	// Verify structured final answer request was logged
	hasRetryLog := false
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "Requesting structured final answer") {
			hasRetryLog = true
			break
		}
	}
	if !hasRetryLog {
		t.Errorf("Expected observer to log structured-final-answer request, got logs: %v", observer.logs)
	}

	// Verify errors were recorded on the span (parse error)
	if len(observer.errors) == 0 {
		t.Error("Expected span to record parse error, but none were recorded")
	}
}

func TestExecute_ParseRetry_AllFails(t *testing.T) {
	// Both the initial response and the retry response have invalid JSON for
	// the target type. Execute should return a parse error.
	mem := inmemory.New()

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{Content: "not json at all", FinishReason: "stop", ToolCalls: []ai.ToolCall{}},
			{Content: "still not json either", FinishReason: "stop", ToolCalls: []ai.ToolCall{}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	_, err = reactPattern.Execute(context.Background(), "Give me something unparseable")

	if err == nil {
		t.Fatal("Expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse final answer after retry") {
		t.Errorf("Expected 'failed to parse final answer after retry' in error, got: %v", err)
	}
}

func TestExecute_ParseRetry_AllFails_WithObserver(t *testing.T) {
	// Same double-fail scenario with observer. Both calls to observeParseError
	// should fire, plus the structured-final-answer request.
	observer := newTestObserver()
	mem := inmemory.New()

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{Content: "garbage1", FinishReason: "stop", ToolCalls: []ai.ToolCall{}},
			{Content: "garbage2", FinishReason: "stop", ToolCalls: []ai.ToolCall{}},
		},
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem), client.WithObserver(observer))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	_, err = reactPattern.Execute(context.Background(), "Double fail")

	if err == nil {
		t.Fatal("Expected parse error, got nil")
	}

	// Count how many parse error logs were recorded (should be 2)
	parseErrorCount := 0
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "Failed to parse final answer") {
			parseErrorCount++
		}
	}
	if parseErrorCount != 2 {
		t.Errorf("Expected 2 parse error logs, got %d; logs: %v", parseErrorCount, observer.logs)
	}

	// Span should have recorded errors for both parse failures
	if len(observer.errors) < 2 {
		t.Errorf("Expected at least 2 span errors for parse failures, got %d", len(observer.errors))
	}
}

func TestExecute_WithObserverFromContext(t *testing.T) {
	// Inject an observer via context (not via client.WithObserver) and verify
	// that Execute picks it up and records spans.
	observer := newTestObserver()
	mem := inmemory.New()
	testTool := &mockTool{name: "ctx_tool", result: `{"ok": true}`}

	mockLLM := &mockProvider{
		responses: []*ai.ChatResponse{
			{Content: "Using tool", FinishReason: "tool_calls", ToolCalls: []ai.ToolCall{
				{Type: "function", Function: ai.ToolCallFunction{Name: "ctx_tool", Arguments: "{}"}},
			}},
			{Content: `{"answer":"done","reason":"from context observer"}`, FinishReason: "stop", ToolCalls: []ai.ToolCall{}},
		},
	}

	// Create client WITHOUT observer — observer comes from context only
	baseClient, err := client.New(mockLLM, client.WithMemory(mem), client.WithTools(testTool))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(5))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	// Inject observer into context
	ctx := observability.ContextWithObserver(context.Background(), observer)
	result, err := reactPattern.Execute(ctx, "Test with context observer")

	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if result == nil || result.Data == nil {
		t.Fatal("Expected structured result, got nil")
	}
	if result.Data.Answer != "done" {
		t.Errorf("Expected answer 'done', got: %s", result.Data.Answer)
	}

	// Verify observer recorded spans (react.execute)
	hasReactSpan := false
	for _, span := range observer.spans {
		if span == "react.execute" {
			hasReactSpan = true
			break
		}
	}
	if !hasReactSpan {
		t.Errorf("Expected 'react.execute' span from context-injected observer, got spans: %v", observer.spans)
	}

	// Verify logs were captured
	if len(observer.logs) == 0 {
		t.Error("Expected observer logs from context-injected observer, got none")
	}

	// Verify success metrics
	if val, ok := observer.metrics["react.executions.total"]; !ok || val < 1 {
		t.Errorf("Expected react.executions.total ≥ 1, got: %v", val)
	}
}

func TestExecute_IterationError_WithObserver(t *testing.T) {
	// Force an iteration-level error by having the provider return an error
	// on the first call. This covers observeIterationError with all its
	// branches (span, timer, execTimer).
	observer := newTestObserver()
	mem := inmemory.New()

	mockLLM := &mockProvider{
		err: errors.New("provider unavailable"),
	}

	baseClient, err := client.New(mockLLM, client.WithMemory(mem), client.WithObserver(observer))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	reactPattern, err := New[testAnswer](baseClient, WithMaxIterations(3))
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	_, err = reactPattern.Execute(context.Background(), "This will fail")

	if err == nil {
		t.Fatal("Expected error from provider failure, got nil")
	}
	if !strings.Contains(err.Error(), "iteration 1 failed") {
		t.Errorf("Expected 'iteration 1 failed' in error, got: %v", err)
	}

	// Verify the observer logged the iteration failure
	hasIterErrorLog := false
	for _, logMsg := range observer.logs {
		if strings.Contains(logMsg, "Iteration failed") {
			hasIterErrorLog = true
			break
		}
	}
	if !hasIterErrorLog {
		t.Errorf("Expected observer to log 'Iteration failed', got logs: %v", observer.logs)
	}

	// Verify span recorded the error
	if len(observer.errors) == 0 {
		t.Error("Expected span to record iteration error, but none were recorded")
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
				Content:      "Using tool",
				FinishReason: "tool_calls",
				ToolCalls: []ai.ToolCall{
					{Type: "function", Function: ai.ToolCallFunction{Name: "tool1", Arguments: "{}"}},
				},
			},
			{
				Content:      `{"result": "structured response"}`,
				FinishReason: "stop",
				ToolCalls:    []ai.ToolCall{},
			},
		},
	}

	baseClient, err := client.New(
		mockLLM,
		client.WithMemory(memory),
		client.WithTools(mockTool1, mockTool2, mockTool3),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create ReAct - catalog normalization happens internally during Execute
	reactPattern, err := New[string](baseClient)
	if err != nil {
		t.Fatalf("Failed to create ReAct: %v", err)
	}

	// Execute to trigger catalog normalization
	ctx := context.Background()
	overview, err := reactPattern.Execute(ctx, "Test prompt")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if overview == nil {
		t.Fatal("Expected overview, got nil")
	}

	if overview.LastResponse == nil {
		t.Fatal("Expected last response, got nil")
	}

	// Verify tool was called (proves case-insensitive lookup worked)
	if mockTool1.callCount != 1 {
		t.Errorf("Expected tool1 to be called once, got: %d", mockTool1.callCount)
	}
}
