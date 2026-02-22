package gemini

import "github.com/leofalp/aigo/providers/ai"

// Capabilities describes what the Gemini API supports for a specific model.
// These are informational only - no runtime validation is performed.
// If a feature is used but unsupported by the model, the API will return an error.
type Capabilities struct {
	SupportsMultimodal        bool // Vision/audio/video input
	SupportsImageOutput       bool // Image generation output
	SupportsAudioOutput       bool // Audio/TTS generation output
	SupportsVideoOutput       bool // Video generation output
	SupportsStructuredOutputs bool // JSON Schema enforcement
	SupportsStreaming         bool // SSE streaming (future)
	SupportsThinking          bool // Reasoning/thinking mode
	SupportsBuiltinTools      bool // google_search, url_context, code_execution
	SupportsFunctionCalling   bool // User-defined functions
}

// detectCapabilities returns capabilities for a specific Gemini model.
// When a model is found in the ModelRegistry, capabilities are derived from its
// declared input/output modalities. Unknown models get conservative defaults.
func detectCapabilities(model string) Capabilities {
	info, found := GetModelInfo(model)
	if !found {
		// Conservative defaults for unknown models: assume text-only chat model
		return Capabilities{
			SupportsMultimodal:        true,
			SupportsStructuredOutputs: true,
			SupportsStreaming:         false,
			SupportsThinking:          true,
			SupportsBuiltinTools:      true,
			SupportsFunctionCalling:   true,
		}
	}

	capabilities := Capabilities{
		// All Gemini chat models support structured outputs, thinking, and tools
		SupportsStructuredOutputs: true,
		SupportsStreaming:         false, // Not implemented yet
		SupportsThinking:          true,
		SupportsBuiltinTools:      true,
		SupportsFunctionCalling:   true,
	}

	// Derive multimodal input support from declared input modalities
	for _, modality := range info.InputModalities {
		if modality == ai.ModalityImage || modality == ai.ModalityAudio || modality == ai.ModalityVideo {
			capabilities.SupportsMultimodal = true
			break
		}
	}

	// Derive output capabilities from declared output modalities
	for _, modality := range info.OutputModalities {
		switch modality {
		case ai.ModalityImage:
			capabilities.SupportsImageOutput = true
		case ai.ModalityAudio:
			capabilities.SupportsAudioOutput = true
		case ai.ModalityVideo:
			capabilities.SupportsVideoOutput = true
		}
	}

	return capabilities
}
