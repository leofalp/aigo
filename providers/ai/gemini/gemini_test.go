package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
)

func TestNew(t *testing.T) {
	provider := New()
	if provider == nil {
		t.Fatal("New() returned nil")
	}
	if provider.baseURL != defaultBaseURL {
		t.Errorf("expected baseURL %q, got %q", defaultBaseURL, provider.baseURL)
	}
}

func TestWithAPIKey(t *testing.T) {
	provider := New().WithAPIKey("test-key").(*GeminiProvider)
	if provider.apiKey != "test-key" {
		t.Errorf("expected apiKey %q, got %q", "test-key", provider.apiKey)
	}
}

func TestWithBaseURL(t *testing.T) {
	provider := New().WithBaseURL("https://custom.api.com").(*GeminiProvider)
	if provider.baseURL != "https://custom.api.com" {
		t.Errorf("expected baseURL %q, got %q", "https://custom.api.com", provider.baseURL)
	}
}

func TestSendMessage_Basic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify Gemini auth header
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("missing or incorrect x-goog-api-key header: %s", r.Header.Get("x-goog-api-key"))
		}

		// Verify no Bearer auth
		if r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
		}

		// Parse request
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify request structure
		if len(req.Contents) == 0 {
			t.Error("expected contents in request")
		}

		// Return mock response
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role:  "model",
					Parts: []part{{Text: "Hello! How can I help you?"}},
				},
				FinishReason: "STOP",
			}},
			UsageMetadata: &usageMetadata{
				PromptTokenCount:     10,
				CandidatesTokenCount: 8,
				TotalTokenCount:      18,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.0-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if response.Content != "Hello! How can I help you?" {
		t.Errorf("unexpected content: %s", response.Content)
	}

	if response.FinishReason != "stop" {
		t.Errorf("expected finish_reason 'stop', got %q", response.FinishReason)
	}

	if response.Usage == nil {
		t.Fatal("expected usage in response")
	}

	if response.Usage.TotalTokens != 18 {
		t.Errorf("expected 18 total tokens, got %d", response.Usage.TotalTokens)
	}
}

func TestSendMessage_WithGoogleSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify Google Search tool is present
		hasGoogleSearch := false
		for _, tool := range req.Tools {
			if tool.GoogleSearch != nil {
				hasGoogleSearch = true
			}
		}
		if !hasGoogleSearch {
			t.Error("expected google_search in tools")
		}

		// Return mock response with grounding
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role:  "model",
					Parts: []part{{Text: "Based on my search, the answer is..."}},
				},
				FinishReason: "STOP",
				GroundingMetadata: &groundingMetadata{
					WebSearchQueries: []string{"test query"},
				},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.0-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Search for something"}},
		Tools: []ai.ToolDescription{
			{Name: ai.ToolGoogleSearch},
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if response.Content == "" {
		t.Error("expected content in response")
	}
}

func TestSendMessage_WithFunctionCalling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify function declarations are present
		hasFunctions := false
		for _, tool := range req.Tools {
			if len(tool.FunctionDeclarations) > 0 {
				hasFunctions = true
				if tool.FunctionDeclarations[0].Name != "get_weather" {
					t.Errorf("expected function name 'get_weather', got %q", tool.FunctionDeclarations[0].Name)
				}
			}
		}
		if !hasFunctions {
			t.Error("expected functionDeclarations in tools")
		}

		// Return mock response with function call
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role: "model",
					Parts: []part{{
						FunctionCall: &functionCall{
							Name: "get_weather",
							Args: json.RawMessage(`{"location": "New York"}`),
						},
					}},
				},
				FinishReason: "STOP",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"location": {Type: "string"},
		},
	}

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.0-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "What's the weather in New York?"}},
		Tools: []ai.ToolDescription{
			{Name: "get_weather", Description: "Get weather for a location", Parameters: schema},
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if len(response.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(response.ToolCalls))
	}

	if response.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected function name 'get_weather', got %q", response.ToolCalls[0].Function.Name)
	}

	if response.FinishReason != "tool_calls" {
		t.Errorf("expected finish_reason 'tool_calls', got %q", response.FinishReason)
	}
}

func TestSendMessage_WithThinkingConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify thinking config is present
		if req.GenerationConfig == nil || req.GenerationConfig.ThinkingConfig == nil {
			t.Error("expected thinkingConfig in generationConfig")
		} else {
			if req.GenerationConfig.ThinkingConfig.ThinkingBudget == nil || *req.GenerationConfig.ThinkingConfig.ThinkingBudget != 1024 {
				t.Errorf("expected thinkingBudget 1024, got %v", req.GenerationConfig.ThinkingConfig.ThinkingBudget)
			}
			if !req.GenerationConfig.ThinkingConfig.IncludeThoughts {
				t.Error("expected includeThoughts to be true")
			}
		}

		// Return mock response
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role:  "model",
					Parts: []part{{Text: "The answer is 42."}},
				},
				FinishReason: "STOP",
			}},
			UsageMetadata: &usageMetadata{
				PromptTokenCount:     10,
				CandidatesTokenCount: 20,
				TotalTokenCount:      30,
				ThoughtsTokenCount:   100,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	budget := 1024
	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.5-pro",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Think about this problem"}},
		GenerationConfig: &ai.GenerationConfig{
			ThinkingBudget:  &budget,
			IncludeThoughts: true,
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if response.Usage == nil {
		t.Fatal("expected usage in response")
	}

	if response.Usage.ReasoningTokens != 100 {
		t.Errorf("expected 100 reasoning tokens, got %d", response.Usage.ReasoningTokens)
	}
}

func TestSendMessage_WithThinkingContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return mock response with both thought and content parts
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role: "model",
					Parts: []part{
						{Text: "Let me think about this problem...", Thought: true},
						{Text: "After considering the options...", Thought: true},
						{Text: "The answer is 42."},
					},
				},
				FinishReason: "STOP",
			}},
			UsageMetadata: &usageMetadata{
				PromptTokenCount:     10,
				CandidatesTokenCount: 30,
				TotalTokenCount:      40,
				ThoughtsTokenCount:   20,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	budget := 1024
	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.5-pro",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "What is the meaning of life?"}},
		GenerationConfig: &ai.GenerationConfig{
			ThinkingBudget:  &budget,
			IncludeThoughts: true,
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify content contains only non-thought parts
	if response.Content != "The answer is 42." {
		t.Errorf("expected content 'The answer is 42.', got %q", response.Content)
	}

	// Verify reasoning contains thought parts joined by newline
	expectedReasoning := "Let me think about this problem...\nAfter considering the options..."
	if response.Reasoning != expectedReasoning {
		t.Errorf("expected reasoning %q, got %q", expectedReasoning, response.Reasoning)
	}

	// Verify usage metadata
	if response.Usage == nil {
		t.Fatal("expected usage in response")
	}
	if response.Usage.ReasoningTokens != 20 {
		t.Errorf("expected 20 reasoning tokens, got %d", response.Usage.ReasoningTokens)
	}
}

func TestGeminiToGeneric_ThoughtSeparation(t *testing.T) {
	tests := []struct {
		name              string
		parts             []part
		expectedContent   string
		expectedReasoning string
	}{
		{
			name: "only content",
			parts: []part{
				{Text: "Hello world"},
			},
			expectedContent:   "Hello world",
			expectedReasoning: "",
		},
		{
			name: "only thoughts",
			parts: []part{
				{Text: "Thinking...", Thought: true},
			},
			expectedContent:   "",
			expectedReasoning: "Thinking...",
		},
		{
			name: "mixed thoughts and content",
			parts: []part{
				{Text: "First thought", Thought: true},
				{Text: "Second thought", Thought: true},
				{Text: "Actual answer"},
			},
			expectedContent:   "Actual answer",
			expectedReasoning: "First thought\nSecond thought",
		},
		{
			name: "multiple content parts",
			parts: []part{
				{Text: "Part 1"},
				{Text: "Part 2"},
			},
			expectedContent:   "Part 1\nPart 2",
			expectedReasoning: "",
		},
		{
			name: "interleaved thoughts and content",
			parts: []part{
				{Text: "Thought 1", Thought: true},
				{Text: "Content 1"},
				{Text: "Thought 2", Thought: true},
				{Text: "Content 2"},
			},
			expectedContent:   "Content 1\nContent 2",
			expectedReasoning: "Thought 1\nThought 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := generateContentResponse{
				Candidates: []candidate{{
					Content: &content{
						Role:  "model",
						Parts: tt.parts,
					},
					FinishReason: "STOP",
				}},
			}

			result := geminiToGeneric(resp)

			if result.Content != tt.expectedContent {
				t.Errorf("expected content %q, got %q", tt.expectedContent, result.Content)
			}
			if result.Reasoning != tt.expectedReasoning {
				t.Errorf("expected reasoning %q, got %q", tt.expectedReasoning, result.Reasoning)
			}
		})
	}
}

