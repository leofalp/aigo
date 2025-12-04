package slogobs

import (
	"os"
	"testing"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Format
	}{
		{"compact lowercase", "compact", FormatCompact},
		{"compact uppercase", "COMPACT", FormatCompact},
		{"pretty lowercase", "pretty", FormatPretty},
		{"pretty uppercase", "PRETTY", FormatPretty},
		{"json lowercase", "json", FormatJSON},
		{"json uppercase", "JSON", FormatJSON},
		{"unknown defaults to compact", "unknown", FormatCompact},
		{"empty defaults to compact", "", FormatCompact},
		{"whitespace defaults to compact", "  ", FormatCompact},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseFormat(tt.input)
			if result != tt.expected {
				t.Errorf("ParseFormat(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetFormatFromEnv(t *testing.T) {
	tests := []struct {
		name             string
		aigoLogFormat    string
		logFormat        string
		expected         Format
		setAigoLogFormat bool
		setLogFormat     bool
	}{
		{
			name:             "AIGO_LOG_FORMAT takes precedence",
			aigoLogFormat:    "pretty",
			logFormat:        "json",
			expected:         FormatPretty,
			setAigoLogFormat: true,
			setLogFormat:     true,
		},
		{
			name:             "fallback to LOG_FORMAT",
			logFormat:        "json",
			expected:         FormatJSON,
			setAigoLogFormat: false,
			setLogFormat:     true,
		},
		{
			name:             "default to compact when neither set",
			expected:         FormatCompact,
			setAigoLogFormat: false,
			setLogFormat:     false,
		},
		{
			name:             "AIGO_LOG_FORMAT only",
			aigoLogFormat:    "pretty",
			expected:         FormatPretty,
			setAigoLogFormat: true,
			setLogFormat:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			_ = os.Unsetenv("AIGO_LOG_FORMAT")
			_ = os.Unsetenv("LOG_FORMAT")

			// Set environment variables
			if tt.setAigoLogFormat {
				_ = os.Setenv("AIGO_LOG_FORMAT", tt.aigoLogFormat)
			}
			if tt.setLogFormat {
				_ = os.Setenv("LOG_FORMAT", tt.logFormat)
			}

			result := GetFormatFromEnv()
			if result != tt.expected {
				t.Errorf("GetFormatFromEnv() = %v, want %v", result, tt.expected)
			}

			// Cleanup
			_ = os.Unsetenv("AIGO_LOG_FORMAT")
			_ = os.Unsetenv("LOG_FORMAT")
		})
	}
}

func TestFormatString(t *testing.T) {
	tests := []struct {
		format   Format
		expected string
	}{
		{FormatCompact, "compact"},
		{FormatPretty, "pretty"},
		{FormatJSON, "json"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.format.String()
			if result != tt.expected {
				t.Errorf("Format.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}
