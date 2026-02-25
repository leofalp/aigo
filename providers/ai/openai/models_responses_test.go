package openai

import (
	"testing"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
)

func TestRequestToResponses_SystemPrompt(t *testing.T) {
	req := ai.ChatRequest{
		SystemPrompt: "You are a helpful assistant.",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Hello"},
		},
	}

	respReq := requestToResponses(req)
	input, ok := respReq.Input.([]inputItem)
	if !ok {
		t.Fatalf("expected input to be []inputItem, got %T", respReq.Input)
	}

	if len(input) != 2 {
		t.Fatalf("expected 2 input items, got %d", len(input))
	}

	if input[0].Role != "developer" || input[0].Content != "You are a helpful assistant." {
		t.Errorf("expected developer role with system prompt, got %v", input[0])
	}
}

func TestRequestToResponses_ContentParts(t *testing.T) {
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

	respReq := requestToResponses(req)
	input, ok := respReq.Input.([]inputItem)
	if !ok {
		t.Fatalf("expected input to be []inputItem, got %T", respReq.Input)
	}

	if len(input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(input))
	}

	parts, ok := input[0].Content.([]inputContentPart)
	if !ok {
		t.Fatalf("expected content to be []inputContentPart, got %T", input[0].Content)
	}

	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}

	if parts[0].Type != "input_text" || parts[0].Text != "Look at this image" {
		t.Errorf("unexpected part 0: %v", parts[0])
	}
	if parts[1].Type != "input_image" || parts[1].ImageURL != "https://example.com/image.jpg" {
		t.Errorf("unexpected part 1: %v", parts[1])
	}
	if parts[2].Type != "input_image" || parts[2].ImageURL != "data:image/png;base64,base64data" {
		t.Errorf("unexpected part 2: %v", parts[2])
	}
	if parts[3].Type != "input_audio" || parts[3].InputAudio.Data != "audiodata" || parts[3].InputAudio.Format != "mp3" {
		t.Errorf("unexpected part 3: %v", parts[3])
	}
}

func TestRequestToResponses_GenerationConfig(t *testing.T) {
	req := ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		GenerationConfig: &ai.GenerationConfig{
			Temperature:     0.7,
			TopP:            0.9,
			MaxOutputTokens: 100,
		},
	}

	respReq := requestToResponses(req)
	if respReq.Temperature == nil || float32(*respReq.Temperature) != 0.7 {
		t.Errorf("expected temperature 0.7, got %v", *respReq.Temperature)
	}
	if respReq.TopP == nil || float32(*respReq.TopP) != 0.9 {
		t.Errorf("expected top_p 0.9, got %v", *respReq.TopP)
	}
	if respReq.MaxOutputTokens == nil || *respReq.MaxOutputTokens != 100 {
		t.Errorf("expected max_output_tokens 100, got %v", *respReq.MaxOutputTokens)
	}

	// Test fallback to MaxTokens
	req2 := ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		GenerationConfig: &ai.GenerationConfig{
			MaxTokens: 200,
		},
	}
	respReq2 := requestToResponses(req2)
	if respReq2.MaxOutputTokens == nil || *respReq2.MaxOutputTokens != 200 {
		t.Errorf("expected max_output_tokens 200, got %v", *respReq2.MaxOutputTokens)
	}
}

func TestRequestToResponses_ToolsAndToolChoice(t *testing.T) {
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
			ToolChoiceForced: "auto",
		},
	}

	respReq := requestToResponses(req)
	if len(respReq.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(respReq.Tools))
	}
	if respReq.Tools[0].Name != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got %s", respReq.Tools[0].Name)
	}
	if respReq.ToolChoice != "auto" {
		t.Errorf("expected tool_choice 'auto', got %v", respReq.ToolChoice)
	}

	// Test AtLeastOneRequired
	req.ToolChoice = &ai.ToolChoice{AtLeastOneRequired: true}
	respReq = requestToResponses(req)
	if respReq.ToolChoice != "required" {
		t.Errorf("expected tool_choice 'required', got %v", respReq.ToolChoice)
	}

	// Test RequiredTools (single)
	req.ToolChoice = &ai.ToolChoice{
		RequiredTools: []*ai.ToolDescription{{Name: "test_tool"}},
	}
	respReq = requestToResponses(req)
	tcMap, ok := respReq.ToolChoice.(map[string]any)
	if !ok || tcMap["name"] != "test_tool" {
		t.Errorf("expected tool_choice map with name 'test_tool', got %v", respReq.ToolChoice)
	}

	// Test RequiredTools (multiple)
	req.ToolChoice = &ai.ToolChoice{
		RequiredTools: []*ai.ToolDescription{{Name: "tool1"}, {Name: "tool2"}},
	}
	respReq = requestToResponses(req)
	tcArr, ok := respReq.ToolChoice.([]map[string]any)
	if !ok || len(tcArr) != 2 || tcArr[0]["name"] != "tool1" {
		t.Errorf("expected tool_choice array, got %v", respReq.ToolChoice)
	}
}