func TestSendMessage_WithStructuredOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify response format
		if req.GenerationConfig == nil {
			t.Error("expected generationConfig")
		} else {
			if req.GenerationConfig.ResponseMimeType != "application/json" {
				t.Errorf("expected responseMimeType 'application/json', got %q", req.GenerationConfig.ResponseMimeType)
			}
			if req.GenerationConfig.ResponseSchema == nil {
				t.Error("expected responseSchema")
			}
		}

		// Return mock JSON response
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role:  "model",
					Parts: []part{{Text: `{"name": "John", "age": 30}`}},
				},
				FinishReason: "STOP",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.0-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Give me info about John"}},
		ResponseFormat: &ai.ResponseFormat{
			OutputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {Type: "string"},
					"age":  {Type: "integer"},
				},
			},
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if response.Content != `{"name": "John", "age": 30}` {
		t.Errorf("unexpected content: %s", response.Content)
	}
}

func TestSendMessage_WithSafetySettings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify safety settings
		if len(req.SafetySettings) != 1 {
			t.Errorf("expected 1 safety setting, got %d", len(req.SafetySettings))
		} else {
			if req.SafetySettings[0].Category != ai.HarmCategoryDangerousContent {
				t.Errorf("expected category %q, got %q", ai.HarmCategoryDangerousContent, req.SafetySettings[0].Category)
			}
			if req.SafetySettings[0].Threshold != ai.BlockNone {
				t.Errorf("expected threshold %q, got %q", ai.BlockNone, req.SafetySettings[0].Threshold)
			}
		}

		// Return mock response
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role:  "model",
					Parts: []part{{Text: "Response with safety settings"}},
				},
				FinishReason: "STOP",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.0-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Test safety"}},
		GenerationConfig: &ai.GenerationConfig{
			SafetySettings: []ai.SafetySetting{
				{Category: ai.HarmCategoryDangerousContent, Threshold: ai.BlockNone},
			},
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if response.Content == "" {
		t.Error("expected content in response")
	}
}

func TestSendMessage_WithSystemPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify system instruction
		if req.SystemInstruction == nil {
			t.Error("expected systemInstruction")
		} else if len(req.SystemInstruction.Parts) == 0 {
			t.Error("expected parts in systemInstruction")
		} else if req.SystemInstruction.Parts[0].Text != "You are a helpful assistant." {
			t.Errorf("unexpected system instruction: %s", req.SystemInstruction.Parts[0].Text)
		}

		// Return mock response
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role:  "model",
					Parts: []part{{Text: "I am a helpful assistant!"}},
				},
				FinishReason: "STOP",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:        "gemini-2.0-flash",
		SystemPrompt: "You are a helpful assistant.",
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if response.Content == "" {
		t.Error("expected content in response")
	}
}

func TestSendMessage_MissingAPIKey(t *testing.T) {
	provider := New().WithBaseURL("https://example.com").(*GeminiProvider)
	// API key is not set

	_, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hello"}},
	})

	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestSendMessage_ContentBlocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response with blocked content
		resp := generateContentResponse{
			Candidates: []candidate{},
			PromptFeedback: &promptFeedback{
				BlockReason: "SAFETY",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.0-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Blocked content"}},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if response.FinishReason != "content_filter" {
		t.Errorf("expected finish_reason 'content_filter', got %q", response.FinishReason)
	}

	if response.Refusal != "SAFETY" {
		t.Errorf("expected refusal 'SAFETY', got %q", response.Refusal)
	}
}

func TestIsStopMessage(t *testing.T) {
	provider := New()

	tests := []struct {
		name     string
		message  *ai.ChatResponse
		expected bool
	}{
		{
			name:     "nil message",
			message:  nil,
			expected: true,
		},
		{
			name: "stop finish reason",
			message: &ai.ChatResponse{
				Content:      "Hello",
				FinishReason: "stop",
			},
			expected: true,
		},
		{
			name: "length finish reason",
			message: &ai.ChatResponse{
				Content:      "Hello",
				FinishReason: "length",
			},
			expected: true,
		},
		{
			name: "content_filter finish reason",
			message: &ai.ChatResponse{
				FinishReason: "content_filter",
			},
			expected: true,
		},
		{
			name: "with tool calls",
			message: &ai.ChatResponse{
				Content:      "",
				FinishReason: "stop",
				ToolCalls: []ai.ToolCall{
					{ID: "1", Type: "function", Function: ai.ToolCallFunction{Name: "test"}},
				},
			},
			expected: false,
		},
		{
			name: "empty content no tool calls",
			message: &ai.ChatResponse{
				Content: "",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.IsStopMessage(tt.message)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBuildContents_ConversationHistory(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "Hello"},
		{Role: ai.RoleAssistant, Content: "Hi there!"},
		{Role: ai.RoleUser, Content: "How are you?"},
		{Role: ai.RoleAssistant, Content: "I'm doing well!"},
	}

	contents := buildContents(messages)

	if len(contents) != 4 {
		t.Fatalf("expected 4 contents, got %d", len(contents))
	}

	// Verify roles are mapped correctly
	expectedRoles := []string{"user", "model", "user", "model"}
	for i, c := range contents {
		if c.Role != expectedRoles[i] {
			t.Errorf("content[%d]: expected role %q, got %q", i, expectedRoles[i], c.Role)
		}
	}
}

func TestBuildContents_ToolResponse(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "What's the weather?"},
		{
			Role: ai.RoleAssistant,
			ToolCalls: []ai.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: ai.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location": "NYC"}`,
					},
				},
			},
		},
		{
			Role:       ai.RoleTool,
			Name:       "get_weather",
			ToolCallID: "call_1",
			Content:    `{"temperature": 72}`,
		},
	}

	contents := buildContents(messages)

	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}

	// Verify tool call in assistant message
	if contents[1].Role != "model" {
		t.Errorf("expected role 'model', got %q", contents[1].Role)
	}
	if len(contents[1].Parts) == 0 || contents[1].Parts[0].FunctionCall == nil {
		t.Error("expected function call in assistant message")
	}

	// Verify tool response
	if contents[2].Role != "user" {
		t.Errorf("expected role 'user' for tool response, got %q", contents[2].Role)
	}
	if len(contents[2].Parts) == 0 || contents[2].Parts[0].FunctionResponse == nil {
		t.Error("expected function response in tool message")
	}
	if contents[2].Parts[0].FunctionResponse.Name != "get_weather" {
		t.Errorf("expected function name 'get_weather', got %q", contents[2].Parts[0].FunctionResponse.Name)
	}
}

func TestBuildTools_MixedTools(t *testing.T) {
	aiTools := []ai.ToolDescription{
		{Name: ai.ToolGoogleSearch},
		{Name: ai.ToolCodeExecution},
		{Name: "custom_function", Description: "A custom function", Parameters: &jsonschema.Schema{Type: "object"}},
	}

	tools := buildTools(aiTools)

	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	// Verify built-in tools
	hasGoogleSearch := false
	hasCodeExecution := false
	hasFunctions := false

	for _, tool := range tools {
		if tool.GoogleSearch != nil {
			hasGoogleSearch = true
		}
		if tool.CodeExecution != nil {
			hasCodeExecution = true
		}
		if len(tool.FunctionDeclarations) > 0 {
			hasFunctions = true
			if tool.FunctionDeclarations[0].Name != "custom_function" {
				t.Errorf("expected function name 'custom_function', got %q", tool.FunctionDeclarations[0].Name)
			}
		}
	}

	if !hasGoogleSearch {
		t.Error("expected google_search tool")
	}
	if !hasCodeExecution {
		t.Error("expected code_execution tool")
	}
	if !hasFunctions {
		t.Error("expected function declarations")
	}
}

