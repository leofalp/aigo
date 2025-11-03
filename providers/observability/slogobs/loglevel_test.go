package slogobs

import (
	"log/slog"
	"os"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected slog.Level
	}{
		{"Debug uppercase", "DEBUG", slog.LevelDebug},
		{"Debug lowercase", "debug", slog.LevelDebug},
		{"Debug mixed case", "DeBuG", slog.LevelDebug},
		{"Info uppercase", "INFO", slog.LevelInfo},
		{"Info lowercase", "info", slog.LevelInfo},
		{"Warn uppercase", "WARN", slog.LevelWarn},
		{"Warn lowercase", "warn", slog.LevelWarn},
		{"Warning uppercase", "WARNING", slog.LevelWarn},
		{"Warning lowercase", "warning", slog.LevelWarn},
		{"Error uppercase", "ERROR", slog.LevelError},
		{"Error lowercase", "error", slog.LevelError},
		{"Unknown value", "UNKNOWN", slog.LevelInfo},
		{"Empty string", "", slog.LevelInfo},
		{"Whitespace", "  ", slog.LevelInfo},
		{"With whitespace", "  DEBUG  ", slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetLogLevelFromEnv(t *testing.T) {
	tests := []struct {
		name             string
		aigoLogLevel     string
		logLevel         string
		expectedLevel    slog.Level
		shouldSetAigo    bool
		shouldSetGeneric bool
	}{
		{
			name:             "AIGO_LOG_LEVEL takes precedence",
			aigoLogLevel:     "DEBUG",
			logLevel:         "ERROR",
			expectedLevel:    slog.LevelDebug,
			shouldSetAigo:    true,
			shouldSetGeneric: true,
		},
		{
			name:             "Fallback to LOG_LEVEL",
			logLevel:         "WARN",
			expectedLevel:    slog.LevelWarn,
			shouldSetGeneric: true,
		},
		{
			name:          "Default to INFO when neither set",
			expectedLevel: slog.LevelInfo,
		},
		{
			name:          "AIGO_LOG_LEVEL only",
			aigoLogLevel:  "ERROR",
			expectedLevel: slog.LevelError,
			shouldSetAigo: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and clear environment variables
			oldAigo := os.Getenv("AIGO_LOG_LEVEL")
			oldGeneric := os.Getenv("LOG_LEVEL")
			os.Unsetenv("AIGO_LOG_LEVEL")
			os.Unsetenv("LOG_LEVEL")

			// Set test environment variables
			if tt.shouldSetAigo {
				os.Setenv("AIGO_LOG_LEVEL", tt.aigoLogLevel)
			}
			if tt.shouldSetGeneric {
				os.Setenv("LOG_LEVEL", tt.logLevel)
			}

			// Test
			result := GetLogLevelFromEnv()
			if result != tt.expectedLevel {
				t.Errorf("GetLogLevelFromEnv() = %v, want %v", result, tt.expectedLevel)
			}

			// Restore environment variables
			os.Unsetenv("AIGO_LOG_LEVEL")
			os.Unsetenv("LOG_LEVEL")
			if oldAigo != "" {
				os.Setenv("AIGO_LOG_LEVEL", oldAigo)
			}
			if oldGeneric != "" {
				os.Setenv("LOG_LEVEL", oldGeneric)
			}
		})
	}
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		name     string
		level    slog.Level
		expected string
	}{
		{"Debug level", slog.LevelDebug, "DEBUG"},
		{"Info level", slog.LevelInfo, "INFO"},
		{"Warn level", slog.LevelWarn, "WARN"},
		{"Error level", slog.LevelError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LogLevelString(tt.level)
			if result != tt.expected {
				t.Errorf("LogLevelString(%v) = %q, want %q", tt.level, result, tt.expected)
			}
		})
	}
}

func TestLogLevelRoundTrip(t *testing.T) {
	levels := []struct {
		str   string
		level slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
	}

	for _, tt := range levels {
		t.Run(tt.str, func(t *testing.T) {
			// Parse string to level
			parsed := ParseLogLevel(tt.str)
			if parsed != tt.level {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.str, parsed, tt.level)
			}

			// Convert level back to string
			str := LogLevelString(parsed)
			if str != tt.str {
				t.Errorf("LogLevelString(%v) = %q, want %q", parsed, str, tt.str)
			}
		})
	}
}
