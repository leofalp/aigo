package slogobs

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestHandler_Compact(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Format: FormatCompact,
		Level:  slog.LevelDebug,
		Output: &buf,
		Colors: false,
	})

	logger := slog.New(handler)
	logger.Info("Test message", "key1", "value1", "key2", 42)

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected INFO level in output, got: %s", output)
	}
	if !strings.Contains(output, "Test message") {
		t.Errorf("Expected message in output, got: %s", output)
	}
	if !strings.Contains(output, "→") {
		t.Errorf("Expected → separator in output, got: %s", output)
	}
	if !strings.Contains(output, `"key1":"value1"`) {
		t.Errorf("Expected JSON attributes in output, got: %s", output)
	}
	if !strings.Contains(output, `"key2":42`) {
		t.Errorf("Expected JSON attributes in output, got: %s", output)
	}
}

func TestHandler_Pretty(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Format: FormatPretty,
		Level:  slog.LevelDebug,
		Output: &buf,
		Colors: false,
	})

	logger := slog.New(handler)
	logger.Info("Test message", "key1", "value1", "key2", 42)

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected INFO level in output, got: %s", output)
	}
	if !strings.Contains(output, "Test message") {
		t.Errorf("Expected message in output, got: %s", output)
	}
	if !strings.Contains(output, "🟢") {
		t.Errorf("Expected 🟢 emoji in output, got: %s", output)
	}
	// Check for tree-style symbols
	if !strings.Contains(output, "├─") && !strings.Contains(output, "└─") {
		t.Errorf("Expected tree symbols (├─ or └─) in output, got: %s", output)
	}
	if !strings.Contains(output, "key1: value1") {
		t.Errorf("Expected key-value pair in output, got: %s", output)
	}
	if !strings.Contains(output, "key2: 42") {
		t.Errorf("Expected key-value pair in output, got: %s", output)
	}
}

func TestHandler_JSON(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Format: FormatJSON,
		Level:  slog.LevelDebug,
		Output: &buf,
		Colors: false,
	})

	logger := slog.New(handler)
	logger.Info("Test message", "key1", "value1", "key2", 42)

	output := buf.String()
	if !strings.Contains(output, `"level":"INFO"`) {
		t.Errorf("Expected level in JSON output, got: %s", output)
	}
	if !strings.Contains(output, `"msg":"Test message"`) {
		t.Errorf("Expected msg in JSON output, got: %s", output)
	}
	if !strings.Contains(output, `"key1":"value1"`) {
		t.Errorf("Expected key1 in JSON output, got: %s", output)
	}
	if !strings.Contains(output, `"key2":42`) {
		t.Errorf("Expected key2 in JSON output, got: %s", output)
	}
	if !strings.Contains(output, `"time":"`) {
		t.Errorf("Expected time in JSON output, got: %s", output)
	}
}

func TestHandler_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Format: FormatCompact,
		Level:  slog.LevelWarn,
		Output: &buf,
		Colors: false,
	})

	logger := slog.New(handler)
	logger.Debug("Should not appear")
	logger.Info("Should not appear")
	logger.Warn("Should appear")

	output := buf.String()
	if strings.Contains(output, "Should not appear") {
		t.Errorf("Expected DEBUG and INFO to be filtered out, got: %s", output)
	}
	if !strings.Contains(output, "Should appear") {
		t.Errorf("Expected WARN to appear, got: %s", output)
	}
}

func TestHandler_NoAttributes(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Format: FormatCompact,
		Level:  slog.LevelDebug,
		Output: &buf,
		Colors: false,
	})

	logger := slog.New(handler)
	logger.Info("Message without attributes")

	output := buf.String()
	if strings.Contains(output, "→") {
		t.Errorf("Expected no → separator when no attributes, got: %s", output)
	}
	if strings.Contains(output, "{}") {
		t.Errorf("Expected no empty JSON when no attributes, got: %s", output)
	}
}

