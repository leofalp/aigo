package cost

import (
	"testing"
)

func TestToolMetrics(t *testing.T) {
	metrics := ToolMetrics{
		Amount:   0.001,
		Currency: "USD",
	}

	if metrics.Amount != 0.001 {
		t.Errorf("Expected amount 0.001, got %f", metrics.Amount)
	}

	if metrics.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", metrics.Currency)
	}
}

func TestToolMetricsWithCustomCurrency(t *testing.T) {
	metrics := ToolMetrics{
		Amount:   0.05,
		Currency: "EUR",
	}

	if metrics.Amount != 0.05 {
		t.Errorf("Expected amount 0.05, got %f", metrics.Amount)
	}

	if metrics.Currency != "EUR" {
		t.Errorf("Expected currency EUR, got %s", metrics.Currency)
	}
}

func TestToolMetricsString(t *testing.T) {
	metrics := ToolMetrics{
		Amount:   0.001,
		Currency: "USD",
	}
	expected := "0.001000 USD"

	if metrics.String() != expected {
		t.Errorf("Expected %s, got %s", expected, metrics.String())
	}
}

func TestToolMetricsStringWithCostDescription(t *testing.T) {
	metrics := ToolMetrics{
		Amount:          0.001,
		Currency:        "USD",
		CostDescription: "per API call",
	}
	expected := "0.001000 USD (per API call)"

	if metrics.String() != expected {
		t.Errorf("Expected %s, got %s", expected, metrics.String())
	}
}

func TestModelCost(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:  2.50,
		OutputCostPerMillion: 10.00,
	}

	if mc.InputCostPerMillion != 2.50 {
		t.Errorf("Expected input cost 2.50, got %f", mc.InputCostPerMillion)
	}

	if mc.OutputCostPerMillion != 10.00 {
		t.Errorf("Expected output cost 10.00, got %f", mc.OutputCostPerMillion)
	}
}

func TestModelCostWithCachedCost(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:       2.50,
		OutputCostPerMillion:      10.00,
		CachedInputCostPerMillion: 1.25,
	}

	if mc.CachedInputCostPerMillion != 1.25 {
		t.Errorf("Expected cached cost 1.25, got %f", mc.CachedInputCostPerMillion)
	}
}

func TestModelCostWithReasoningCost(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:     2.50,
		OutputCostPerMillion:    10.00,
		ReasoningCostPerMillion: 5.00,
	}

	if mc.ReasoningCostPerMillion != 5.00 {
		t.Errorf("Expected reasoning cost 5.00, got %f", mc.ReasoningCostPerMillion)
	}
}

func TestModelCostCalculateInputCost(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:  2.50,
		OutputCostPerMillion: 10.00,
	}

	// Test with 1 million tokens
	cost := mc.CalculateInputCost(1_000_000)
	expected := 2.50

	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}

	// Test with 500k tokens
	cost = mc.CalculateInputCost(500_000)
	expected = 1.25

	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}
}

func TestModelCostCalculateInputCostWithTiers(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion: 2.50,
		ContextTiers: []ContextTier{
			{InputTokenThreshold: 200_000, InputCostPerMillion: 5.00},
		},
	}

	cost := mc.CalculateInputCostWithTiers(100_000)
	expected := 0.25
	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}

	cost = mc.CalculateInputCostWithTiers(300_000)
	expected = 1.50
	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}
}

func TestModelCostCalculateOutputCost(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:  2.50,
		OutputCostPerMillion: 10.00,
	}

	// Test with 1 million tokens
	cost := mc.CalculateOutputCost(1_000_000)
	expected := 10.00

	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}

	// Test with 250k tokens
	cost = mc.CalculateOutputCost(250_000)
	expected = 2.50

	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}
}

func TestModelCostCalculateOutputCostWithTiers(t *testing.T) {
	mc := ModelCost{
		OutputCostPerMillion: 10.00,
		ContextTiers: []ContextTier{
			{OutputTokenThreshold: 200_000, OutputCostPerMillion: 20.00},
		},
	}

	cost := mc.CalculateOutputCostWithTiers(100_000)
	expected := 1.00
	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}

	cost = mc.CalculateOutputCostWithTiers(300_000)
	expected = 6.00
	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}
}

func TestModelCostCalculateCachedCost(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:       2.50,
		OutputCostPerMillion:      10.00,
		CachedInputCostPerMillion: 1.25,
	}

	cost := mc.CalculateCachedCost(1_000_000)
	expected := 1.25

	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}
}

