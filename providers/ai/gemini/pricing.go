package gemini

import (
	"strings"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/providers/ai"
)

// Model name constants for Gemini models.
// Use these constants instead of raw strings for type safety and autocompletion.
const (
	// Gemini 2.5 models
	Model25Pro              = "gemini-2.5-pro"
	Model25ProLatest        = "gemini-2.5-pro-latest"
	Model25ProPreview       = "gemini-2.5-pro-preview-05-06"
	Model25Flash            = "gemini-2.5-flash"
	Model25FlashLatest      = "gemini-2.5-flash-latest"
	Model25FlashPreview     = "gemini-2.5-flash-preview-04-17"
	Model25FlashLite        = "gemini-2.5-flash-lite"
	Model25FlashLiteLatest  = "gemini-2.5-flash-lite-latest"
	Model25FlashLitePreview = "gemini-2.5-flash-lite-preview-06-17"

	// Gemini 2.0 models
	Model20Flash       = "gemini-2.0-flash"
	Model20FlashLatest = "gemini-2.0-flash-latest"
	Model20FlashExp    = "gemini-2.0-flash-exp"
	Model20FlashLite   = "gemini-2.0-flash-lite"

	// Gemini 1.5 models (legacy)
	Model15Pro        = "gemini-1.5-pro"
	Model15ProLatest  = "gemini-1.5-pro-latest"
	Model15Flash      = "gemini-1.5-flash"
	Model15Flash8B    = "gemini-1.5-flash-8b"
	Model15Flash8BExp = "gemini-1.5-flash-8b-exp-0924"

	// Gemini 3.0 Preview models (experimental)
	Model30ProPreview   = "gemini-3-pro-preview"
	Model30FlashPreview = "gemini-3-flash-preview"
)

