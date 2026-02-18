package graph

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/observability"
)

// --- Mock Types ---

// mockProvider is a mock LLM provider for testing graph construction.
// It satisfies ai.Provider with minimal behavior.
type mockProvider struct {
	responses []*ai.ChatResponse
	callIndex int
	err       error
}

var _ ai.Provider = (*mockProvider)(nil)

func (provider *mockProvider) SendMessage(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
	if provider.err != nil {
		return nil, provider.err
	}
	if provider.callIndex >= len(provider.responses) {
		return nil, errors.New("no more mock responses")
	}
	response := provider.responses[provider.callIndex]
	provider.callIndex++
	return response, nil
}

func (provider *mockProvider) IsStopMessage(response *ai.ChatResponse) bool {
	return len(response.ToolCalls) == 0
}

func (provider *mockProvider) WithAPIKey(_ string) ai.Provider  { return provider }
func (provider *mockProvider) WithBaseURL(_ string) ai.Provider { return provider }
func (provider *mockProvider) WithHttpClient(_ *http.Client) ai.Provider {
	return provider
}

// mockTool is a minimal GenericTool mock for testing node tools.
type mockTool struct {
	name   string
	result string
	err    error
}

func (tool *mockTool) ToolInfo() ai.ToolDescription {
	return ai.ToolDescription{
		Name:        tool.name,
		Description: "Mock tool for testing",
	}
}

func (tool *mockTool) Call(_ context.Context, _ string) (string, error) {
	if tool.err != nil {
		return "", tool.err
	}
	return tool.result, nil
}

func (tool *mockTool) GetMetrics() *cost.ToolMetrics {
	return nil
}

// testObserver implements observability.Provider for verifying observe calls.
type testObserver struct {
	mu      sync.Mutex
	spans   []string
	logs    []string
	errors  []error
	metrics map[string]float64
}

var _ observability.Provider = (*testObserver)(nil)

func newTestObserver() *testObserver {
	return &testObserver{
		spans:   make([]string, 0),
		logs:    make([]string, 0),
		errors:  make([]error, 0),
		metrics: make(map[string]float64),
	}
}

func (observer *testObserver) StartSpan(ctx context.Context, name string, _ ...observability.Attribute) (context.Context, observability.Span) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.spans = append(observer.spans, name)
	span := &testSpan{name: name, observer: observer}
	return ctx, span
}

func (observer *testObserver) Info(_ context.Context, msg string, _ ...observability.Attribute) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.logs = append(observer.logs, msg)
}

func (observer *testObserver) Debug(_ context.Context, msg string, _ ...observability.Attribute) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.logs = append(observer.logs, msg)
}

func (observer *testObserver) Warn(_ context.Context, msg string, _ ...observability.Attribute) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.logs = append(observer.logs, msg)
}

func (observer *testObserver) Error(_ context.Context, msg string, _ ...observability.Attribute) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.logs = append(observer.logs, msg)
}

func (observer *testObserver) Trace(_ context.Context, msg string, _ ...observability.Attribute) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.logs = append(observer.logs, msg)
}

func (observer *testObserver) Counter(name string) observability.Counter {
	return &testCounter{name: name, observer: observer}
}

func (observer *testObserver) Histogram(name string) observability.Histogram {
	return &testHistogram{name: name, observer: observer}
}

// testSpan is a mock span for testing observability.
type testSpan struct {
	name     string
	observer *testObserver
}

func (span *testSpan) End()                                            {}
func (span *testSpan) SetAttributes(_ ...observability.Attribute)      {}
func (span *testSpan) SetStatus(_ observability.StatusCode, _ string)  {}
func (span *testSpan) RecordError(err error)                           {}
func (span *testSpan) AddEvent(_ string, _ ...observability.Attribute) {}

// testCounter is a mock counter for testing observability.
type testCounter struct {
	name     string
	observer *testObserver
}

func (counter *testCounter) Add(_ context.Context, value int64, _ ...observability.Attribute) {
	counter.observer.mu.Lock()
	defer counter.observer.mu.Unlock()
	counter.observer.metrics[counter.name] += float64(value)
}

// testHistogram is a mock histogram for testing observability.
type testHistogram struct {
	name     string
	observer *testObserver
}

func (histogram *testHistogram) Record(_ context.Context, value float64, _ ...observability.Attribute) {
	histogram.observer.mu.Lock()
	defer histogram.observer.mu.Unlock()
	histogram.observer.metrics[histogram.name] = value
}

// --- Helpers ---

// newTestClient creates a minimal client.Client for testing.
func newTestClient(testingHelper *testing.T) *client.Client {
	testingHelper.Helper()
	testClient, err := client.New(&mockProvider{})
	if err != nil {
		testingHelper.Fatalf("failed to create test client: %v", err)
	}
	return testClient
}

// newTestClientWithObserver creates a client with a test observer attached.
func newTestClientWithObserver(testingHelper *testing.T, observer observability.Provider) *client.Client {
	testingHelper.Helper()
	testClient, err := client.New(&mockProvider{}, client.WithObserver(observer))
	if err != nil {
		testingHelper.Fatalf("failed to create test client with observer: %v", err)
	}
	return testClient
}

// successExecutor returns a NodeExecutorFunc that succeeds with the given output.
func successExecutor(output any) NodeExecutorFunc {
	return func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
		return &NodeResult{Output: output}, nil
	}
}

// failingExecutor returns a NodeExecutorFunc that always fails with the given error.
func failingExecutor(err error) NodeExecutorFunc {
	return func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
		return nil, err
	}
}

