package utils

import (
	"strings"
	"testing"
)

// TestJSONToString_Compact verifies that JSONToString produces compact JSON by default.
func TestJSONToString_Compact(t *testing.T) {
	input := map[string]int{"a": 1, "b": 2}
	result := JSONToString(input)

	// Must be valid JSON and must not contain a newline (compact mode).
	if strings.Contains(result, "\n") {
		t.Errorf("JSONToString() compact mode should not contain newlines, got: %q", result)
	}
	if !strings.Contains(result, `"a"`) {
		t.Errorf("JSONToString() result missing key 'a': %q", result)
	}
}

// TestJSONToString_Indented verifies that passing indent=true produces
// pretty-printed JSON with newlines.
func TestJSONToString_Indented(t *testing.T) {
	input := map[string]int{"x": 42}
	result := JSONToString(input, true)

	if !strings.Contains(result, "\n") {
		t.Errorf("JSONToString(indent=true) should contain newlines, got: %q", result)
	}
	if !strings.Contains(result, "  ") {
		t.Errorf("JSONToString(indent=true) should contain two-space indentation, got: %q", result)
	}
}

// TestJSONToString_MarshalError verifies that JSONToString returns an error
// sentinel string rather than panicking when the value cannot be marshaled.
func TestJSONToString_MarshalError(t *testing.T) {
	// Channels cannot be marshaled to JSON.
	input := make(chan int)
	result := JSONToString(input)

	if !strings.HasPrefix(result, `{"error":`) {
		t.Errorf("JSONToString() on unmarshalable value should return error JSON, got: %q", result)
	}
}

// TestToString verifies that ToString is a thin wrapper returning the same
// compact JSON as JSONToString with no indentation flag.
func TestToString(t *testing.T) {
	input := struct{ Name string }{"alice"}
	wantPrefix := `{"Name":"alice"}`

	got := ToString(input)
	if got != wantPrefix {
		t.Errorf("ToString() = %q, want %q", got, wantPrefix)
	}
}

// TestTruncateString table-driven tests cover: string shorter than maxLen,
// string exactly at maxLen, string longer than maxLen, zero maxLen (uses
// DefaultMaxStringLength), and negative maxLen.
func TestTruncateString(t *testing.T) {
	testCases := []struct {
		name   string
		input  string
		maxLen int
		// wantTruncated indicates whether the result should contain the truncation suffix
		wantTruncated bool
	}{
		{
			name:          "shorter than maxLen returns unchanged",
			input:         "hello",
			maxLen:        10,
			wantTruncated: false,
		},
		{
			name:          "exactly at maxLen returns unchanged",
			input:         "hello",
			maxLen:        5,
			wantTruncated: false,
		},
		{
			name:          "longer than maxLen gets truncated",
			input:         "hello world",
			maxLen:        5,
			wantTruncated: true,
		},
		{
			name:          "zero maxLen uses DefaultMaxStringLength",
			input:         strings.Repeat("a", DefaultMaxStringLength+1),
			maxLen:        0,
			wantTruncated: true,
		},
		{
			name:          "negative maxLen uses DefaultMaxStringLength",
			input:         strings.Repeat("b", DefaultMaxStringLength+1),
			maxLen:        -1,
			wantTruncated: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := TruncateString(testCase.input, testCase.maxLen)

			hasSuffix := strings.Contains(got, "... (truncated, total:")
			if hasSuffix != testCase.wantTruncated {
				t.Errorf("TruncateString(%q, %d) truncated=%v, want truncated=%v; got %q",
					testCase.input, testCase.maxLen, hasSuffix, testCase.wantTruncated, got)
			}
		})
	}
}

// TestTruncateString_ContentPreserved verifies that the prefix before the
// ellipsis exactly matches the first maxLen characters of the input.
func TestTruncateString_ContentPreserved(t *testing.T) {
	input := "abcdefghij"
	got := TruncateString(input, 4)

	if !strings.HasPrefix(got, "abcd") {
		t.Errorf("TruncateString() should start with first 4 chars, got: %q", got)
	}
}

// TestTruncateStringDefault verifies that TruncateStringDefault delegates to
// TruncateString with DefaultMaxStringLength.
func TestTruncateStringDefault(t *testing.T) {
	// String shorter than the default: should be returned unchanged.
	short := "short"
	if got := TruncateStringDefault(short); got != short {
		t.Errorf("TruncateStringDefault(%q) = %q, want %q", short, got, short)
	}

	// String longer than the default: should be truncated.
	long := strings.Repeat("x", DefaultMaxStringLength+10)
	got := TruncateStringDefault(long)
	if !strings.Contains(got, "... (truncated, total:") {
		t.Errorf("TruncateStringDefault() should truncate long string, got: %q", got[:50])
	}
}
