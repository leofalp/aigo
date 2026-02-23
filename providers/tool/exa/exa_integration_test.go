//go:build integration

package exa

import (
	"context"
	"os"
	"testing"
)

func TestFindSimilar_Integration(t *testing.T) {
	if os.Getenv("EXA_API_KEY") == "" {
		t.Skip("EXA_API_KEY not set")
	}

	ctx := context.Background()
	input := SimilarInput{
		URL:        "https://go.dev",
		NumResults: 3,
	}

	output, err := FindSimilar(ctx, input)
	if err != nil {
		t.Fatalf("FindSimilar failed: %v", err)
	}

	if output.Source != "https://go.dev" {
		t.Errorf("expected source 'https://go.dev', got '%s'", output.Source)
	}

	if len(output.Results) == 0 {
		t.Error("expected non-empty results")
	}

	if output.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestSearchAdvanced_Integration(t *testing.T) {
	if os.Getenv("EXA_API_KEY") == "" {
		t.Skip("EXA_API_KEY not set")
	}

	ctx := context.Background()
	input := SearchInput{
		Query:      "golang programming language",
		NumResults: 3,
	}

	output, err := SearchAdvanced(ctx, input)
	if err != nil {
		t.Fatalf("SearchAdvanced failed: %v", err)
	}

	if output.Query != "golang programming language" {
		t.Errorf("expected query 'golang programming language', got '%s'", output.Query)
	}

	if len(output.Results) == 0 {
		t.Error("expected non-empty results")
	}

	if output.ResolvedSearchType == "" {
		t.Error("expected non-empty resolved search type")
	}
}

func TestAnswer_Integration(t *testing.T) {
	if os.Getenv("EXA_API_KEY") == "" {
		t.Skip("EXA_API_KEY not set")
	}

	ctx := context.Background()
	input := AnswerInput{
		Query:       "Who created the Go programming language?",
		IncludeText: true,
	}

	output, err := Answer(ctx, input)
	if err != nil {
		t.Fatalf("Answer failed: %v", err)
	}

	if output.Query != "Who created the Go programming language?" {
		t.Errorf("expected query 'Who created the Go programming language?', got '%s'", output.Query)
	}

	if output.Answer == "" {
		t.Error("expected non-empty answer")
	}

	if len(output.Citations) == 0 {
		t.Error("expected non-empty citations")
	}
}
