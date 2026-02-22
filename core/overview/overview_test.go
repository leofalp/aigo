package overview

import (
	"context"
	"testing"
	"time"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/providers/ai"
)

// ========== OverviewFromContext / ToContext ==========

// TestOverviewFromContext_CreatesNew verifies that a new Overview is created and
// injected into the context pointer when none is stored yet.
func TestOverviewFromContext_CreatesNew(t *testing.T) {
	ctx := context.Background()
	overview := OverviewFromContext(&ctx)

	if overview == nil {
		t.Fatal("expected a new Overview, got nil")
	}

	// The context pointer should now carry the Overview.
	retrieved := ctx.Value(overviewContextKey)
	if retrieved == nil {
		t.Error("expected context to be updated with the new Overview")
	}
}

// TestOverviewFromContext_ReturnsExisting verifies that the same Overview pointer
// is returned when one is already present in the context.
func TestOverviewFromContext_ReturnsExisting(t *testing.T) {
	ctx := context.Background()

	// Create the first one.
	first := OverviewFromContext(&ctx)
	// Call again — must return the same instance.
	second := OverviewFromContext(&ctx)

	if first != second {
		t.Error("expected the same Overview pointer on second call, got different pointer")
	}
}

// TestOverviewFromContext_WrongType verifies that nil is returned when the context
// carries a value under the overview key but of the wrong type.
func TestOverviewFromContext_WrongType(t *testing.T) {
	// Manually store a non-Overview value under the key.
	ctx := context.WithValue(context.Background(), overviewContextKey, "not-an-overview")
	result := OverviewFromContext(&ctx)
	if result != nil {
		t.Errorf("expected nil for wrong type, got %v", result)
	}
}

// TestToContext_NilContext verifies that ToContext uses context.Background() when
// the provided context is nil, rather than panicking.
func TestToContext_NilContext(t *testing.T) {
	overview := &Overview{}
	// Pass a typed nil context to exercise the nil-guard inside ToContext.
	// A typed nil is required to satisfy staticcheck SA1012.
	var nilCtx context.Context
	result := overview.ToContext(nilCtx)
	if result == nil {
		t.Error("expected non-nil context when nil is passed to ToContext")
	}
}

// TestToContext_Roundtrip verifies that storing and retrieving an Overview via
// ToContext and OverviewFromContext returns the same pointer.
func TestToContext_Roundtrip(t *testing.T) {
	original := &Overview{TotalUsage: ai.Usage{TotalTokens: 42}}
	ctx := original.ToContext(context.Background())

	ctxPtr := ctx
	retrieved := OverviewFromContext(&ctxPtr)
	if retrieved != original {
		t.Errorf("expected the same Overview pointer after roundtrip, got different pointer")
	}
}

// ========== IncludeUsage ==========

// TestIncludeUsage_NilUsage verifies that passing a nil *ai.Usage is a no-op and
// does not panic.
func TestIncludeUsage_NilUsage(t *testing.T) {
	overview := &Overview{}
	overview.IncludeUsage(nil) // must not panic

	if overview.TotalUsage.TotalTokens != 0 {
		t.Errorf("expected no tokens after nil IncludeUsage, got %d", overview.TotalUsage.TotalTokens)
	}
}

// TestIncludeUsage_AccumulatesTokens verifies that multiple IncludeUsage calls
// sum all token fields correctly.
func TestIncludeUsage_AccumulatesTokens(t *testing.T) {
	overview := &Overview{}

	usage1 := &ai.Usage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		ReasoningTokens:  5,
		CachedTokens:     3,
	}
	usage2 := &ai.Usage{
		PromptTokens:     15,
		CompletionTokens: 25,
		TotalTokens:      40,
		ReasoningTokens:  7,
		CachedTokens:     2,
	}

	overview.IncludeUsage(usage1)
	overview.IncludeUsage(usage2)

	if overview.TotalUsage.PromptTokens != 25 {
		t.Errorf("PromptTokens: expected 25, got %d", overview.TotalUsage.PromptTokens)
	}
	if overview.TotalUsage.CompletionTokens != 45 {
		t.Errorf("CompletionTokens: expected 45, got %d", overview.TotalUsage.CompletionTokens)
	}
	if overview.TotalUsage.TotalTokens != 70 {
		t.Errorf("TotalTokens: expected 70, got %d", overview.TotalUsage.TotalTokens)
	}
	if overview.TotalUsage.ReasoningTokens != 12 {
		t.Errorf("ReasoningTokens: expected 12, got %d", overview.TotalUsage.ReasoningTokens)
	}
	if overview.TotalUsage.CachedTokens != 5 {
		t.Errorf("CachedTokens: expected 5, got %d", overview.TotalUsage.CachedTokens)
	}
}

