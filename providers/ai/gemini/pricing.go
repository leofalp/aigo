package gemini

import (
	"strings"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/providers/ai"
)

// Model name constants for Gemini models.
// Use these constants instead of raw strings for type safety and autocompletion.
const (
	// Gemini 3.0 Preview models (experimental)
	Model30ProPreview      = "gemini-3-pro-preview"
	Model30ProImagePreview = "gemini-3-pro-image-preview"
	Model30FlashPreview    = "gemini-3-flash-preview"

	// Gemini 2.5 models
	Model25Pro              = "gemini-2.5-pro"
	Model25ProLatest        = "gemini-2.5-pro-latest"
	Model25ProPreview       = "gemini-2.5-pro-preview-05-06"
	Model25Flash            = "gemini-2.5-flash"
	Model25FlashLatest      = "gemini-2.5-flash-latest"
	Model25FlashPreview     = "gemini-2.5-flash-preview-04-17"
	Model25FlashImage       = "gemini-2.5-flash-image"
	Model25FlashNativeAudio = "gemini-2.5-flash-native-audio-preview-12-2025"
	Model25FlashLite        = "gemini-2.5-flash-lite"
	Model25FlashLiteLatest  = "gemini-2.5-flash-lite-latest"
	Model25FlashLitePreview = "gemini-2.5-flash-lite-preview-06-17"
	Model25ProTTS           = "gemini-2.5-pro-preview-tts"
	Model25FlashTTS         = "gemini-2.5-flash-preview-tts"

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

	// Specialized models (registry-only, no chat API integration)
	ModelRoboticsER15 = "gemini-robotics-er-1.5-preview"
	ModelImagen4      = "imagen-4.0-generate-001"
	ModelImagen4Ultra = "imagen-4.0-ultra-generate-001"
	ModelImagen4Fast  = "imagen-4.0-fast-generate-001"
	ModelVeo31        = "veo-3.1-generate-preview"
	ModelVeo31Fast    = "veo-3.1-fast-generate-preview"
	ModelVeo20        = "veo-2.0-generate-001"
)

