package gemini

// Capabilities describes what the Gemini API supports.
// These are informational only - no runtime validation is performed.
// If a feature is used but unsupported by the model, the API will return an error.
type Capabilities struct {
	SupportsMultimodal        bool // Vision/audio input
	SupportsStructuredOutputs bool // JSON Schema enforcement
	SupportsStreaming         bool // SSE streaming (future)
	SupportsThinking          bool // Reasoning/thinking mode
	SupportsBuiltinTools      bool // google_search, url_context, code_execution
	SupportsFunctionCalling   bool // User-defined functions
}

// detectCapabilities returns capabilities for the Gemini API.
// All features are marked as supported - actual support depends on the model.
func detectCapabilities() Capabilities {
	return Capabilities{
		SupportsMultimodal:        true,
		SupportsStructuredOutputs: true,
		SupportsStreaming:         false, // Not implemented yet
		SupportsThinking:          true,
		SupportsBuiltinTools:      true,
		SupportsFunctionCalling:   true,
	}
}