// ModelPricing contains pricing information for all supported Gemini models.
// Prices are in USD per million tokens.
// Source: https://ai.google.dev/gemini-api/docs/pricing (January 2025)
//
// Note: Some models have tiered pricing based on context length (≤200k vs >200k tokens).
// This implementation uses the standard tier (≤200k) pricing.
// For high-volume use cases, consider implementing context-aware pricing.
var ModelPricing = map[string]cost.ModelCost{
	// Gemini 2.5 Pro - Most capable model
	// Input: $1.25/M (≤200k), $2.50/M (>200k)
	// Output: $10.00/M (≤200k), $15.00/M (>200k)
	// Cached: 50% discount on input
	Model25Pro: {
		InputCostPerMillion:       1.25,
		OutputCostPerMillion:      10.00,
		CachedInputCostPerMillion: 0.625, // 50% of input
		ReasoningCostPerMillion:   10.00, // Same as output
	},
	Model25ProLatest: {
		InputCostPerMillion:       1.25,
		OutputCostPerMillion:      10.00,
		CachedInputCostPerMillion: 0.625,
		ReasoningCostPerMillion:   10.00,
	},
	Model25ProPreview: {
		InputCostPerMillion:       1.25,
		OutputCostPerMillion:      10.00,
		CachedInputCostPerMillion: 0.625,
		ReasoningCostPerMillion:   10.00,
	},

	// Gemini 2.5 Flash - Fast and efficient
	// Input: $0.30/M (text/image/video), $1.00/M (audio)
	// Output: $2.50/M
	Model25Flash: {
		InputCostPerMillion:       0.30,
		OutputCostPerMillion:      2.50,
		CachedInputCostPerMillion: 0.15, // 50% of input
		ReasoningCostPerMillion:   2.50, // Same as output
	},
	Model25FlashLatest: {
		InputCostPerMillion:       0.30,
		OutputCostPerMillion:      2.50,
		CachedInputCostPerMillion: 0.15,
		ReasoningCostPerMillion:   2.50,
	},
	Model25FlashPreview: {
		InputCostPerMillion:       0.30,
		OutputCostPerMillion:      2.50,
		CachedInputCostPerMillion: 0.15,
		ReasoningCostPerMillion:   2.50,
	},

	// Gemini 2.5 Flash Lite - Most cost-effective
	// Input: $0.10/M (text/image/video), $0.30/M (audio)
	// Output: $0.40/M
	Model25FlashLite: {
		InputCostPerMillion:       0.10,
		OutputCostPerMillion:      0.40,
		CachedInputCostPerMillion: 0.05, // 50% of input
		ReasoningCostPerMillion:   0.40, // Same as output
	},
	Model25FlashLiteLatest: {
		InputCostPerMillion:       0.10,
		OutputCostPerMillion:      0.40,
		CachedInputCostPerMillion: 0.05,
		ReasoningCostPerMillion:   0.40,
	},
	Model25FlashLitePreview: {
		InputCostPerMillion:       0.10,
		OutputCostPerMillion:      0.40,
		CachedInputCostPerMillion: 0.05,
		ReasoningCostPerMillion:   0.40,
	},

	// Gemini 2.0 Flash
	// Input: $0.10/M (text/image/video), $0.70/M (audio)
	// Output: $0.40/M
	Model20Flash: {
		InputCostPerMillion:       0.10,
		OutputCostPerMillion:      0.40,
		CachedInputCostPerMillion: 0.05,
		ReasoningCostPerMillion:   0.40,
	},
	Model20FlashLatest: {
		InputCostPerMillion:       0.10,
		OutputCostPerMillion:      0.40,
		CachedInputCostPerMillion: 0.05,
		ReasoningCostPerMillion:   0.40,
	},
	Model20FlashExp: {
		InputCostPerMillion:       0.10,
		OutputCostPerMillion:      0.40,
		CachedInputCostPerMillion: 0.05,
		ReasoningCostPerMillion:   0.40,
	},

	// Gemini 2.0 Flash Lite - Most cost-effective for simple tasks
	// Input: $0.075/M
	// Output: $0.30/M
	Model20FlashLite: {
		InputCostPerMillion:       0.075,
		OutputCostPerMillion:      0.30,
		CachedInputCostPerMillion: 0.0375, // 50% of input
		ReasoningCostPerMillion:   0.30,   // Same as output
	},

	// Gemini 1.5 Pro (legacy)
	// Input: $1.25/M (≤128k), $2.50/M (>128k)
	// Output: $5.00/M (≤128k), $10.00/M (>128k)
	Model15Pro: {
		InputCostPerMillion:       1.25,
		OutputCostPerMillion:      5.00,
		CachedInputCostPerMillion: 0.3125, // 75% discount
		ReasoningCostPerMillion:   5.00,
	},
	Model15ProLatest: {
		InputCostPerMillion:       1.25,
		OutputCostPerMillion:      5.00,
		CachedInputCostPerMillion: 0.3125,
		ReasoningCostPerMillion:   5.00,
	},

	// Gemini 1.5 Flash (legacy)
	// Input: $0.075/M (≤128k), $0.15/M (>128k)
	// Output: $0.30/M (≤128k), $0.60/M (>128k)
	Model15Flash: {
		InputCostPerMillion:       0.075,
		OutputCostPerMillion:      0.30,
		CachedInputCostPerMillion: 0.01875, // 75% discount
		ReasoningCostPerMillion:   0.30,
	},

	// Gemini 1.5 Flash 8B (legacy, smaller model)
	// Input: $0.0375/M (≤128k), $0.075/M (>128k)
	// Output: $0.15/M (≤128k), $0.30/M (>128k)
	Model15Flash8B: {
		InputCostPerMillion:       0.0375,
		OutputCostPerMillion:      0.15,
		CachedInputCostPerMillion: 0.009375, // 75% discount
		ReasoningCostPerMillion:   0.15,
	},
	Model15Flash8BExp: {
		InputCostPerMillion:       0.0375,
		OutputCostPerMillion:      0.15,
		CachedInputCostPerMillion: 0.009375,
		ReasoningCostPerMillion:   0.15,
	},

	// Gemini 3.0 Preview models (experimental pricing)
	// Input: $2.00/M (≤200k), $4.00/M (>200k)
	// Output: $12.00/M (≤200k), $18.00/M (>200k)
	Model30ProPreview: {
		InputCostPerMillion:       2.00,
		OutputCostPerMillion:      12.00,
		CachedInputCostPerMillion: 1.00, // 50% of input
		ReasoningCostPerMillion:   12.00,
	},

	// Gemini 3.0 Flash Preview
	// Input: $0.50/M
	// Output: $3.00/M
	Model30FlashPreview: {
		InputCostPerMillion:       0.50,
		OutputCostPerMillion:      3.00,
		CachedInputCostPerMillion: 0.25, // 50% of input
		ReasoningCostPerMillion:   3.00,
	},
}

