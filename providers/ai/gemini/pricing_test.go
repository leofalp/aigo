package gemini

import (
	"testing"

	"github.com/leofalp/aigo/providers/ai"
)

func TestGetModelCost_KnownModels(t *testing.T) {
	tests := []struct {
		model              string
		expectedInputCost  float64
		expectedOutputCost float64
	}{
		{Model25Pro, 1.25, 10.00},
		{Model25Flash, 0.30, 2.50},
		{Model25FlashLite, 0.10, 0.40},
		{Model20Flash, 0.10, 0.40},
		{Model20FlashLite, 0.075, 0.30},
		{Model15Pro, 1.25, 5.00},
		{Model15Flash, 0.075, 0.30},
		{Model30ProPreview, 2.00, 12.00},
		{Model30FlashPreview, 0.50, 3.00},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			mc := GetModelCost(tt.model)
			if mc.InputCostPerMillion != tt.expectedInputCost {
				t.Errorf("GetModelCost(%q).InputCostPerMillion = %v, want %v",
					tt.model, mc.InputCostPerMillion, tt.expectedInputCost)
			}
			if mc.OutputCostPerMillion != tt.expectedOutputCost {
				t.Errorf("GetModelCost(%q).OutputCostPerMillion = %v, want %v",
					tt.model, mc.OutputCostPerMillion, tt.expectedOutputCost)
			}
		})
	}
}

func TestGetModelCost_UnknownModel_ReturnsFallback(t *testing.T) {
	// Unknown models should return the default (gemini-2.0-flash-lite) pricing
	mc := GetModelCost("unknown-model-xyz")
	expected := ModelPricing[Model20FlashLite]

	if mc.InputCostPerMillion != expected.InputCostPerMillion {
		t.Errorf("GetModelCost(unknown).InputCostPerMillion = %v, want %v",
			mc.InputCostPerMillion, expected.InputCostPerMillion)
	}
	if mc.OutputCostPerMillion != expected.OutputCostPerMillion {
		t.Errorf("GetModelCost(unknown).OutputCostPerMillion = %v, want %v",
			mc.OutputCostPerMillion, expected.OutputCostPerMillion)
	}
}

func TestGetModelCost_NormalizesVersionedModels(t *testing.T) {
	// Versioned model names should be normalized to base model
	tests := []struct {
		model    string
		expected string
	}{
		{"gemini-2.0-flash-001", Model20Flash},
		{"gemini-2.5-pro-exp-0827", Model25Pro},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			mc := GetModelCost(tt.model)
			expectedMc := GetModelCost(tt.expected)

			if mc.InputCostPerMillion != expectedMc.InputCostPerMillion {
				t.Errorf("GetModelCost(%q).InputCostPerMillion = %v, want %v",
					tt.model, mc.InputCostPerMillion, expectedMc.InputCostPerMillion)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		usage    *ai.Usage
		expected float64
	}{
		{
			name:     "nil usage returns zero",
			model:    Model20FlashLite,
			usage:    nil,
			expected: 0,
		},
		{
			name:  "1M input tokens for gemini-2.0-flash-lite",
			model: Model20FlashLite,
			usage: &ai.Usage{
				PromptTokens:     1_000_000,
				CompletionTokens: 0,
			},
			expected: 0.075, // $0.075 per 1M input tokens
		},
		{
			name:  "1M output tokens for gemini-2.0-flash-lite",
			model: Model20FlashLite,
			usage: &ai.Usage{
				PromptTokens:     0,
				CompletionTokens: 1_000_000,
			},
			expected: 0.30, // $0.30 per 1M output tokens
		},
		{
			name:  "mixed usage for gemini-2.5-pro",
			model: Model25Pro,
			usage: &ai.Usage{
				PromptTokens:     500_000, // 0.5M * $1.25 = $0.625
				CompletionTokens: 100_000, // 0.1M * $10 = $1.00
				CachedTokens:     200_000, // 0.2M * $0.625 = $0.125
				ReasoningTokens:  50_000,  // 0.05M * $10 = $0.50
			},
			expected: 2.25, // $0.625 + $1.00 + $0.125 + $0.50
		},
		{
			name:  "typical small request",
			model: Model20FlashLite,
			usage: &ai.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
			// 1000/1M * $0.075 + 500/1M * $0.30 = $0.000075 + $0.00015 = $0.000225
			expected: 0.000225,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateCost(tt.model, tt.usage)
			// Use a small epsilon for floating point comparison
			epsilon := 0.0000001
			if diff := result - tt.expected; diff > epsilon || diff < -epsilon {
				t.Errorf("CalculateCost(%q, usage) = %v, want %v",
					tt.model, result, tt.expected)
			}
		})
	}
}

