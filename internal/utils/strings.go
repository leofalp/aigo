package utils

import (
	"encoding/json"
	"fmt"
)

const (
	// DefaultMaxStringLength is the default maximum length for truncated strings
	DefaultMaxStringLength = 500
)

// JSONToString converts v to its JSON representation.
func JSONToString(object interface{}, indent ...bool) string {
	var encoded []byte
	var err error
	if indent != nil && len(indent) > 0 && indent[0] {
		encoded, err = json.MarshalIndent(object, "", "  ")
	} else {
		encoded, err = json.Marshal(object)
	}
	if err != nil {
		return "{\"error\": \"failed to marshal to JSON: " + err.Error() + "\"}"
	}
	return string(encoded)
}

// ToString uses JSONToString and returns the JSON string.
// If an error occurs, it returns only the error text.
func ToString(object interface{}) string {
	return JSONToString(object)
}

// TruncateString truncates a string to maxLen characters, adding a suffix with the original length
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