// delayedExecutor returns a NodeExecutorFunc that waits the given duration before succeeding.
func delayedExecutor(delay time.Duration, output any) NodeExecutorFunc {
	return func(ctx context.Context, _ *NodeInput) (*NodeResult, error) {
		select {
		case <-time.After(delay):
			return &NodeResult{Output: output}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// trackingExecutor returns a NodeExecutorFunc that records its invocation order.
func trackingExecutor(executionOrder *[]string, mu *sync.Mutex, nodeID string, output any) NodeExecutorFunc {
	return func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
		mu.Lock()
		*executionOrder = append(*executionOrder, nodeID)
		mu.Unlock()
		return &NodeResult{Output: output}, nil
	}
}

// --- Builder Validation Tests ---

func TestNewGraphBuilder_DefaultConfig(testCase *testing.T) {
	testClient := newTestClient(testCase)
	builder := NewGraphBuilder[string](testClient)

	if builder.defaultClient != testClient {
		testCase.Errorf("expected default client to be set")
	}
	if builder.config.errorStrategy != ErrorStrategyFailFast {
		testCase.Errorf("expected default error strategy to be fail_fast, got %s", builder.config.errorStrategy)
	}
	if builder.config.maxConcurrency != 0 {
		testCase.Errorf("expected default maxConcurrency to be 0, got %d", builder.config.maxConcurrency)
	}
	if builder.config.executionTimeout != 0 {
		testCase.Errorf("expected default executionTimeout to be 0, got %v", builder.config.executionTimeout)
	}
}

func TestNewGraphBuilder_WithOptions(testCase *testing.T) {
	testClient := newTestClient(testCase)
	stateProvider := NewInMemoryStateProvider(nil)

	builder := NewGraphBuilder[string](testClient,
		WithMaxConcurrency(3),
		WithExecutionTimeout(5*time.Minute),
		WithErrorStrategy(ErrorStrategyContinueOnError),
		WithOutputNode("output"),
		WithStateProvider(stateProvider),
	)

	if builder.config.maxConcurrency != 3 {
		testCase.Errorf("expected maxConcurrency 3, got %d", builder.config.maxConcurrency)
	}
	if builder.config.executionTimeout != 5*time.Minute {
		testCase.Errorf("expected executionTimeout 5m, got %v", builder.config.executionTimeout)
	}
	if builder.config.errorStrategy != ErrorStrategyContinueOnError {
		testCase.Errorf("expected error strategy continue_on_error, got %s", builder.config.errorStrategy)
	}
	if builder.config.outputNodeID != "output" {
		testCase.Errorf("expected output node 'output', got %q", builder.config.outputNodeID)
	}
	if builder.config.stateProvider != stateProvider {
		testCase.Errorf("expected custom state provider")
	}
}

func TestBuild_EmptyGraph(testCase *testing.T) {
	testClient := newTestClient(testCase)
	_, err := NewGraphBuilder[string](testClient).Build()

	if err == nil {
		testCase.Fatal("expected error for empty graph, got nil")
	}
	if !strings.Contains(err.Error(), "at least one node") {
		testCase.Errorf("expected 'at least one node' error, got: %v", err)
	}
}

func TestBuild_EmptyNodeID(testCase *testing.T) {
	testClient := newTestClient(testCase)
	_, err := NewGraphBuilder[string](testClient).
		AddNode("", successExecutor("ok")).
		Build()

	if err == nil {
		testCase.Fatal("expected error for empty node ID, got nil")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		testCase.Errorf("expected 'must not be empty' error, got: %v", err)
	}
}

func TestBuild_NilExecutor(testCase *testing.T) {
	testClient := newTestClient(testCase)
	_, err := NewGraphBuilder[string](testClient).
		AddNode("node1", nil).
		Build()

	if err == nil {
		testCase.Fatal("expected error for nil executor, got nil")
	}
	if !strings.Contains(err.Error(), "must not be nil") {
		testCase.Errorf("expected 'must not be nil' error, got: %v", err)
	}
}

func TestBuild_DuplicateNodeID(testCase *testing.T) {
	testClient := newTestClient(testCase)
	_, err := NewGraphBuilder[string](testClient).
		AddNode("node1", successExecutor("a")).
		AddNode("node1", successExecutor("b")).
		Build()

	if err == nil {
		testCase.Fatal("expected error for duplicate node ID, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate node ID") {
		testCase.Errorf("expected 'duplicate node ID' error, got: %v", err)
	}
}

func TestBuild_EdgeReferencesNonExistentNode(testCase *testing.T) {
	testClient := newTestClient(testCase)
	_, err := NewGraphBuilder[string](testClient).
		AddNode("node1", successExecutor("a")).
		AddEdge("node1", "nonexistent").
		Build()

	if err == nil {
		testCase.Fatal("expected error for non-existent edge target, got nil")
	}
	if !strings.Contains(err.Error(), "non-existent target node") {
		testCase.Errorf("expected 'non-existent target node' error, got: %v", err)
	}
}

func TestBuild_SelfLoop(testCase *testing.T) {
	testClient := newTestClient(testCase)
	_, err := NewGraphBuilder[string](testClient).
		AddNode("node1", successExecutor("a")).
		AddEdge("node1", "node1").
		Build()

	if err == nil {
		testCase.Fatal("expected error for self-loop, got nil")
	}
	if !strings.Contains(err.Error(), "self-loop") {
		testCase.Errorf("expected 'self-loop' error, got: %v", err)
	}
}

func TestBuild_DuplicateEdge(testCase *testing.T) {
	testClient := newTestClient(testCase)
	_, err := NewGraphBuilder[string](testClient).
		AddNode("a", successExecutor("a")).
		AddNode("b", successExecutor("b")).
		AddEdge("a", "b").
		AddEdge("a", "b").
		Build()

	if err == nil {
		testCase.Fatal("expected error for duplicate edge, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate edge") {
		testCase.Errorf("expected 'duplicate edge' error, got: %v", err)
	}
}

func TestBuild_CycleDetection(testCase *testing.T) {
	testClient := newTestClient(testCase)
	_, err := NewGraphBuilder[string](testClient).
		AddNode("a", successExecutor("a")).
		AddNode("b", successExecutor("b")).
		AddNode("c", successExecutor("c")).
		AddEdge("a", "b").
		AddEdge("b", "c").
		AddEdge("c", "a").
		Build()

	if err == nil {
		testCase.Fatal("expected error for cycle, got nil")
	}
	if !strings.Contains(err.Error(), "cycle detected") {
		testCase.Errorf("expected 'cycle detected' error, got: %v", err)
	}
}

func TestBuild_NonExistentOutputNode(testCase *testing.T) {
	testClient := newTestClient(testCase)
	_, err := NewGraphBuilder[string](testClient,
		WithOutputNode("nonexistent"),
	).
		AddNode("a", successExecutor("a")).
		Build()

	if err == nil {
		testCase.Fatal("expected error for non-existent output node, got nil")
	}
	if !strings.Contains(err.Error(), "output node") {
		testCase.Errorf("expected 'output node' error, got: %v", err)
	}
}

func TestBuild_EmptyEdgeEndpoints(testCase *testing.T) {
	testClient := newTestClient(testCase)
	_, err := NewGraphBuilder[string](testClient).
		AddNode("a", successExecutor("a")).
		AddEdge("", "a").
		Build()

	if err == nil {
		testCase.Fatal("expected error for empty edge endpoints, got nil")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		testCase.Errorf("expected 'must not be empty' error, got: %v", err)
	}
}

func TestBuild_SingleNode(testCase *testing.T) {
	testClient := newTestClient(testCase)
	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("only", successExecutor("result")).
		Build()

	if err != nil {
		testCase.Fatalf("unexpected build error: %v", err)
	}

	if executionGraph.outputNodeID != "only" {
		testCase.Errorf("expected output node 'only', got %q", executionGraph.outputNodeID)
	}
	if len(executionGraph.levels) != 1 {
		testCase.Errorf("expected 1 level, got %d", len(executionGraph.levels))
	}
	if len(executionGraph.topologicalOrder) != 1 {
		testCase.Errorf("expected 1 node in topological order, got %d", len(executionGraph.topologicalOrder))
	}
}

func TestBuild_LinearChain(testCase *testing.T) {
	testClient := newTestClient(testCase)
	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("a", successExecutor("a")).
		AddNode("b", successExecutor("b")).
		AddNode("c", successExecutor("c")).
		AddEdge("a", "b").
		AddEdge("b", "c").
		Build()

	if err != nil {
		testCase.Fatalf("unexpected build error: %v", err)
	}

	// Should have 3 levels (linear chain).
	if len(executionGraph.levels) != 3 {
		testCase.Errorf("expected 3 levels for linear chain, got %d", len(executionGraph.levels))
	}

	// Output node should be "c" (last in topo order).
	if executionGraph.outputNodeID != "c" {
		testCase.Errorf("expected output node 'c', got %q", executionGraph.outputNodeID)
	}

	// Topological order should be a, b, c.
	expectedOrder := []string{"a", "b", "c"}
	if len(executionGraph.topologicalOrder) != len(expectedOrder) {
		testCase.Fatalf("expected %d nodes in topo order, got %d", len(expectedOrder), len(executionGraph.topologicalOrder))
	}
	for index, nodeID := range expectedOrder {
		if executionGraph.topologicalOrder[index] != nodeID {
			testCase.Errorf("expected topo order[%d]=%q, got %q", index, nodeID, executionGraph.topologicalOrder[index])
		}
	}
}

func TestBuild_DiamondTopology(testCase *testing.T) {
	testClient := newTestClient(testCase)
	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("root", successExecutor("root")).
		AddNode("left", successExecutor("left")).
		AddNode("right", successExecutor("right")).
		AddNode("merge", successExecutor("merge")).
		AddEdge("root", "left").
		AddEdge("root", "right").
		AddEdge("left", "merge").
		AddEdge("right", "merge").
		Build()

	if err != nil {
		testCase.Fatalf("unexpected build error: %v", err)
	}

	// Diamond: 3 levels (root -> left,right -> merge).
	if len(executionGraph.levels) != 3 {
		testCase.Errorf("expected 3 levels for diamond, got %d", len(executionGraph.levels))
	}

	// Level 0: root, Level 1: left+right, Level 2: merge.
	if len(executionGraph.levels[0]) != 1 || executionGraph.levels[0][0] != "root" {
		testCase.Errorf("expected level 0 = [root], got %v", executionGraph.levels[0])
	}
	if len(executionGraph.levels[1]) != 2 {
		testCase.Errorf("expected level 1 to have 2 nodes, got %d", len(executionGraph.levels[1]))
	}
	if len(executionGraph.levels[2]) != 1 || executionGraph.levels[2][0] != "merge" {
		testCase.Errorf("expected level 2 = [merge], got %v", executionGraph.levels[2])
	}

	// Output should be "merge".
	if executionGraph.outputNodeID != "merge" {
		testCase.Errorf("expected output node 'merge', got %q", executionGraph.outputNodeID)
	}
}

func TestBuild_WithOutputNode(testCase *testing.T) {
	testClient := newTestClient(testCase)
	executionGraph, err := NewGraphBuilder[string](testClient,
		WithOutputNode("left"),
	).
		AddNode("root", successExecutor("root")).
		AddNode("left", successExecutor("left")).
		AddNode("right", successExecutor("right")).
		AddEdge("root", "left").
		AddEdge("root", "right").
		Build()

	if err != nil {
		testCase.Fatalf("unexpected build error: %v", err)
	}

	if executionGraph.outputNodeID != "left" {
		testCase.Errorf("expected output node 'left', got %q", executionGraph.outputNodeID)
	}
}

func TestBuild_NodeOptions(testCase *testing.T) {
	testClient := newTestClient(testCase)
	customClient := newTestClient(testCase)
	testTool := &mockTool{name: "test_tool", result: "ok"}

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("node1", successExecutor("a"),
			WithNodeClient(customClient),
			WithNodeTools(testTool),
			WithNodeParams(map[string]any{"key": "value"}),
			WithNodeTimeout(30*time.Second),
		).
		Build()

	if err != nil {
		testCase.Fatalf("unexpected build error: %v", err)
	}

	graphNode := executionGraph.nodes["node1"]
	if graphNode.nodeClient != customClient {
		testCase.Error("expected custom client to be set on node")
	}
	if len(graphNode.nodeTools) != 1 || graphNode.nodeTools[0] != testTool {
		testCase.Error("expected tool to be set on node")
	}
	if graphNode.params["key"] != "value" {
		testCase.Error("expected params to be set on node")
	}
	if graphNode.timeout != 30*time.Second {
		testCase.Errorf("expected 30s timeout, got %v", graphNode.timeout)
	}
}

// --- Execution Tests ---

func TestExecute_SingleNode_StringOutput(testCase *testing.T) {
	testClient := newTestClient(testCase)
	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("output", successExecutor("hello world")).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}
	if result.Data == nil {
		testCase.Fatal("expected non-nil data")
	}
	if *result.Data != "hello world" {
		testCase.Errorf("expected 'hello world', got %q", *result.Data)
	}
}

func TestExecute_SingleNode_StructOutput(testCase *testing.T) {
	type Report struct {
		Summary string `json:"summary"`
		Score   int    `json:"score"`
	}

	testClient := newTestClient(testCase)
	executionGraph, err := NewGraphBuilder[Report](testClient).
		AddNode("output", successExecutor(`{"summary":"test","score":42}`)).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}
	if result.Data == nil {
		testCase.Fatal("expected non-nil data")
	}
	if result.Data.Summary != "test" {
		testCase.Errorf("expected summary 'test', got %q", result.Data.Summary)
	}
	if result.Data.Score != 42 {
		testCase.Errorf("expected score 42, got %d", result.Data.Score)
	}
}

func TestExecute_SingleNode_DirectTypeAssertion(testCase *testing.T) {
	type Report struct {
		Summary string
		Score   int
	}

	testClient := newTestClient(testCase)
	executionGraph, err := NewGraphBuilder[Report](testClient).
		AddNode("output", NodeExecutorFunc(func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
			return &NodeResult{Output: Report{Summary: "direct", Score: 99}}, nil
		})).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}
	if result.Data.Summary != "direct" {
		testCase.Errorf("expected summary 'direct', got %q", result.Data.Summary)
	}
	if result.Data.Score != 99 {
		testCase.Errorf("expected score 99, got %d", result.Data.Score)
	}
}