func TestCalculateCostBreakdown(t *testing.T) {
	usage := &ai.Usage{
		PromptTokens:     100_000,
		CompletionTokens: 50_000,
		CachedTokens:     20_000,
		ReasoningTokens:  10_000,
	}

	breakdown := CalculateCostBreakdown(Model25Pro, usage)

	// Verify all fields are populated
	if breakdown.Model != Model25Pro {
		t.Errorf("breakdown.Model = %q, want %q", breakdown.Model, Model25Pro)
	}
	if breakdown.InputTokens != 100_000 {
		t.Errorf("breakdown.InputTokens = %d, want %d", breakdown.InputTokens, 100_000)
	}
	if breakdown.OutputTokens != 50_000 {
		t.Errorf("breakdown.OutputTokens = %d, want %d", breakdown.OutputTokens, 50_000)
	}
	if breakdown.CachedTokens != 20_000 {
		t.Errorf("breakdown.CachedTokens = %d, want %d", breakdown.CachedTokens, 20_000)
	}
	if breakdown.ReasoningTokens != 10_000 {
		t.Errorf("breakdown.ReasoningTokens = %d, want %d", breakdown.ReasoningTokens, 10_000)
	}

	// Verify costs are calculated correctly
	// Input: 100k/1M * $1.25 = $0.125
	// Output: 50k/1M * $10 = $0.50
	// Cached: 20k/1M * $0.625 = $0.0125
	// Reasoning: 10k/1M * $10 = $0.10
	// Total: $0.7375

	epsilon := 0.0000001
	expectedTotal := 0.7375
	if diff := breakdown.TotalCost - expectedTotal; diff > epsilon || diff < -epsilon {
		t.Errorf("breakdown.TotalCost = %v, want %v", breakdown.TotalCost, expectedTotal)
	}
}

func TestCalculateCostBreakdown_NilUsage(t *testing.T) {
	breakdown := CalculateCostBreakdown(Model20FlashLite, nil)

	if breakdown.Model != Model20FlashLite {
		t.Errorf("breakdown.Model = %q, want %q", breakdown.Model, Model20FlashLite)
	}
	if breakdown.TotalCost != 0 {
		t.Errorf("breakdown.TotalCost = %v, want 0", breakdown.TotalCost)
	}
}

func TestNormalizeModelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gemini-2.0-flash-001", "gemini-2.0-flash"},
		{"gemini-2.0-flash-002", "gemini-2.0-flash"},
		{"gemini-2.5-pro-exp-0827", "gemini-2.5-pro"},
		{"gemini-1.5-flash-8b-exp-0924", "gemini-1.5-flash-8b"},
		{"gemini-2.5-flash-preview-04-17", "gemini-2.5-flash"},
		{"gemini-2.0-flash", "gemini-2.0-flash"}, // No change needed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeModelName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeModelName(%q) = %q, want %q",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestModelPricingMap_AllModelsHaveRequiredFields(t *testing.T) {
	for model, pricing := range ModelPricing {
		t.Run(model, func(t *testing.T) {
			if pricing.InputCostPerMillion <= 0 {
				t.Errorf("Model %q has zero or negative InputCostPerMillion", model)
			}
			if pricing.OutputCostPerMillion <= 0 {
				t.Errorf("Model %q has zero or negative OutputCostPerMillion", model)
			}
			// CachedInputCostPerMillion should be less than InputCostPerMillion
			if pricing.CachedInputCostPerMillion >= pricing.InputCostPerMillion {
				t.Errorf("Model %q has CachedInputCostPerMillion >= InputCostPerMillion", model)
			}
		})
	}
}