func TestHandler_Enabled(t *testing.T) {
	handler := NewHandler(&HandlerOptions{
		Format: FormatCompact,
		Level:  slog.LevelInfo,
		Output: &bytes.Buffer{},
		Colors: false,
	})

	ctx := context.Background()
	if handler.Enabled(ctx, slog.LevelDebug) {
		t.Error("Expected DEBUG to be disabled when level is INFO")
	}
	if !handler.Enabled(ctx, slog.LevelInfo) {
		t.Error("Expected INFO to be enabled when level is INFO")
	}
	if !handler.Enabled(ctx, slog.LevelWarn) {
		t.Error("Expected WARN to be enabled when level is INFO")
	}
	if !handler.Enabled(ctx, slog.LevelError) {
		t.Error("Expected ERROR to be enabled when level is INFO")
	}
}

func TestHandler_TraceLevel(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Format: FormatCompact,
		Level:  slog.LevelDebug - 4,
		Output: &buf,
		Colors: false,
	})

	logger := slog.New(handler)
	logger.Log(context.Background(), slog.LevelDebug-4, "Trace message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "TRACE") {
		t.Errorf("Expected TRACE level in output, got: %s", output)
	}
	if !strings.Contains(output, "Trace message") {
		t.Errorf("Expected trace message in output, got: %s", output)
	}
}

// TestWithAttrs_AttrsAppearInOutput verifies that attributes registered via
// WithAttrs are included in every subsequent log record emitted through the
// returned handler.
func TestWithAttrs_AttrsAppearInOutput(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := NewHandler(&HandlerOptions{
		Format: FormatCompact,
		Level:  slog.LevelDebug,
		Output: &buf,
		Colors: false,
	})

	// Derive a handler that carries a pre-set attribute.
	derived := baseHandler.WithAttrs([]slog.Attr{slog.String("k", "v")})

	logger := slog.New(derived)
	logger.Info("attrs test message")

	output := buf.String()
	if !strings.Contains(output, "k") {
		t.Errorf("Expected pre-set attribute key 'k' in output, got: %s", output)
	}
	if !strings.Contains(output, "v") {
		t.Errorf("Expected pre-set attribute value 'v' in output, got: %s", output)
	}
}

// TestWithGroup_GroupPrefixesAttrs verifies that a handler created via
// WithGroup prefixes attribute keys with the group name in the output.
func TestWithGroup_GroupPrefixesAttrs(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := NewHandler(&HandlerOptions{
		Format: FormatCompact,
		Level:  slog.LevelDebug,
		Output: &buf,
		Colors: false,
	})

	grouped := baseHandler.WithGroup("mygroup")
	logger := slog.New(grouped)
	logger.Info("group test", "field", "data")

	output := buf.String()
	if !strings.Contains(output, "mygroup") {
		t.Errorf("Expected group prefix 'mygroup' in output, got: %s", output)
	}
}

// TestColorForLevel_ReturnsNonEmpty ensures colorForLevel returns a
// non-empty ANSI code for every standard log level and for unknown levels.
func TestColorForLevel_ReturnsNonEmpty(t *testing.T) {
	levels := []struct {
		name  string
		level slog.Level
	}{
		{"Trace", slog.LevelDebug - 4},
		{"Debug", slog.LevelDebug},
		{"Info", slog.LevelInfo},
		{"Warn", slog.LevelWarn},
		{"Error", slog.LevelError},
		{"Unknown high level", slog.Level(99)},
	}

	for _, tt := range levels {
		t.Run(tt.name, func(t *testing.T) {
			color := colorForLevel(tt.level)
			if color == "" {
				t.Errorf("colorForLevel(%v) returned empty string", tt.level)
			}
		})
	}
}

// TestEmojiForLevel_AllLevels verifies emojiForLevel returns the correct
// emoji for every branch: Trace (< Debug), Debug, Info, Warn, Error, and
// an unknown high level that falls into the Error/default branch.
func TestEmojiForLevel_AllLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    slog.Level
		expected string
	}{
		{"Trace", slog.LevelDebug - 4, "🔍"},
		{"Debug", slog.LevelDebug, "🔵"},
		{"Info", slog.LevelInfo, "🟢"},
		{"Warn", slog.LevelWarn, "🟡"},
		{"Error", slog.LevelError, "🔴"},
		{"Unknown high level", slog.Level(99), "🔴"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emoji := emojiForLevel(tt.level)
			if emoji != tt.expected {
				t.Errorf("emojiForLevel(%v) = %q, want %q", tt.level, emoji, tt.expected)
			}
		})
	}
}