func TestMapFinishReason(t *testing.T) {
	tests := []struct {
		gemini   string
		expected string
	}{
		{"STOP", "stop"},
		{"MAX_TOKENS", "length"},
		{"SAFETY", "content_filter"},
		{"RECITATION", "content_filter"},
		{"OTHER", "stop"},
		{"UNKNOWN", "stop"},
	}

	for _, tt := range tests {
		t.Run(tt.gemini, func(t *testing.T) {
			result := mapFinishReason(tt.gemini)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMapGroundingMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    *groundingMetadata
		validate func(*testing.T, *ai.GroundingMetadata)
	}{
		{
			name:  "nil input",
			input: nil,
			validate: func(t *testing.T, result *ai.GroundingMetadata) {
				if result != nil {
					t.Error("expected nil result for nil input")
				}
			},
		},
		{
			name: "with search queries only",
			input: &groundingMetadata{
				WebSearchQueries: []string{"query1", "query2"},
			},
			validate: func(t *testing.T, result *ai.GroundingMetadata) {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if len(result.SearchQueries) != 2 {
					t.Errorf("expected 2 search queries, got %d", len(result.SearchQueries))
				}
				if result.SearchQueries[0] != "query1" {
					t.Errorf("expected 'query1', got %q", result.SearchQueries[0])
				}
			},
		},
		{
			name: "with grounding chunks",
			input: &groundingMetadata{
				GroundingChunks: []groundingChunk{
					{Web: &webChunk{URI: "https://example.com/1", Title: "Source 1"}},
					{Web: &webChunk{URI: "https://example.com/2", Title: "Source 2"}},
				},
			},
			validate: func(t *testing.T, result *ai.GroundingMetadata) {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if len(result.Sources) != 2 {
					t.Fatalf("expected 2 sources, got %d", len(result.Sources))
				}
				if result.Sources[0].Index != 0 {
					t.Errorf("expected index 0, got %d", result.Sources[0].Index)
				}
				if result.Sources[0].URI != "https://example.com/1" {
					t.Errorf("expected URI 'https://example.com/1', got %q", result.Sources[0].URI)
				}
				if result.Sources[0].Title != "Source 1" {
					t.Errorf("expected title 'Source 1', got %q", result.Sources[0].Title)
				}
				if result.Sources[1].Index != 1 {
					t.Errorf("expected index 1, got %d", result.Sources[1].Index)
				}
			},
		},
		{
			name: "with grounding supports",
			input: &groundingMetadata{
				GroundingSupports: []groundingSupport{
					{
						Segment:               &segment{StartIndex: 0, EndIndex: 50, Text: "Some cited text"},
						GroundingChunkIndices: []int{0, 1},
						ConfidenceScores:      []float64{0.9, 0.85},
					},
				},
			},
			validate: func(t *testing.T, result *ai.GroundingMetadata) {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if len(result.Citations) != 1 {
					t.Fatalf("expected 1 citation, got %d", len(result.Citations))
				}
				cit := result.Citations[0]
				if cit.Text != "Some cited text" {
					t.Errorf("expected text 'Some cited text', got %q", cit.Text)
				}
				if cit.StartIndex != 0 {
					t.Errorf("expected start index 0, got %d", cit.StartIndex)
				}
				if cit.EndIndex != 50 {
					t.Errorf("expected end index 50, got %d", cit.EndIndex)
				}
				if len(cit.SourceIndices) != 2 || cit.SourceIndices[0] != 0 || cit.SourceIndices[1] != 1 {
					t.Errorf("expected source indices [0, 1], got %v", cit.SourceIndices)
				}
				if len(cit.Confidence) != 2 || cit.Confidence[0] != 0.9 {
					t.Errorf("expected confidence [0.9, 0.85], got %v", cit.Confidence)
				}
			},
		},
		{
			name: "full grounding metadata",
			input: &groundingMetadata{
				WebSearchQueries: []string{"company info"},
				GroundingChunks: []groundingChunk{
					{Web: &webChunk{URI: "https://company.com", Title: "Company Website"}},
					{Web: &webChunk{URI: "https://linkedin.com/company", Title: "LinkedIn"}},
				},
				GroundingSupports: []groundingSupport{
					{
						Segment:               &segment{StartIndex: 0, EndIndex: 100, Text: "Company is a leader..."},
						GroundingChunkIndices: []int{0},
						ConfidenceScores:      []float64{0.95},
					},
					{
						Segment:               &segment{StartIndex: 100, EndIndex: 200, Text: "Founded in 2010..."},
						GroundingChunkIndices: []int{0, 1},
						ConfidenceScores:      []float64{0.88, 0.82},
					},
				},
			},
			validate: func(t *testing.T, result *ai.GroundingMetadata) {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				if len(result.SearchQueries) != 1 {
					t.Errorf("expected 1 search query, got %d", len(result.SearchQueries))
				}
				if len(result.Sources) != 2 {
					t.Errorf("expected 2 sources, got %d", len(result.Sources))
				}
				if len(result.Citations) != 2 {
					t.Errorf("expected 2 citations, got %d", len(result.Citations))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapGroundingMetadata(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestGeminiToGeneric_WithGroundingMetadata(t *testing.T) {
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role:  "model",
				Parts: []part{{Text: "Based on my research, the company is a leader in innovation [1]."}},
			},
			FinishReason: "STOP",
			GroundingMetadata: &groundingMetadata{
				WebSearchQueries: []string{"company info query"},
				GroundingChunks: []groundingChunk{
					{Web: &webChunk{URI: "https://example.com/about", Title: "About Us"}},
				},
				GroundingSupports: []groundingSupport{
					{
						Segment:               &segment{StartIndex: 25, EndIndex: 63, Text: "the company is a leader in innovation"},
						GroundingChunkIndices: []int{0},
						ConfidenceScores:      []float64{0.92},
					},
				},
			},
		}},
		UsageMetadata: &usageMetadata{
			PromptTokenCount:     50,
			CandidatesTokenCount: 20,
			TotalTokenCount:      70,
		},
	}

	result := geminiToGeneric(resp)

	// Verify grounding is populated
	if result.Grounding == nil {
		t.Fatal("expected grounding metadata in response")
	}

	// Verify search queries
	if len(result.Grounding.SearchQueries) != 1 || result.Grounding.SearchQueries[0] != "company info query" {
		t.Errorf("expected search query 'company info query', got %v", result.Grounding.SearchQueries)
	}

	// Verify sources
	if len(result.Grounding.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(result.Grounding.Sources))
	}
	if result.Grounding.Sources[0].URI != "https://example.com/about" {
		t.Errorf("expected URI 'https://example.com/about', got %q", result.Grounding.Sources[0].URI)
	}

	// Verify citations
	if len(result.Grounding.Citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(result.Grounding.Citations))
	}
	if result.Grounding.Citations[0].Text != "the company is a leader in innovation" {
		t.Errorf("expected citation text, got %q", result.Grounding.Citations[0].Text)
	}
}

func TestBuildContents_WithImageInput(t *testing.T) {
	messages := []ai.Message{
		{
			Role: ai.RoleUser,
			ContentParts: []ai.ContentPart{
				ai.NewTextPart("What is in this image?"),
				ai.NewImagePart("image/png", "iVBORw0KGgoAAAANSUhEUg=="),
			},
		},
	}

	contents := buildContents(messages)

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	if contents[0].Role != "user" {
		t.Errorf("expected role 'user', got %q", contents[0].Role)
	}

	if len(contents[0].Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(contents[0].Parts))
	}

	// Verify text part
	if contents[0].Parts[0].Text != "What is in this image?" {
		t.Errorf("expected text 'What is in this image?', got %q", contents[0].Parts[0].Text)
	}

	// Verify inlineData part
	if contents[0].Parts[1].InlineData == nil {
		t.Fatal("expected inlineData in second part")
	}
	if contents[0].Parts[1].InlineData.MimeType != "image/png" {
		t.Errorf("expected mimeType 'image/png', got %q", contents[0].Parts[1].InlineData.MimeType)
	}
	if contents[0].Parts[1].InlineData.Data != "iVBORw0KGgoAAAANSUhEUg==" {
		t.Errorf("expected base64 data, got %q", contents[0].Parts[1].InlineData.Data)
	}
}

func TestBuildContents_BackwardCompatible(t *testing.T) {
	// Message with only Content (no ContentParts) should work the same as before
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "Hello, world!"},
		{Role: ai.RoleAssistant, Content: "Hi there!"},
	}

	contents := buildContents(messages)

	if len(contents) != 2 {
		t.Fatalf("expected 2 contents, got %d", len(contents))
	}

	// User message should have single text part
	if len(contents[0].Parts) != 1 {
		t.Fatalf("expected 1 part for user message, got %d", len(contents[0].Parts))
	}
	if contents[0].Parts[0].Text != "Hello, world!" {
		t.Errorf("expected text 'Hello, world!', got %q", contents[0].Parts[0].Text)
	}

	// Assistant message should have single text part
	if len(contents[1].Parts) != 1 {
		t.Fatalf("expected 1 part for assistant message, got %d", len(contents[1].Parts))
	}
	if contents[1].Parts[0].Text != "Hi there!" {
		t.Errorf("expected text 'Hi there!', got %q", contents[1].Parts[0].Text)
	}
}

func TestGeminiToGeneric_WithImageOutput(t *testing.T) {
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role: "model",
				Parts: []part{
					{Text: "Here is the generated image:"},
					{InlineData: &inlineData{MimeType: "image/png", Data: "iVBORw0KGgoAAAANSUhEUg=="}},
				},
			},
			FinishReason: "STOP",
		}},
	}

	result := geminiToGeneric(resp)

	if result.Content != "Here is the generated image:" {
		t.Errorf("expected text content, got %q", result.Content)
	}

	if len(result.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(result.Images))
	}

	if result.Images[0].MimeType != "image/png" {
		t.Errorf("expected mime type 'image/png', got %q", result.Images[0].MimeType)
	}

	if result.Images[0].Data != "iVBORw0KGgoAAAANSUhEUg==" {
		t.Errorf("expected base64 data, got %q", result.Images[0].Data)
	}
}

func TestGeminiToGeneric_ImageOnlyResponse(t *testing.T) {
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role: "model",
				Parts: []part{
					{InlineData: &inlineData{MimeType: "image/jpeg", Data: "/9j/4AAQSkZJRg=="}},
				},
			},
			FinishReason: "STOP",
		}},
	}

	result := geminiToGeneric(resp)

	// No text content expected
	if result.Content != "" {
		t.Errorf("expected empty content, got %q", result.Content)
	}

	// Image should be populated
	if len(result.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(result.Images))
	}

	if result.Images[0].MimeType != "image/jpeg" {
		t.Errorf("expected mime type 'image/jpeg', got %q", result.Images[0].MimeType)
	}
}

