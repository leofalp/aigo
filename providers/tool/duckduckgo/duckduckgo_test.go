package duckduckgo

import (
	"context"
	"testing"
)

// Common test queries used across different test cases
var testQueries = []struct {
	name  string
	query string
}{
	{"Programming query", "Golang programming language"},
	{"Math query", "2+2"},
	{"Definition query", "serendipity"},
}

func TestSearch(t *testing.T) {
	for _, tt := range testQueries {
		t.Run(tt.name, func(t *testing.T) {
			input := Input{Query: tt.query}
			output, err := Search(context.Background(), input)

			if err != nil {
				t.Errorf("Search() error = %v", err)
				return
			}

			if output.Query != tt.query {
				t.Errorf("Search() output.Query = %v, want %v", output.Query, tt.query)
			}
			if output.Summary == "" {
				t.Errorf("Search() output.Summary is empty")
			}

			t.Logf("Query: %s, Summary length: %d", output.Query, len(output.Summary))
		})
	}
}

func TestSearchAdvanced(t *testing.T) {
	for _, tt := range testQueries {
		t.Run(tt.name, func(t *testing.T) {
			input := Input{Query: tt.query}
			output, err := SearchAdvanced(context.Background(), input)

			if err != nil {
				t.Errorf("SearchAdvanced() error = %v", err)
				return
			}

			if output.Query != tt.query {
				t.Errorf("SearchAdvanced() output.Query = %v, want %v", output.Query, tt.query)
			}

			// Log structured output details
			t.Logf("Query: %s, Type: %s", output.Query, output.Type)
			if output.Abstract != "" {
				t.Logf("  Has Abstract from %s", output.AbstractSource)
			}
			if output.Answer != "" {
				t.Logf("  Has Answer: %s", output.AnswerType)
			}
			if len(output.RelatedTopics) > 0 {
				t.Logf("  Related Topics: %d", len(output.RelatedTopics))
			}
		})
	}
}

func TestToolCreation(t *testing.T) {
	t.Run("Base tool", func(t *testing.T) {
		tool := NewDuckDuckGoSearchTool()
		if tool.Name != "DuckDuckGoSearch" {
			t.Errorf("Tool name = %v, want DuckDuckGoSearch", tool.Name)
		}
		if tool.Description == "" {
			t.Error("Tool description is empty")
		}
		if tool.Function == nil {
			t.Error("Tool function is nil")
		}
	})

	t.Run("Advanced tool", func(t *testing.T) {
		tool := NewDuckDuckGoSearchAdvancedTool()
		if tool.Name != "DuckDuckGoSearchAdvanced" {
			t.Errorf("Tool name = %v, want DuckDuckGoSearchAdvanced", tool.Name)
		}
		if tool.Description == "" {
			t.Error("Tool description is empty")
		}
		if tool.Function == nil {
			t.Error("Tool function is nil")
		}
	})
}