func TestExecute_LinearChain(testCase *testing.T) {
	testClient := newTestClient(testCase)

	// Track execution order.
	var executionOrder []string
	var orderMutex sync.Mutex

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("step1", trackingExecutor(&executionOrder, &orderMutex, "step1", "result1")).
		AddNode("step2", NodeExecutorFunc(func(_ context.Context, input *NodeInput) (*NodeResult, error) {
			orderMutex.Lock()
			executionOrder = append(executionOrder, "step2")
			orderMutex.Unlock()

			// Verify upstream result is available.
			upstream, exists := input.UpstreamResults["step1"]
			if !exists {
				return nil, errors.New("missing upstream result from step1")
			}
			return &NodeResult{Output: fmt.Sprintf("step2(%s)", upstream.Output)}, nil
		})).
		AddNode("step3", NodeExecutorFunc(func(_ context.Context, input *NodeInput) (*NodeResult, error) {
			orderMutex.Lock()
			executionOrder = append(executionOrder, "step3")
			orderMutex.Unlock()

			upstream, exists := input.UpstreamResults["step2"]
			if !exists {
				return nil, errors.New("missing upstream result from step2")
			}
			return &NodeResult{Output: fmt.Sprintf("step3(%s)", upstream.Output)}, nil
		})).
		AddEdge("step1", "step2").
		AddEdge("step2", "step3").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	// Verify chain execution order.
	if len(executionOrder) != 3 {
		testCase.Fatalf("expected 3 executions, got %d", len(executionOrder))
	}
	expectedOrder := []string{"step1", "step2", "step3"}
	for index, nodeID := range expectedOrder {
		if executionOrder[index] != nodeID {
			testCase.Errorf("expected execution[%d]=%q, got %q", index, nodeID, executionOrder[index])
		}
	}

	// Verify data flows through chain.
	if result.Data == nil || *result.Data != "step3(step2(result1))" {
		testCase.Errorf("expected chained output, got %v", result.Data)
	}
}

