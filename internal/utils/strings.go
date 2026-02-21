package utils

import (
	"encoding/json"
	"fmt"
)

const (
	// DefaultMaxStringLength is the default maximum length for truncated strings
	DefaultMaxStringLength = 500
)

// JSONToString serialises object to its JSON representation and returns it as a
// string. When the optional indent argument is true the output is
// pretty-printed with two-space indentation. On marshalling failure it returns
// a JSON-formatted error string rather than panicking, so the result is always
// safe to use in log output.
func JSONToString(object interface{}, indent ...bool) string {
	var encoded []byte
	var err error
	if len(indent) > 0 && indent[0] {
		encoded, err = json.MarshalIndent(object, "", "  ")
	} else {
		encoded, err = json.Marshal(object)
	}
	if err != nil {
		return "{\"error\": \"failed to marshal to JSON: " + err.Error() + "\"}"
	}
	return string(encoded)
}

// ToString returns the compact JSON representation of object. It is a
// convenience wrapper around [JSONToString] for the common case where
// indentation is not needed.
func ToString(object interface{}) string {
	return JSONToString(object)
}

// TruncateString shortens s to at most maxLen characters, appending a suffix
// that records the original total length so callers know data was omitted.
// If maxLen is zero or negative, [DefaultMaxStringLength] is used instead.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 0 {
		maxLen = DefaultMaxStringLength
	}
	return fmt.Sprintf("%s... (truncated, total: %d chars)", s[:maxLen], len(s))
}

// TruncateStringDefault truncates a string using DefaultMaxStringLength
func TruncateStringDefault(s string) string {
	return TruncateString(s, DefaultMaxStringLength)
}
