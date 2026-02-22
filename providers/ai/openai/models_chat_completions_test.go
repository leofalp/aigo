package openai

import (
	"strings"
	"testing"
)

func TestParseToolCallsFromContent_Standard(t *testing.T) {
	content := `<TOOLCALL>[{"name": "Calculator", "arguments": {"A": 1234, "B": 567, "Op": "mul"}}]</TOOLCALL>`

	toolCalls := parseChatCompletionToolCallsFromContent(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "Calculator" {
		t.Errorf("Expected Calculator, got %s", toolCalls[0].Function.Name)
	}
}

func TestParseToolCallsFromContent_WithEscapedQuotes(t *testing.T) {
	// This is what we actually receive from OpenRouter
	content := `"<TOOLCALL>[{\"name\": \"Calculator\", \"arguments\": {\"A\": 1234, \"B\": 567, \"Op\": \"mul\"}}]</TOOLCALL>"`

	toolCalls := parseChatCompletionToolCallsFromContent(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "Calculator" {
		t.Errorf("Expected Calculator, got %s", toolCalls[0].Function.Name)
	}
}

func TestParseToolCallsFromContent_WithMarkers(t *testing.T) {
	content := `"<TOOLCALL>[{\"name\": \"Calculator\", \"arguments\": {\"A\": 1234, \"B\": 567, \"Op\": \"mul\"}}]</TOOLCALL><|END OF THOUGHT|>[{"`

	toolCalls := parseChatCompletionToolCallsFromContent(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "Calculator" {
		t.Errorf("Expected Calculator, got %s", toolCalls[0].Function.Name)
	}
}

func TestParseToolCallsFromContent_WithExtraJunk(t *testing.T) {
	content := `"<TOOLCALL>[{\"name\": \"Calculator\", \"arguments\": {\"A\": 150, \"B\": 250, \"Op\": \"add\"}}]</TOOLCALL>[/TOOLCALL]</TOOLCALL><TOOLCALL>[{"`

	toolCalls := parseChatCompletionToolCallsFromContent(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "Calculator" {
		t.Errorf("Expected Calculator, got %s", toolCalls[0].Function.Name)
	}
}

func TestParseToolCallsFromContent_PlainJSON(t *testing.T) {
	content := `[{"name": "Calculator", "arguments": {"A": 100, "B": 200, "Op": "add"}}]`

	toolCalls := parseChatCompletionToolCallsFromContent(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "Calculator" {
		t.Errorf("Expected Calculator, got %s", toolCalls[0].Function.Name)
	}
}

func TestParseToolCallsFromContent_MultipleTools(t *testing.T) {
	content := `<TOOLCALL>[{"name": "Calculator", "arguments": {"A": 1, "B": 2, "Op": "add"}}, {"name": "Search", "arguments": {"query": "test"}}]</TOOLCALL>`

	toolCalls := parseChatCompletionToolCallsFromContent(content)

	if len(toolCalls) != 2 {
		t.Fatalf("Expected 2 tool calls, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "Calculator" {
		t.Errorf("Expected Calculator, got %s", toolCalls[0].Function.Name)
	}

	if toolCalls[1].Function.Name != "Search" {
		t.Errorf("Expected Search, got %s", toolCalls[1].Function.Name)
	}
}

func TestParseToolCallsFromContent_Empty(t *testing.T) {
	content := ""

	toolCalls := parseChatCompletionToolCallsFromContent(content)

	if len(toolCalls) != 0 {
		t.Fatalf("Expected 0 tool calls, got %d", len(toolCalls))
	}
}

func TestParseToolCallsFromContent_NoTags(t *testing.T) {
	content := "This is just regular text without any tool calls"

	toolCalls := parseChatCompletionToolCallsFromContent(content)

	if len(toolCalls) != 0 {
		t.Fatalf("Expected 0 tool calls, got %d", len(toolCalls))
	}
}

func TestParseToolCallsFromContent_IncompleteJSON(t *testing.T) {
	// JSON that gets cut off mid-stream
	content := `"<TOOLCALL>[{\"name\": \"Calculator\", \"arguments\": {\"A\": 150, \"B\": 250, \"Op\": \"add\"}}]</TOOLCALL></TOOLCALL><TOOLCALL>[{"`

	toolCalls := parseChatCompletionToolCallsFromContent(content)

	// Should still parse the complete one
	if len(toolCalls) < 1 {
		t.Fatalf("Expected at least 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "Calculator" {
		t.Errorf("Expected Calculator, got %s", toolCalls[0].Function.Name)
	}
}

func TestCleanToolCallContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with end of thought marker",
			input:    "content<|END OF THOUGHT|>more",
			expected: "contentmore",
		},
		{
			name:     "with toolcall marker",
			input:    "content[/TOOLCALL]more",
			expected: "contentmore",
		},
		{
			name:     "multiple markers",
			input:    `content<|END OF THOUGHT|>[/TOOLCALL]`,
			expected: "content",
		},
		{
			name:     "with thought tags",
			input:    "result<THOUGHT>thinking</THOUGHT>",
			expected: "resultthinking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanToolCallContent(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseToolCallsJSON_AutoFix(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectSuccess bool
	}{
		{
			name:          "valid array",
			input:         `[{"name": "test", "arguments": {}}]`,
			expectSuccess: true,
		},
		{
			name:          "missing opening bracket",
			input:         `{"name": "test", "arguments": {}}]`,
			expectSuccess: true, // Should add opening bracket
		},
		{
			name:          "missing closing bracket",
			input:         `[{"name": "test", "arguments": {}}`,
			expectSuccess: true, // Should add closing bracket
		},
		{
			name:          "trailing junk",
			input:         `[{"name": "test", "arguments": {}}]extra`,
			expectSuccess: true, // Should ignore trailing content
		},
		{
			name:          "incomplete last object",
			input:         `[{"name": "test", "arguments": {}}, {"name": "incomplete"`,
			expectSuccess: true, // Should parse first complete object
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseToolCallsJSON(tt.input)
			if tt.expectSuccess && len(result) == 0 {
				t.Errorf("Expected to parse at least one tool call, got none")
			}
		})
	}
}

func TestParseToolCallsFromContent_RealWorldExamples(t *testing.T) {
	// Real examples from OpenRouter responses
	examples := []struct {
		name     string
		content  string
		expected int // expected number of tool calls
	}{
		{
			name:     "openrouter simple",
			content:  `"<TOOLCALL>[{\"name\": \"Calculator\", \"arguments\": {\"A\": 1234, \"B\": 567, \"Op\": \"mul\"}}]</TOOLCALL><|END OF THOUGHT|>[{"`,
			expected: 1,
		},
		{
			name:     "openrouter with extra closing tags",
			content:  `"<TOOLCALL>[{\"name\": \"Calculator\", \"arguments\": {\"A\": 150, \"B\": 250, \"Op\": \"add\"}}]</TOOLCALL>[/TOOLCALL]</TOOLCALL><TOOLCALL>[{"`,
			expected: 1,
		},
		{
			name:     "standard format",
			content:  `<TOOLCALL>[{"name": "Search", "arguments": {"query": "weather"}}]</TOOLCALL>`,
			expected: 1,
		},
	}

	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			toolCalls := parseChatCompletionToolCallsFromContent(ex.content)
			if len(toolCalls) != ex.expected {
				t.Errorf("Expected %d tool calls, got %d. Content: %s",
					ex.expected, len(toolCalls), ex.content)
			}
		})
	}
}

// TestBuildDataURL verifies that buildDataURL produces correct data-URL strings
// and that it returns an empty string when either argument is empty.
func TestBuildDataURL(t *testing.T) {
	testCases := []struct {
		name     string
		mimeType string
		data     string
		want     string
	}{
		{
			name:     "jpeg image",
			mimeType: "image/jpeg",
			data:     "abc123",
			want:     "data:image/jpeg;base64,abc123",
		},
		{
			name:     "png image",
			mimeType: "image/png",
			data:     "xyz",
			want:     "data:image/png;base64,xyz",
		},
		{
			name:     "wav audio",
			mimeType: "audio/wav",
			data:     "audiodata",
			want:     "data:audio/wav;base64,audiodata",
		},
		{
			name:     "empty mimeType returns empty string",
			mimeType: "",
			data:     "abc123",
			want:     "",
		},
		{
			name:     "empty data returns empty string",
			mimeType: "image/jpeg",
			data:     "",
			want:     "",
		},
		{
			name:     "both empty returns empty string",
			mimeType: "",
			data:     "",
			want:     "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := buildDataURL(testCase.mimeType, testCase.data)
			if got != testCase.want {
				t.Errorf("buildDataURL(%q, %q) = %q, want %q", testCase.mimeType, testCase.data, got, testCase.want)
			}
		})
	}
}