// ModelRegistry contains metadata, capabilities, and pricing for all supported Gemini models.
// Each entry maps a canonical model ID to its ModelInfo. Alias models (e.g., "gemini-2.5-pro-latest")
// are resolved via normalizeModelName and share the same ModelInfo as their canonical counterpart.
//
// Models with nil Pricing are registry-only: they are tracked for capability metadata
// but have no published pricing (e.g., preview/experimental models, Imagen, Veo, TTS).
//
// Source: https://ai.google.dev/gemini-api/docs/pricing (2025)
var ModelRegistry = map[string]ai.ModelInfo{
	// --- Gemini 3.0 Preview models ---

	Model30ProPreview: {
		ID:               Model30ProPreview,
		Name:             "Gemini 3 Pro Preview",
		Description:      "Most capable Gemini model (experimental preview)",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityImage, ai.ModalityAudio, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText},
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       2.00,
			OutputCostPerMillion:      12.00,
			CachedInputCostPerMillion: 1.00,
			ReasoningCostPerMillion:   12.00,
			ContextTiers: []cost.ContextTier{
				{InputTokenThreshold: 200_000, InputCostPerMillion: 4.00, OutputTokenThreshold: 200_000, OutputCostPerMillion: 18.00},
			},
		},
	},
	Model30ProImagePreview: {
		ID:               Model30ProImagePreview,
		Name:             "Gemini 3 Pro Image Preview",
		Description:      "Gemini 3 Pro with image generation output",
		InputModalities:  []ai.Modality{ai.ModalityText},
		OutputModalities: []ai.Modality{ai.ModalityImage},
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       2.00,
			OutputCostPerMillion:      12.00,
			CachedInputCostPerMillion: 1.00,
			ReasoningCostPerMillion:   12.00,
			ImageOutputCostPerUnit:    0.134,
		},
	},
	Model30FlashPreview: {
		ID:               Model30FlashPreview,
		Name:             "Gemini 3 Flash Preview",
		Description:      "Fast and efficient Gemini 3 model (experimental preview)",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityImage, ai.ModalityAudio, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText},
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       0.50,
			OutputCostPerMillion:      3.00,
			CachedInputCostPerMillion: 0.25,
			ReasoningCostPerMillion:   3.00,
		},
	},

	// --- Gemini 2.5 models ---

	Model25Pro: {
		ID:               Model25Pro,
		Name:             "Gemini 2.5 Pro",
		Description:      "Most capable Gemini 2.5 model with thinking",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityImage, ai.ModalityAudio, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText},
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       1.25,
			OutputCostPerMillion:      10.00,
			CachedInputCostPerMillion: 0.625,
			ReasoningCostPerMillion:   10.00,
			ContextTiers: []cost.ContextTier{
				{InputTokenThreshold: 200_000, InputCostPerMillion: 2.50, OutputTokenThreshold: 200_000, OutputCostPerMillion: 15.00},
			},
		},
	},
	Model25Flash: {
		ID:               Model25Flash,
		Name:             "Gemini 2.5 Flash",
		Description:      "Fast and efficient Gemini 2.5 model with thinking",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityImage, ai.ModalityAudio, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText},
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       0.30,
			OutputCostPerMillion:      2.50,
			CachedInputCostPerMillion: 0.15,
			ReasoningCostPerMillion:   2.50,
		},
	},
	Model25FlashImage: {
		ID:               Model25FlashImage,
		Name:             "Gemini 2.5 Flash Image",
		Description:      "Gemini 2.5 Flash with image generation output",
		InputModalities:  []ai.Modality{ai.ModalityText},
		OutputModalities: []ai.Modality{ai.ModalityImage},
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       0.30,
			OutputCostPerMillion:      2.50,
			CachedInputCostPerMillion: 0.15,
			ReasoningCostPerMillion:   2.50,
			ImageOutputCostPerUnit:    0.039,
		},
	},
	Model25FlashLite: {
		ID:               Model25FlashLite,
		Name:             "Gemini 2.5 Flash Lite",
		Description:      "Most cost-effective Gemini 2.5 model",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityImage, ai.ModalityAudio, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText},
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       0.10,
			OutputCostPerMillion:      0.40,
			CachedInputCostPerMillion: 0.05,
			ReasoningCostPerMillion:   0.40,
		},
	},

	// --- Gemini 2.5 specialized models (registry-only, no chat API integration yet) ---

	Model25FlashNativeAudio: {
		ID:               Model25FlashNativeAudio,
		Name:             "Gemini 2.5 Flash Native Audio",
		Description:      "Native audio understanding and generation (preview)",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityAudio, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText, ai.ModalityAudio},
		Pricing:          nil, // Preview model, pricing not published
	},
	Model25ProTTS: {
		ID:               Model25ProTTS,
		Name:             "Gemini 2.5 Pro TTS",
		Description:      "Text-to-speech using Gemini 2.5 Pro (preview)",
		InputModalities:  []ai.Modality{ai.ModalityText},
		OutputModalities: []ai.Modality{ai.ModalityAudio},
		Pricing:          nil,
	},
	Model25FlashTTS: {
		ID:               Model25FlashTTS,
		Name:             "Gemini 2.5 Flash TTS",
		Description:      "Text-to-speech using Gemini 2.5 Flash (preview)",
		InputModalities:  []ai.Modality{ai.ModalityText},
		OutputModalities: []ai.Modality{ai.ModalityAudio},
		Pricing:          nil,
	},

	// --- Gemini 2.0 models ---

	Model20Flash: {
		ID:               Model20Flash,
		Name:             "Gemini 2.0 Flash",
		Description:      "Fast and versatile Gemini 2.0 model",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityImage, ai.ModalityAudio, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText},
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       0.10,
			OutputCostPerMillion:      0.40,
			CachedInputCostPerMillion: 0.05,
			ReasoningCostPerMillion:   0.40,
		},
	},
	Model20FlashLite: {
		ID:               Model20FlashLite,
		Name:             "Gemini 2.0 Flash Lite",
		Description:      "Most cost-effective model for simple tasks",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityImage, ai.ModalityAudio, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText},
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       0.075,
			OutputCostPerMillion:      0.30,
			CachedInputCostPerMillion: 0.0375,
			ReasoningCostPerMillion:   0.30,
		},
	},

	// --- Gemini 1.5 models (legacy) ---

	Model15Pro: {
		ID:               Model15Pro,
		Name:             "Gemini 1.5 Pro",
		Description:      "Previous-generation Pro model (legacy)",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityImage, ai.ModalityAudio, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText},
		Deprecated:       true,
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       1.25,
			OutputCostPerMillion:      5.00,
			CachedInputCostPerMillion: 0.3125,
			ReasoningCostPerMillion:   5.00,
			ContextTiers: []cost.ContextTier{
				{InputTokenThreshold: 128_000, InputCostPerMillion: 2.50, OutputTokenThreshold: 128_000, OutputCostPerMillion: 10.00},
			},
		},
	},
	Model15Flash: {
		ID:               Model15Flash,
		Name:             "Gemini 1.5 Flash",
		Description:      "Previous-generation Flash model (legacy)",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityImage, ai.ModalityAudio, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText},
		Deprecated:       true,
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       0.075,
			OutputCostPerMillion:      0.30,
			CachedInputCostPerMillion: 0.01875,
			ReasoningCostPerMillion:   0.30,
			ContextTiers: []cost.ContextTier{
				{InputTokenThreshold: 128_000, InputCostPerMillion: 0.15, OutputTokenThreshold: 128_000, OutputCostPerMillion: 0.60},
			},
		},
	},
	Model15Flash8B: {
		ID:               Model15Flash8B,
		Name:             "Gemini 1.5 Flash 8B",
		Description:      "Smallest previous-generation model (legacy)",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityImage, ai.ModalityAudio, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText},
		Deprecated:       true,
		Pricing: &cost.ModelCost{
			InputCostPerMillion:       0.0375,
			OutputCostPerMillion:      0.15,
			CachedInputCostPerMillion: 0.009375,
			ReasoningCostPerMillion:   0.15,
			ContextTiers: []cost.ContextTier{
				{InputTokenThreshold: 128_000, InputCostPerMillion: 0.075, OutputTokenThreshold: 128_000, OutputCostPerMillion: 0.30},
			},
		},
	},

	// --- Specialized non-generative models (registry-only) ---

	ModelRoboticsER15: {
		ID:               ModelRoboticsER15,
		Name:             "Gemini Robotics ER 1.5",
		Description:      "Embodied reasoning for robotics applications (preview)",
		InputModalities:  []ai.Modality{ai.ModalityText, ai.ModalityImage, ai.ModalityVideo},
		OutputModalities: []ai.Modality{ai.ModalityText},
		Pricing:          nil,
	},
	ModelImagen4: {
		ID:               ModelImagen4,
		Name:             "Imagen 4",
		Description:      "High-quality image generation",
		InputModalities:  []ai.Modality{ai.ModalityText},
		OutputModalities: []ai.Modality{ai.ModalityImage},
		Pricing:          nil,
	},
	ModelImagen4Ultra: {
		ID:               ModelImagen4Ultra,
		Name:             "Imagen 4 Ultra",
		Description:      "Highest-quality image generation",
		InputModalities:  []ai.Modality{ai.ModalityText},
		OutputModalities: []ai.Modality{ai.ModalityImage},
		Pricing:          nil,
	},
	ModelImagen4Fast: {
		ID:               ModelImagen4Fast,
		Name:             "Imagen 4 Fast",
		Description:      "Fast image generation with lower latency",
		InputModalities:  []ai.Modality{ai.ModalityText},
		OutputModalities: []ai.Modality{ai.ModalityImage},
		Pricing:          nil,
	},
	ModelVeo31: {
		ID:               ModelVeo31,
		Name:             "Veo 3.1",
		Description:      "High-quality video generation",
		InputModalities:  []ai.Modality{ai.ModalityText},
		OutputModalities: []ai.Modality{ai.ModalityVideo},
		Pricing:          nil,
	},
	ModelVeo31Fast: {
		ID:               ModelVeo31Fast,
		Name:             "Veo 3.1 Fast",
		Description:      "Fast video generation with lower latency",
		InputModalities:  []ai.Modality{ai.ModalityText},
		OutputModalities: []ai.Modality{ai.ModalityVideo},
		Pricing:          nil,
	},
	ModelVeo20: {
		ID:               ModelVeo20,
		Name:             "Veo 2.0",
		Description:      "Previous-generation video generation",
		InputModalities:  []ai.Modality{ai.ModalityText},
		OutputModalities: []ai.Modality{ai.ModalityVideo},
		Pricing:          nil,
	},
}