func TestBuildGenerationConfig_WithResponseModalities(t *testing.T) {
	cfg := &ai.GenerationConfig{
		ResponseModalities: []string{"TEXT", "IMAGE"},
	}

	gc := buildGenerationConfig(cfg, nil)

	if gc == nil {
		t.Fatal("expected non-nil generation config")
	}

	if len(gc.ResponseModalities) != 2 {
		t.Fatalf("expected 2 response modalities, got %d", len(gc.ResponseModalities))
	}

	if gc.ResponseModalities[0] != "TEXT" {
		t.Errorf("expected first modality 'TEXT', got %q", gc.ResponseModalities[0])
	}

	if gc.ResponseModalities[1] != "IMAGE" {
		t.Errorf("expected second modality 'IMAGE', got %q", gc.ResponseModalities[1])
	}
}

func TestSendMessage_WithImageInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify the request has multimodal content
		if len(req.Contents) != 1 {
			t.Fatalf("expected 1 content, got %d", len(req.Contents))
		}

		if len(req.Contents[0].Parts) != 2 {
			t.Fatalf("expected 2 parts (text + image), got %d", len(req.Contents[0].Parts))
		}

		// Verify text part
		if req.Contents[0].Parts[0].Text != "Describe this image" {
			t.Errorf("expected text 'Describe this image', got %q", req.Contents[0].Parts[0].Text)
		}

		// Verify inlineData part
		if req.Contents[0].Parts[1].InlineData == nil {
			t.Fatal("expected inlineData in second part")
		}
		if req.Contents[0].Parts[1].InlineData.MimeType != "image/png" {
			t.Errorf("expected mimeType 'image/png', got %q", req.Contents[0].Parts[1].InlineData.MimeType)
		}

		// Return response with text + image
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role: "model",
					Parts: []part{
						{Text: "This is a test image showing..."},
						{InlineData: &inlineData{MimeType: "image/png", Data: "generatedBase64=="}},
					},
				},
				FinishReason: "STOP",
			}},
			UsageMetadata: &usageMetadata{
				PromptTokenCount:     100,
				CandidatesTokenCount: 50,
				TotalTokenCount:      150,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model: "gemini-2.0-flash",
		Messages: []ai.Message{
			{
				Role: ai.RoleUser,
				ContentParts: []ai.ContentPart{
					ai.NewTextPart("Describe this image"),
					ai.NewImagePart("image/png", "testBase64Data=="),
				},
			},
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if response.Content != "This is a test image showing..." {
		t.Errorf("unexpected content: %s", response.Content)
	}

	if len(response.Images) != 1 {
		t.Fatalf("expected 1 image in response, got %d", len(response.Images))
	}

	if response.Images[0].MimeType != "image/png" {
		t.Errorf("expected image mime type 'image/png', got %q", response.Images[0].MimeType)
	}

	if response.Images[0].Data != "generatedBase64==" {
		t.Errorf("expected image data 'generatedBase64==', got %q", response.Images[0].Data)
	}
}

func TestIsStopMessage_WithImages(t *testing.T) {
	provider := New()

	// Image-only response (no text) should NOT be treated as stop
	imageOnlyResponse := &ai.ChatResponse{
		Content: "",
		Images: []ai.ImageData{
			{MimeType: "image/png", Data: "base64data=="},
		},
	}

	if provider.IsStopMessage(imageOnlyResponse) {
		t.Error("expected image-only response to NOT be a stop message")
	}

	// Response with both text and images should still respect finish reason
	textAndImageResponse := &ai.ChatResponse{
		Content:      "Here is the image",
		FinishReason: "stop",
		Images: []ai.ImageData{
			{MimeType: "image/png", Data: "base64data=="},
		},
	}

	if !provider.IsStopMessage(textAndImageResponse) {
		t.Error("expected response with stop finish reason to be a stop message")
	}

	// Empty response with no content and no images should be stop
	emptyResponse := &ai.ChatResponse{
		Content: "",
	}

	if !provider.IsStopMessage(emptyResponse) {
		t.Error("expected empty response to be a stop message")
	}
}

func TestBuildContents_WithAudioInput(t *testing.T) {
	messages := []ai.Message{
		{
			Role: ai.RoleUser,
			ContentParts: []ai.ContentPart{
				ai.NewTextPart("Transcribe this audio"),
				ai.NewAudioPart("audio/wav", "UklGRiQAAABXQVZFZm10=="),
			},
		},
	}

	contents := buildContents(messages)

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	if len(contents[0].Parts) != 2 {
		t.Fatalf("expected 2 parts (text + audio), got %d", len(contents[0].Parts))
	}

	// Verify text part
	if contents[0].Parts[0].Text != "Transcribe this audio" {
		t.Errorf("expected text 'Transcribe this audio', got %q", contents[0].Parts[0].Text)
	}

	// Verify audio inlineData part
	if contents[0].Parts[1].InlineData == nil {
		t.Fatal("expected inlineData in second part")
	}
	if contents[0].Parts[1].InlineData.MimeType != "audio/wav" {
		t.Errorf("expected mimeType 'audio/wav', got %q", contents[0].Parts[1].InlineData.MimeType)
	}
	if contents[0].Parts[1].InlineData.Data != "UklGRiQAAABXQVZFZm10==" {
		t.Errorf("expected base64 data, got %q", contents[0].Parts[1].InlineData.Data)
	}
}

func TestBuildContents_WithVideoInputURI(t *testing.T) {
	messages := []ai.Message{
		{
			Role: ai.RoleUser,
			ContentParts: []ai.ContentPart{
				ai.NewTextPart("Describe this video"),
				ai.NewVideoPartFromURI("video/mp4", "gs://bucket/video.mp4"),
			},
		},
	}

	contents := buildContents(messages)

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	if len(contents[0].Parts) != 2 {
		t.Fatalf("expected 2 parts (text + video), got %d", len(contents[0].Parts))
	}

	// Verify text part
	if contents[0].Parts[0].Text != "Describe this video" {
		t.Errorf("expected text 'Describe this video', got %q", contents[0].Parts[0].Text)
	}

	// URI-based video should use FileData, not InlineData
	if contents[0].Parts[1].FileData == nil {
		t.Fatal("expected fileData in second part for URI-based video")
	}
	if contents[0].Parts[1].InlineData != nil {
		t.Error("expected no inlineData when URI is used")
	}
	if contents[0].Parts[1].FileData.MimeType != "video/mp4" {
		t.Errorf("expected mimeType 'video/mp4', got %q", contents[0].Parts[1].FileData.MimeType)
	}
	if contents[0].Parts[1].FileData.FileURI != "gs://bucket/video.mp4" {
		t.Errorf("expected fileUri 'gs://bucket/video.mp4', got %q", contents[0].Parts[1].FileData.FileURI)
	}
}

func TestBuildContents_WithDocumentInput(t *testing.T) {
	messages := []ai.Message{
		{
			Role: ai.RoleUser,
			ContentParts: []ai.ContentPart{
				ai.NewTextPart("Summarize this PDF"),
				ai.NewDocumentPart("application/pdf", "JVBERi0xLjQK=="),
			},
		},
	}

	contents := buildContents(messages)

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	if len(contents[0].Parts) != 2 {
		t.Fatalf("expected 2 parts (text + document), got %d", len(contents[0].Parts))
	}

	// Verify document inlineData part
	if contents[0].Parts[1].InlineData == nil {
		t.Fatal("expected inlineData in second part")
	}
	if contents[0].Parts[1].InlineData.MimeType != "application/pdf" {
		t.Errorf("expected mimeType 'application/pdf', got %q", contents[0].Parts[1].InlineData.MimeType)
	}
	if contents[0].Parts[1].InlineData.Data != "JVBERi0xLjQK==" {
		t.Errorf("expected base64 data, got %q", contents[0].Parts[1].InlineData.Data)
	}
}

func TestContentPartsToGeminiParts_URIPrecedence(t *testing.T) {
	// When both Data and URI are set, URI should take precedence
	contentParts := []ai.ContentPart{
		{
			Type: ai.ContentTypeImage,
			Image: &ai.ImageData{
				MimeType: "image/png",
				Data:     "inlineBase64Data==",
				URI:      "gs://bucket/image.png",
			},
		},
		{
			Type: ai.ContentTypeAudio,
			Audio: &ai.AudioData{
				MimeType: "audio/wav",
				Data:     "audioBase64Data==",
				URI:      "https://storage.example.com/audio.wav",
			},
		},
	}

	parts := contentPartsToGeminiParts(contentParts)

	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}

	// Image: URI should win over Data
	if parts[0].FileData == nil {
		t.Fatal("expected fileData for image when both Data and URI are set")
	}
	if parts[0].InlineData != nil {
		t.Error("expected no inlineData when URI takes precedence")
	}
	if parts[0].FileData.FileURI != "gs://bucket/image.png" {
		t.Errorf("expected fileUri 'gs://bucket/image.png', got %q", parts[0].FileData.FileURI)
	}

	// Audio: URI should win over Data
	if parts[1].FileData == nil {
		t.Fatal("expected fileData for audio when both Data and URI are set")
	}
	if parts[1].InlineData != nil {
		t.Error("expected no inlineData when URI takes precedence")
	}
	if parts[1].FileData.FileURI != "https://storage.example.com/audio.wav" {
		t.Errorf("expected correct audio URI, got %q", parts[1].FileData.FileURI)
	}
}

