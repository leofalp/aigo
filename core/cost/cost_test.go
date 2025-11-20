package cost

import (
	"testing"
)

func TestToolCost(t *testing.T) {
	cost := ToolCost{
		Amount:   0.001,
		Currency: "USD",
	}

	if cost.Amount != 0.001 {
		t.Errorf("Expected amount 0.001, got %f", cost.Amount)
	}

	if cost.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", cost.Currency)
	}
}

func TestToolCostWithCustomCurrency(t *testing.T) {
	cost := ToolCost{
		Amount:   0.05,
		Currency: "EUR",
	}

	if cost.Amount != 0.05 {
		t.Errorf("Expected amount 0.05, got %f", cost.Amount)
	}

	if cost.Currency != "EUR" {
		t.Errorf("Expected currency EUR, got %s", cost.Currency)
	}
}

func TestToolCostString(t *testing.T) {
	cost := ToolCost{
		Amount:   0.001,
		Currency: "USD",
	}
	expected := "0.001000 USD"

	if cost.String() != expected {
		t.Errorf("Expected %s, got %s", expected, cost.String())
	}
}

func TestToolCostStringWithDescription(t *testing.T) {
	cost := ToolCost{
		Amount:      0.001,
		Currency:    "USD",
		Description: "per API call",
	}
	expected := "0.001000 USD (per API call)"

	if cost.String() != expected {
		t.Errorf("Expected %s, got %s", expected, cost.String())
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
		{OptimizeForQuality, "quality"},
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

func TestToolCostMetricsString(t *testing.T) {
	tests := []struct {
		name     string
		cost     ToolCost
		expected string
	}{
		{
			name: "with accuracy only",
			cost: ToolCost{
				Accuracy: 0.95,
			},
			expected: "Accuracy: 95.0%",
		},
		{
			name: "with speed only",
			cost: ToolCost{
				Speed: 1.5,
			},
			expected: "Speed: 1.50s",
		},
		{
			name: "with quality only",
			cost: ToolCost{
				Quality: 0.88,
			},
			expected: "Quality: 88.0%",
		},
		{
			name: "with all metrics",
			cost: ToolCost{
				Accuracy: 0.95,
				Speed:    1.5,
				Quality:  0.88,
			},
			expected: "Accuracy: 95.0%, Speed: 1.50s, Quality: 88.0%",
		},
		{
			name: "with accuracy and speed",
			cost: ToolCost{
				Accuracy: 0.99,
				Speed:    0.5,
			},
			expected: "Accuracy: 99.0%, Speed: 0.50s",
		},
		{
			name:     "with no metrics",
			cost:     ToolCost{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cost.MetricsString()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestToolCostCostEffectivenessScore(t *testing.T) {
	tests := []struct {
		name     string
		cost     ToolCost
		expected float64
	}{
		{
			name: "with quality and cost",
			cost: ToolCost{
				Amount:  0.01,
				Quality: 0.9,
			},
			expected: 90.0, // 0.9 / 0.01
		},
		{
			name: "with accuracy and cost (quality fallback)",
			cost: ToolCost{
				Amount:   0.05,
				Accuracy: 0.85,
			},
			expected: 17.0, // 0.85 / 0.05
		},
		{
			name: "quality takes precedence over accuracy",
			cost: ToolCost{
				Amount:   0.02,
				Quality:  0.95,
				Accuracy: 0.80,
			},
			expected: 47.5, // 0.95 / 0.02
		},
		{
			name: "zero cost",
			cost: ToolCost{
				Amount:  0,
				Quality: 0.9,
			},
			expected: 0,
		},
		{
			name: "zero quality and accuracy",
			cost: ToolCost{
				Amount: 0.01,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cost.CostEffectivenessScore()
			epsilon := 0.0001
			if result < tt.expected-epsilon || result > tt.expected+epsilon {
				t.Errorf("Expected %.4f, got %.4f", tt.expected, result)
			}
		})
	}
}

func TestToolCostStringWithMetrics(t *testing.T) {
	tc := ToolCost{
		Amount:      0.001,
		Currency:    "USD",
		Description: "per API call",
		Accuracy:    0.95,
		Speed:       1.2,
		Quality:     0.90,
	}

	result := tc.String()
	expected := "0.001000 USD (per API call)"

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}

	// Metrics should be separate
	metricsResult := tc.MetricsString()
	expectedMetrics := "Accuracy: 95.0%, Speed: 1.20s, Quality: 90.0%"

	if metricsResult != expectedMetrics {
		t.Errorf("Expected metrics %s, got %s", expectedMetrics, metricsResult)
	}
}

// Note: CostTracker has been removed - costs are now calculated directly in Overview
