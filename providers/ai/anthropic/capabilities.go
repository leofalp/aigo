package anthropic

import "strings"

// Known beta feature header values for Anthropic's anthropic-beta header.
// Users can pass these (or any future beta string) via Capabilities.BetaFeatures.
const (
	// BetaInterleavedThinking enables interleaved thinking blocks in responses.
	// Not needed on Claude 4.6 models when using adaptive thinking.
	BetaInterleavedThinking = "interleaved-thinking-2025-05-14"

	// BetaAdvancedToolUse enables programmatic tool calling and tool search.
	BetaAdvancedToolUse = "advanced-tool-use-2025-11-20"

	// BetaToolExamples enables the input_examples field on tool definitions.
	BetaToolExamples = "tool-examples-2025-10-29"

	// BetaCodeExecution enables the server-side code execution sandbox.
	BetaCodeExecution = "code-execution-2025-08-25"

	// BetaContextManagement enables the memory tool.
	BetaContextManagement = "context-management-2025-06-27"

	// BetaWebFetch enables dynamic web fetch filtering.
	BetaWebFetch = "web-fetch-2026-02-09"

	// BetaContextCompaction enables server-side context compaction for long conversations.
	BetaContextCompaction = "context-compaction-2026-02-14"
)

// Capabilities describes configurable features for the Anthropic provider.
// Unlike Gemini, there is no per-model auto-detection because Anthropic uses a
// single API endpoint. All fields default to false/empty; set them via
// [AnthropicProvider.WithCapabilities].
type Capabilities struct {
	ExtendedThinking bool     // Enable extended thinking (thinking blocks in responses)
	PDFInput         bool     // Model supports PDF document input
	PromptCaching    bool     // Endpoint supports prompt caching (cache_control on content blocks)
	Vision           bool     // Model supports image/multimodal input
	Effort           string   // Output effort level: "low", "medium", "high", "max" (GA on 4.6 models)
	Speed            string   // Speed mode: "fast" for research preview fast mode (Opus 4.6 only)
	BetaFeatures     []string // Optional list of anthropic-beta header values to send
}

// betaHeaderValue returns the comma-joined anthropic-beta header value.
// If ExtendedThinking is true, it automatically includes the interleaved-thinking
// beta header unless already present in BetaFeatures. Returns an empty string
// when no beta features are configured.
func (capabilities Capabilities) betaHeaderValue() string {
	features := make([]string, 0, len(capabilities.BetaFeatures)+1)
	features = append(features, capabilities.BetaFeatures...)

	// Auto-add interleaved thinking beta when ExtendedThinking is enabled
	if capabilities.ExtendedThinking {
		found := false
		for _, feature := range features {
			if feature == BetaInterleavedThinking {
				found = true
				break
			}
		}
		if !found {
			features = append(features, BetaInterleavedThinking)
		}
	}

	if len(features) == 0 {
		return ""
	}
	return strings.Join(features, ",")
}
