package openai

import "strings"

// Capabilities represents the complete feature set supported by a given
// OpenAI-compatible provider endpoint. It drives endpoint selection, wire-format
// choice, and optional features such as structured outputs and parallel tool calls.
// Capabilities are populated automatically by [detectCapabilities] but can be
// overridden via [OpenAIProvider.WithCapabilities] for non-standard hosts.
type Capabilities struct {
	// Endpoint support
	SupportsResponses bool // true only for real OpenAI API

	// Tool calling modes
	ToolCallMode ToolCallMode // "tools", "functions", or "both"

	// Feature flags
	SupportsMultimodal        bool // Vision support
	SupportsAudio             bool // Audio input/output
	SupportsStructuredOutputs bool // Strict JSON schema
	SupportsStreaming         bool // SSE streaming
	SupportsParallelTools     bool // Parallel tool calls
	SupportsContentFilters    bool // Azure/OpenAI safety filters
	SupportsReasoning         bool // o1/o3/gpt-5 reasoning
}

// ToolCallMode specifies which tool-calling wire format a provider understands.
// Providers that pre-date the structured tools API require the legacy functions
// format; modern providers support the newer tools format or both.
type ToolCallMode string

const (
	// ToolCallModeTools selects the modern tools/tool_choice request format.
	// This is the preferred format for OpenAI and most current providers.
	ToolCallModeTools ToolCallMode = "tools"

	// ToolCallModeFunctions selects the legacy functions/function_call request format.
	// Use this for older provider deployments that do not recognize the tools field.
	ToolCallModeFunctions ToolCallMode = "functions"

	// ToolCallModeBoth indicates the provider accepts either format.
	// The provider implementation may choose whichever format is most appropriate.
	ToolCallModeBoth ToolCallMode = "both"
)

// detectCapabilities attempts to detect provider capabilities based on baseURL
func detectCapabilities(baseURL string) Capabilities {
	baseURL = strings.ToLower(baseURL)

	// Real OpenAI API
	if strings.Contains(baseURL, "api.openai.com") {
		return Capabilities{
			SupportsResponses:         true,
			ToolCallMode:              ToolCallModeTools,
			SupportsMultimodal:        true,
			SupportsAudio:             true,
			SupportsStructuredOutputs: true,
			SupportsStreaming:         true,
			SupportsParallelTools:     true,
			SupportsContentFilters:    true,
			SupportsReasoning:         true,
		}
	}

	// Azure OpenAI
	if strings.Contains(baseURL, "azure.com") || strings.Contains(baseURL, "openai.azure") {
		return Capabilities{
			SupportsResponses:         false, // Azure uses chat completions
			ToolCallMode:              ToolCallModeTools,
			SupportsMultimodal:        true,
			SupportsAudio:             false,
			SupportsStructuredOutputs: true,
			SupportsStreaming:         true,
			SupportsParallelTools:     true, // Recent deployments
			SupportsContentFilters:    true,
			SupportsReasoning:         false,
		}
	}

	// Ollama
	if strings.Contains(baseURL, "localhost:11434") || strings.Contains(baseURL, "127.0.0.1:11434") {
		return Capabilities{
			SupportsResponses:         false,
			ToolCallMode:              ToolCallModeBoth, // Supports both formats
			SupportsMultimodal:        true,             // Vision models
			SupportsAudio:             false,
			SupportsStructuredOutputs: false,
			SupportsStreaming:         true,
			SupportsParallelTools:     false,
			SupportsContentFilters:    false,
			SupportsReasoning:         false,
		}
	}

	// OpenRouter
	if strings.Contains(baseURL, "openrouter.ai") {
		return Capabilities{
			SupportsResponses:         false,
			ToolCallMode:              ToolCallModeTools,
			SupportsMultimodal:        true,
			SupportsAudio:             false,
			SupportsStructuredOutputs: true, // Depends on model
			SupportsStreaming:         true,
			SupportsParallelTools:     true, // Depends on model
			SupportsContentFilters:    false,
			SupportsReasoning:         false,
		}
	}

	// Conservative defaults for unknown providers
	return Capabilities{
		SupportsResponses:         false,
		ToolCallMode:              ToolCallModeTools,
		SupportsMultimodal:        false,
		SupportsAudio:             false,
		SupportsStructuredOutputs: false,
		SupportsStreaming:         true,
		SupportsParallelTools:     false,
		SupportsContentFilters:    false,
		SupportsReasoning:         false,
	}
}