// ========== AddToolCalls ==========

// TestAddToolCalls_InitializesMap verifies that AddToolCalls lazily initializes
// the ToolCallStats map when it is nil.
func TestAddToolCalls_InitializesMap(t *testing.T) {
	overview := &Overview{} // ToolCallStats is nil

	overview.AddToolCalls([]ai.ToolCall{
		{Function: ai.ToolCallFunction{Name: "search"}},
	})

	if overview.ToolCallStats == nil {
		t.Fatal("expected ToolCallStats to be initialized, got nil")
	}
	if overview.ToolCallStats["search"] != 1 {
		t.Errorf("expected search count 1, got %d", overview.ToolCallStats["search"])
	}
}

// TestAddToolCalls_CountsCorrectly verifies that calling the same tool multiple
// times increments the counter correctly.
func TestAddToolCalls_CountsCorrectly(t *testing.T) {
	overview := &Overview{}

	overview.AddToolCalls([]ai.ToolCall{
		{Function: ai.ToolCallFunction{Name: "calculator"}},
		{Function: ai.ToolCallFunction{Name: "search"}},
		{Function: ai.ToolCallFunction{Name: "calculator"}},
	})

	if overview.ToolCallStats["calculator"] != 2 {
		t.Errorf("expected calculator count 2, got %d", overview.ToolCallStats["calculator"])
	}
	if overview.ToolCallStats["search"] != 1 {
		t.Errorf("expected search count 1, got %d", overview.ToolCallStats["search"])
	}
}

// ========== AddRequest / AddResponse ==========

// TestAddRequest_AppendsToSlice verifies that AddRequest appends entries to the
// Requests history.
func TestAddRequest_AppendsToSlice(t *testing.T) {
	overview := &Overview{}

	req1 := &ai.ChatRequest{Model: "m1"}
	req2 := &ai.ChatRequest{Model: "m2"}
	overview.AddRequest(req1)
	overview.AddRequest(req2)

	if len(overview.Requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(overview.Requests))
	}
	if overview.Requests[0] != req1 {
		t.Error("expected first request to be req1")
	}
	if overview.Requests[1] != req2 {
		t.Error("expected second request to be req2")
	}
}

// TestAddResponse_AppendsAndSetsLastResponse verifies that AddResponse appends
// entries to the Responses history and keeps LastResponse pointing to the most
// recent one.
func TestAddResponse_AppendsAndSetsLastResponse(t *testing.T) {
	overview := &Overview{}

	resp1 := &ai.ChatResponse{Content: "first"}
	resp2 := &ai.ChatResponse{Content: "second"}
	overview.AddResponse(resp1)
	overview.AddResponse(resp2)

	if len(overview.Responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(overview.Responses))
	}
	if overview.LastResponse != resp2 {
		t.Errorf("expected LastResponse to be resp2 (most recent), got %v", overview.LastResponse)
	}
}

// ========== AddToolExecutionCost ==========

// TestAddToolExecutionCost_NilMetrics verifies that nil ToolMetrics is a no-op.
func TestAddToolExecutionCost_NilMetrics(t *testing.T) {
	overview := &Overview{}
	overview.AddToolExecutionCost("calculator", nil) // must not panic

	if len(overview.ToolCosts) != 0 {
		t.Errorf("expected no tool costs after nil metrics, got %v", overview.ToolCosts)
	}
}

