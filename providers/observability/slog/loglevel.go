package slog

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// GetLogLevelFromEnv returns the log level configured via environment variables.
// It checks AIGO_LOG_LEVEL first, then falls back to LOG_LEVEL.
// Supported values: DEBUG, INFO, WARN, WARNING, ERROR
// Default: INFO
func GetLogLevelFromEnv() slog.Level {
	level := os.Getenv("AIGO_LOG_LEVEL")
	if level == "" {
		level = os.Getenv("LOG_LEVEL")
	}
	if level == "" {
		return slog.LevelInfo // default
	}

	return ParseLogLevel(level)
}

// ParseLogLevel parses a log level string into slog.Level.
// Supported values: DEBUG, INFO, WARN, WARNING, ERROR (case-insensitive)
// Returns INFO for unknown values and prints a warning to stderr.
func ParseLogLevel(level string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		fmt.Fprintf(os.Stderr, "Warning: Unknown log level '%s', using INFO\n", level)
		return slog.LevelInfo
	}
}

// LogLevelString returns a human-readable string for the log level.
func LogLevelString(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARN"
	case slog.LevelError:
		return "ERROR"
	default:
		return fmt.Sprintf("LEVEL(%d)", level)
	}
}