// ModelPricing provides backward-compatible access to pricing for models that have published costs.
// It is derived from ModelRegistry at init time. Entries with nil Pricing are excluded.
//
// Deprecated: Use ModelRegistry and GetModelInfo instead.
var ModelPricing map[string]cost.ModelCost

func init() {
	ModelPricing = make(map[string]cost.ModelCost, len(ModelRegistry))
	for modelID, info := range ModelRegistry {
		if info.Pricing != nil {
			ModelPricing[modelID] = *info.Pricing
		}
	}

	// Add alias entries so that "-latest", "-preview", and "-exp" variants
	// resolve to the same pricing as the canonical model.
	aliases := map[string]string{
		Model25ProLatest:        Model25Pro,
		Model25ProPreview:       Model25Pro,
		Model25FlashLatest:      Model25Flash,
		Model25FlashPreview:     Model25Flash,
		Model25FlashLiteLatest:  Model25FlashLite,
		Model25FlashLitePreview: Model25FlashLite,
		Model20FlashLatest:      Model20Flash,
		Model20FlashExp:         Model20Flash,
		Model15ProLatest:        Model15Pro,
		Model15Flash8BExp:       Model15Flash8B,
	}
	for alias, canonical := range aliases {
		if info, ok := ModelRegistry[canonical]; ok && info.Pricing != nil {
			ModelPricing[alias] = *info.Pricing
		}
	}
}

