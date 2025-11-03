package slogobs

import (
	"bytes"
	"log/slog"
	"os"
	"testing"
)

func TestWithFormat(t *testing.T) {
	cfg := defaultConfig()
	WithFormat(FormatPretty)(cfg)

	if cfg.format != FormatPretty {
		t.Errorf("WithFormat(FormatPretty) = %v, want %v", cfg.format, FormatPretty)
	}
}

func TestWithLevel(t *testing.T) {
	cfg := defaultConfig()
	WithLevel(slog.LevelError)(cfg)

	if cfg.level != slog.LevelError {
		t.Errorf("WithLevel(LevelError) = %v, want %v", cfg.level, slog.LevelError)
	}
}

func TestWithOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := defaultConfig()
	WithOutput(buf)(cfg)

	if cfg.output != buf {
		t.Error("WithOutput did not set the correct output writer")
	}
}

func TestWithColors(t *testing.T) {
	cfg := defaultConfig()
	WithColors(true)(cfg)

	if !cfg.colors {
		t.Error("WithColors(true) did not enable colors")
	}

	WithColors(false)(cfg)
	if cfg.colors {
		t.Error("WithColors(false) did not disable colors")
	}
}

func TestWithLogger(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := defaultConfig()
	WithLogger(logger)(cfg)

	if cfg.logger != logger {
		t.Error("WithLogger did not set the correct logger")
	}
}

func TestDefaultConfig(t *testing.T) {
	// Clear env vars
	os.Unsetenv("AIGO_LOG_FORMAT")
	os.Unsetenv("LOG_FORMAT")
	os.Unsetenv("AIGO_LOG_LEVEL")
	os.Unsetenv("LOG_LEVEL")

	cfg := defaultConfig()

	if cfg.format != FormatCompact {
		t.Errorf("defaultConfig().format = %v, want %v", cfg.format, FormatCompact)
	}
	if cfg.level != slog.LevelInfo {
		t.Errorf("defaultConfig().level = %v, want %v", cfg.level, slog.LevelInfo)
	}
	if cfg.output != os.Stdout {
		t.Error("defaultConfig().output should be os.Stdout")
	}
	if cfg.colors {
		t.Error("defaultConfig().colors should be false")
	}
	if cfg.logger != nil {
		t.Error("defaultConfig().logger should be nil")
	}
}

func TestApplyOptions(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := applyOptions(
		WithFormat(FormatJSON),
		WithLevel(slog.LevelDebug),
		WithOutput(buf),
		WithColors(true),
	)

	if cfg.format != FormatJSON {
		t.Errorf("applyOptions format = %v, want %v", cfg.format, FormatJSON)
	}
	if cfg.level != slog.LevelDebug {
		t.Errorf("applyOptions level = %v, want %v", cfg.level, slog.LevelDebug)
	}
	if cfg.output != buf {
		t.Error("applyOptions did not set the correct output")
	}
	if !cfg.colors {
		t.Error("applyOptions did not enable colors")
	}
}
