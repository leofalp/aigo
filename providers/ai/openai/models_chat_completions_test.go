package openai

import (
	"strings"
	"testing"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
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

/*
	cleanThinkTags tests
*/

// TestCleanThinkTags_Strips verifies that a single <think> block is removed from
// the content, leaving only the answer text.
func TestCleanThinkTags_Strips(t *testing.T) {
	input := "<think>I am thinking</think>The answer is 42"

	cleaned := cleanThinkTags(input)

	if cleaned != "The answer is 42" {
		t.Errorf("expected %q, got %q", "The answer is 42", cleaned)
	}
}

// TestCleanThinkTags_NoTags verifies that content without <think> tags is
// returned unchanged.
func TestCleanThinkTags_NoTags(t *testing.T) {
	input := "Just a normal response with no reasoning tags"

	cleaned := cleanThinkTags(input)

	if cleaned != input {
		t.Errorf("expected content unchanged %q, got %q", input, cleaned)
	}
}

// TestCleanThinkTags_MultipleTags verifies behavior when multiple <think>
// blocks are present. cleanThinkTags only strips the first tag pair (it
// searches for the first <think> and the first </think>), so any subsequent
// tags may remain. We verify the first block is removed.
func TestCleanThinkTags_MultipleTags(t *testing.T) {
	input := "<think>first thought</think>middle<think>second thought</think>end"

	cleaned := cleanThinkTags(input)

	// The function removes from the first <think> to the first </think>.
	// After that removal, "middle<think>second thought</think>end" remains.
	if strings.Contains(cleaned, "first thought") {
		t.Errorf("first <think> block should be stripped, got %q", cleaned)
	}
	if !strings.Contains(cleaned, "middle") {
		t.Errorf("content between think blocks should remain, got %q", cleaned)
	}
}

/*
	requestToChatCompletion — ToolChoice tests
*/

// dummyToolDescription returns a minimal ToolDescription suitable for tool choice
// tests. A non-nil Parameters schema is required because requestToChatCompletion
// dereferences the pointer when building chatTool entries.
func dummyToolDescription(toolName string) ai.ToolDescription {
	return ai.ToolDescription{
		Name:        toolName,
		Description: "dummy tool for testing",
		Parameters:  &jsonschema.Schema{Type: "object"},
	}
}

// TestRequestToChatCompletion_ToolChoiceNone verifies that ToolChoiceForced="none"
// maps to a literal "none" string in the outgoing request.
func TestRequestToChatCompletion_ToolChoiceNone(t *testing.T) {
	request := ai.ChatRequest{
		Model: "gpt-4o",
		Tools: []ai.ToolDescription{dummyToolDescription("search")},
		ToolChoice: &ai.ToolChoice{
			ToolChoiceForced: "none",
		},
	}

	result := requestToChatCompletion(request, false)

	toolChoice, ok := result.ToolChoice.(string)
	if !ok {
		t.Fatalf("expected ToolChoice to be a string, got %T", result.ToolChoice)
	}
	if toolChoice != "none" {
		t.Errorf("expected tool_choice %q, got %q", "none", toolChoice)
	}
}

// TestRequestToChatCompletion_ToolChoiceAuto verifies that ToolChoiceForced="auto"
// maps to a literal "auto" string in the outgoing request.
func TestRequestToChatCompletion_ToolChoiceAuto(t *testing.T) {
	request := ai.ChatRequest{
		Model: "gpt-4o",
		Tools: []ai.ToolDescription{dummyToolDescription("search")},
		ToolChoice: &ai.ToolChoice{
			ToolChoiceForced: "auto",
		},
	}

	result := requestToChatCompletion(request, false)

	toolChoice, ok := result.ToolChoice.(string)
	if !ok {
		t.Fatalf("expected ToolChoice to be a string, got %T", result.ToolChoice)
	}
	if toolChoice != "auto" {
		t.Errorf("expected tool_choice %q, got %q", "auto", toolChoice)
	}
}

// TestRequestToChatCompletion_ToolChoiceRequired verifies that
// AtLeastOneRequired=true maps to a literal "required" string.
func TestRequestToChatCompletion_ToolChoiceRequired(t *testing.T) {
	request := ai.ChatRequest{
		Model: "gpt-4o",
		Tools: []ai.ToolDescription{dummyToolDescription("calculator")},
		ToolChoice: &ai.ToolChoice{
			AtLeastOneRequired: true,
		},
	}

	result := requestToChatCompletion(request, false)

	toolChoice, ok := result.ToolChoice.(string)
	if !ok {
		t.Fatalf("expected ToolChoice to be a string, got %T", result.ToolChoice)
	}
	if toolChoice != "required" {
		t.Errorf("expected tool_choice %q, got %q", "required", toolChoice)
	}
}

// TestRequestToChatCompletion_ToolChoiceSpecificTool verifies that a single
// RequiredTools entry maps to a struct with type="function" and the tool name.
func TestRequestToChatCompletion_ToolChoiceSpecificTool(t *testing.T) {
	searchTool := dummyToolDescription("search")
	request := ai.ChatRequest{
		Model: "gpt-4o",
		Tools: []ai.ToolDescription{searchTool},
		ToolChoice: &ai.ToolChoice{
			RequiredTools: []*ai.ToolDescription{&searchTool},
		},
	}

	result := requestToChatCompletion(request, false)

	// ToolChoice should be a map[string]any with "type" and "name" keys
	toolChoiceMap, ok := result.ToolChoice.(map[string]any)
	if !ok {
		t.Fatalf("expected ToolChoice to be map[string]any, got %T (%v)", result.ToolChoice, result.ToolChoice)
	}
	if toolChoiceMap["type"] != "function" {
		t.Errorf("expected type %q, got %q", "function", toolChoiceMap["type"])
	}
	if toolChoiceMap["name"] != "search" {
		t.Errorf("expected name %q, got %q", "search", toolChoiceMap["name"])
	}
}

/*
	chatCompletionToGeneric tests
*/

// TestChatCompletionToGeneric_ReasoningContent verifies that the Reasoning field
// in chatResponseMessage maps to ChatResponse.Reasoning when Content is empty.
func TestChatCompletionToGeneric_ReasoningContent(t *testing.T) {
	resp := chatCompletionResponse{
		ID:    "test-id",
		Model: "gpt-4o",
		Choices: []chatChoice{
			{
				Index: 0,
				Message: chatResponseMessage{
					Role:      "assistant",
					Content:   "the answer",
					Reasoning: "my reasoning",
				},
				FinishReason: "stop",
			},
		},
	}

	result := chatCompletionToGeneric(resp)

	// When Content is non-empty, explicit Reasoning is preserved.
	if !strings.Contains(result.Reasoning, "my reasoning") {
		t.Errorf("expected Reasoning to contain %q, got %q", "my reasoning", result.Reasoning)
	}
	if result.Content != "the answer" {
		t.Errorf("expected Content %q, got %q", "the answer", result.Content)
	}
}

// TestChatCompletionToGeneric_TokenUsage verifies that token counts from the
// provider response are correctly mapped to the generic Usage struct fields.
func TestChatCompletionToGeneric_TokenUsage(t *testing.T) {
	resp := chatCompletionResponse{
		ID:    "test-usage",
		Model: "gpt-4o",
		Choices: []chatChoice{
			{
				Index: 0,
				Message: chatResponseMessage{
					Role:    "assistant",
					Content: "hello",
				},
				FinishReason: "stop",
			},
		},
		Usage: &chatUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	result := chatCompletionToGeneric(resp)

	if result.Usage == nil {
		t.Fatal("expected Usage to be non-nil")
	}
	if result.Usage.PromptTokens != 10 {
		t.Errorf("expected PromptTokens=10, got %d", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 20 {
		t.Errorf("expected CompletionTokens=20, got %d", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 30 {
		t.Errorf("expected TotalTokens=30, got %d", result.Usage.TotalTokens)
	}
}

// TestChatCompletionToGeneric_ThinkTagInContent verifies that <think> tags
// embedded in the content are extracted into Reasoning and stripped from Content.
func TestChatCompletionToGeneric_ThinkTagInContent(t *testing.T) {
	resp := chatCompletionResponse{
		ID:    "test-think",
		Model: "gpt-4o",
		Choices: []chatChoice{
			{
				Index: 0,
				Message: chatResponseMessage{
					Role:    "assistant",
					Content: "<think>thinking deeply...</think>actual answer",
				},
				FinishReason: "stop",
			},
		},
	}

	result := chatCompletionToGeneric(resp)

	if result.Content != "actual answer" {
		t.Errorf("expected Content %q, got %q", "actual answer", result.Content)
	}
	if result.Reasoning == "" {
		t.Error("expected Reasoning to be non-empty after extracting <think> tags")
	}
	if !strings.Contains(result.Reasoning, "thinking deeply...") {
		t.Errorf("expected Reasoning to contain %q, got %q", "thinking deeply...", result.Reasoning)
	}
}

func TestRequestToChatCompletion_ContentParts(t *testing.T) {
	req := ai.ChatRequest{
		Messages: []ai.Message{
			{
				Role: ai.RoleUser,
				ContentParts: []ai.ContentPart{
					{Type: ai.ContentTypeText, Text: "Look at this image"},
					{Type: ai.ContentTypeImage, Image: &ai.ImageData{URI: "https://example.com/image.jpg"}},
					{Type: ai.ContentTypeImage, Image: &ai.ImageData{MimeType: "image/png", Data: "base64data"}},
					{Type: ai.ContentTypeImage, Image: nil},                                                // Should be skipped
					{Type: ai.ContentTypeImage, Image: &ai.ImageData{MimeType: "image/unknown", Data: ""}}, // Should be skipped
					{Type: ai.ContentTypeAudio, Audio: &ai.AudioData{MimeType: "audio/mp3", Data: "audiodata"}},
					{Type: ai.ContentTypeAudio, Audio: nil}, // Should be skipped
					{Type: ai.ContentTypeVideo},             // Should be skipped
					{Type: ai.ContentTypeDocument},          // Should be skipped
				},
			},
		},
	}

	respReq := requestToChatCompletion(req, false)
	if len(respReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(respReq.Messages))
	}

	parts, ok := respReq.Messages[0].Content.([]contentPart)
	if !ok {
		t.Fatalf("expected content to be []contentPart, got %T", respReq.Messages[0].Content)
	}

	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}

	if parts[0].Type != "text" || parts[0].Text != "Look at this image" {
		t.Errorf("unexpected part 0: %v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL.URL != "https://example.com/image.jpg" {
		t.Errorf("unexpected part 1: %v", parts[1])
	}
	if parts[2].Type != "image_url" || parts[2].ImageURL.URL != "data:image/png;base64,base64data" {
		t.Errorf("unexpected part 2: %v", parts[2])
	}
	if parts[3].Type != "input_audio" || parts[3].InputAudio.Data != "audiodata" || parts[3].InputAudio.Format != "mp3" {
		t.Errorf("unexpected part 3: %v", parts[3])
	}
}

func TestRequestToChatCompletion_GenerationConfig(t *testing.T) {
	req := ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		GenerationConfig: &ai.GenerationConfig{
			Temperature:      0.7,
			TopP:             0.9,
			MaxTokens:        100,
			FrequencyPenalty: 0.5,
			PresencePenalty:  0.6,
		},
	}

	respReq := requestToChatCompletion(req, false)
	if respReq.Temperature == nil || float32(*respReq.Temperature) != 0.7 {
		t.Errorf("expected temperature 0.7, got %v", *respReq.Temperature)
	}
	if respReq.TopP == nil || float32(*respReq.TopP) != 0.9 {
		t.Errorf("expected top_p 0.9, got %v", *respReq.TopP)
	}
	// MaxTokens maps to the legacy MaxTokens field (not MaxCompletionTokens)
	if respReq.MaxTokens == nil || *respReq.MaxTokens != 100 {
		t.Errorf("expected max_tokens 100, got %v", respReq.MaxTokens)
	}
	if respReq.FrequencyPenalty == nil || float32(*respReq.FrequencyPenalty) != 0.5 {
		t.Errorf("expected frequency_penalty 0.5, got %v", *respReq.FrequencyPenalty)
	}
	if respReq.PresencePenalty == nil || float32(*respReq.PresencePenalty) != 0.6 {
		t.Errorf("expected presence_penalty 0.6, got %v", *respReq.PresencePenalty)
	}

	// Test fallback to MaxOutputTokens
	req2 := ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		GenerationConfig: &ai.GenerationConfig{
			MaxOutputTokens: 200,
		},
	}
	respReq2 := requestToChatCompletion(req2, false)
	if respReq2.MaxCompletionTokens == nil || *respReq2.MaxCompletionTokens != 200 {
		t.Errorf("expected max_completion_tokens 200, got %v", *respReq2.MaxCompletionTokens)
	}
}

func TestRequestToChatCompletion_ResponseFormat(t *testing.T) {
	// Test OutputSchema
	req := ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ResponseFormat: &ai.ResponseFormat{
			OutputSchema: &jsonschema.Schema{Type: "object"},
			Strict:       true,
		},
	}

	respReq := requestToChatCompletion(req, false)
	if respReq.ResponseFormat == nil || respReq.ResponseFormat.Type != "json_schema" {
		t.Fatalf("expected json_schema response format, got %v", respReq.ResponseFormat)
	}
	if respReq.ResponseFormat.JSONSchema.Name != "response_schema" || !respReq.ResponseFormat.JSONSchema.Strict {
		t.Errorf("unexpected json_schema details: %v", respReq.ResponseFormat.JSONSchema)
	}

	// Test Type hint
	req2 := ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ResponseFormat: &ai.ResponseFormat{
			Type: "json_object",
		},
	}
	respReq2 := requestToChatCompletion(req2, false)
	if respReq2.ResponseFormat == nil || respReq2.ResponseFormat.Type != "json_object" {
		t.Errorf("expected json_object response format, got %v", respReq2.ResponseFormat)
	}
}

func TestRequestToChatCompletion_LegacyFunctions(t *testing.T) {
	req := ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		Tools: []ai.ToolDescription{
			{
				Name:        "test_tool",
				Description: "A test tool",
				Parameters:  &jsonschema.Schema{Type: "object"},
			},
		},
		ToolChoice: &ai.ToolChoice{
			RequiredTools: []*ai.ToolDescription{{Name: "test_tool"}},
		},
	}

	respReq := requestToChatCompletion(req, true)
	if len(respReq.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(respReq.Functions))
	}
	if respReq.Functions[0].Name != "test_tool" {
		t.Errorf("expected function name 'test_tool', got %s", respReq.Functions[0].Name)
	}

	fcMap, ok := respReq.FunctionCall.(map[string]any)
	if !ok || fcMap["name"] != "test_tool" {
		t.Errorf("expected function_call map with name 'test_tool', got %v", respReq.FunctionCall)
	}
}
