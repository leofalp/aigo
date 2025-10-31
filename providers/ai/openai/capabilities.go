package openai

import "strings"

// Capabilities represents the feature set supported by an OpenAI-compatible provider
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

type ToolCallMode string

const (
	ToolCallModeTools     ToolCallMode = "tools"     // New format (tools + tool_choice)
	ToolCallModeFunctions ToolCallMode = "functions" // Legacy format (functions + function_call)
	ToolCallModeBoth      ToolCallMode = "both"      // Supports both formats
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
