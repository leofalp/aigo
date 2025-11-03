package slogobs

import (
	"os"
	"strings"
)

// Format represents the output format for logs.
type Format string

const (
	// FormatCompact is a single-line format with JSON attributes (default for development).
	// Example: 2025-11-03 10:40:35 DEBUG Message → {"key":"value"}
	FormatCompact Format = "compact"

	// FormatPretty is a multi-line format with indented attributes (for debugging).
	// Example:
	// [2025-11-03 10:40:35] DEBUG | Message
	//   • key = value
	FormatPretty Format = "pretty"

	// FormatJSON is standard JSON format (for production/log aggregation).
	// Example: {"time":"2025-11-03T10:40:35","level":"DEBUG","msg":"Message","key":"value"}
	FormatJSON Format = "json"
)

// ParseFormat parses a format string and returns the corresponding Format.
// If the format is invalid, it returns FormatCompact (default).
func ParseFormat(s string) Format {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "compact":
		return FormatCompact
	case "pretty":
		return FormatPretty
	case "json":
		return FormatJSON
	default:
		return FormatCompact
	}
}

// GetFormatFromEnv retrieves the log format from environment variables.
// It checks AIGO_LOG_FORMAT first, then falls back to LOG_FORMAT.
// If neither is set, it returns FormatCompact (default).
func GetFormatFromEnv() Format {
	// Check AIGO_LOG_FORMAT first (highest priority)
	if format := os.Getenv("AIGO_LOG_FORMAT"); format != "" {
		return ParseFormat(format)
	}

	// Fall back to LOG_FORMAT
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		return ParseFormat(format)
	}

	// Default to compact format
	return FormatCompact
}

// String returns the string representation of the Format.
func (f Format) String() string {
	return string(f)
}