func TestModelCostCalculateReasoningCost(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:     2.50,
		OutputCostPerMillion:    10.00,
		ReasoningCostPerMillion: 5.00,
	}

	cost := mc.CalculateReasoningCost(1_000_000)
	expected := 5.00

	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}
}

func TestModelCostCalculateTotalCost(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:       2.50,
		OutputCostPerMillion:      10.00,
		CachedInputCostPerMillion: 1.25,
		ReasoningCostPerMillion:   5.00,
	}

	// 1M input, 500k output, 200k cached, 100k reasoning
	cost := mc.CalculateTotalCost(1_000_000, 500_000, 200_000, 100_000)

	// Expected: 2.50 + 5.00 + 0.25 + 0.50 = 8.25
	expected := 2.50 + 5.00 + 0.25 + 0.50

	if cost != expected {
		t.Errorf("Expected cost %f, got %f", expected, cost)
	}
}

func TestModelCostString(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:  2.50,
		OutputCostPerMillion: 10.00,
	}
	expected := "Input: $2.5000/M, Output: $10.0000/M"

	if mc.String() != expected {
		t.Errorf("Expected %s, got %s", expected, mc.String())
	}
}

func TestModelCostString_WithTiers(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:  1.25,
		OutputCostPerMillion: 10.00,
		ContextTiers: []ContextTier{
			{
				InputTokenThreshold:  200_000,
				InputCostPerMillion:  2.50,
				OutputTokenThreshold: 200_000,
				OutputCostPerMillion: 15.00,
			},
		},
	}

	result := mc.String()
	expectedPrefix := "Input: $1.2500/M, Output: $10.0000/M"
	if result[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("Expected string to start with %q, got %q", expectedPrefix, result)
	}
	if !containsMiddle(result, "Tier 1") {
		t.Errorf("Expected string to contain tier info, got %q", result)
	}
}

func TestModelCostString_WithMediaCosts(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:    2.00,
		OutputCostPerMillion:   12.00,
		ImageOutputCostPerUnit: 0.134,
	}

	result := mc.String()
	if !containsMiddle(result, "Image: $0.1340/unit") {
		t.Errorf("Expected string to contain image cost, got %q", result)
	}
}

func TestOptimizationStrategyString(t *testing.T) {
	tests := []struct {
		strategy OptimizationStrategy
		expected string
	}{
		{OptimizeForCost, "cost"},
		{OptimizeForAccuracy, "accuracy"},
		{OptimizeForSpeed, "speed"},
		{OptimizeBalanced, "balanced"},
		{OptimizeCostEffective, "cost_effective"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.strategy.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.strategy.String())
			}
		})
	}
}