// TestAddToolExecutionCost_AccumulatesPerTool verifies that costs are accumulated
// independently per tool name across multiple calls.
func TestAddToolExecutionCost_AccumulatesPerTool(t *testing.T) {
	overview := &Overview{}

	overview.AddToolExecutionCost("search", &cost.ToolMetrics{Amount: 0.005})
	overview.AddToolExecutionCost("search", &cost.ToolMetrics{Amount: 0.003})
	overview.AddToolExecutionCost("calc", &cost.ToolMetrics{Amount: 0.001})

	const epsilon = 1e-9
	if diff := overview.ToolCosts["search"] - 0.008; diff > epsilon || diff < -epsilon {
		t.Errorf("expected search cost 0.008, got %f", overview.ToolCosts["search"])
	}
	if diff := overview.ToolCosts["calc"] - 0.001; diff > epsilon || diff < -epsilon {
		t.Errorf("expected calc cost 0.001, got %f", overview.ToolCosts["calc"])
	}
}

// ========== ExecutionDuration ==========

// TestExecutionDuration_NotStarted verifies that ExecutionDuration returns 0 when
// neither StartExecution nor EndExecution has been called.
func TestExecutionDuration_NotStarted(t *testing.T) {
	overview := &Overview{}
	if d := overview.ExecutionDuration(); d != 0 {
		t.Errorf("expected 0 duration before execution, got %v", d)
	}
}

// TestExecutionDuration_OnlyStarted verifies that ExecutionDuration returns 0 when
// StartExecution was called but EndExecution has not been.
func TestExecutionDuration_OnlyStarted(t *testing.T) {
	overview := &Overview{}
	overview.StartExecution()
	if d := overview.ExecutionDuration(); d != 0 {
		t.Errorf("expected 0 duration when only started, got %v", d)
	}
}

// TestExecutionDuration_Valid verifies that ExecutionDuration returns a positive
// duration after both StartExecution and EndExecution have been called.
func TestExecutionDuration_Valid(t *testing.T) {
	overview := &Overview{}
	overview.StartExecution()
	time.Sleep(2 * time.Millisecond)
	overview.EndExecution()

	duration := overview.ExecutionDuration()
	if duration <= 0 {
		t.Errorf("expected positive duration, got %v", duration)
	}
}

// ========== CostSummary / TotalCost ==========

// TestCostSummary_NoCosts verifies that CostSummary returns all-zero costs when
// no tools were called and no model/compute configuration is set.
func TestCostSummary_NoCosts(t *testing.T) {
	overview := &Overview{}
	summary := overview.CostSummary()

	if summary.TotalCost != 0 {
		t.Errorf("expected TotalCost 0, got %f", summary.TotalCost)
	}
	if summary.TotalToolCost != 0 {
		t.Errorf("expected TotalToolCost 0, got %f", summary.TotalToolCost)
	}
	if summary.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", summary.Currency)
	}
}

// TestCostSummary_ToolsOnly verifies that tool execution costs are correctly
// summed when no model or compute cost is configured.
func TestCostSummary_ToolsOnly(t *testing.T) {
	overview := &Overview{}
	overview.AddToolExecutionCost("search", &cost.ToolMetrics{Amount: 0.01})
	overview.AddToolExecutionCost("calc", &cost.ToolMetrics{Amount: 0.005})

	summary := overview.CostSummary()

	const epsilon = 1e-9
	if diff := summary.TotalToolCost - 0.015; diff > epsilon || diff < -epsilon {
		t.Errorf("expected TotalToolCost 0.015, got %f", summary.TotalToolCost)
	}
	if diff := summary.TotalCost - 0.015; diff > epsilon || diff < -epsilon {
		t.Errorf("expected TotalCost 0.015, got %f", summary.TotalCost)
	}
}