func TestGeminiToGeneric_WithAudioOutput(t *testing.T) {
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role: "model",
				Parts: []part{
					{Text: "Here is the generated audio:"},
					{InlineData: &inlineData{MimeType: "audio/wav", Data: "UklGRiQAAABXQVZFZm10=="}},
				},
			},
			FinishReason: "STOP",
		}},
	}

	result := geminiToGeneric(resp)

	if result.Content != "Here is the generated audio:" {
		t.Errorf("expected text content, got %q", result.Content)
	}

	// Audio MIME type should route to result.Audio, not result.Images
	if len(result.Audio) != 1 {
		t.Fatalf("expected 1 audio output, got %d", len(result.Audio))
	}
	if result.Audio[0].MimeType != "audio/wav" {
		t.Errorf("expected mime type 'audio/wav', got %q", result.Audio[0].MimeType)
	}
	if result.Audio[0].Data != "UklGRiQAAABXQVZFZm10==" {
		t.Errorf("expected base64 audio data, got %q", result.Audio[0].Data)
	}

	// Images should be empty since audio is routed separately
	if len(result.Images) != 0 {
		t.Errorf("expected 0 images, got %d (audio was incorrectly routed to images)", len(result.Images))
	}
}

func TestGeminiToGeneric_WithFileDataResponse(t *testing.T) {
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role: "model",
				Parts: []part{
					{Text: "Here are the generated files:"},
					{FileData: &fileData{MimeType: "image/png", FileURI: "gs://bucket/output.png"}},
					{FileData: &fileData{MimeType: "audio/mp3", FileURI: "gs://bucket/output.mp3"}},
				},
			},
			FinishReason: "STOP",
		}},
	}

	result := geminiToGeneric(resp)

	if result.Content != "Here are the generated files:" {
		t.Errorf("expected text content, got %q", result.Content)
	}

	// Image FileData should go to result.Images with URI set
	if len(result.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(result.Images))
	}
	if result.Images[0].MimeType != "image/png" {
		t.Errorf("expected mime type 'image/png', got %q", result.Images[0].MimeType)
	}
	if result.Images[0].URI != "gs://bucket/output.png" {
		t.Errorf("expected URI 'gs://bucket/output.png', got %q", result.Images[0].URI)
	}
	if result.Images[0].Data != "" {
		t.Errorf("expected empty Data for URI-based image, got %q", result.Images[0].Data)
	}

	// Audio FileData should go to result.Audio with URI set
	if len(result.Audio) != 1 {
		t.Fatalf("expected 1 audio, got %d", len(result.Audio))
	}
	if result.Audio[0].MimeType != "audio/mp3" {
		t.Errorf("expected mime type 'audio/mp3', got %q", result.Audio[0].MimeType)
	}
	if result.Audio[0].URI != "gs://bucket/output.mp3" {
		t.Errorf("expected URI 'gs://bucket/output.mp3', got %q", result.Audio[0].URI)
	}
}

func TestDetectCapabilities_PerModel(t *testing.T) {
	tests := []struct {
		name              string
		model             string
		expectMultimodal  bool
		expectImageOutput bool
		expectAudioOutput bool
		expectVideoOutput bool
	}{
		{
			name:              "text-only model (flash-lite)",
			model:             Model20FlashLite,
			expectMultimodal:  true, // Gemini flash-lite still accepts multimodal input
			expectImageOutput: false,
			expectAudioOutput: false,
			expectVideoOutput: false,
		},
		{
			name:              "image output model (text-only input)",
			model:             Model30ProImagePreview,
			expectMultimodal:  false, // Model30ProImagePreview accepts text-only input
			expectImageOutput: true,
			expectAudioOutput: false,
			expectVideoOutput: false,
		},
		{
			name:              "audio output model (native audio)",
			model:             Model25FlashNativeAudio,
			expectMultimodal:  true,
			expectImageOutput: false,
			expectAudioOutput: true,
			expectVideoOutput: false,
		},
		{
			name:              "unknown model gets conservative defaults",
			model:             "some-future-model-xyz",
			expectMultimodal:  true,
			expectImageOutput: false,
			expectAudioOutput: false,
			expectVideoOutput: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			capabilities := detectCapabilities(testCase.model)

			if capabilities.SupportsMultimodal != testCase.expectMultimodal {
				t.Errorf("SupportsMultimodal: expected %v, got %v", testCase.expectMultimodal, capabilities.SupportsMultimodal)
			}
			if capabilities.SupportsImageOutput != testCase.expectImageOutput {
				t.Errorf("SupportsImageOutput: expected %v, got %v", testCase.expectImageOutput, capabilities.SupportsImageOutput)
			}
			if capabilities.SupportsAudioOutput != testCase.expectAudioOutput {
				t.Errorf("SupportsAudioOutput: expected %v, got %v", testCase.expectAudioOutput, capabilities.SupportsAudioOutput)
			}
			if capabilities.SupportsVideoOutput != testCase.expectVideoOutput {
				t.Errorf("SupportsVideoOutput: expected %v, got %v", testCase.expectVideoOutput, capabilities.SupportsVideoOutput)
			}

			// All models should support these base capabilities
			if !capabilities.SupportsStructuredOutputs {
				t.Error("expected SupportsStructuredOutputs to be true")
			}
			if !capabilities.SupportsThinking {
				t.Error("expected SupportsThinking to be true")
			}
			if !capabilities.SupportsFunctionCalling {
				t.Error("expected SupportsFunctionCalling to be true")
			}
		})
	}
}

func TestIsStopMessage_WithAudioOnly(t *testing.T) {
	provider := New()

	// Audio-only response (no text, no images) should NOT be a stop message
	audioOnlyResponse := &ai.ChatResponse{
		Content: "",
		Audio: []ai.AudioData{
			{MimeType: "audio/wav", Data: "audioBase64Data=="},
		},
	}

	if provider.IsStopMessage(audioOnlyResponse) {
		t.Error("expected audio-only response to NOT be a stop message")
	}

	// Response with audio and stop finish reason should be a stop message
	audioWithStopResponse := &ai.ChatResponse{
		Content:      "Here is the audio",
		FinishReason: "stop",
		Audio: []ai.AudioData{
			{MimeType: "audio/wav", Data: "audioBase64Data=="},
		},
	}

	if !provider.IsStopMessage(audioWithStopResponse) {
		t.Error("expected response with stop finish reason to be a stop message")
	}
}

func TestSendMessage_WithGoogleSearchReturnsGrounding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response with full grounding metadata
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role:  "model",
					Parts: []part{{Text: "Based on search results, here's the answer."}},
				},
				FinishReason: "STOP",
				GroundingMetadata: &groundingMetadata{
					WebSearchQueries: []string{"test search query"},
					GroundingChunks: []groundingChunk{
						{Web: &webChunk{URI: "https://source1.com", Title: "Source 1"}},
						{Web: &webChunk{URI: "https://source2.com", Title: "Source 2"}},
					},
					GroundingSupports: []groundingSupport{
						{
							Segment:               &segment{StartIndex: 0, EndIndex: 44, Text: "Based on search results, here's the answer."},
							GroundingChunkIndices: []int{0, 1},
							ConfidenceScores:      []float64{0.9, 0.85},
						},
					},
				},
			}},
			UsageMetadata: &usageMetadata{
				PromptTokenCount:     20,
				CandidatesTokenCount: 15,
				TotalTokenCount:      35,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-2.0-flash",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Search for something"}},
		Tools: []ai.ToolDescription{
			{Name: ai.ToolGoogleSearch},
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify grounding metadata is accessible
	if response.Grounding == nil {
		t.Fatal("expected grounding metadata in response")
	}

	if len(response.Grounding.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(response.Grounding.Sources))
	}

	if len(response.Grounding.Citations) != 1 {
		t.Errorf("expected 1 citation, got %d", len(response.Grounding.Citations))
	}

	if len(response.Grounding.SearchQueries) != 1 {
		t.Errorf("expected 1 search query, got %d", len(response.Grounding.SearchQueries))
	}
}

// ========================
// Code Execution Tests
// ========================

func TestGeminiToGeneric_WithCodeExecution(t *testing.T) {
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role: "model",
				Parts: []part{
					{Text: "I'll calculate that for you."},
					{ExecutableCode: &executableCode{
						Language: "PYTHON",
						Code:     "print(sum(range(1, 51)))",
					}},
					{CodeExecutionResult: &codeExecutionResult{
						Outcome: "OUTCOME_OK",
						Output:  "1275\n",
					}},
					{Text: "The sum of the first 50 numbers is 1275."},
				},
			},
			FinishReason: "STOP",
		}},
		UsageMetadata: &usageMetadata{
			PromptTokenCount:     15,
			CandidatesTokenCount: 30,
			TotalTokenCount:      45,
		},
	}

	result := geminiToGeneric(resp)

	// Verify text content (should concatenate non-code text parts)
	expectedContent := "I'll calculate that for you.\nThe sum of the first 50 numbers is 1275."
	if result.Content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, result.Content)
	}

	// Verify code execution results
	if len(result.CodeExecutions) != 1 {
		t.Fatalf("expected 1 code execution, got %d", len(result.CodeExecutions))
	}

	codeExec := result.CodeExecutions[0]
	if codeExec.Language != "PYTHON" {
		t.Errorf("expected language 'PYTHON', got %q", codeExec.Language)
	}
	if codeExec.Code != "print(sum(range(1, 51)))" {
		t.Errorf("expected code 'print(sum(range(1, 51)))', got %q", codeExec.Code)
	}
	if codeExec.Outcome != "OUTCOME_OK" {
		t.Errorf("expected outcome 'OUTCOME_OK', got %q", codeExec.Outcome)
	}
	if codeExec.Output != "1275\n" {
		t.Errorf("expected output '1275\\n', got %q", codeExec.Output)
	}

	// Verify finish reason
	if result.FinishReason != "stop" {
		t.Errorf("expected finish reason 'stop', got %q", result.FinishReason)
	}
}