func TestExecute_ParallelNodes(testCase *testing.T) {
	testClient := newTestClient(testCase)

	// Use atomic counter to verify parallel execution.
	var concurrentCount atomic.Int32
	var maxConcurrent atomic.Int32

	parallelExecutor := func(nodeID string) NodeExecutorFunc {
		return func(ctx context.Context, _ *NodeInput) (*NodeResult, error) {
			current := concurrentCount.Add(1)
			// Track maximum concurrency observed.
			for {
				currentMax := maxConcurrent.Load()
				if current <= currentMax || maxConcurrent.CompareAndSwap(currentMax, current) {
					break
				}
			}
			// Simulate some work.
			time.Sleep(50 * time.Millisecond)
			concurrentCount.Add(-1)
			return &NodeResult{Output: nodeID + "_done"}, nil
		}
	}

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("root", successExecutor("start")).
		AddNode("parallel1", parallelExecutor("p1")).
		AddNode("parallel2", parallelExecutor("p2")).
		AddNode("parallel3", parallelExecutor("p3")).
		AddNode("merge", successExecutor("merged")).
		AddEdge("root", "parallel1").
		AddEdge("root", "parallel2").
		AddEdge("root", "parallel3").
		AddEdge("parallel1", "merge").
		AddEdge("parallel2", "merge").
		AddEdge("parallel3", "merge").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	_, err = executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	// Verify that at least 2 nodes ran concurrently (they are at the same level).
	if maxConcurrent.Load() < 2 {
		testCase.Errorf("expected parallel execution (max concurrent >= 2), got %d", maxConcurrent.Load())
	}
}

func TestExecute_DiamondTopology(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("root", successExecutor("input_data")).
		AddNode("left", NodeExecutorFunc(func(_ context.Context, input *NodeInput) (*NodeResult, error) {
			upstream := input.UpstreamResults["root"]
			return &NodeResult{Output: "left(" + upstream.Output.(string) + ")"}, nil
		})).
		AddNode("right", NodeExecutorFunc(func(_ context.Context, input *NodeInput) (*NodeResult, error) {
			upstream := input.UpstreamResults["root"]
			return &NodeResult{Output: "right(" + upstream.Output.(string) + ")"}, nil
		})).
		AddNode("merge", NodeExecutorFunc(func(_ context.Context, input *NodeInput) (*NodeResult, error) {
			leftResult := input.UpstreamResults["left"].Output.(string)
			rightResult := input.UpstreamResults["right"].Output.(string)
			return &NodeResult{Output: leftResult + "+" + rightResult}, nil
		})).
		AddEdge("root", "left").
		AddEdge("root", "right").
		AddEdge("left", "merge").
		AddEdge("right", "merge").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	if result.Data == nil {
		testCase.Fatal("expected non-nil data")
	}

	// The merge node should have received both left and right results.
	output := *result.Data
	if !strings.Contains(output, "left(input_data)") || !strings.Contains(output, "right(input_data)") {
		testCase.Errorf("expected merged output with both branches, got %q", output)
	}
}

func TestExecute_WithInitialState(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("reader", NodeExecutorFunc(func(ctx context.Context, input *NodeInput) (*NodeResult, error) {
			value, exists, stateError := input.SharedState.Get(ctx, "greeting")
			if stateError != nil {
				return nil, stateError
			}
			if !exists {
				return nil, errors.New("greeting not found in state")
			}
			return &NodeResult{Output: value.(string)}, nil
		})).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), map[string]any{
		"greeting": "hello from state",
	})
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	if *result.Data != "hello from state" {
		testCase.Errorf("expected 'hello from state', got %q", *result.Data)
	}
}