// GetModelCost returns the cost configuration for a given model name.
// It handles model name variations (e.g., "gemini-2.0-flash" matches "gemini-2.0-flash-latest").
// Returns a zero-value ModelCost if the model is not found.
func GetModelCost(model string) cost.ModelCost {
	// Direct lookup first
	if mc, ok := ModelPricing[model]; ok {
		return mc
	}

	// Try to find a matching model by prefix
	// This handles cases like "gemini-2.0-flash-001" -> "gemini-2.0-flash"
	normalizedModel := normalizeModelName(model)
	if mc, ok := ModelPricing[normalizedModel]; ok {
		return mc
	}

	// Default fallback to gemini-2.0-flash-lite (most cost-effective)
	return ModelPricing[Model20FlashLite]
}

// normalizeModelName attempts to normalize model names to match our pricing map.
// Examples:
//   - "gemini-2.0-flash-001" -> "gemini-2.0-flash"
//   - "gemini-2.5-pro-exp-0827" -> "gemini-2.5-pro"
func normalizeModelName(model string) string {
	// Common patterns to strip
	suffixes := []string{
		"-001", "-002", "-003",
		"-exp-0827", "-exp-0924", "-exp",
		"-preview-04-17", "-preview-05-06", "-preview-06-17",
	}

	normalized := model
	for _, suffix := range suffixes {
		if strings.HasSuffix(normalized, suffix) {
			normalized = strings.TrimSuffix(normalized, suffix)
			break
		}
	}

	return normalized
}

// CalculateCost calculates the total cost for a given model and usage.
// It takes into account input, output, cached, and reasoning tokens.
func CalculateCost(model string, usage *ai.Usage) float64 {
	if usage == nil {
		return 0
	}

	mc := GetModelCost(model)
	return mc.CalculateTotalCost(
		usage.PromptTokens,
		usage.CompletionTokens,
		usage.CachedTokens,
		usage.ReasoningTokens,
	)
}

// CostBreakdown provides a detailed breakdown of costs for a single request.
type CostBreakdown struct {
	Model           string  `json:"model"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	CachedTokens    int     `json:"cached_tokens"`
	ReasoningTokens int     `json:"reasoning_tokens"`
	InputCost       float64 `json:"input_cost"`
	OutputCost      float64 `json:"output_cost"`
	CachedCost      float64 `json:"cached_cost"`
	ReasoningCost   float64 `json:"reasoning_cost"`
	TotalCost       float64 `json:"total_cost"`
}

// CalculateCostBreakdown returns a detailed breakdown of costs for a given model and usage.
func CalculateCostBreakdown(model string, usage *ai.Usage) CostBreakdown {
	if usage == nil {
		return CostBreakdown{Model: model}
	}

	mc := GetModelCost(model)

	inputCost := mc.CalculateInputCost(usage.PromptTokens)
	outputCost := mc.CalculateOutputCost(usage.CompletionTokens)
	cachedCost := mc.CalculateCachedCost(usage.CachedTokens)
	reasoningCost := mc.CalculateReasoningCost(usage.ReasoningTokens)

	return CostBreakdown{
		Model:           model,
		InputTokens:     usage.PromptTokens,
		OutputTokens:    usage.CompletionTokens,
		CachedTokens:    usage.CachedTokens,
		ReasoningTokens: usage.ReasoningTokens,
		InputCost:       inputCost,
		OutputCost:      outputCost,
		CachedCost:      cachedCost,
		ReasoningCost:   reasoningCost,
		TotalCost:       inputCost + outputCost + cachedCost + reasoningCost,
	}
}