func TestGeminiToGeneric_WithMultipleCodeExecutions(t *testing.T) {
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role: "model",
				Parts: []part{
					{Text: "Let me solve this step by step."},
					{ExecutableCode: &executableCode{
						Language: "PYTHON",
						Code:     "import math\nprint(math.factorial(10))",
					}},
					{CodeExecutionResult: &codeExecutionResult{
						Outcome: "OUTCOME_OK",
						Output:  "3628800\n",
					}},
					{Text: "Now let me verify:"},
					{ExecutableCode: &executableCode{
						Language: "PYTHON",
						Code:     "result = 1\nfor i in range(1, 11):\n    result *= i\nprint(result)",
					}},
					{CodeExecutionResult: &codeExecutionResult{
						Outcome: "OUTCOME_OK",
						Output:  "3628800\n",
					}},
					{Text: "Both methods confirm 10! = 3,628,800."},
				},
			},
			FinishReason: "STOP",
		}},
	}

	result := geminiToGeneric(resp)

	// Verify multiple code executions are captured
	if len(result.CodeExecutions) != 2 {
		t.Fatalf("expected 2 code executions, got %d", len(result.CodeExecutions))
	}

	// First execution
	if result.CodeExecutions[0].Code != "import math\nprint(math.factorial(10))" {
		t.Errorf("unexpected first code: %q", result.CodeExecutions[0].Code)
	}
	if result.CodeExecutions[0].Outcome != "OUTCOME_OK" {
		t.Errorf("expected first outcome 'OUTCOME_OK', got %q", result.CodeExecutions[0].Outcome)
	}

	// Second execution
	if result.CodeExecutions[1].Code != "result = 1\nfor i in range(1, 11):\n    result *= i\nprint(result)" {
		t.Errorf("unexpected second code: %q", result.CodeExecutions[1].Code)
	}
	if result.CodeExecutions[1].Output != "3628800\n" {
		t.Errorf("unexpected second output: %q", result.CodeExecutions[1].Output)
	}

	// Verify all text parts are concatenated
	expectedContent := "Let me solve this step by step.\nNow let me verify:\nBoth methods confirm 10! = 3,628,800."
	if result.Content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, result.Content)
	}
}

func TestGeminiToGeneric_WithFailedCodeExecution(t *testing.T) {
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role: "model",
				Parts: []part{
					{ExecutableCode: &executableCode{
						Language: "PYTHON",
						Code:     "print(1/0)",
					}},
					{CodeExecutionResult: &codeExecutionResult{
						Outcome: "OUTCOME_FAILED",
						Output:  "ZeroDivisionError: division by zero\n",
					}},
					{Text: "The code failed due to a division by zero error."},
				},
			},
			FinishReason: "STOP",
		}},
	}

	result := geminiToGeneric(resp)

	if len(result.CodeExecutions) != 1 {
		t.Fatalf("expected 1 code execution, got %d", len(result.CodeExecutions))
	}

	if result.CodeExecutions[0].Outcome != "OUTCOME_FAILED" {
		t.Errorf("expected outcome 'OUTCOME_FAILED', got %q", result.CodeExecutions[0].Outcome)
	}

	if result.CodeExecutions[0].Output != "ZeroDivisionError: division by zero\n" {
		t.Errorf("expected error output, got %q", result.CodeExecutions[0].Output)
	}
}

func TestGeminiToGeneric_WithCodeExecutionResultOnly(t *testing.T) {
	// Defensive test: codeExecutionResult without preceding executableCode
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role: "model",
				Parts: []part{
					{CodeExecutionResult: &codeExecutionResult{
						Outcome: "OUTCOME_OK",
						Output:  "42\n",
					}},
				},
			},
			FinishReason: "STOP",
		}},
	}

	result := geminiToGeneric(resp)

	if len(result.CodeExecutions) != 1 {
		t.Fatalf("expected 1 code execution, got %d", len(result.CodeExecutions))
	}

	// Standalone result should have outcome/output but no code
	if result.CodeExecutions[0].Code != "" {
		t.Errorf("expected empty code, got %q", result.CodeExecutions[0].Code)
	}
	if result.CodeExecutions[0].Outcome != "OUTCOME_OK" {
		t.Errorf("expected outcome 'OUTCOME_OK', got %q", result.CodeExecutions[0].Outcome)
	}
	if result.CodeExecutions[0].Output != "42\n" {
		t.Errorf("expected output '42\\n', got %q", result.CodeExecutions[0].Output)
	}
}

func TestGeminiToGeneric_WithCodeExecutionAndFunctionCalls(t *testing.T) {
	// Test response with both code execution and function calls
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role: "model",
				Parts: []part{
					{Text: "Let me calculate and then search."},
					{ExecutableCode: &executableCode{
						Language: "PYTHON",
						Code:     "print(2 ** 10)",
					}},
					{CodeExecutionResult: &codeExecutionResult{
						Outcome: "OUTCOME_OK",
						Output:  "1024\n",
					}},
					{FunctionCall: &functionCall{
						Name: "search",
						Args: json.RawMessage(`{"query":"1024"}`),
					}},
				},
			},
			FinishReason: "STOP",
		}},
	}

	result := geminiToGeneric(resp)

	// Verify code execution
	if len(result.CodeExecutions) != 1 {
		t.Fatalf("expected 1 code execution, got %d", len(result.CodeExecutions))
	}
	if result.CodeExecutions[0].Code != "print(2 ** 10)" {
		t.Errorf("unexpected code: %q", result.CodeExecutions[0].Code)
	}

	// Verify function call
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Function.Name != "search" {
		t.Errorf("expected function name 'search', got %q", result.ToolCalls[0].Function.Name)
	}

	// Finish reason should be tool_calls since we have tool calls
	if result.FinishReason != "tool_calls" {
		t.Errorf("expected finish reason 'tool_calls', got %q", result.FinishReason)
	}
}

// ========================
// URL Context Tests
// ========================

func TestGeminiToGeneric_WithURLContextMetadata(t *testing.T) {
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role:  "model",
				Parts: []part{{Text: "Based on the content from those URLs, here is a comparison."}},
			},
			FinishReason: "STOP",
			URLContextMetadata: []urlContextMeta{
				{
					URL:                    "https://example.com/recipe1",
					Status:                 "SUCCESS",
					RetrievedContentLength: 12345,
				},
				{
					URL:                    "https://example.com/recipe2",
					Status:                 "SUCCESS",
					RetrievedContentLength: 6789,
				},
			},
		}},
		UsageMetadata: &usageMetadata{
			PromptTokenCount:     100,
			CandidatesTokenCount: 50,
			TotalTokenCount:      150,
		},
	}

	result := geminiToGeneric(resp)

	// Verify grounding metadata is created for URL context
	if result.Grounding == nil {
		t.Fatal("expected grounding metadata with URL context sources")
	}

	if len(result.Grounding.URLContextSources) != 2 {
		t.Fatalf("expected 2 URL context sources, got %d", len(result.Grounding.URLContextSources))
	}

	// Verify first URL
	source1 := result.Grounding.URLContextSources[0]
	if source1.URL != "https://example.com/recipe1" {
		t.Errorf("expected URL 'https://example.com/recipe1', got %q", source1.URL)
	}
	if source1.Status != "SUCCESS" {
		t.Errorf("expected status 'SUCCESS', got %q", source1.Status)
	}
	if source1.RetrievedContentLength != 12345 {
		t.Errorf("expected content length 12345, got %d", source1.RetrievedContentLength)
	}

	// Verify second URL
	source2 := result.Grounding.URLContextSources[1]
	if source2.URL != "https://example.com/recipe2" {
		t.Errorf("expected URL 'https://example.com/recipe2', got %q", source2.URL)
	}
	if source2.RetrievedContentLength != 6789 {
		t.Errorf("expected content length 6789, got %d", source2.RetrievedContentLength)
	}
}

func TestGeminiToGeneric_WithURLContextAndGroundingMetadata(t *testing.T) {
	// Test that URL context metadata merges with existing grounding metadata
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role:  "model",
				Parts: []part{{Text: "Combined search and URL context response."}},
			},
			FinishReason: "STOP",
			GroundingMetadata: &groundingMetadata{
				WebSearchQueries: []string{"test query"},
				GroundingChunks: []groundingChunk{
					{Web: &webChunk{URI: "https://search-result.com", Title: "Search Result"}},
				},
			},
			URLContextMetadata: []urlContextMeta{
				{
					URL:                    "https://user-provided-url.com/page",
					Status:                 "SUCCESS",
					RetrievedContentLength: 5000,
				},
			},
		}},
	}

	result := geminiToGeneric(resp)

	if result.Grounding == nil {
		t.Fatal("expected grounding metadata")
	}

	// Verify both search grounding and URL context are present
	if len(result.Grounding.SearchQueries) != 1 {
		t.Errorf("expected 1 search query, got %d", len(result.Grounding.SearchQueries))
	}
	if len(result.Grounding.Sources) != 1 {
		t.Errorf("expected 1 grounding source, got %d", len(result.Grounding.Sources))
	}
	if len(result.Grounding.URLContextSources) != 1 {
		t.Fatalf("expected 1 URL context source, got %d", len(result.Grounding.URLContextSources))
	}
	if result.Grounding.URLContextSources[0].URL != "https://user-provided-url.com/page" {
		t.Errorf("expected URL 'https://user-provided-url.com/page', got %q", result.Grounding.URLContextSources[0].URL)
	}
}