func TestExecute_SharedStateBetweenNodes(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("writer", NodeExecutorFunc(func(ctx context.Context, input *NodeInput) (*NodeResult, error) {
			if err := input.SharedState.Set(ctx, "computed_value", "42"); err != nil {
				return nil, err
			}
			return &NodeResult{Output: "written"}, nil
		})).
		AddNode("reader", NodeExecutorFunc(func(ctx context.Context, input *NodeInput) (*NodeResult, error) {
			value, exists, stateError := input.SharedState.Get(ctx, "computed_value")
			if stateError != nil {
				return nil, stateError
			}
			if !exists {
				return nil, errors.New("computed_value not found")
			}
			return &NodeResult{Output: "read:" + value.(string)}, nil
		})).
		AddEdge("writer", "reader").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	if *result.Data != "read:42" {
		testCase.Errorf("expected 'read:42', got %q", *result.Data)
	}
}

func TestExecute_NodeWithParams(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("paramNode", NodeExecutorFunc(func(_ context.Context, input *NodeInput) (*NodeResult, error) {
			entityType, exists := input.Params["entity_type"]
			if !exists {
				return nil, errors.New("entity_type param not found")
			}
			return &NodeResult{Output: "type:" + entityType.(string)}, nil
		}),
			WithNodeParams(map[string]any{"entity_type": "person"}),
		).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	if *result.Data != "type:person" {
		testCase.Errorf("expected 'type:person', got %q", *result.Data)
	}
}

func TestExecute_NodeWithCustomClient(testCase *testing.T) {
	defaultClient := newTestClient(testCase)
	customClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](defaultClient).
		AddNode("defaultNode", NodeExecutorFunc(func(_ context.Context, input *NodeInput) (*NodeResult, error) {
			if input.Client == customClient {
				return nil, errors.New("expected default client, got custom client")
			}
			return &NodeResult{Output: "default"}, nil
		})).
		AddNode("customNode", NodeExecutorFunc(func(_ context.Context, input *NodeInput) (*NodeResult, error) {
			if input.Client != customClient {
				return nil, errors.New("expected custom client, got default client")
			}
			return &NodeResult{Output: "custom"}, nil
		}), WithNodeClient(customClient)).
		AddEdge("defaultNode", "customNode").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	if *result.Data != "custom" {
		testCase.Errorf("expected 'custom', got %q", *result.Data)
	}
}

// --- Error Handling Tests ---

func TestExecute_FailFast_StopsOnFirstError(testCase *testing.T) {
	testClient := newTestClient(testCase)
	expectedError := errors.New("intentional failure")

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("root", successExecutor("ok")).
		AddNode("failing", failingExecutor(expectedError)).
		AddNode("downstream", successExecutor("should not reach")).
		AddEdge("root", "failing").
		AddEdge("failing", "downstream").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	_, execErr := executionGraph.Execute(context.Background(), nil)
	if execErr == nil {
		testCase.Fatal("expected execution error, got nil")
	}

	if !strings.Contains(execErr.Error(), "intentional failure") {
		testCase.Errorf("expected error to contain 'intentional failure', got: %v", execErr)
	}
}

func TestExecute_ContinueOnError_SkipsDownstream(testCase *testing.T) {
	testClient := newTestClient(testCase)

	var downstreamExecuted atomic.Bool

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithErrorStrategy(ErrorStrategyContinueOnError),
		WithOutputNode("success_branch"),
	).
		AddNode("root", successExecutor("ok")).
		AddNode("failing", failingExecutor(errors.New("fail"))).
		AddNode("downstream_of_fail", NodeExecutorFunc(func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
			downstreamExecuted.Store(true)
			return &NodeResult{Output: "should not run"}, nil
		})).
		AddNode("success_branch", successExecutor("success")).
		AddEdge("root", "failing").
		AddEdge("root", "success_branch").
		AddEdge("failing", "downstream_of_fail").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, execErr := executionGraph.Execute(context.Background(), nil)
	if execErr != nil {
		testCase.Fatalf("expected no error with continue-on-error, got: %v", execErr)
	}

	if downstreamExecuted.Load() {
		testCase.Error("downstream of failed node should have been skipped")
	}

	if result.Data == nil || *result.Data != "success" {
		testCase.Errorf("expected 'success' from success branch, got %v", result.Data)
	}
}

func TestExecute_NodeReturnsNilResult(testCase *testing.T) {
	testClient := newTestClient(testCase)

	// A node that returns nil result should be handled gracefully.
	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("nil_result", NodeExecutorFunc(func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
			return nil, nil
		})).
		AddNode("after", successExecutor("final")).
		AddEdge("nil_result", "after").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	if result.Data == nil || *result.Data != "final" {
		testCase.Errorf("expected 'final', got %v", result.Data)
	}
}

// --- Timeout Tests ---

func TestExecute_GraphTimeout(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithExecutionTimeout(100*time.Millisecond),
	).
		AddNode("slow", delayedExecutor(5*time.Second, "too slow")).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	_, execErr := executionGraph.Execute(context.Background(), nil)
	if execErr == nil {
		testCase.Fatal("expected timeout error, got nil")
	}
}

func TestExecute_NodeTimeout(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("fast", successExecutor("fast_result")).
		AddNode("slow", delayedExecutor(5*time.Second, "too slow"),
			WithNodeTimeout(100*time.Millisecond),
		).
		AddEdge("fast", "slow").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	_, execErr := executionGraph.Execute(context.Background(), nil)
	if execErr == nil {
		testCase.Fatal("expected timeout error, got nil")
	}
}

func TestExecute_NodeTimeoutDoesNotAffectOthers(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithErrorStrategy(ErrorStrategyContinueOnError),
		WithOutputNode("fast"),
	).
		AddNode("root", successExecutor("ok")).
		AddNode("slow", delayedExecutor(5*time.Second, "too slow"),
			WithNodeTimeout(50*time.Millisecond),
		).
		AddNode("fast", successExecutor("fast_result")).
		AddEdge("root", "slow").
		AddEdge("root", "fast").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, execErr := executionGraph.Execute(context.Background(), nil)
	if execErr != nil {
		testCase.Fatalf("expected no error (continue-on-error), got: %v", execErr)
	}

	if result.Data == nil || *result.Data != "fast_result" {
		testCase.Errorf("expected 'fast_result', got %v", result.Data)
	}
}