// TestCostSummary_WithModelCost verifies that model input and output costs are
// calculated from token usage and the configured ModelCost pricing.
func TestCostSummary_WithModelCost(t *testing.T) {
	overview := &Overview{}
	overview.SetModelCost(&cost.ModelCost{
		InputCostPerMillion:  1.0, // $1 per million input tokens
		OutputCostPerMillion: 2.0, // $2 per million output tokens
	})
	overview.IncludeUsage(&ai.Usage{
		PromptTokens:     1_000_000, // exactly 1M input tokens → $1.00
		CompletionTokens: 500_000,   // 0.5M output tokens → $1.00
		TotalTokens:      1_500_000,
	})

	summary := overview.CostSummary()

	const epsilon = 1e-6
	if diff := summary.ModelInputCost - 1.0; diff > epsilon || diff < -epsilon {
		t.Errorf("expected ModelInputCost 1.0, got %f", summary.ModelInputCost)
	}
	if diff := summary.ModelOutputCost - 1.0; diff > epsilon || diff < -epsilon {
		t.Errorf("expected ModelOutputCost 1.0, got %f", summary.ModelOutputCost)
	}
	if diff := summary.TotalModelCost - 2.0; diff > epsilon || diff < -epsilon {
		t.Errorf("expected TotalModelCost 2.0, got %f", summary.TotalModelCost)
	}
}

// TestCostSummary_WithComputeCost verifies that infrastructure cost is calculated
// from execution duration and the configured ComputeCost rate.
func TestCostSummary_WithComputeCost(t *testing.T) {
	overview := &Overview{}
	// Set a fixed start/end time to avoid test flakiness from sleep timing.
	overview.ExecutionStartTime = time.Unix(0, 0)
	overview.ExecutionEndTime = time.Unix(2, 0) // 2 seconds

	overview.SetComputeCost(&cost.ComputeCost{
		CostPerSecond: 0.5, // $0.50/s → 2s = $1.00
	})

	summary := overview.CostSummary()

	const epsilon = 1e-6
	if diff := summary.ComputeCost - 1.0; diff > epsilon || diff < -epsilon {
		t.Errorf("expected ComputeCost 1.0, got %f", summary.ComputeCost)
	}
	if diff := summary.ExecutionDurationSeconds - 2.0; diff > epsilon || diff < -epsilon {
		t.Errorf("expected ExecutionDurationSeconds 2.0, got %f", summary.ExecutionDurationSeconds)
	}
}

// TestTotalCost_DelegatesToCostSummary verifies that TotalCost() returns the same
// value as CostSummary().TotalCost so there is no divergence between the two APIs.
func TestTotalCost_DelegatesToCostSummary(t *testing.T) {
	overview := &Overview{}
	overview.AddToolExecutionCost("search", &cost.ToolMetrics{Amount: 0.007})

	if overview.TotalCost() != overview.CostSummary().TotalCost {
		t.Error("TotalCost() and CostSummary().TotalCost should return the same value")
	}
}

// TestCostSummary_AllCostsCombined verifies that TotalCost is the correct sum of
// tool, model, and compute costs when all three are configured.
func TestCostSummary_AllCostsCombined(t *testing.T) {
	overview := &Overview{}

	// Tool cost: $0.01
	overview.AddToolExecutionCost("search", &cost.ToolMetrics{Amount: 0.01})

	// Model cost: $1 per million input → 1M tokens = $1.00
	overview.SetModelCost(&cost.ModelCost{InputCostPerMillion: 1.0})
	overview.IncludeUsage(&ai.Usage{PromptTokens: 1_000_000, TotalTokens: 1_000_000})

	// Compute cost: $0.50/s × 2s = $1.00
	overview.ExecutionStartTime = time.Unix(0, 0)
	overview.ExecutionEndTime = time.Unix(2, 0)
	overview.SetComputeCost(&cost.ComputeCost{CostPerSecond: 0.5})

	summary := overview.CostSummary()
	expectedTotal := 0.01 + 1.0 + 1.0

	const epsilon = 1e-6
	if diff := summary.TotalCost - expectedTotal; diff > epsilon || diff < -epsilon {
		t.Errorf("expected TotalCost %f, got %f", expectedTotal, summary.TotalCost)
	}
}
