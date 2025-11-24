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
	expected := "Input: $2.500000/M, Output: $10.000000/M"

	if mc.String() != expected {
		t.Errorf("Expected %s, got %s", expected, mc.String())
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

// Note: CostTracker has been removed - costs are now calculated directly in Overview