func TestExecute_ContextCancellation(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("slow", delayedExecutor(5*time.Second, "result")).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, execErr := executionGraph.Execute(ctx, nil)
	if execErr == nil {
		testCase.Fatal("expected cancellation error, got nil")
	}
}

// --- Conditional Edge Tests ---

func TestExecute_ConditionalEdge_Satisfied(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("check", NodeExecutorFunc(func(ctx context.Context, input *NodeInput) (*NodeResult, error) {
			_ = input.SharedState.Set(ctx, "quality", 0.9)
			return &NodeResult{Output: "checked"}, nil
		})).
		AddNode("premium", successExecutor("premium_output")).
		AddEdge("check", "premium", WithEdgeCondition(func(result *NodeResult, state StateProvider) bool {
			value, _, _ := state.Get(context.Background(), "quality")
			return value.(float64) > 0.8
		})).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	if *result.Data != "premium_output" {
		testCase.Errorf("expected 'premium_output', got %q", *result.Data)
	}
}

func TestExecute_ConditionalEdge_NotSatisfied(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithOutputNode("check"),
	).
		AddNode("check", NodeExecutorFunc(func(ctx context.Context, input *NodeInput) (*NodeResult, error) {
			_ = input.SharedState.Set(ctx, "quality", 0.3)
			return &NodeResult{Output: "checked"}, nil
		})).
		AddNode("premium", successExecutor("premium_output")).
		AddEdge("check", "premium", WithEdgeCondition(func(result *NodeResult, state StateProvider) bool {
			value, _, _ := state.Get(context.Background(), "quality")
			return value.(float64) > 0.8
		})).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	// Output node is "check", premium should be skipped.
	if *result.Data != "checked" {
		testCase.Errorf("expected 'checked', got %q", *result.Data)
	}
}

func TestExecute_ConditionalEdge_MultiplePaths(testCase *testing.T) {
	testClient := newTestClient(testCase)

	var pathATaken atomic.Bool
	var pathBTaken atomic.Bool

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithErrorStrategy(ErrorStrategyContinueOnError),
		WithOutputNode("router"),
	).
		AddNode("router", NodeExecutorFunc(func(ctx context.Context, input *NodeInput) (*NodeResult, error) {
			_ = input.SharedState.Set(ctx, "route", "A")
			return &NodeResult{Output: "routed"}, nil
		})).
		AddNode("pathA", NodeExecutorFunc(func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
			pathATaken.Store(true)
			return &NodeResult{Output: "path_a"}, nil
		})).
		AddNode("pathB", NodeExecutorFunc(func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
			pathBTaken.Store(true)
			return &NodeResult{Output: "path_b"}, nil
		})).
		AddEdge("router", "pathA", WithEdgeCondition(func(_ *NodeResult, state StateProvider) bool {
			value, _, _ := state.Get(context.Background(), "route")
			return value.(string) == "A"
		})).
		AddEdge("router", "pathB", WithEdgeCondition(func(_ *NodeResult, state StateProvider) bool {
			value, _, _ := state.Get(context.Background(), "route")
			return value.(string) == "B"
		})).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	_, err = executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	if !pathATaken.Load() {
		testCase.Error("expected path A to be taken")
	}
	if pathBTaken.Load() {
		testCase.Error("expected path B to be skipped")
	}
}

// --- MaxConcurrency Tests ---

func TestExecute_MaxConcurrency(testCase *testing.T) {
	testClient := newTestClient(testCase)

	var maxConcurrent atomic.Int32
	var currentCount atomic.Int32

	concurrencyTracker := func(nodeID string) NodeExecutorFunc {
		return func(ctx context.Context, _ *NodeInput) (*NodeResult, error) {
			current := currentCount.Add(1)
			for {
				currentMax := maxConcurrent.Load()
				if current <= currentMax || maxConcurrent.CompareAndSwap(currentMax, current) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			currentCount.Add(-1)
			return &NodeResult{Output: nodeID}, nil
		}
	}

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithMaxConcurrency(2),
	).
		AddNode("root", successExecutor("ok")).
		AddNode("a", concurrencyTracker("a")).
		AddNode("b", concurrencyTracker("b")).
		AddNode("c", concurrencyTracker("c")).
		AddNode("d", concurrencyTracker("d")).
		AddNode("merge", successExecutor("done")).
		AddEdge("root", "a").
		AddEdge("root", "b").
		AddEdge("root", "c").
		AddEdge("root", "d").
		AddEdge("a", "merge").
		AddEdge("b", "merge").
		AddEdge("c", "merge").
		AddEdge("d", "merge").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	_, err = executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	if maxConcurrent.Load() > 2 {
		testCase.Errorf("expected max concurrency <= 2, got %d", maxConcurrent.Load())
	}
}

// --- Reset Tests ---

func TestReset_AllowsReExecution(testCase *testing.T) {
	testClient := newTestClient(testCase)
	var callCount atomic.Int32

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("counter", NodeExecutorFunc(func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
			count := callCount.Add(1)
			return &NodeResult{Output: fmt.Sprintf("run_%d", count)}, nil
		})).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	// First execution.
	result1, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("first execute error: %v", err)
	}
	if *result1.Data != "run_1" {
		testCase.Errorf("expected 'run_1', got %q", *result1.Data)
	}

	// Reset and re-execute.
	if err := executionGraph.Reset(context.Background(), nil); err != nil {
		testCase.Fatalf("reset error: %v", err)
	}

	result2, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("second execute error: %v", err)
	}
	if *result2.Data != "run_2" {
		testCase.Errorf("expected 'run_2', got %q", *result2.Data)
	}
}

func TestReset_WithNewInitialState(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("reader", NodeExecutorFunc(func(ctx context.Context, input *NodeInput) (*NodeResult, error) {
			value, _, _ := input.SharedState.Get(ctx, "name")
			return &NodeResult{Output: "hello " + value.(string)}, nil
		})).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	// First execution.
	result1, err := executionGraph.Execute(context.Background(), map[string]any{"name": "Alice"})
	if err != nil {
		testCase.Fatalf("first execute error: %v", err)
	}
	if *result1.Data != "hello Alice" {
		testCase.Errorf("expected 'hello Alice', got %q", *result1.Data)
	}

	// Reset with new state and re-execute.
	if err := executionGraph.Reset(context.Background(), map[string]any{"name": "Bob"}); err != nil {
		testCase.Fatalf("reset error: %v", err)
	}

	result2, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("second execute error: %v", err)
	}
	if *result2.Data != "hello Bob" {
		testCase.Errorf("expected 'hello Bob', got %q", *result2.Data)
	}
}

