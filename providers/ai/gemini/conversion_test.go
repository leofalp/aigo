package gemini

import (
	"testing"

	"github.com/leofalp/aigo/providers/ai"
)

// TestBuildToolConfig_AllModes exercises every branch in buildToolConfig,
// verifying that each ai.ToolChoice configuration maps to the correct Gemini
// FunctionCallingMode and AllowedFunctionNames.
func TestBuildToolConfig_AllModes(t *testing.T) {
	tests := []struct {
		name                     string
		input                    *ai.ToolChoice
		wantNil                  bool
		wantMode                 string
		wantAllowedFunctionNames []string
	}{
		{
			name:    "nil ToolChoice returns nil config",
			input:   nil,
			wantNil: true,
		},
		{
			name:     "ToolChoiceForced none maps to NONE mode",
			input:    &ai.ToolChoice{ToolChoiceForced: "none"},
			wantMode: "NONE",
		},
		{
			name:     "ToolChoiceForced None (mixed case) maps to NONE mode",
			input:    &ai.ToolChoice{ToolChoiceForced: "None"},
			wantMode: "NONE",
		},
		{
			name:     "ToolChoiceForced auto maps to AUTO mode",
			input:    &ai.ToolChoice{ToolChoiceForced: "auto"},
			wantMode: "AUTO",
		},
		{
			name:     "ToolChoiceForced AUTO (uppercase) maps to AUTO mode",
			input:    &ai.ToolChoice{ToolChoiceForced: "AUTO"},
			wantMode: "AUTO",
		},
		{
			name:     "ToolChoiceForced required maps to ANY mode",
			input:    &ai.ToolChoice{ToolChoiceForced: "required"},
			wantMode: "ANY",
		},
		{
			name:     "ToolChoiceForced Required (mixed case) maps to ANY mode",
			input:    &ai.ToolChoice{ToolChoiceForced: "Required"},
			wantMode: "ANY",
		},
		{
			name:                     "ToolChoiceForced specific tool name maps to ANY with AllowedFunctionNames",
			input:                    &ai.ToolChoice{ToolChoiceForced: "get_weather"},
			wantMode:                 "ANY",
			wantAllowedFunctionNames: []string{"get_weather"},
		},
		{
			name:     "AtLeastOneRequired maps to ANY mode without AllowedFunctionNames",
			input:    &ai.ToolChoice{AtLeastOneRequired: true},
			wantMode: "ANY",
		},
		{
			name: "RequiredTools with single tool maps to ANY with AllowedFunctionNames",
			input: &ai.ToolChoice{
				RequiredTools: []*ai.ToolDescription{
					{Name: "search_database"},
				},
			},
			wantMode:                 "ANY",
			wantAllowedFunctionNames: []string{"search_database"},
		},
		{
			name: "RequiredTools with multiple tools maps to ANY with all names listed",
			input: &ai.ToolChoice{
				RequiredTools: []*ai.ToolDescription{
					{Name: "search_database"},
					{Name: "send_email"},
					{Name: "create_ticket"},
				},
			},
			wantMode:                 "ANY",
			wantAllowedFunctionNames: []string{"search_database", "send_email", "create_ticket"},
		},
		{
			// ToolChoiceForced takes precedence over AtLeastOneRequired and RequiredTools
			// because the if/else chain checks ToolChoiceForced first.
			name: "ToolChoiceForced takes precedence over AtLeastOneRequired",
			input: &ai.ToolChoice{
				ToolChoiceForced:   "none",
				AtLeastOneRequired: true,
			},
			wantMode: "NONE",
		},
		{
			// ToolChoiceForced takes precedence over RequiredTools for the same reason.
			name: "ToolChoiceForced takes precedence over RequiredTools",
			input: &ai.ToolChoice{
				ToolChoiceForced: "auto",
				RequiredTools: []*ai.ToolDescription{
					{Name: "ignored_tool"},
				},
			},
			wantMode: "AUTO",
		},
		{
			// AtLeastOneRequired takes precedence over RequiredTools because the
			// else-if chain evaluates AtLeastOneRequired before RequiredTools.
			name: "AtLeastOneRequired takes precedence over RequiredTools",
			input: &ai.ToolChoice{
				AtLeastOneRequired: true,
				RequiredTools: []*ai.ToolDescription{
					{Name: "should_be_ignored"},
				},
			},
			wantMode: "ANY",
		},
		{
			// Empty ToolChoice (all zero values) still returns a non-nil config
			// with an empty FunctionCallingConfig (mode defaults to empty string).
			name:     "empty ToolChoice returns config with empty mode",
			input:    &ai.ToolChoice{},
			wantMode: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildToolConfig(tt.input)

			// Nil check
			if tt.wantNil {
				if result != nil {
					t.Fatalf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil toolConfig, got nil")
			}

			if result.FunctionCallingConfig == nil {
				t.Fatal("expected non-nil FunctionCallingConfig, got nil")
			}

			// Verify mode
			gotMode := result.FunctionCallingConfig.Mode
			if gotMode != tt.wantMode {
				t.Errorf("Mode: got %q, want %q", gotMode, tt.wantMode)
			}

			// Verify AllowedFunctionNames
			gotNames := result.FunctionCallingConfig.AllowedFunctionNames
			if tt.wantAllowedFunctionNames == nil {
				if len(gotNames) != 0 {
					t.Errorf("AllowedFunctionNames: expected empty, got %v", gotNames)
				}
			} else {
				if len(gotNames) != len(tt.wantAllowedFunctionNames) {
					t.Fatalf("AllowedFunctionNames length: got %d, want %d (got %v)",
						len(gotNames), len(tt.wantAllowedFunctionNames), gotNames)
				}
				for i, wantName := range tt.wantAllowedFunctionNames {
					if gotNames[i] != wantName {
						t.Errorf("AllowedFunctionNames[%d]: got %q, want %q", i, gotNames[i], wantName)
					}
				}
			}
		})
	}
}