// GetModelInfo returns the full model metadata for a given model name.
// It handles model name variations (e.g., "gemini-2.0-flash-001" resolves to "gemini-2.0-flash").
// Returns the ModelInfo and true if found, or a zero-value ModelInfo and false if not found.
func GetModelInfo(model string) (ai.ModelInfo, bool) {
	// Direct lookup first
	if info, ok := ModelRegistry[model]; ok {
		return info, true
	}

	// Try to find a matching model by stripping version suffixes
	normalizedModel := normalizeModelName(model)
	if info, ok := ModelRegistry[normalizedModel]; ok {
		return info, true
	}

	return ai.ModelInfo{}, false
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
	ImageCount      int     `json:"image_count,omitempty"`
	ImageCost       float64 `json:"image_cost,omitempty"`
	VideoCount      int     `json:"video_count,omitempty"`
	VideoCost       float64 `json:"video_cost,omitempty"`
	AudioCount      int     `json:"audio_count,omitempty"`
	AudioCost       float64 `json:"audio_cost,omitempty"`
	TotalCost       float64 `json:"total_cost"`
}

// CalculateCostBreakdown returns a detailed breakdown of costs for a given model and usage.
// For media costs, use CalculateCostBreakdownWithMedia.
func CalculateCostBreakdown(model string, usage *ai.Usage) CostBreakdown {
	return CalculateCostBreakdownWithMedia(model, usage, 0, 0, 0)
}

// CalculateCostBreakdownWithMedia returns a detailed breakdown of costs including media output costs.
func CalculateCostBreakdownWithMedia(model string, usage *ai.Usage, images, videos, audios int) CostBreakdown {
	if usage == nil {
		return CostBreakdown{Model: model}
	}

	mc := GetModelCost(model)

	inputCost := mc.CalculateInputCost(usage.PromptTokens)
	outputCost := mc.CalculateOutputCost(usage.CompletionTokens)
	cachedCost := mc.CalculateCachedCost(usage.CachedTokens)
	reasoningCost := mc.CalculateReasoningCost(usage.ReasoningTokens)
	imageCost := mc.CalculateImageOutputCost(images)
	videoCost := mc.CalculateVideoOutputCost(videos)
	audioCost := mc.CalculateAudioOutputCost(audios)

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
		ImageCount:      images,
		ImageCost:       imageCost,
		VideoCount:      videos,
		VideoCost:       videoCost,
		AudioCount:      audios,
		AudioCost:       audioCost,
		TotalCost:       inputCost + outputCost + cachedCost + reasoningCost + imageCost + videoCost + audioCost,
	}
}