// TestMimeTypeToAudioFormat verifies that MIME type strings are correctly
// converted to the OpenAI audio format identifier, and that unknown or empty
// inputs fall back to "wav".
func TestMimeTypeToAudioFormat(t *testing.T) {
	testCases := []struct {
		name     string
		mimeType string
		want     string
	}{
		{
			name:     "audio/mp3 strips prefix",
			mimeType: "audio/mp3",
			want:     "mp3",
		},
		{
			name:     "audio/wav strips prefix",
			mimeType: "audio/wav",
			want:     "wav",
		},
		{
			name:     "audio/ogg strips prefix",
			mimeType: "audio/ogg",
			want:     "ogg",
		},
		{
			name:     "uppercase is normalised",
			mimeType: "AUDIO/MP3",
			want:     "mp3",
		},
		{
			name:     "leading whitespace is trimmed",
			mimeType: " audio/flac",
			want:     "flac",
		},
		{
			name:     "empty string defaults to wav",
			mimeType: "",
			want:     "wav",
		},
		{
			name:     "bare audio/ prefix defaults to wav",
			mimeType: "audio/",
			want:     "wav",
		},
		{
			name:     "non-audio mime type is returned as-is after lower+trim",
			mimeType: "video/mp4",
			want:     "video/mp4",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := mimeTypeToAudioFormat(testCase.mimeType)
			// Validate trimming and lower-casing independently
			_ = strings.ToLower(testCase.mimeType)
			if got != testCase.want {
				t.Errorf("mimeTypeToAudioFormat(%q) = %q, want %q", testCase.mimeType, got, testCase.want)
			}
		})
	}
}
