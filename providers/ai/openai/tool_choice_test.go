package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
)

// TestToolChoice_ToolChoiceForced tests explicit forced tool choice
func TestToolChoice_ToolChoiceForced(t *testing.T) {
	tests := []struct {
		name          string
		forcedChoice  string
		useLegacy     bool
		expectedField string
		expectedValue string
		useResponses  bool
	}{
		{
			name:          "forced none - chat completions",
			forcedChoice:  "none",
			useLegacy:     false,
			expectedField: "tool_choice",
			expectedValue: "none",
			useResponses:  false,
		},
		{
			name:          "forced auto - chat completions",
			forcedChoice:  "auto",
			useLegacy:     false,
			expectedField: "tool_choice",
			expectedValue: "auto",
			useResponses:  false,
		},
		{
			name:          "forced required - chat completions",
			forcedChoice:  "required",
			useLegacy:     false,
			expectedField: "tool_choice",
			expectedValue: "required",
			useResponses:  false,
		},
		{
			name:          "forced none - legacy functions",
			forcedChoice:  "none",
			useLegacy:     true,
			expectedField: "function_call",
			expectedValue: "none",
			useResponses:  false,
		},
		{
			name:          "forced none - responses",
			forcedChoice:  "none",
			useLegacy:     false,
			expectedField: "tool_choice",
			expectedValue: "none",
			useResponses:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				mustDecodeJSON(t, r, &body)

				if body[tt.expectedField] != tt.expectedValue {
					t.Errorf("expected %s=%s, got %v", tt.expectedField, tt.expectedValue, body[tt.expectedField])
				}

				w.Header().Set("Content-Type", "application/json")
				if tt.useResponses {
					mustEncodeJSON(t, w, map[string]any{
						"id":         "r",
						"object":     "response",
						"created_at": 1,
						"model":      "m",
						"output":     []map[string]any{{"id": "o", "type": "message", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": "ok"}}}},
						"status":     "completed",
					})
				} else {
					mustEncodeJSON(t, w, map[string]any{
						"id":      "c",
						"object":  "chat.completion",
						"created": 1,
						"model":   "m",
						"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
					})
				}
			}))
			defer server.Close()

			schema := &jsonschema.Schema{Type: "object"}
			p := New().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)

			var toolCallMode ToolCallMode
			if tt.useLegacy {
				toolCallMode = ToolCallModeFunctions
			} else {
				toolCallMode = ToolCallModeTools
			}
			p = p.WithCapabilities(Capabilities{
				SupportsResponses: tt.useResponses,
				ToolCallMode:      toolCallMode,
			})

			_, err := p.SendMessage(context.Background(), ai.ChatRequest{
				Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
				Tools:    []ai.ToolDescription{{Name: "get_weather", Description: "d", Parameters: schema}},
				ToolChoice: &ai.ToolChoice{
					ToolChoiceForced: tt.forcedChoice,
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestToolChoice_AtLeastOneRequired tests the AtLeastOneRequired flag
func TestToolChoice_AtLeastOneRequired(t *testing.T) {
	tests := []struct {
		name          string
		useLegacy     bool
		expectedField string
		useResponses  bool
	}{
		{
			name:          "at least one - chat completions",
			useLegacy:     false,
			expectedField: "tool_choice",
			useResponses:  false,
		},
		{
			name:          "at least one - legacy functions",
			useLegacy:     true,
			expectedField: "function_call",
			useResponses:  false,
		},
		{
			name:          "at least one - responses",
			useLegacy:     false,
			expectedField: "tool_choice",
			useResponses:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				mustDecodeJSON(t, r, &body)

				if body[tt.expectedField] != "required" {
					t.Errorf("expected %s=required, got %v", tt.expectedField, body[tt.expectedField])
				}

				w.Header().Set("Content-Type", "application/json")
				if tt.useResponses {
					mustEncodeJSON(t, w, map[string]any{
						"id":         "r",
						"object":     "response",
						"created_at": 1,
						"model":      "m",
						"output":     []map[string]any{{"id": "o", "type": "message", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": "ok"}}}},
						"status":     "completed",
					})
				} else {
					mustEncodeJSON(t, w, map[string]any{
						"id":      "c",
						"object":  "chat.completion",
						"created": 1,
						"model":   "m",
						"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
					})
				}
			}))
			defer server.Close()

			schema := &jsonschema.Schema{Type: "object"}
			p := New().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)

			var toolCallMode ToolCallMode
			if tt.useLegacy {
				toolCallMode = ToolCallModeFunctions
			} else {
				toolCallMode = ToolCallModeTools
			}
			p = p.WithCapabilities(Capabilities{
				SupportsResponses: tt.useResponses,
				ToolCallMode:      toolCallMode,
			})

			_, err := p.SendMessage(context.Background(), ai.ChatRequest{
				Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
				Tools: []ai.ToolDescription{
					{Name: "get_weather", Description: "d", Parameters: schema},
					{Name: "get_time", Description: "d", Parameters: schema},
				},
				ToolChoice: &ai.ToolChoice{
					AtLeastOneRequired: true,
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestToolChoice_SingleRequiredTool tests forcing a single specific tool
func TestToolChoice_SingleRequiredTool(t *testing.T) {
	tests := []struct {
		name         string
		useLegacy    bool
		useResponses bool
	}{
		{
			name:         "single required - chat completions",
			useLegacy:    false,
			useResponses: false,
		},
		{
			name:         "single required - legacy functions",
			useLegacy:    true,
			useResponses: false,
		},
		{
			name:         "single required - responses",
			useLegacy:    false,
			useResponses: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				mustDecodeJSON(t, r, &body)

				var fieldName string
				if tt.useLegacy {
					fieldName = "function_call"
				} else {
					fieldName = "tool_choice"
				}

				toolChoice, ok := body[fieldName].(map[string]any)
				if !ok {
					t.Errorf("expected %s to be an object, got %T: %v", fieldName, body[fieldName], body[fieldName])
					return
				}

				if toolChoice["type"] != "function" {
					t.Errorf("expected type=function, got %v", toolChoice["type"])
				}

				if toolChoice["name"] != "get_weather" {
					t.Errorf("expected name=get_weather, got %v", toolChoice["name"])
				}

				w.Header().Set("Content-Type", "application/json")
				if tt.useResponses {
					mustEncodeJSON(t, w, map[string]any{
						"id":         "r",
						"object":     "response",
						"created_at": 1,
						"model":      "m",
						"output":     []map[string]any{{"id": "o", "type": "message", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": "ok"}}}},
						"status":     "completed",
					})
				} else {
					mustEncodeJSON(t, w, map[string]any{
						"id":      "c",
						"object":  "chat.completion",
						"created": 1,
						"model":   "m",
						"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
					})
				}
			}))
			defer server.Close()

			schema := &jsonschema.Schema{Type: "object"}
			p := New().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)

			var toolCallMode ToolCallMode
			if tt.useLegacy {
				toolCallMode = ToolCallModeFunctions
			} else {
				toolCallMode = ToolCallModeTools
			}
			p = p.WithCapabilities(Capabilities{
				SupportsResponses: tt.useResponses,
				ToolCallMode:      toolCallMode,
			})

			_, err := p.SendMessage(context.Background(), ai.ChatRequest{
				Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
				Tools: []ai.ToolDescription{
					{Name: "get_weather", Description: "d", Parameters: schema},
					{Name: "get_time", Description: "d", Parameters: schema},
				},
				ToolChoice: &ai.ToolChoice{
					RequiredTools: []*ai.ToolDescription{
						{Name: "get_weather"},
					},
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestToolChoice_MultipleRequiredTools tests forcing multiple specific tools
func TestToolChoice_MultipleRequiredTools(t *testing.T) {
	tests := []struct {
		name          string
		useLegacy     bool
		useResponses  bool
		expectArray   bool
		expectAllowed bool
	}{
		{
			name:          "multiple required - chat completions new format",
			useLegacy:     false,
			useResponses:  false,
			expectArray:   false,
			expectAllowed: true, // Should use allowed_tools
		},
		{
			name:          "multiple required - legacy functions (fallback to required)",
			useLegacy:     true,
			useResponses:  false,
			expectArray:   false,
			expectAllowed: false, // Legacy doesn't support, should fallback to "required"
		},
		{
			name:          "multiple required - responses (array format)",
			useLegacy:     false,
			useResponses:  true,
			expectArray:   true, // Responses API supports arrays
			expectAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				mustDecodeJSON(t, r, &body)

				var fieldName string
				if tt.useLegacy {
					fieldName = "function_call"
				} else {
					fieldName = "tool_choice"
				}

				if tt.expectArray {
					// Responses API with array
					toolChoiceArray, ok := body[fieldName].([]any)
					if !ok {
						t.Errorf("expected %s to be an array, got %T: %v", fieldName, body[fieldName], body[fieldName])
						return
					}

					if len(toolChoiceArray) != 2 {
						t.Errorf("expected 2 tools in array, got %d", len(toolChoiceArray))
					}

					// Check first tool
					tool1, ok := toolChoiceArray[0].(map[string]any)
					if !ok || tool1["type"] != "function" || tool1["name"] != "get_weather" {
						t.Errorf("unexpected first tool in array: %v", toolChoiceArray[0])
					}

					// Check second tool
					tool2, ok := toolChoiceArray[1].(map[string]any)
					if !ok || tool2["type"] != "function" || tool2["name"] != "get_time" {
						t.Errorf("unexpected second tool in array: %v", toolChoiceArray[1])
					}
				} else if tt.expectAllowed {
					// New format with allowed_tools
					toolChoice, ok := body[fieldName].(map[string]any)
					if !ok {
						t.Errorf("expected %s to be an object, got %T: %v", fieldName, body[fieldName], body[fieldName])
						return
					}

					if toolChoice["type"] != "allowed_tools" {
						t.Errorf("expected type=allowed_tools, got %v", toolChoice["type"])
					}

					if toolChoice["mode"] != "required" {
						t.Errorf("expected mode=required, got %v", toolChoice["mode"])
					}

					tools, ok := toolChoice["tools"].([]any)
					if !ok {
						t.Errorf("expected tools to be an array, got %T", toolChoice["tools"])
						return
					}

					if len(tools) != 2 {
						t.Errorf("expected 2 tools, got %d", len(tools))
					}
				} else {
					// Legacy fallback to "required"
					if body[fieldName] != "required" {
						t.Errorf("expected %s=required (fallback), got %v", fieldName, body[fieldName])
					}
				}

				w.Header().Set("Content-Type", "application/json")
				if tt.useResponses {
					mustEncodeJSON(t, w, map[string]any{
						"id":         "r",
						"object":     "response",
						"created_at": 1,
						"model":      "m",
						"output":     []map[string]any{{"id": "o", "type": "message", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": "ok"}}}},
						"status":     "completed",
					})
				} else {
					mustEncodeJSON(t, w, map[string]any{
						"id":      "c",
						"object":  "chat.completion",
						"created": 1,
						"model":   "m",
						"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
					})
				}
			}))
			defer server.Close()

			schema := &jsonschema.Schema{Type: "object"}
			p := New().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)

			var toolCallMode ToolCallMode
			if tt.useLegacy {
				toolCallMode = ToolCallModeFunctions
			} else {
				toolCallMode = ToolCallModeTools
			}
			p = p.WithCapabilities(Capabilities{
				SupportsResponses: tt.useResponses,
				ToolCallMode:      toolCallMode,
			})

			_, err := p.SendMessage(context.Background(), ai.ChatRequest{
				Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
				Tools: []ai.ToolDescription{
					{Name: "get_weather", Description: "d", Parameters: schema},
					{Name: "get_time", Description: "d", Parameters: schema},
					{Name: "get_location", Description: "d", Parameters: schema},
				},
				ToolChoice: &ai.ToolChoice{
					RequiredTools: []*ai.ToolDescription{
						{Name: "get_weather"},
						{Name: "get_time"},
					},
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestToolChoice_PriorityOrder tests that priority is respected
func TestToolChoice_PriorityOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		mustDecodeJSON(t, r, &body)

		// ToolChoiceForced should take priority over everything
		if body["tool_choice"] != "none" {
			t.Errorf("expected tool_choice=none (forced should override), got %v", body["tool_choice"])
		}

		w.Header().Set("Content-Type", "application/json")
		mustEncodeJSON(t, w, map[string]any{
			"id":      "c",
			"object":  "chat.completion",
			"created": 1,
			"model":   "m",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
		})
	}))
	defer server.Close()

	schema := &jsonschema.Schema{Type: "object"}
	p := New().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: false, ToolCallMode: ToolCallModeTools})

	_, err := p.SendMessage(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
		Tools: []ai.ToolDescription{
			{Name: "get_weather", Description: "d", Parameters: schema},
		},
		ToolChoice: &ai.ToolChoice{
			ToolChoiceForced:   "none", // This should take priority
			AtLeastOneRequired: true,   // This should be ignored
			RequiredTools: []*ai.ToolDescription{
				{Name: "get_weather"}, // This should be ignored
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestToolChoice_DefaultAuto tests default "auto" behavior
func TestToolChoice_DefaultAuto(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		mustDecodeJSON(t, r, &body)

		// Should default to "auto" when no ToolChoice specified
		if body["tool_choice"] != "auto" {
			t.Errorf("expected tool_choice=auto (default), got %v", body["tool_choice"])
		}

		w.Header().Set("Content-Type", "application/json")
		mustEncodeJSON(t, w, map[string]any{
			"id":      "c",
			"object":  "chat.completion",
			"created": 1,
			"model":   "m",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
		})
	}))
	defer server.Close()

	schema := &jsonschema.Schema{Type: "object"}
	p := New().WithAPIKey("k").WithBaseURL(server.URL).(*OpenAIProvider)
	p = p.WithCapabilities(Capabilities{SupportsResponses: false, ToolCallMode: ToolCallModeTools})

	_, err := p.SendMessage(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
		Tools: []ai.ToolDescription{
			{Name: "get_weather", Description: "d", Parameters: schema},
		},
		// No ToolChoice specified
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