// --- State Provider Tests ---

func TestInMemoryStateProvider_BasicOperations(testCase *testing.T) {
	provider := NewInMemoryStateProvider(map[string]any{
		"initial": "value",
	})
	ctx := context.Background()

	// Test initial state.
	value, exists, err := provider.Get(ctx, "initial")
	if err != nil {
		testCase.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		testCase.Fatal("expected key to exist")
	}
	if value != "value" {
		testCase.Errorf("expected 'value', got %v", value)
	}

	// Test set and get.
	if err := provider.Set(ctx, "new_key", 42); err != nil {
		testCase.Fatalf("set error: %v", err)
	}
	newValue, exists, err := provider.Get(ctx, "new_key")
	if err != nil || !exists || newValue != 42 {
		testCase.Errorf("expected 42, got %v (exists=%v, err=%v)", newValue, exists, err)
	}

	// Test non-existent key.
	_, exists, err = provider.Get(ctx, "nonexistent")
	if err != nil {
		testCase.Fatalf("unexpected error: %v", err)
	}
	if exists {
		testCase.Error("expected key to not exist")
	}

	// Test GetAll returns a copy.
	allState, err := provider.GetAll(ctx)
	if err != nil {
		testCase.Fatalf("GetAll error: %v", err)
	}
	if len(allState) != 2 {
		testCase.Errorf("expected 2 entries, got %d", len(allState))
	}
	// Modifying the copy should not affect the provider.
	allState["mutated"] = true
	_, exists, _ = provider.Get(ctx, "mutated")
	if exists {
		testCase.Error("modifying GetAll result should not affect provider")
	}
}

func TestInMemoryStateProvider_NodeStatusAndResult(testCase *testing.T) {
	provider := NewInMemoryStateProvider(nil)
	ctx := context.Background()

	// Default status is pending.
	status, err := provider.GetNodeStatus(ctx, "node1")
	if err != nil {
		testCase.Fatalf("unexpected error: %v", err)
	}
	if status != NodePending {
		testCase.Errorf("expected pending, got %s", status)
	}

	// Set and verify status transitions.
	_ = provider.SetNodeStatus(ctx, "node1", NodeRunning)
	status, _ = provider.GetNodeStatus(ctx, "node1")
	if status != NodeRunning {
		testCase.Errorf("expected running, got %s", status)
	}

	// Set and get result.
	result := &NodeResult{Output: "data", Duration: time.Second}
	_ = provider.SetNodeResult(ctx, "node1", result)
	retrieved, err := provider.GetNodeResult(ctx, "node1")
	if err != nil || retrieved == nil {
		testCase.Fatalf("expected result, got err=%v, result=%v", err, retrieved)
	}
	if retrieved.Output != "data" {
		testCase.Errorf("expected output 'data', got %v", retrieved.Output)
	}

	// Non-existent result.
	nilResult, err := provider.GetNodeResult(ctx, "nonexistent")
	if err != nil {
		testCase.Fatalf("unexpected error: %v", err)
	}
	if nilResult != nil {
		testCase.Error("expected nil result for non-existent node")
	}
}

func TestInMemoryStateProvider_InitializeAndReset(testCase *testing.T) {
	provider := NewInMemoryStateProvider(nil)

	// Initialize nodes.
	provider.initializeNodes([]string{"a", "b", "c"})

	statuses := provider.getAllNodeStatuses()
	if len(statuses) != 3 {
		testCase.Fatalf("expected 3 statuses, got %d", len(statuses))
	}
	for _, nodeID := range []string{"a", "b", "c"} {
		if statuses[nodeID] != NodePending {
			testCase.Errorf("expected %q to be pending, got %s", nodeID, statuses[nodeID])
		}
	}

	// Set one to completed.
	ctx := context.Background()
	_ = provider.SetNodeStatus(ctx, "a", NodeCompleted)
	_ = provider.SetNodeResult(ctx, "a", &NodeResult{Output: "done"})

	// Reset node "a".
	provider.resetNodeState("a")
	status, _ := provider.GetNodeStatus(ctx, "a")
	if status != NodePending {
		testCase.Errorf("expected pending after reset, got %s", status)
	}
	result, _ := provider.GetNodeResult(ctx, "a")
	if result != nil {
		testCase.Error("expected nil result after reset")
	}
}

func TestValidateStateProvider(testCase *testing.T) {
	provider := NewInMemoryStateProvider(nil)
	err := validateStateProvider(context.Background(), provider)
	if err != nil {
		testCase.Errorf("expected valid state provider, got error: %v", err)
	}
}

// --- Observability Tests ---

func TestExecute_WithObservability(testCase *testing.T) {
	observer := newTestObserver()
	testClient := newTestClientWithObserver(testCase, observer)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("a", successExecutor("result_a")).
		AddNode("b", successExecutor("result_b")).
		AddEdge("a", "b").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	_, err = executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	// Verify spans were created.
	observer.mu.Lock()
	defer observer.mu.Unlock()

	if len(observer.spans) == 0 {
		testCase.Error("expected observability spans to be created")
	}

	// Should have root span and node spans.
	hasRootSpan := false
	nodeSpanCount := 0
	for _, spanName := range observer.spans {
		if spanName == spanGraphExecute {
			hasRootSpan = true
		}
		if spanName == spanGraphNodeExecute {
			nodeSpanCount++
		}
	}

	if !hasRootSpan {
		testCase.Error("expected root graph.execute span")
	}
	if nodeSpanCount != 2 {
		testCase.Errorf("expected 2 node spans, got %d", nodeSpanCount)
	}
}

func TestExecute_WithoutObservability(testCase *testing.T) {
	// Ensure no panics when observer is nil.
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("a", successExecutor("ok")).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}
	if *result.Data != "ok" {
		testCase.Errorf("expected 'ok', got %q", *result.Data)
	}
}

// --- NodeExecutorFunc Tests ---

func TestNodeExecutorFunc_SatisfiesInterface(testCase *testing.T) {
	var executor NodeExecutor = NodeExecutorFunc(func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
		return &NodeResult{Output: "test"}, nil
	})

	result, err := executor.Execute(context.Background(), &NodeInput{})
	if err != nil {
		testCase.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "test" {
		testCase.Errorf("expected 'test', got %v", result.Output)
	}
}