func TestRequestToResponses_ResponseFormat(t *testing.T) {
	// Test OutputSchema
	req := ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ResponseFormat: &ai.ResponseFormat{
			OutputSchema: &jsonschema.Schema{Type: "object"},
			Strict:       true,
		},
	}

	respReq := requestToResponses(req)
	if respReq.ResponseFormat == nil || respReq.ResponseFormat.Type != "json_schema" {
		t.Fatalf("expected json_schema response format, got %v", respReq.ResponseFormat)
	}
	if respReq.ResponseFormat.JsonSchema.Name != "response_schema" || !respReq.ResponseFormat.JsonSchema.Strict {
		t.Errorf("unexpected json_schema details: %v", respReq.ResponseFormat.JsonSchema)
	}

	// Test Type hint
	req2 := ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ResponseFormat: &ai.ResponseFormat{
			Type: "text",
		},
	}
	respReq2 := requestToResponses(req2)
	if respReq2.ResponseFormat == nil || respReq2.ResponseFormat.Type != "text" {
		t.Errorf("expected text response format, got %v", respReq2.ResponseFormat)
	}

	// Test Type hint json_schema fallback
	req3 := ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ResponseFormat: &ai.ResponseFormat{
			Type: "json_schema",
		},
	}
	respReq3 := requestToResponses(req3)
	if respReq3.ResponseFormat == nil || respReq3.ResponseFormat.Type != "json_object" {
		t.Errorf("expected json_object response format fallback, got %v", respReq3.ResponseFormat)
	}
}

func TestResponsesToGeneric(t *testing.T) {
	resp := responseCreateResponse{
		ID:        "resp_123",
		Model:     "gpt-4",
		Object:    "response",
		CreatedAt: 1234567890,
		Status:    "completed",
		Output: []outputItem{
			{
				Type: "message",
				Content: []contentOutput{
					{Type: "output_text", Text: "Hello"},
					{Type: "output_text", Text: "World"},
				},
			},
			{
				Type:      "function_call",
				Name:      "get_weather",
				Arguments: `{"location":"Paris"}`,
			},
			{
				Type: "reasoning", // Should be ignored
			},
			{
				Type: "web_search_call", // Should be ignored
			},
		},
		Usage: &usageDetails{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	}

	chatResp := responsesToGeneric(resp)
	if chatResp.Id != "resp_123" || chatResp.Model != "gpt-4" || chatResp.Object != "response" || chatResp.Created != 1234567890 {
		t.Errorf("unexpected basic fields: %v", chatResp)
	}

	if chatResp.Content != "Hello\nWorld" {
		t.Errorf("expected 'Hello\\nWorld', got %q", chatResp.Content)
	}

	if len(chatResp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(chatResp.ToolCalls))
	}
	if chatResp.ToolCalls[0].Function.Name != "get_weather" || chatResp.ToolCalls[0].Function.Arguments != `{"location":"Paris"}` {
		t.Errorf("unexpected tool call: %v", chatResp.ToolCalls[0])
	}

	if chatResp.Usage == nil || chatResp.Usage.TotalTokens != 30 {
		t.Errorf("unexpected usage: %v", chatResp.Usage)
	}

	if chatResp.FinishReason != "tool_calls" {
		t.Errorf("expected finish reason 'tool_calls', got %q", chatResp.FinishReason)
	}

	// Test other statuses
	resp.Status = "failed"
	chatResp = responsesToGeneric(resp)
	if chatResp.FinishReason != "error" {
		t.Errorf("expected finish reason 'error', got %q", chatResp.FinishReason)
	}

	resp.Status = "cancelled"
	chatResp = responsesToGeneric(resp)
	if chatResp.FinishReason != "cancelled" {
		t.Errorf("expected finish reason 'cancelled', got %q", chatResp.FinishReason)
	}

	resp.Status = "in_progress"
	chatResp = responsesToGeneric(resp)
	if chatResp.FinishReason != "in_progress" {
		t.Errorf("expected finish reason 'in_progress', got %q", chatResp.FinishReason)
	}
}