func TestGeminiToGeneric_WithURLContextEmptyMetadata(t *testing.T) {
	// Empty URL context metadata should not create grounding
	resp := generateContentResponse{
		Candidates: []candidate{{
			Content: &content{
				Role:  "model",
				Parts: []part{{Text: "A plain response."}},
			},
			FinishReason:       "STOP",
			URLContextMetadata: []urlContextMeta{},
		}},
	}

	result := geminiToGeneric(resp)

	if result.Grounding != nil {
		t.Error("expected nil grounding for empty URL context metadata")
	}
}

// ========================
// Code Execution Multi-Turn Round-Trip Tests
// ========================

func TestBuildContents_WithCodeExecutionInAssistantMessage(t *testing.T) {
	messages := []ai.Message{
		{
			Role:    ai.RoleUser,
			Content: "What is the sum of the first 50 primes?",
		},
		{
			Role:    ai.RoleAssistant,
			Content: "The sum is 5117.",
			CodeExecutions: []ai.CodeExecution{
				{
					Language: "PYTHON",
					Code:     "primes = []\nn = 2\nwhile len(primes) < 50:\n    if all(n % p for p in primes):\n        primes.append(n)\n    n += 1\nprint(sum(primes))",
					Outcome:  "OUTCOME_OK",
					Output:   "5117\n",
				},
			},
		},
		{
			Role:    ai.RoleUser,
			Content: "Now find the 51st prime.",
		},
	}

	contents := buildContents(messages)

	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}

	// Verify assistant message has code execution parts
	assistantContent := contents[1]
	if assistantContent.Role != "model" {
		t.Errorf("expected role 'model', got %q", assistantContent.Role)
	}

	// Should have: executableCode + codeExecutionResult + text = 3 parts
	if len(assistantContent.Parts) != 3 {
		t.Fatalf("expected 3 parts in assistant message, got %d", len(assistantContent.Parts))
	}

	// First part: executableCode
	if assistantContent.Parts[0].ExecutableCode == nil {
		t.Fatal("expected executableCode in first part")
	}
	if assistantContent.Parts[0].ExecutableCode.Language != "PYTHON" {
		t.Errorf("expected language 'PYTHON', got %q", assistantContent.Parts[0].ExecutableCode.Language)
	}
	if assistantContent.Parts[0].ExecutableCode.Code != "primes = []\nn = 2\nwhile len(primes) < 50:\n    if all(n % p for p in primes):\n        primes.append(n)\n    n += 1\nprint(sum(primes))" {
		t.Errorf("unexpected code: %q", assistantContent.Parts[0].ExecutableCode.Code)
	}

	// Second part: codeExecutionResult
	if assistantContent.Parts[1].CodeExecutionResult == nil {
		t.Fatal("expected codeExecutionResult in second part")
	}
	if assistantContent.Parts[1].CodeExecutionResult.Outcome != "OUTCOME_OK" {
		t.Errorf("expected outcome 'OUTCOME_OK', got %q", assistantContent.Parts[1].CodeExecutionResult.Outcome)
	}
	if assistantContent.Parts[1].CodeExecutionResult.Output != "5117\n" {
		t.Errorf("expected output '5117\\n', got %q", assistantContent.Parts[1].CodeExecutionResult.Output)
	}

	// Third part: text
	if assistantContent.Parts[2].Text != "The sum is 5117." {
		t.Errorf("expected text 'The sum is 5117.', got %q", assistantContent.Parts[2].Text)
	}
}

func TestBuildContents_WithCodeExecutionAndToolCalls(t *testing.T) {
	// Assistant message with both code execution and tool calls
	messages := []ai.Message{
		{
			Role: ai.RoleAssistant,
			CodeExecutions: []ai.CodeExecution{
				{
					Language: "PYTHON",
					Code:     "print('calculating...')",
					Outcome:  "OUTCOME_OK",
					Output:   "calculating...\n",
				},
			},
			ToolCalls: []ai.ToolCall{
				{
					ID:   "call_0",
					Type: "function",
					Function: ai.ToolCallFunction{
						Name:      "search",
						Arguments: `{"query":"test"}`,
					},
				},
			},
			Content: "Let me search for that.",
		},
	}

	contents := buildContents(messages)

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	// Should have: functionCall + executableCode + codeExecutionResult + text = 4 parts
	if len(contents[0].Parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(contents[0].Parts))
	}

	// Tool calls come first
	if contents[0].Parts[0].FunctionCall == nil {
		t.Error("expected first part to be functionCall")
	}
	if contents[0].Parts[0].FunctionCall.Name != "search" {
		t.Errorf("expected function name 'search', got %q", contents[0].Parts[0].FunctionCall.Name)
	}

	// Then code execution parts
	if contents[0].Parts[1].ExecutableCode == nil {
		t.Error("expected second part to be executableCode")
	}
	if contents[0].Parts[2].CodeExecutionResult == nil {
		t.Error("expected third part to be codeExecutionResult")
	}

	// Then text
	if contents[0].Parts[3].Text != "Let me search for that." {
		t.Errorf("expected text part, got %q", contents[0].Parts[3].Text)
	}
}

func TestBuildContents_WithCodeExecutionCodeOnly(t *testing.T) {
	// Code execution with code but no outcome (edge case)
	messages := []ai.Message{
		{
			Role: ai.RoleAssistant,
			CodeExecutions: []ai.CodeExecution{
				{
					Language: "PYTHON",
					Code:     "print('hello')",
					// No Outcome or Output
				},
			},
			Content: "Executing code...",
		},
	}

	contents := buildContents(messages)

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	// Should have: executableCode + text = 2 parts (no codeExecutionResult since Outcome is empty)
	if len(contents[0].Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(contents[0].Parts))
	}

	if contents[0].Parts[0].ExecutableCode == nil {
		t.Error("expected executableCode in first part")
	}
	if contents[0].Parts[1].Text != "Executing code..." {
		t.Errorf("expected text part, got %q", contents[0].Parts[1].Text)
	}
}

// ========================
// URL Context Tool Request Tests
// ========================

func TestBuildTools_WithURLContext(t *testing.T) {
	aiTools := []ai.ToolDescription{
		{Name: ai.ToolURLContext},
		{Name: "custom_function", Description: "A custom function", Parameters: &jsonschema.Schema{Type: "object"}},
	}

	tools := buildTools(aiTools)

	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	hasURLContext := false
	hasFunctions := false

	for _, builtTool := range tools {
		if builtTool.URLContext != nil {
			hasURLContext = true
		}
		if len(builtTool.FunctionDeclarations) > 0 {
			hasFunctions = true
		}
	}

	if !hasURLContext {
		t.Error("expected url_context tool")
	}
	if !hasFunctions {
		t.Error("expected function declarations")
	}
}

func TestBuildTools_AllBuiltinTools(t *testing.T) {
	aiTools := []ai.ToolDescription{
		{Name: ai.ToolGoogleSearch},
		{Name: ai.ToolURLContext},
		{Name: ai.ToolCodeExecution},
	}

	tools := buildTools(aiTools)

	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	hasGoogleSearch := false
	hasURLContext := false
	hasCodeExecution := false

	for _, builtTool := range tools {
		if builtTool.GoogleSearch != nil {
			hasGoogleSearch = true
		}
		if builtTool.URLContext != nil {
			hasURLContext = true
		}
		if builtTool.CodeExecution != nil {
			hasCodeExecution = true
		}
	}

	if !hasGoogleSearch {
		t.Error("expected google_search tool")
	}
	if !hasURLContext {
		t.Error("expected url_context tool")
	}
	if !hasCodeExecution {
		t.Error("expected code_execution tool")
	}
}

// ========================
// Integration Tests (httptest)
// ========================