// --- Kahn's Algorithm Tests ---

func TestKahnTopologicalSort_LinearChain(testCase *testing.T) {
	inDegree := map[string]int{"a": 0, "b": 1, "c": 1}
	adjacency := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {},
	}
	nodeOrder := []string{"a", "b", "c"}

	order, levels, err := kahnTopologicalSort(inDegree, adjacency, nodeOrder)
	if err != nil {
		testCase.Fatalf("unexpected error: %v", err)
	}

	expectedOrder := []string{"a", "b", "c"}
	for index, nodeID := range expectedOrder {
		if order[index] != nodeID {
			testCase.Errorf("expected order[%d]=%q, got %q", index, nodeID, order[index])
		}
	}

	if len(levels) != 3 {
		testCase.Errorf("expected 3 levels, got %d", len(levels))
	}
}

func TestKahnTopologicalSort_ParallelRoots(testCase *testing.T) {
	inDegree := map[string]int{"a": 0, "b": 0, "c": 1}
	adjacency := map[string][]string{
		"a": {"c"},
		"b": {"c"},
		"c": {},
	}
	nodeOrder := []string{"a", "b", "c"}

	order, levels, err := kahnTopologicalSort(inDegree, adjacency, nodeOrder)
	if err != nil {
		testCase.Fatalf("unexpected error: %v", err)
	}

	if len(levels) != 2 {
		testCase.Errorf("expected 2 levels, got %d", len(levels))
	}

	// Level 0 should have a and b (parallel roots).
	if len(levels[0]) != 2 {
		testCase.Errorf("expected 2 roots at level 0, got %d", len(levels[0]))
	}

	// Last in topological order should be c.
	if order[len(order)-1] != "c" {
		testCase.Errorf("expected 'c' last, got %q", order[len(order)-1])
	}
}

func TestKahnTopologicalSort_Cycle(testCase *testing.T) {
	inDegree := map[string]int{"a": 1, "b": 1}
	adjacency := map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}
	nodeOrder := []string{"a", "b"}

	_, _, err := kahnTopologicalSort(inDegree, adjacency, nodeOrder)
	if err == nil {
		testCase.Fatal("expected cycle detection error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle detected") {
		testCase.Errorf("expected 'cycle detected', got: %v", err)
	}
}

// --- Options Default Tests ---

func TestOptionsDefaults(testCase *testing.T) {
	config := &graphConfig{}

	// Verify zero values.
	if config.maxConcurrency != 0 {
		testCase.Errorf("expected 0 maxConcurrency, got %d", config.maxConcurrency)
	}
	if config.executionTimeout != 0 {
		testCase.Errorf("expected 0 executionTimeout, got %v", config.executionTimeout)
	}
	if config.errorStrategy != "" {
		testCase.Errorf("expected empty error strategy, got %s", config.errorStrategy)
	}
	if config.outputNodeID != "" {
		testCase.Errorf("expected empty output node ID, got %q", config.outputNodeID)
	}
	if config.stateProvider != nil {
		testCase.Error("expected nil state provider")
	}
}

// --- Complex Topology Tests ---

func TestExecute_WideParallelFanOut(testCase *testing.T) {
	testClient := newTestClient(testCase)
	const fanOutWidth = 10
	var completedCount atomic.Int32

	builder := NewGraphBuilder[string](testClient, WithOutputNode("root"))
	builder.AddNode("root", successExecutor("root"))

	for nodeIndex := 0; nodeIndex < fanOutWidth; nodeIndex++ {
		nodeID := fmt.Sprintf("worker_%d", nodeIndex)
		builder.AddNode(nodeID, NodeExecutorFunc(func(_ context.Context, _ *NodeInput) (*NodeResult, error) {
			completedCount.Add(1)
			return &NodeResult{Output: "done"}, nil
		}))
		builder.AddEdge("root", nodeID)
	}

	executionGraph, err := builder.Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	_, err = executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	if completedCount.Load() != int32(fanOutWidth) {
		testCase.Errorf("expected %d completions, got %d", fanOutWidth, completedCount.Load())
	}
}

func TestExecute_OutputNodeNotLast(testCase *testing.T) {
	// Verify that WithOutputNode correctly picks a non-terminal node.
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient,
		WithOutputNode("middle"),
	).
		AddNode("start", successExecutor("start")).
		AddNode("middle", successExecutor("middle_value")).
		AddNode("end", successExecutor("end_value")).
		AddEdge("start", "middle").
		AddEdge("middle", "end").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	if *result.Data != "middle_value" {
		testCase.Errorf("expected 'middle_value', got %q", *result.Data)
	}
}

// --- Edge Condition Tests ---

func TestEvaluateEdgeConditions_RootNode(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("root", successExecutor("ok")).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	// Root nodes (no incoming edges) should always evaluate to true.
	stateProvider := NewInMemoryStateProvider(nil)
	result := executionGraph.evaluateEdgeConditions(context.Background(), "root", stateProvider)
	if !result {
		testCase.Error("expected root node to pass edge condition evaluation")
	}
}

func TestEvaluateEdgeConditions_UnconditionalEdge(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("a", successExecutor("a")).
		AddNode("b", successExecutor("b")).
		AddEdge("a", "b").
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	stateProvider := NewInMemoryStateProvider(nil)
	result := executionGraph.evaluateEdgeConditions(context.Background(), "b", stateProvider)
	if !result {
		testCase.Error("expected unconditional edge to pass")
	}
}

// --- Overview Integration Tests ---

func TestExecute_OverviewPopulated(testCase *testing.T) {
	testClient := newTestClient(testCase)

	executionGraph, err := NewGraphBuilder[string](testClient).
		AddNode("output", successExecutor("result")).
		Build()
	if err != nil {
		testCase.Fatalf("build error: %v", err)
	}

	result, err := executionGraph.Execute(context.Background(), nil)
	if err != nil {
		testCase.Fatalf("execute error: %v", err)
	}

	// Overview should have execution times set.
	if result.ExecutionStartTime.IsZero() {
		testCase.Error("expected ExecutionStartTime to be set")
	}
	if result.ExecutionEndTime.IsZero() {
		testCase.Error("expected ExecutionEndTime to be set")
	}
	if result.ExecutionDuration() <= 0 {
		testCase.Error("expected positive execution duration")
	}
}
