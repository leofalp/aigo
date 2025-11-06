package openai

import (
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
			name:     "with quotes",
			input:    `"some content"`,
			expected: "some content",
		},
		{
			name:     "with escaped quotes",
			input:    `{\"name\": \"test\"}`,
			expected: `{"name": "test"}`,
		},
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
			input:    `"content<|END OF THOUGHT|>[/TOOLCALL]"`,
			expected: "content",
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
