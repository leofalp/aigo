package exa

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestNewExaSearchTool(t *testing.T) {
	tool := NewExaSearchTool()

	if tool.Name != "ExaSearch" {
		t.Errorf("expected tool name 'ExaSearch', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("expected non-empty description")
	}

	if tool.Metrics == nil {
		t.Error("expected metrics to be set")
	}

	if tool.Metrics.Amount <= 0 {
		t.Error("expected positive cost amount")
	}

	if tool.Metrics.Accuracy <= 0 || tool.Metrics.Accuracy > 1 {
		t.Errorf("expected accuracy between 0 and 1, got %f", tool.Metrics.Accuracy)
	}
}

func TestNewExaSearchAdvancedTool(t *testing.T) {
	tool := NewExaSearchAdvancedTool()

	if tool.Name != "ExaSearchAdvanced" {
		t.Errorf("expected tool name 'ExaSearchAdvanced', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("expected non-empty description")
	}

	if tool.Metrics == nil {
		t.Error("expected metrics to be set")
	}

	// Advanced should have higher cost
	basicTool := NewExaSearchTool()
	if tool.Metrics.Amount <= basicTool.Metrics.Amount {
		t.Error("expected advanced tool to have higher cost than basic")
	}
}

func TestNewExaFindSimilarTool(t *testing.T) {
	tool := NewExaFindSimilarTool()

	if tool.Name != "ExaFindSimilar" {
		t.Errorf("expected tool name 'ExaFindSimilar', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("expected non-empty description")
	}

	if tool.Metrics == nil {
		t.Error("expected metrics to be set")
	}
}

func TestNewExaAnswerTool(t *testing.T) {
	tool := NewExaAnswerTool()

	if tool.Name != "ExaAnswer" {
		t.Errorf("expected tool name 'ExaAnswer', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("expected non-empty description")
	}

	if tool.Metrics == nil {
		t.Error("expected metrics to be set")
	}

	// Answer should have higher cost due to LLM processing
	searchTool := NewExaSearchTool()
	if tool.Metrics.Amount <= searchTool.Metrics.Amount {
		t.Error("expected answer tool to have higher cost than search")
	}
}

func TestSearch_MissingAPIKey(t *testing.T) {
	originalKey := os.Getenv("EXA_API_KEY")
	os.Unsetenv("EXA_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("EXA_API_KEY", originalKey)
		}
	}()

	ctx := context.Background()
	input := SearchInput{
		Query: "test query",
	}

	_, err := Search(ctx, input)
	if err == nil {
		t.Error("expected error when API key is missing")
	}

	if !strings.Contains(err.Error(), "EXA_API_KEY") {
		t.Errorf("expected error message to mention EXA_API_KEY, got: %s", err.Error())
	}
}

func TestFindSimilar_MissingAPIKey(t *testing.T) {
	originalKey := os.Getenv("EXA_API_KEY")
	os.Unsetenv("EXA_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("EXA_API_KEY", originalKey)
		}
	}()

	ctx := context.Background()
	input := SimilarInput{
		URL: "https://example.com",
	}

	_, err := FindSimilar(ctx, input)
	if err == nil {
		t.Error("expected error when API key is missing")
	}

	if !strings.Contains(err.Error(), "EXA_API_KEY") {
		t.Errorf("expected error message to mention EXA_API_KEY, got: %s", err.Error())
	}
}

func TestFindSimilar_MissingURLAndText(t *testing.T) {
	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	input := SimilarInput{
		// Neither URL nor Text provided
	}

	_, err := FindSimilar(ctx, input)
	if err == nil {
		t.Error("expected error when neither URL nor text is provided")
	}

	if !strings.Contains(err.Error(), "url or text") {
		t.Errorf("expected error about url or text, got: %s", err.Error())
	}
}

func TestAnswer_MissingAPIKey(t *testing.T) {
	originalKey := os.Getenv("EXA_API_KEY")
	os.Unsetenv("EXA_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("EXA_API_KEY", originalKey)
		}
	}()

	ctx := context.Background()
	input := AnswerInput{
		Query: "test question",
	}

	_, err := Answer(ctx, input)
	if err == nil {
		t.Error("expected error when API key is missing")
	}

	if !strings.Contains(err.Error(), "EXA_API_KEY") {
		t.Errorf("expected error message to mention EXA_API_KEY, got: %s", err.Error())
	}
}

func TestAnswer_EmptyQuery(t *testing.T) {
	os.Setenv("EXA_API_KEY", "test-api-key")
	defer os.Unsetenv("EXA_API_KEY")

	ctx := context.Background()
	input := AnswerInput{
		Query: "",
	}

	_, err := Answer(ctx, input)
	if err == nil {
		t.Error("expected error when query is empty")
	}

	if !strings.Contains(err.Error(), "query") {
		t.Errorf("expected error about query, got: %s", err.Error())
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "truncated",
			input:    "hello world",
			maxLen:   5,
			expected: "hello...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestSearchInput_Validation(t *testing.T) {
	input := SearchInput{
		Query:              "test query",
		Type:               "neural",
		NumResults:         10,
		IncludeDomains:     []string{"example.com"},
		ExcludeDomains:     []string{"spam.com"},
		StartPublishedDate: "2024-01-01",
		EndPublishedDate:   "2024-12-31",
		Category:           "research paper",
		IncludeText:        true,
		IncludeHighlights:  true,
	}

	if input.Query == "" {
		t.Error("query should not be empty")
	}

	if input.Type != "neural" {
		t.Errorf("expected type 'neural', got %s", input.Type)
	}

	if input.Category != "research paper" {
		t.Errorf("expected category 'research paper', got %s", input.Category)
	}
}

func TestSimilarInput_Validation(t *testing.T) {
	// Test with URL
	inputURL := SimilarInput{
		URL:        "https://example.com/article",
		NumResults: 5,
	}

	if inputURL.URL == "" {
		t.Error("URL should not be empty")
	}

	// Test with Text
	inputText := SimilarInput{
		Text:       "This is some sample text to find similar content for.",
		NumResults: 5,
	}

	if inputText.Text == "" {
		t.Error("Text should not be empty")
	}
}

func TestAnswerInput_Validation(t *testing.T) {
	input := AnswerInput{
		Query:       "What is the meaning of life?",
		IncludeText: true,
	}

	if input.Query == "" {
		t.Error("query should not be empty")
	}

	if !input.IncludeText {
		t.Error("include_text should be true")
	}
}

func TestToolMetrics_Consistency(t *testing.T) {
	searchTool := NewExaSearchTool()
	advancedTool := NewExaSearchAdvancedTool()
	similarTool := NewExaFindSimilarTool()
	answerTool := NewExaAnswerTool()

	// All tools should have metrics
	if searchTool.Metrics == nil {
		t.Error("ExaSearch: expected metrics to be set")
	}
	if advancedTool.Metrics == nil {
		t.Error("ExaSearchAdvanced: expected metrics to be set")
	}
	if similarTool.Metrics == nil {
		t.Error("ExaFindSimilar: expected metrics to be set")
	}
	if answerTool.Metrics == nil {
		t.Error("ExaAnswer: expected metrics to be set")
	}

	// Verify accuracy is within valid range for all tools
	allMetrics := []struct {
		name     string
		accuracy float64
	}{
		{"ExaSearch", searchTool.Metrics.Accuracy},
		{"ExaSearchAdvanced", advancedTool.Metrics.Accuracy},
		{"ExaFindSimilar", similarTool.Metrics.Accuracy},
		{"ExaAnswer", answerTool.Metrics.Accuracy},
	}

	for _, m := range allMetrics {
		if m.accuracy <= 0 || m.accuracy > 1 {
			t.Errorf("%s: accuracy %f should be between 0 and 1", m.name, m.accuracy)
		}
	}
}