func TestSendMessage_WithCodeExecution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request contains code_execution tool
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		hasCodeExec := false
		for _, requestTool := range req.Tools {
			if requestTool.CodeExecution != nil {
				hasCodeExec = true
			}
		}
		if !hasCodeExec {
			t.Error("expected code_execution tool in request")
		}

		// Return response with code execution results
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role: "model",
					Parts: []part{
						{Text: "Let me calculate that."},
						{ExecutableCode: &executableCode{
							Language: "PYTHON",
							Code:     "def fibonacci(n):\n    if n <= 1:\n        return n\n    return fibonacci(n-1) + fibonacci(n-2)\nprint(fibonacci(20))",
						}},
						{CodeExecutionResult: &codeExecutionResult{
							Outcome: "OUTCOME_OK",
							Output:  "6765\n",
						}},
						{Text: "The 20th Fibonacci number is 6765."},
					},
				},
				FinishReason: "STOP",
			}},
			UsageMetadata: &usageMetadata{
				PromptTokenCount:     25,
				CandidatesTokenCount: 40,
				TotalTokenCount:      65,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-3-flash-preview",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "What is the 20th Fibonacci number?"}},
		Tools: []ai.ToolDescription{
			{Name: ai.ToolCodeExecution},
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify text content
	expectedContent := "Let me calculate that.\nThe 20th Fibonacci number is 6765."
	if response.Content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, response.Content)
	}

	// Verify code execution
	if len(response.CodeExecutions) != 1 {
		t.Fatalf("expected 1 code execution, got %d", len(response.CodeExecutions))
	}
	if response.CodeExecutions[0].Language != "PYTHON" {
		t.Errorf("expected language 'PYTHON', got %q", response.CodeExecutions[0].Language)
	}
	if response.CodeExecutions[0].Outcome != "OUTCOME_OK" {
		t.Errorf("expected outcome 'OUTCOME_OK', got %q", response.CodeExecutions[0].Outcome)
	}
	if response.CodeExecutions[0].Output != "6765\n" {
		t.Errorf("expected output '6765\\n', got %q", response.CodeExecutions[0].Output)
	}

	// Verify usage
	if response.Usage == nil {
		t.Fatal("expected usage metadata")
	}
	if response.Usage.TotalTokens != 65 {
		t.Errorf("expected 65 total tokens, got %d", response.Usage.TotalTokens)
	}

	// Verify finish reason is stop (no tool calls, just code execution)
	if response.FinishReason != "stop" {
		t.Errorf("expected finish reason 'stop', got %q", response.FinishReason)
	}
}

func TestSendMessage_WithURLContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request contains url_context tool
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		hasURLCtx := false
		for _, requestTool := range req.Tools {
			if requestTool.URLContext != nil {
				hasURLCtx = true
			}
		}
		if !hasURLCtx {
			t.Error("expected url_context tool in request")
		}

		// Return response with URL context metadata
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role:  "model",
					Parts: []part{{Text: "Based on the article at that URL, the main topic is..."}},
				},
				FinishReason: "STOP",
				URLContextMetadata: []urlContextMeta{
					{
						URL:                    "https://example.com/article",
						Status:                 "SUCCESS",
						RetrievedContentLength: 8500,
					},
				},
			}},
			UsageMetadata: &usageMetadata{
				PromptTokenCount:     30,
				CandidatesTokenCount: 25,
				TotalTokenCount:      55,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-3-flash-preview",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Summarize https://example.com/article"}},
		Tools: []ai.ToolDescription{
			{Name: ai.ToolURLContext},
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify text content
	if response.Content != "Based on the article at that URL, the main topic is..." {
		t.Errorf("unexpected content: %q", response.Content)
	}

	// Verify URL context metadata
	if response.Grounding == nil {
		t.Fatal("expected grounding metadata with URL context")
	}
	if len(response.Grounding.URLContextSources) != 1 {
		t.Fatalf("expected 1 URL context source, got %d", len(response.Grounding.URLContextSources))
	}

	urlSource := response.Grounding.URLContextSources[0]
	if urlSource.URL != "https://example.com/article" {
		t.Errorf("expected URL 'https://example.com/article', got %q", urlSource.URL)
	}
	if urlSource.Status != "SUCCESS" {
		t.Errorf("expected status 'SUCCESS', got %q", urlSource.Status)
	}
	if urlSource.RetrievedContentLength != 8500 {
		t.Errorf("expected content length 8500, got %d", urlSource.RetrievedContentLength)
	}
}

func TestSendMessage_WithCodeExecutionMultiTurn(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if callCount == 2 {
			// Second call: verify the assistant's code execution parts are sent back
			if len(req.Contents) < 2 {
				t.Fatalf("expected at least 2 contents in multi-turn, got %d", len(req.Contents))
			}

			modelContent := req.Contents[1]
			if modelContent.Role != "model" {
				t.Errorf("expected role 'model' for second content, got %q", modelContent.Role)
			}

			// Verify code execution parts are present
			hasExecCode := false
			hasExecResult := false
			for _, messagePart := range modelContent.Parts {
				if messagePart.ExecutableCode != nil {
					hasExecCode = true
					if messagePart.ExecutableCode.Language != "PYTHON" {
						t.Errorf("expected language 'PYTHON', got %q", messagePart.ExecutableCode.Language)
					}
				}
				if messagePart.CodeExecutionResult != nil {
					hasExecResult = true
					if messagePart.CodeExecutionResult.Outcome != "OUTCOME_OK" {
						t.Errorf("expected outcome 'OUTCOME_OK', got %q", messagePart.CodeExecutionResult.Outcome)
					}
				}
			}
			if !hasExecCode {
				t.Error("expected executableCode part in multi-turn model message")
			}
			if !hasExecResult {
				t.Error("expected codeExecutionResult part in multi-turn model message")
			}
		}

		// Return a simple response
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role:  "model",
					Parts: []part{{Text: "Response for call " + string(rune('0'+callCount))}},
				},
				FinishReason: "STOP",
			}},
			UsageMetadata: &usageMetadata{TotalTokenCount: 10},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	// Second call includes the previous model response with code execution
	_, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model: "gemini-3-flash-preview",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Calculate fibonacci(10)"},
			{
				Role:    ai.RoleAssistant,
				Content: "The answer is 55.",
				CodeExecutions: []ai.CodeExecution{
					{
						Language: "PYTHON",
						Code:     "def fib(n): return n if n<2 else fib(n-1)+fib(n-2)\nprint(fib(10))",
						Outcome:  "OUTCOME_OK",
						Output:   "55\n",
					},
				},
			},
			{Role: ai.RoleUser, Content: "Now calculate fibonacci(20)"},
		},
		Tools: []ai.ToolDescription{
			{Name: ai.ToolCodeExecution},
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestSendMessage_WithAllBuiltinTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify all three built-in tools are present
		hasGoogleSearch := false
		hasURLContext := false
		hasCodeExecution := false

		for _, requestTool := range req.Tools {
			if requestTool.GoogleSearch != nil {
				hasGoogleSearch = true
			}
			if requestTool.URLContext != nil {
				hasURLContext = true
			}
			if requestTool.CodeExecution != nil {
				hasCodeExecution = true
			}
		}

		if !hasGoogleSearch {
			t.Error("expected google_search tool in request")
		}
		if !hasURLContext {
			t.Error("expected url_context tool in request")
		}
		if !hasCodeExecution {
			t.Error("expected code_execution tool in request")
		}

		// Return response with both code execution and URL context metadata
		resp := generateContentResponse{
			Candidates: []candidate{{
				Content: &content{
					Role: "model",
					Parts: []part{
						{Text: "After searching and analyzing the URL:"},
						{ExecutableCode: &executableCode{
							Language: "PYTHON",
							Code:     "print('analysis complete')",
						}},
						{CodeExecutionResult: &codeExecutionResult{
							Outcome: "OUTCOME_OK",
							Output:  "analysis complete\n",
						}},
						{Text: "Here are the results."},
					},
				},
				FinishReason: "STOP",
				GroundingMetadata: &groundingMetadata{
					WebSearchQueries: []string{"test query"},
					GroundingChunks: []groundingChunk{
						{Web: &webChunk{URI: "https://source.com", Title: "Source"}},
					},
				},
				URLContextMetadata: []urlContextMeta{
					{URL: "https://analyzed-url.com", Status: "SUCCESS", RetrievedContentLength: 3000},
				},
			}},
			UsageMetadata: &usageMetadata{TotalTokenCount: 100},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := New().
		WithAPIKey("test-key").
		WithBaseURL(server.URL).(*GeminiProvider)

	response, err := provider.SendMessage(context.Background(), ai.ChatRequest{
		Model:    "gemini-3-flash-preview",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "Search, analyze URL, and compute"}},
		Tools: []ai.ToolDescription{
			{Name: ai.ToolGoogleSearch},
			{Name: ai.ToolURLContext},
			{Name: ai.ToolCodeExecution},
		},
	})

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify code execution
	if len(response.CodeExecutions) != 1 {
		t.Fatalf("expected 1 code execution, got %d", len(response.CodeExecutions))
	}
	if response.CodeExecutions[0].Outcome != "OUTCOME_OK" {
		t.Errorf("expected outcome 'OUTCOME_OK', got %q", response.CodeExecutions[0].Outcome)
	}

	// Verify grounding has both search results and URL context
	if response.Grounding == nil {
		t.Fatal("expected grounding metadata")
	}
	if len(response.Grounding.SearchQueries) != 1 {
		t.Errorf("expected 1 search query, got %d", len(response.Grounding.SearchQueries))
	}
	if len(response.Grounding.Sources) != 1 {
		t.Errorf("expected 1 grounding source, got %d", len(response.Grounding.Sources))
	}
	if len(response.Grounding.URLContextSources) != 1 {
		t.Fatalf("expected 1 URL context source, got %d", len(response.Grounding.URLContextSources))
	}
	if response.Grounding.URLContextSources[0].URL != "https://analyzed-url.com" {
		t.Errorf("unexpected URL context URL: %q", response.Grounding.URLContextSources[0].URL)
	}

	// Verify text content
	expectedContent := "After searching and analyzing the URL:\nHere are the results."
	if response.Content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, response.Content)
	}
}
