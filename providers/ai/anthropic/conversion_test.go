package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
)

// intPtr is a helper that converts an int literal to *int, used throughout
// these tests to construct ThinkingBudget values without intermediate variables.
func intPtr(v int) *int { return &v }

// ── requestToAnthropic ────────────────────────────────────────────────────────

// TestRequestToAnthropic_SystemPrompt verifies that the system prompt is encoded
// as a plain JSON string when prompt caching is disabled, and as a content-block
// array (with cache_control attached) when prompt caching is enabled.
func TestRequestToAnthropic_SystemPrompt(t *testing.T) {
	t.Run("no caching → plain JSON string", func(t *testing.T) {
		request := ai.ChatRequest{SystemPrompt: "hello system"}
		result, err := requestToAnthropic(request, Capabilities{PromptCaching: false})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// System must be the JSON string "hello system".
		var decoded string
		if err := json.Unmarshal(result.System, &decoded); err != nil {
			t.Fatalf("expected JSON string, got unmarshal error: %v", err)
		}
		if decoded != "hello system" {
			t.Errorf("system prompt: got %q, want %q", decoded, "hello system")
		}
	})

	t.Run("prompt caching → content-block array with cache_control", func(t *testing.T) {
		request := ai.ChatRequest{SystemPrompt: "hello system"}
		result, err := requestToAnthropic(request, Capabilities{PromptCaching: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// System must decode as a slice of anthropicContentBlock.
		var blocks []anthropicContentBlock
		if err := json.Unmarshal(result.System, &blocks); err != nil {
			t.Fatalf("expected JSON array of content blocks, got unmarshal error: %v", err)
		}
		if len(blocks) != 1 {
			t.Fatalf("expected 1 system block, got %d", len(blocks))
		}
		block := blocks[0]
		if block.Type != "text" {
			t.Errorf("block.Type: got %q, want %q", block.Type, "text")
		}
		if block.Text != "hello system" {
			t.Errorf("block.Text: got %q, want %q", block.Text, "hello system")
		}
		if block.CacheControl == nil {
			t.Fatal("expected CacheControl to be set, got nil")
		}
		if block.CacheControl.Type != "ephemeral" {
			t.Errorf("CacheControl.Type: got %q, want %q", block.CacheControl.Type, "ephemeral")
		}
	})
}

// TestRequestToAnthropic_MaxTokensDefault checks that a request without any
// GenerationConfig still sends the required max_tokens field with the safe
// default of 4096.
func TestRequestToAnthropic_MaxTokensDefault(t *testing.T) {
	request := ai.ChatRequest{}
	result, err := requestToAnthropic(request, Capabilities{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MaxTokens != 4096 {
		t.Errorf("MaxTokens: got %d, want 4096", result.MaxTokens)
	}
}

// TestRequestToAnthropic_MaxTokensPrecedence verifies that MaxOutputTokens takes
// precedence over the legacy MaxTokens field when both are set, and that MaxTokens
// is used correctly when MaxOutputTokens is zero.
func TestRequestToAnthropic_MaxTokensPrecedence(t *testing.T) {
	t.Run("MaxOutputTokens wins when set", func(t *testing.T) {
		request := ai.ChatRequest{
			GenerationConfig: &ai.GenerationConfig{
				MaxOutputTokens: 8192,
				MaxTokens:       2048,
			},
		}
		result, err := requestToAnthropic(request, Capabilities{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.MaxTokens != 8192 {
			t.Errorf("MaxTokens: got %d, want 8192", result.MaxTokens)
		}
	})

	t.Run("MaxTokens used when MaxOutputTokens is zero", func(t *testing.T) {
		request := ai.ChatRequest{
			GenerationConfig: &ai.GenerationConfig{
				MaxOutputTokens: 0,
				MaxTokens:       2048,
			},
		}
		result, err := requestToAnthropic(request, Capabilities{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.MaxTokens != 2048 {
			t.Errorf("MaxTokens: got %d, want 2048", result.MaxTokens)
		}
	})
}

// TestRequestToAnthropic_Temperature checks that a non-zero Temperature in
// GenerationConfig is forwarded as a pointer on the wire request.
func TestRequestToAnthropic_Temperature(t *testing.T) {
	request := ai.ChatRequest{
		GenerationConfig: &ai.GenerationConfig{Temperature: 0.7},
	}
	result, err := requestToAnthropic(request, Capabilities{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Temperature == nil {
		t.Fatal("Temperature: got nil, want pointer")
	}
	const wantTemp = 0.7
	const tolerance = 1e-6
	if diff := *result.Temperature - wantTemp; diff > tolerance || diff < -tolerance {
		t.Errorf("Temperature: got %v, want ~%v", *result.Temperature, wantTemp)
	}
}

// TestRequestToAnthropic_AdaptiveThinking verifies that setting IncludeThoughts=true
// without providing a ThinkingBudget produces an adaptive thinking config.
func TestRequestToAnthropic_AdaptiveThinking(t *testing.T) {
	request := ai.ChatRequest{
		GenerationConfig: &ai.GenerationConfig{
			IncludeThoughts: true,
			ThinkingBudget:  nil,
		},
	}
	result, err := requestToAnthropic(request, Capabilities{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Thinking == nil {
		t.Fatal("Thinking: got nil, want adaptive config")
	}
	if result.Thinking.Type != "adaptive" {
		t.Errorf("Thinking.Type: got %q, want %q", result.Thinking.Type, "adaptive")
	}
}

// TestRequestToAnthropic_ManualThinking verifies that a positive ThinkingBudget
// produces an enabled thinking config with the exact token budget preserved.
func TestRequestToAnthropic_ManualThinking(t *testing.T) {
	request := ai.ChatRequest{
		GenerationConfig: &ai.GenerationConfig{
			ThinkingBudget: intPtr(5000),
		},
	}
	result, err := requestToAnthropic(request, Capabilities{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Thinking == nil {
		t.Fatal("Thinking: got nil, want enabled config")
	}
	if result.Thinking.Type != "enabled" {
		t.Errorf("Thinking.Type: got %q, want %q", result.Thinking.Type, "enabled")
	}
	if result.Thinking.BudgetTokens != 5000 {
		t.Errorf("Thinking.BudgetTokens: got %d, want 5000", result.Thinking.BudgetTokens)
	}
}

// TestRequestToAnthropic_ThinkingDisabled verifies that an explicit ThinkingBudget
// of zero suppresses thinking entirely (nil Thinking field on the wire).
func TestRequestToAnthropic_ThinkingDisabled(t *testing.T) {
	request := ai.ChatRequest{
		GenerationConfig: &ai.GenerationConfig{
			// Explicit zero means "disable thinking" even if IncludeThoughts were true.
			ThinkingBudget: intPtr(0),
		},
	}
	result, err := requestToAnthropic(request, Capabilities{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Thinking != nil {
		t.Errorf("Thinking: got %+v, want nil", result.Thinking)
	}
}

// TestRequestToAnthropic_Effort confirms that a non-empty Effort capability is
// forwarded as an OutputConfig on the wire request.
func TestRequestToAnthropic_Effort(t *testing.T) {
	result, err := requestToAnthropic(ai.ChatRequest{}, Capabilities{Effort: "high"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OutputConfig == nil {
		t.Fatal("OutputConfig: got nil, want {effort: high}")
	}
	if result.OutputConfig.Effort != "high" {
		t.Errorf("OutputConfig.Effort: got %q, want %q", result.OutputConfig.Effort, "high")
	}
}

// TestRequestToAnthropic_Speed confirms that a non-empty Speed capability is
// forwarded verbatim on the wire request.
func TestRequestToAnthropic_Speed(t *testing.T) {
	result, err := requestToAnthropic(ai.ChatRequest{}, Capabilities{Speed: "fast"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Speed != "fast" {
		t.Errorf("Speed: got %q, want %q", result.Speed, "fast")
	}
}

// ── buildThinkingConfig ───────────────────────────────────────────────────────

// TestBuildThinkingConfig covers all three branches: nil budget → adaptive,
// positive budget → enabled, zero budget → nil (disabled).
func TestBuildThinkingConfig(t *testing.T) {
	t.Run("nil → adaptive", func(t *testing.T) {
		cfg := buildThinkingConfig(nil)
		if cfg == nil {
			t.Fatal("got nil, want adaptive config")
		}
		if cfg.Type != "adaptive" {
			t.Errorf("Type: got %q, want %q", cfg.Type, "adaptive")
		}
	})

	t.Run("-1 → adaptive", func(t *testing.T) {
		cfg := buildThinkingConfig(intPtr(-1))
		if cfg == nil {
			t.Fatal("got nil, want adaptive config")
		}
		if cfg.Type != "adaptive" {
			t.Errorf("Type: got %q, want %q", cfg.Type, "adaptive")
		}
	})

	t.Run("positive → enabled with budget", func(t *testing.T) {
		cfg := buildThinkingConfig(intPtr(3000))
		if cfg == nil {
			t.Fatal("got nil, want enabled config")
		}
		if cfg.Type != "enabled" {
			t.Errorf("Type: got %q, want %q", cfg.Type, "enabled")
		}
		if cfg.BudgetTokens != 3000 {
			t.Errorf("BudgetTokens: got %d, want 3000", cfg.BudgetTokens)
		}
	})

	t.Run("zero → nil (disabled)", func(t *testing.T) {
		cfg := buildThinkingConfig(intPtr(0))
		if cfg != nil {
			t.Errorf("got %+v, want nil", cfg)
		}
	})
}

// ── buildMessages ─────────────────────────────────────────────────────────────

// TestBuildMessages_BasicRoles verifies that user and assistant messages are
// round-tripped with the correct role and a single text content block.
func TestBuildMessages_BasicRoles(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "hello"},
		{Role: ai.RoleAssistant, Content: "world"},
	}
	result := buildMessages(messages)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	userMsg := result[0]
	if userMsg.Role != "user" {
		t.Errorf("result[0].Role: got %q, want %q", userMsg.Role, "user")
	}
	if len(userMsg.Content) != 1 || userMsg.Content[0].Type != "text" {
		t.Errorf("result[0].Content: got %+v, want single text block", userMsg.Content)
	}
	if userMsg.Content[0].Text != "hello" {
		t.Errorf("result[0].Content[0].Text: got %q, want %q", userMsg.Content[0].Text, "hello")
	}

	assistantMsg := result[1]
	if assistantMsg.Role != "assistant" {
		t.Errorf("result[1].Role: got %q, want %q", assistantMsg.Role, "assistant")
	}
}

// TestBuildMessages_AssistantWithReasoning verifies that an assistant message
// carrying a Reasoning string produces a leading "thinking" block followed by
// the "text" block — the order required by the Anthropic API.
func TestBuildMessages_AssistantWithReasoning(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleAssistant, Reasoning: "thought", Content: "reply"},
	}
	result := buildMessages(messages)

	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	content := result[0].Content
	if len(content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(content))
	}
	if content[0].Type != "thinking" {
		t.Errorf("content[0].Type: got %q, want %q", content[0].Type, "thinking")
	}
	if content[0].Thinking != "thought" {
		t.Errorf("content[0].Thinking: got %q, want %q", content[0].Thinking, "thought")
	}
	if content[1].Type != "text" {
		t.Errorf("content[1].Type: got %q, want %q", content[1].Type, "text")
	}
	if content[1].Text != "reply" {
		t.Errorf("content[1].Text: got %q, want %q", content[1].Text, "reply")
	}
}

// TestBuildMessages_AssistantWithToolCalls verifies that assistant tool calls
// are converted to tool_use content blocks with the correct ID, name, and input.
func TestBuildMessages_AssistantWithToolCalls(t *testing.T) {
	messages := []ai.Message{
		{
			Role: ai.RoleAssistant,
			ToolCalls: []ai.ToolCall{
				{
					ID:   "call_abc",
					Type: "function",
					Function: ai.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"city":"Paris"}`,
					},
				},
			},
		},
	}
	result := buildMessages(messages)

	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	content := result[0].Content
	if len(content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(content))
	}
	block := content[0]
	if block.Type != "tool_use" {
		t.Errorf("block.Type: got %q, want %q", block.Type, "tool_use")
	}
	if block.ID != "call_abc" {
		t.Errorf("block.ID: got %q, want %q", block.ID, "call_abc")
	}
	if block.Name != "get_weather" {
		t.Errorf("block.Name: got %q, want %q", block.Name, "get_weather")
	}
	if string(block.Input) != `{"city":"Paris"}` {
		t.Errorf("block.Input: got %q, want %q", string(block.Input), `{"city":"Paris"}`)
	}
}

// TestBuildMessages_ToolResults_Merged verifies that two consecutive ai.RoleTool
// messages are collapsed into a single user message containing two tool_result
// blocks, which is the layout required by the Anthropic API.
func TestBuildMessages_ToolResults_Merged(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleTool, ToolCallID: "id1", Content: "result1"},
		{Role: ai.RoleTool, ToolCallID: "id2", Content: "result2"},
	}
	result := buildMessages(messages)

	if len(result) != 1 {
		t.Fatalf("expected 1 merged user message, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("Role: got %q, want %q", result[0].Role, "user")
	}
	if len(result[0].Content) != 2 {
		t.Fatalf("expected 2 tool_result blocks, got %d", len(result[0].Content))
	}
	if result[0].Content[0].Type != "tool_result" {
		t.Errorf("block[0].Type: got %q, want %q", result[0].Content[0].Type, "tool_result")
	}
	if result[0].Content[0].ToolUseID != "id1" {
		t.Errorf("block[0].ToolUseID: got %q, want %q", result[0].Content[0].ToolUseID, "id1")
	}
	if result[0].Content[1].Type != "tool_result" {
		t.Errorf("block[1].Type: got %q, want %q", result[0].Content[1].Type, "tool_result")
	}
	if result[0].Content[1].ToolUseID != "id2" {
		t.Errorf("block[1].ToolUseID: got %q, want %q", result[0].Content[1].ToolUseID, "id2")
	}
}

// TestBuildMessages_ToolResults_NotMerged verifies that tool results separated
// by a normal user turn are NOT merged: each becomes its own user message.
func TestBuildMessages_ToolResults_NotMerged(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleTool, ToolCallID: "id1", Content: "result1"},
		{Role: ai.RoleUser, Content: "a user message in between"},
		{Role: ai.RoleTool, ToolCallID: "id2", Content: "result2"},
	}
	result := buildMessages(messages)

	// We expect 3 separate user messages (tool_result, user-text, tool_result).
	if len(result) != 3 {
		t.Fatalf("expected 3 separate messages, got %d", len(result))
	}
	if result[0].Content[0].Type != "tool_result" {
		t.Errorf("result[0] should be tool_result, got %q", result[0].Content[0].Type)
	}
	if result[1].Content[0].Type != "text" {
		t.Errorf("result[1] should be text, got %q", result[1].Content[0].Type)
	}
	if result[2].Content[0].Type != "tool_result" {
		t.Errorf("result[2] should be tool_result, got %q", result[2].Content[0].Type)
	}
}

// TestBuildMessages_SystemInMessages verifies that system-role messages embedded
// in the messages slice are handled defensively as user messages, preventing
// silent data loss even though the caller should normally use SystemPrompt.
func TestBuildMessages_SystemInMessages(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleSystem, Content: "system instruction"},
	}
	result := buildMessages(messages)

	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("Role: got %q, want %q", result[0].Role, "user")
	}
	if len(result[0].Content) != 1 || result[0].Content[0].Text != "system instruction" {
		t.Errorf("Content: got %+v, want single text block with 'system instruction'", result[0].Content)
	}
}

// ── isAllToolResults ──────────────────────────────────────────────────────────

// TestIsAllToolResults exercises the helper predicate used to decide whether a
// previous message can absorb another tool result block.
func TestIsAllToolResults(t *testing.T) {
	toolResultMsg := anthropicMessage{
		Role: "user",
		Content: []anthropicContentBlock{
			{Type: "tool_result"},
			{Type: "tool_result"},
		},
	}
	if !isAllToolResults(toolResultMsg) {
		t.Error("expected true for all-tool_result message, got false")
	}

	mixedMsg := anthropicMessage{
		Role: "user",
		Content: []anthropicContentBlock{
			{Type: "tool_result"},
			{Type: "text"},
		},
	}
	if isAllToolResults(mixedMsg) {
		t.Error("expected false for mixed content, got true")
	}

	assistantMsg := anthropicMessage{
		Role:    "assistant",
		Content: []anthropicContentBlock{{Type: "tool_result"}},
	}
	if isAllToolResults(assistantMsg) {
		t.Error("expected false for assistant-role message, got true")
	}

	emptyMsg := anthropicMessage{Role: "user", Content: nil}
	if isAllToolResults(emptyMsg) {
		t.Error("expected false for empty content, got true")
	}
}

// ── contentPartsToAnthropicBlocks ─────────────────────────────────────────────

// TestContentPartsToAnthropicBlocks_Text verifies plain text parts produce a
// "text" block.
func TestContentPartsToAnthropicBlocks_Text(t *testing.T) {
	parts := []ai.ContentPart{
		{Type: ai.ContentTypeText, Text: "hello"},
	}
	blocks := contentPartsToAnthropicBlocks(parts)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != "text" {
		t.Errorf("Type: got %q, want %q", blocks[0].Type, "text")
	}
	if blocks[0].Text != "hello" {
		t.Errorf("Text: got %q, want %q", blocks[0].Text, "hello")
	}
}

// TestContentPartsToAnthropicBlocks_ImageBase64 verifies that an image with
// base64 data is converted to an "image" block with a base64 source.
func TestContentPartsToAnthropicBlocks_ImageBase64(t *testing.T) {
	parts := []ai.ContentPart{
		{
			Type: ai.ContentTypeImage,
			Image: &ai.ImageData{
				MimeType: "image/png",
				Data:     "abc123",
			},
		},
	}
	blocks := contentPartsToAnthropicBlocks(parts)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	block := blocks[0]
	if block.Type != "image" {
		t.Errorf("Type: got %q, want %q", block.Type, "image")
	}
	if block.Source == nil {
		t.Fatal("Source: got nil, want base64 source")
	}
	if block.Source.Type != "base64" {
		t.Errorf("Source.Type: got %q, want %q", block.Source.Type, "base64")
	}
	if block.Source.MediaType != "image/png" {
		t.Errorf("Source.MediaType: got %q, want %q", block.Source.MediaType, "image/png")
	}
	if block.Source.Data != "abc123" {
		t.Errorf("Source.Data: got %q, want %q", block.Source.Data, "abc123")
	}
}

// TestContentPartsToAnthropicBlocks_ImageURL verifies that an image with a URI
// is converted to an "image" block with a url source.
func TestContentPartsToAnthropicBlocks_ImageURL(t *testing.T) {
	parts := []ai.ContentPart{
		{
			Type: ai.ContentTypeImage,
			Image: &ai.ImageData{
				MimeType: "image/jpeg",
				URI:      "https://example.com/img.jpg",
			},
		},
	}
	blocks := contentPartsToAnthropicBlocks(parts)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	block := blocks[0]
	if block.Type != "image" {
		t.Errorf("Type: got %q, want %q", block.Type, "image")
	}
	if block.Source == nil {
		t.Fatal("Source: got nil, want url source")
	}
	if block.Source.Type != "url" {
		t.Errorf("Source.Type: got %q, want %q", block.Source.Type, "url")
	}
	if block.Source.URL != "https://example.com/img.jpg" {
		t.Errorf("Source.URL: got %q, want %q", block.Source.URL, "https://example.com/img.jpg")
	}
}

// TestContentPartsToAnthropicBlocks_Document verifies that document parts are
// converted to a "document" block with a base64 source.
func TestContentPartsToAnthropicBlocks_Document(t *testing.T) {
	parts := []ai.ContentPart{
		{
			Type: ai.ContentTypeDocument,
			Document: &ai.DocumentData{
				MimeType: "application/pdf",
				Data:     "pdfbase64",
			},
		},
	}
	blocks := contentPartsToAnthropicBlocks(parts)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	block := blocks[0]
	if block.Type != "document" {
		t.Errorf("Type: got %q, want %q", block.Type, "document")
	}
	if block.Source == nil {
		t.Fatal("Source: got nil, want base64 source")
	}
	if block.Source.Type != "base64" {
		t.Errorf("Source.Type: got %q, want %q", block.Source.Type, "base64")
	}
	if block.Source.MediaType != "application/pdf" {
		t.Errorf("Source.MediaType: got %q, want %q", block.Source.MediaType, "application/pdf")
	}
	if block.Source.Data != "pdfbase64" {
		t.Errorf("Source.Data: got %q, want %q", block.Source.Data, "pdfbase64")
	}
}

// ── buildAnthropicTools ───────────────────────────────────────────────────────

// TestBuildAnthropicTools_Basic verifies that a normal tool definition is
// forwarded with its name, description, and JSON-serialised input schema.
func TestBuildAnthropicTools_Basic(t *testing.T) {
	params := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"city": {Type: "string"},
		},
	}
	tools := []ai.ToolDescription{
		{Name: "get_weather", Description: "Get the weather", Parameters: params},
	}
	result := buildAnthropicTools(tools, false)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	tool := result[0]
	if tool.Name != "get_weather" {
		t.Errorf("Name: got %q, want %q", tool.Name, "get_weather")
	}
	if tool.Description != "Get the weather" {
		t.Errorf("Description: got %q, want %q", tool.Description, "Get the weather")
	}
	if len(tool.InputSchema) == 0 {
		t.Error("InputSchema: got empty, want serialised schema")
	}
	// Round-trip the schema to confirm it serialised correctly.
	var parsedSchema jsonschema.Schema
	if err := json.Unmarshal(tool.InputSchema, &parsedSchema); err != nil {
		t.Fatalf("InputSchema unmarshal error: %v", err)
	}
	if parsedSchema.Type != "object" {
		t.Errorf("InputSchema.Type: got %q, want %q", parsedSchema.Type, "object")
	}
}

// TestBuildAnthropicTools_SkipsBuiltIn confirms that Anthropic pseudo-tools
// (prefixed with "_") are excluded from the tool list sent to the API.
func TestBuildAnthropicTools_SkipsBuiltIn(t *testing.T) {
	tools := []ai.ToolDescription{
		{Name: "_google_search", Description: "Google Search"},
		{Name: "my_tool", Description: "My real tool"},
	}
	result := buildAnthropicTools(tools, false)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool (built-in skipped), got %d", len(result))
	}
	if result[0].Name != "my_tool" {
		t.Errorf("Name: got %q, want %q", result[0].Name, "my_tool")
	}
}

// TestBuildAnthropicTools_NoParams verifies that a tool without parameters still
// receives a valid empty-object schema so that the request remains well-formed.
func TestBuildAnthropicTools_NoParams(t *testing.T) {
	tools := []ai.ToolDescription{
		{Name: "no_params_tool", Description: "A tool with no params"},
	}
	result := buildAnthropicTools(tools, false)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	wantSchema := `{"type":"object","properties":{}}`
	if string(result[0].InputSchema) != wantSchema {
		t.Errorf("InputSchema: got %s, want %s", result[0].InputSchema, wantSchema)
	}
}

// TestBuildAnthropicTools_PromptCaching verifies that when prompt caching is
// enabled, only the last tool in the list receives a cache_control block, which
// is the pattern Anthropic recommends for caching long tool lists efficiently.
func TestBuildAnthropicTools_PromptCaching(t *testing.T) {
	tools := []ai.ToolDescription{
		{Name: "tool_one", Description: "First tool"},
		{Name: "tool_two", Description: "Second tool"},
	}
	result := buildAnthropicTools(tools, true)

	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}
	if result[0].CacheControl != nil {
		t.Errorf("tool_one.CacheControl: got %+v, want nil", result[0].CacheControl)
	}
	if result[1].CacheControl == nil {
		t.Fatal("tool_two.CacheControl: got nil, want ephemeral")
	}
	if result[1].CacheControl.Type != "ephemeral" {
		t.Errorf("tool_two.CacheControl.Type: got %q, want %q", result[1].CacheControl.Type, "ephemeral")
	}
}

// ── buildAnthropicToolChoice ──────────────────────────────────────────────────

// TestBuildAnthropicToolChoice_Forced verifies that a specific tool name forces
// the {type: "tool", name: ...} wire shape.
func TestBuildAnthropicToolChoice_Forced(t *testing.T) {
	tc := &ai.ToolChoice{ToolChoiceForced: "my_tool"}
	result := buildAnthropicToolChoice(tc)

	if result == nil {
		t.Fatal("got nil, want tool choice")
	}
	if result.Type != "tool" {
		t.Errorf("Type: got %q, want %q", result.Type, "tool")
	}
	if result.Name != "my_tool" {
		t.Errorf("Name: got %q, want %q", result.Name, "my_tool")
	}
}

// TestBuildAnthropicToolChoice_Any verifies that AtLeastOneRequired=true maps to
// the "any" Anthropic type.
func TestBuildAnthropicToolChoice_Any(t *testing.T) {
	tc := &ai.ToolChoice{AtLeastOneRequired: true}
	result := buildAnthropicToolChoice(tc)

	if result == nil {
		t.Fatal("got nil, want tool choice")
	}
	if result.Type != "any" {
		t.Errorf("Type: got %q, want %q", result.Type, "any")
	}
}

// TestBuildAnthropicToolChoice_Auto verifies that the reserved string "auto"
// maps to the {type: "auto"} wire shape rather than being treated as a tool name.
func TestBuildAnthropicToolChoice_Auto(t *testing.T) {
	tc := &ai.ToolChoice{ToolChoiceForced: "auto"}
	result := buildAnthropicToolChoice(tc)

	if result == nil {
		t.Fatal("got nil, want auto tool choice")
	}
	if result.Type != "auto" {
		t.Errorf("Type: got %q, want %q", result.Type, "auto")
	}
	if result.Name != "" {
		t.Errorf("Name: got %q, want empty (auto has no name)", result.Name)
	}
}

// TestBuildAnthropicToolChoice_Nil verifies that a nil ToolChoice produces a nil
// result, letting the API apply its default "auto" behavior.
func TestBuildAnthropicToolChoice_Nil(t *testing.T) {
	result := buildAnthropicToolChoice(nil)
	if result != nil {
		t.Errorf("got %+v, want nil", result)
	}
}

// ── anthropicToGeneric ────────────────────────────────────────────────────────

// TestAnthropicToGeneric_TextContent verifies that a single text content block
// in the response is surfaced as the Content field of ChatResponse.
func TestAnthropicToGeneric_TextContent(t *testing.T) {
	response := anthropicResponse{
		ID:    "msg_01",
		Model: "claude-opus-4-5",
		Content: []responseContentBlock{
			{Type: "text", Text: "Hello, world!"},
		},
		StopReason: "end_turn",
		Usage:      anthropicUsage{InputTokens: 10, OutputTokens: 5},
	}
	result := anthropicToGeneric(response)

	if result.Id != "msg_01" {
		t.Errorf("Id: got %q, want %q", result.Id, "msg_01")
	}
	if result.Content != "Hello, world!" {
		t.Errorf("Content: got %q, want %q", result.Content, "Hello, world!")
	}
	if result.Model != "claude-opus-4-5" {
		t.Errorf("Model: got %q, want %q", result.Model, "claude-opus-4-5")
	}
}

// TestAnthropicToGeneric_ThinkingBlocks verifies that "thinking" content blocks
// are joined and placed in the Reasoning field of ChatResponse.
func TestAnthropicToGeneric_ThinkingBlocks(t *testing.T) {
	response := anthropicResponse{
		Content: []responseContentBlock{
			{Type: "thinking", Thinking: "my reasoning"},
			{Type: "text", Text: "my answer"},
		},
		StopReason: "end_turn",
	}
	result := anthropicToGeneric(response)

	if result.Reasoning != "my reasoning" {
		t.Errorf("Reasoning: got %q, want %q", result.Reasoning, "my reasoning")
	}
	if result.Content != "my answer" {
		t.Errorf("Content: got %q, want %q", result.Content, "my answer")
	}
}

// TestAnthropicToGeneric_ToolUse verifies that "tool_use" content blocks are
// converted to ToolCalls with the correct ID, name, and JSON arguments string.
func TestAnthropicToGeneric_ToolUse(t *testing.T) {
	toolInput := json.RawMessage(`{"city":"Paris"}`)
	response := anthropicResponse{
		Content: []responseContentBlock{
			{
				Type:  "tool_use",
				ID:    "call_xyz",
				Name:  "get_weather",
				Input: toolInput,
			},
		},
		StopReason: "tool_use",
	}
	result := anthropicToGeneric(response)

	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	tc := result.ToolCalls[0]
	if tc.ID != "call_xyz" {
		t.Errorf("ToolCall.ID: got %q, want %q", tc.ID, "call_xyz")
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("ToolCall.Function.Name: got %q, want %q", tc.Function.Name, "get_weather")
	}
	if tc.Function.Arguments != `{"city":"Paris"}` {
		t.Errorf("ToolCall.Function.Arguments: got %q, want %q", tc.Function.Arguments, `{"city":"Paris"}`)
	}
	if tc.Type != "function" {
		t.Errorf("ToolCall.Type: got %q, want %q", tc.Type, "function")
	}
}

// TestAnthropicToGeneric_UnknownBlockType verifies that unrecognised content
// block types (like the redacted_thinking Anthropic uses for privacy-filtered
// thinking) are silently ignored without causing an error or empty response.
func TestAnthropicToGeneric_UnknownBlockType(t *testing.T) {
	response := anthropicResponse{
		Content: []responseContentBlock{
			{Type: "redacted_thinking", Thinking: "hidden"},
			{Type: "text", Text: "visible answer"},
		},
		StopReason: "end_turn",
	}
	// Should not panic or error — the function returns *ai.ChatResponse.
	result := anthropicToGeneric(response)

	if result == nil {
		t.Fatal("expected non-nil ChatResponse")
	}
	// The redacted block must be skipped; only the text block should be reflected.
	if result.Reasoning != "" {
		t.Errorf("Reasoning: got %q, want empty (redacted block must be skipped)", result.Reasoning)
	}
	if result.Content != "visible answer" {
		t.Errorf("Content: got %q, want %q", result.Content, "visible answer")
	}
}

// TestAnthropicToGeneric_StopReasonMapping is a table-driven test that covers
// every documented Anthropic stop_reason value plus the fallback for unknowns.
func TestAnthropicToGeneric_StopReasonMapping(t *testing.T) {
	cases := []struct {
		stopReason string
		wantFinish string
	}{
		{"end_turn", "stop"},
		{"stop_sequence", "stop"},
		{"tool_use", "tool_calls"},
		{"max_tokens", "length"},
		{"", "stop"},
		{"some_future_reason", "stop"},
	}

	for _, tc := range cases {
		tc := tc // capture loop variable for parallel sub-tests
		t.Run(tc.stopReason, func(t *testing.T) {
			response := anthropicResponse{StopReason: tc.stopReason}
			result := anthropicToGeneric(response)
			if result.FinishReason != tc.wantFinish {
				t.Errorf("FinishReason: got %q, want %q", result.FinishReason, tc.wantFinish)
			}
		})
	}
}

// TestAnthropicToGeneric_CacheTokens verifies that CacheCreationInputTokens and
// CacheReadInputTokens are summed into the CachedTokens field, allowing the cost
// layer to apply the discounted cache-read rate for both types of cache hits.
func TestAnthropicToGeneric_CacheTokens(t *testing.T) {
	response := anthropicResponse{
		Usage: anthropicUsage{
			InputTokens:              200,
			OutputTokens:             50,
			CacheCreationInputTokens: 100,
			CacheReadInputTokens:     50,
		},
	}
	result := anthropicToGeneric(response)

	if result.Usage == nil {
		t.Fatal("Usage: got nil, want populated")
	}
	if result.Usage.CachedTokens != 150 {
		t.Errorf("CachedTokens: got %d, want 150", result.Usage.CachedTokens)
	}
	if result.Usage.PromptTokens != 200 {
		t.Errorf("PromptTokens: got %d, want 200", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 50 {
		t.Errorf("CompletionTokens: got %d, want 50", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 250 {
		t.Errorf("TotalTokens: got %d, want 250", result.Usage.TotalTokens)
	}
}

// ── mapStopReason ─────────────────────────────────────────────────────────────

// TestMapStopReason is a focused unit test for the mapping helper, complementing
// the end-to-end coverage in TestAnthropicToGeneric_StopReasonMapping.
func TestMapStopReason(t *testing.T) {
	if got := mapStopReason("end_turn"); got != "stop" {
		t.Errorf("end_turn: got %q, want %q", got, "stop")
	}
	if got := mapStopReason("tool_use"); got != "tool_calls" {
		t.Errorf("tool_use: got %q, want %q", got, "tool_calls")
	}
	if got := mapStopReason("max_tokens"); got != "length" {
		t.Errorf("max_tokens: got %q, want %q", got, "length")
	}
}

// ── responseIDOrFallback ──────────────────────────────────────────────────────

// TestResponseIDOrFallback verifies that a non-empty ID is returned as-is, and
// that an empty ID produces a non-empty fallback string (used by the stream
// layer where early chunks may have no ID).
func TestResponseIDOrFallback(t *testing.T) {
	t.Run("non-empty ID is returned unchanged", func(t *testing.T) {
		if got := responseIDOrFallback("msg_123"); got != "msg_123" {
			t.Errorf("got %q, want %q", got, "msg_123")
		}
	})

	t.Run("empty ID produces non-empty fallback", func(t *testing.T) {
		fallback := responseIDOrFallback("")
		if fallback == "" {
			t.Error("got empty string, want non-empty fallback")
		}
	})
}