func TestToolMetricsMetricsString(t *testing.T) {
	tests := []struct {
		name     string
		metrics  ToolMetrics
		expected string
	}{
		{
			name: "with accuracy only",
			metrics: ToolMetrics{
				Accuracy: 0.95,
			},
			expected: "Accuracy: 95.0%",
		},
		{
			name: "with duration only",
			metrics: ToolMetrics{
				AverageDurationInMillis: 1500,
			},
			expected: "Avg Duration: 1500ms",
		},

		{
			name: "with all metrics",
			metrics: ToolMetrics{
				Accuracy:                0.95,
				AverageDurationInMillis: 1500,
			},
			expected: "Accuracy: 95.0%, Avg Duration: 1500ms",
		},
		{
			name: "with accuracy and duration",
			metrics: ToolMetrics{
				Accuracy:                0.99,
				AverageDurationInMillis: 500,
			},
			expected: "Accuracy: 99.0%, Avg Duration: 500ms",
		},
		{
			name:     "with no metrics",
			metrics:  ToolMetrics{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metrics.MetricsString()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestToolMetricsCostEffectivenessScore(t *testing.T) {
	tests := []struct {
		name     string
		metrics  ToolMetrics
		expected float64
	}{
		{
			name: "with accuracy and cost",
			metrics: ToolMetrics{
				Amount:   0.01,
				Accuracy: 0.9,
			},
			expected: 90.0, // 0.9 / 0.01
		},
		{
			name: "with accuracy and cost",
			metrics: ToolMetrics{
				Amount:   0.02,
				Accuracy: 0.8,
			},
			expected: 40.0, // 0.8 / 0.02
		},
		{
			name: "zero cost returns zero",
			metrics: ToolMetrics{
				Amount:   0,
				Accuracy: 0.9,
			},
			expected: 0,
		},
		{
			name: "zero accuracy returns zero",
			metrics: ToolMetrics{
				Amount:   0.01,
				Accuracy: 0,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metrics.CostEffectivenessScore()
			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestToolMetricsStringWithMetrics(t *testing.T) {
	tm := ToolMetrics{
		Amount:                  0.001,
		Currency:                "USD",
		CostDescription:         "per API call",
		Accuracy:                0.95,
		AverageDurationInMillis: 1200,
	}

	result := tm.String()
	expected := "0.001000 USD (per API call)"

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}

	// Metrics should be separate
	metricsResult := tm.MetricsString()
	expectedMetrics := "Accuracy: 95.0%, Avg Duration: 1200ms"

	if metricsResult != expectedMetrics {
		t.Errorf("Expected metrics %s, got %s", expectedMetrics, metricsResult)
	}
}

func TestToolMetricsZeroCostHighAccuracy(t *testing.T) {
	// Test that tools can have zero cost with high accuracy
	// Example: local calculation tools, no external API calls
	tm := ToolMetrics{
		Amount:                  0.0, // Free tool
		Currency:                "USD",
		CostDescription:         "local execution",
		Accuracy:                1.0, // 100% accuracy
		AverageDurationInMillis: 10,  // Very fast
	}

	if tm.Amount != 0.0 {
		t.Errorf("Expected zero cost, got %f", tm.Amount)
	}

	if tm.Accuracy != 1.0 {
		t.Errorf("Expected 100%% accuracy (1.0), got %f", tm.Accuracy)
	}

	// Cost effectiveness should be 0 when cost is 0 (to avoid division by zero)
	score := tm.CostEffectivenessScore()
	if score != 0 {
		t.Errorf("Expected 0 cost effectiveness score for zero cost, got %f", score)
	}

	// Metrics string should still work
	metricsResult := tm.MetricsString()
	expectedMetrics := "Accuracy: 100.0%, Avg Duration: 10ms"
	if metricsResult != expectedMetrics {
		t.Errorf("Expected metrics %q, got %q", expectedMetrics, metricsResult)
	}

	// Cost string should show zero cost
	costString := tm.String()
	expected := "0.000000 USD (local execution)"
	if costString != expected {
		t.Errorf("Expected cost string %q, got %q", expected, costString)
	}
}

func TestComputeCost(t *testing.T) {
	cc := ComputeCost{
		CostPerSecond: 0.00167, // ~$0.10 per minute
	}

	if cc.CostPerSecond != 0.00167 {
		t.Errorf("Expected cost per second 0.00167, got %f", cc.CostPerSecond)
	}

	// Test cost calculation
	cost := cc.CalculateCost(60.0) // 60 seconds = 1 minute
	expected := 0.1002             // 60 * 0.00167
	if cost < expected-0.0001 || cost > expected+0.0001 {
		t.Errorf("Expected cost ~%.4f for 60 seconds, got %.4f", expected, cost)
	}
}

func TestComputeCostString(t *testing.T) {
	cc := ComputeCost{
		CostPerSecond: 0.00167,
	}

	result := cc.String()
	// Should show both per-second and per-minute
	if result == "" {
		t.Error("Expected non-empty string representation")
	}

	// Should contain per-second cost
	if !contains(result, "0.001670") && !contains(result, "0.00167") {
		t.Errorf("Expected string to contain per-second cost, got: %s", result)
	}
}

func TestComputeCostCalculateZeroDuration(t *testing.T) {
	cc := ComputeCost{
		CostPerSecond: 0.00167,
	}

	cost := cc.CalculateCost(0)
	if cost != 0 {
		t.Errorf("Expected 0 cost for 0 duration, got %f", cost)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Note: CostTracker has been removed - costs are now calculated directly in Overview

// --- ContextTier and tiered pricing tests ---

func TestContextTier_InputBelowThreshold_UsesBaseRate(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:  1.25,
		OutputCostPerMillion: 10.00,
		ContextTiers: []ContextTier{
			{
				InputTokenThreshold:  200_000,
				InputCostPerMillion:  2.50,
				OutputTokenThreshold: 200_000,
				OutputCostPerMillion: 15.00,
			},
		},
	}

	// 100k input tokens (below 200k threshold) -> base rate $1.25/M
	totalCost := mc.CalculateTotalCost(100_000, 50_000, 0, 0)
	expectedInput := (100_000.0 / 1_000_000.0) * 1.25  // $0.125
	expectedOutput := (50_000.0 / 1_000_000.0) * 10.00 // $0.50
	expectedTotal := expectedInput + expectedOutput    // $0.625

	if totalCost != expectedTotal {
		t.Errorf("Expected total cost %f, got %f", expectedTotal, totalCost)
	}
}

func TestContextTier_InputAboveThreshold_UsesTierRate(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:  1.25,
		OutputCostPerMillion: 10.00,
		ContextTiers: []ContextTier{
			{
				InputTokenThreshold:  200_000,
				InputCostPerMillion:  2.50,
				OutputTokenThreshold: 200_000,
				OutputCostPerMillion: 15.00,
			},
		},
	}

	// 300k input tokens (above 200k threshold) -> tier rate $2.50/M
	// 300k output tokens (above 200k threshold) -> tier rate $15.00/M
	totalCost := mc.CalculateTotalCost(300_000, 300_000, 0, 0)
	expectedInput := (300_000.0 / 1_000_000.0) * 2.50   // $0.75
	expectedOutput := (300_000.0 / 1_000_000.0) * 15.00 // $4.50
	expectedTotal := expectedInput + expectedOutput     // $5.25

	if totalCost != expectedTotal {
		t.Errorf("Expected total cost %f, got %f", expectedTotal, totalCost)
	}
}

func TestContextTier_IndependentThresholds(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:  1.25,
		OutputCostPerMillion: 10.00,
		ContextTiers: []ContextTier{
			{
				InputTokenThreshold:  200_000,
				InputCostPerMillion:  2.50,
				OutputTokenThreshold: 200_000,
				OutputCostPerMillion: 15.00,
			},
		},
	}

	// Input above threshold (300k), output below threshold (100k)
	// Input should use tier rate $2.50, output should use base rate $10.00
	totalCost := mc.CalculateTotalCost(300_000, 100_000, 0, 0)
	expectedInput := (300_000.0 / 1_000_000.0) * 2.50   // $0.75
	expectedOutput := (100_000.0 / 1_000_000.0) * 10.00 // $1.00
	expectedTotal := expectedInput + expectedOutput     // $1.75

	if totalCost != expectedTotal {
		t.Errorf("Expected total cost %f, got %f", expectedTotal, totalCost)
	}
}

func TestContextTier_ExactlyAtThreshold_UsesBaseRate(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:  1.25,
		OutputCostPerMillion: 10.00,
		ContextTiers: []ContextTier{
			{
				InputTokenThreshold:  200_000,
				InputCostPerMillion:  2.50,
				OutputTokenThreshold: 200_000,
				OutputCostPerMillion: 15.00,
			},
		},
	}

	// Exactly 200k tokens (not exceeding) -> base rate
	totalCost := mc.CalculateTotalCost(200_000, 200_000, 0, 0)
	expectedInput := (200_000.0 / 1_000_000.0) * 1.25   // $0.25
	expectedOutput := (200_000.0 / 1_000_000.0) * 10.00 // $2.00
	expectedTotal := expectedInput + expectedOutput     // $2.25

	if totalCost != expectedTotal {
		t.Errorf("Expected total cost %f, got %f", expectedTotal, totalCost)
	}
}

func TestContextTier_EmptyTiers_UsesBaseRate(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:  1.25,
		OutputCostPerMillion: 10.00,
		ContextTiers:         []ContextTier{},
	}

	totalCost := mc.CalculateTotalCost(300_000, 100_000, 0, 0)
	expectedInput := (300_000.0 / 1_000_000.0) * 1.25
	expectedOutput := (100_000.0 / 1_000_000.0) * 10.00
	expectedTotal := expectedInput + expectedOutput

	if totalCost != expectedTotal {
		t.Errorf("Expected total cost %f, got %f", expectedTotal, totalCost)
	}
}

func TestContextTier_NilTiers_UsesBaseRate(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:  1.25,
		OutputCostPerMillion: 10.00,
	}

	totalCost := mc.CalculateTotalCost(300_000, 100_000, 0, 0)
	expectedInput := (300_000.0 / 1_000_000.0) * 1.25
	expectedOutput := (100_000.0 / 1_000_000.0) * 10.00
	expectedTotal := expectedInput + expectedOutput

	if totalCost != expectedTotal {
		t.Errorf("Expected total cost %f, got %f", expectedTotal, totalCost)
	}
}

func TestContextTier_WithCachedAndReasoning(t *testing.T) {
	mc := ModelCost{
		InputCostPerMillion:       1.25,
		OutputCostPerMillion:      10.00,
		CachedInputCostPerMillion: 0.625,
		ReasoningCostPerMillion:   5.00,
		ContextTiers: []ContextTier{
			{
				InputTokenThreshold:  200_000,
				InputCostPerMillion:  2.50,
				OutputTokenThreshold: 200_000,
				OutputCostPerMillion: 15.00,
			},
		},
	}

	// 300k input (tier), 100k output (base), 50k cached, 20k reasoning
	totalCost := mc.CalculateTotalCost(300_000, 100_000, 50_000, 20_000)
	expectedInput := (300_000.0 / 1_000_000.0) * 2.50    // $0.75 (tier rate)
	expectedOutput := (100_000.0 / 1_000_000.0) * 10.00  // $1.00 (base rate)
	expectedCached := (50_000.0 / 1_000_000.0) * 0.625   // $0.03125
	expectedReasoning := (20_000.0 / 1_000_000.0) * 5.00 // $0.10
	expectedTotal := expectedInput + expectedOutput + expectedCached + expectedReasoning

	if totalCost != expectedTotal {
		t.Errorf("Expected total cost %f, got %f", expectedTotal, totalCost)
	}
}

// --- Media cost tests ---

func TestCalculateImageOutputCost(t *testing.T) {
	mc := ModelCost{
		ImageOutputCostPerUnit: 0.134,
	}

	tests := []struct {
		name     string
		count    int
		expected float64
	}{
		{"zero images", 0, 0.0},
		{"one image", 1, 0.134},
		{"five images", 5, 0.670},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mc.CalculateImageOutputCost(tt.count)
			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestCalculateVideoOutputCost(t *testing.T) {
	mc := ModelCost{
		VideoOutputCostPerUnit: 1.50,
	}

	result := mc.CalculateVideoOutputCost(3)
	expected := 4.50

	if result != expected {
		t.Errorf("Expected %f, got %f", expected, result)
	}
}

func TestCalculateAudioOutputCost(t *testing.T) {
	mc := ModelCost{
		AudioOutputCostPerUnit: 0.25,
	}

	result := mc.CalculateAudioOutputCost(4)
	expected := 1.00

	if result != expected {
		t.Errorf("Expected %f, got %f", expected, result)
	}
}

func TestCalculateMediaCost(t *testing.T) {
	mc := ModelCost{
		ImageOutputCostPerUnit: 0.134,
		VideoOutputCostPerUnit: 1.50,
		AudioOutputCostPerUnit: 0.25,
	}

	result := mc.CalculateMediaCost(2, 1, 3)
	expected := (2 * 0.134) + (1 * 1.50) + (3 * 0.25) // 0.268 + 1.50 + 0.75 = 2.518

	if result != expected {
		t.Errorf("Expected %f, got %f", expected, result)
	}
}

func TestCalculateMediaCost_ZeroCosts(t *testing.T) {
	mc := ModelCost{} // No media costs set

	result := mc.CalculateMediaCost(10, 5, 3)
	if result != 0.0 {
		t.Errorf("Expected 0 media cost when no per-unit costs set, got %f", result)
	}
}

func TestCalculateTotalCost_WithTiersAndMedia_Integration(t *testing.T) {
	// Simulates Gemini 2.5 Pro pricing with image generation
	mc := ModelCost{
		InputCostPerMillion:  1.25,
		OutputCostPerMillion: 10.00,
		ContextTiers: []ContextTier{
			{
				InputTokenThreshold:  200_000,
				InputCostPerMillion:  2.50,
				OutputTokenThreshold: 200_000,
				OutputCostPerMillion: 15.00,
			},
		},
		ImageOutputCostPerUnit: 0.134,
	}

	// Large prompt (250k input above threshold), small output (50k below threshold)
	tokenCost := mc.CalculateTotalCost(250_000, 50_000, 0, 0)
	expectedInput := (250_000.0 / 1_000_000.0) * 2.50   // $0.625 (tier rate)
	expectedOutput := (50_000.0 / 1_000_000.0) * 10.00  // $0.50 (base rate)
	expectedTokenCost := expectedInput + expectedOutput // $1.125

	if tokenCost != expectedTokenCost {
		t.Errorf("Expected token cost %f, got %f", expectedTokenCost, tokenCost)
	}

	// Plus 3 generated images
	mediaCost := mc.CalculateMediaCost(3, 0, 0)
	expectedMediaCost := 3 * 0.134 // $0.402

	if mediaCost != expectedMediaCost {
		t.Errorf("Expected media cost %f, got %f", expectedMediaCost, mediaCost)
	}

	// Total = token cost + media cost
	totalCost := tokenCost + mediaCost
	expectedTotal := expectedTokenCost + expectedMediaCost // $1.527

	if totalCost != expectedTotal {
		t.Errorf("Expected total cost %f, got %f", expectedTotal, totalCost)
	}
}
