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
