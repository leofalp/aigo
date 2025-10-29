package duckduckgo

import (
	"context"
	"strings"
	"testing"
)

func TestURLConversion(t *testing.T) {
	input := Input{Query: "Albert Einstein"}
	output, err := SearchAdvanced(context.Background(), input)
	if err != nil {
		t.Fatalf("SearchAdvanced() error = %v", err)
	}

	// Test image URL is absolute
	if output.Image != "" {
		if !isAbsoluteURL(output.Image) {
			t.Errorf("Image URL is not absolute: %s", output.Image)
		}
		t.Logf("Image URL: %s (absolute: %v)", output.Image, isAbsoluteURL(output.Image))
	}

	// Test related topics icon URLs are absolute
	for i, topic := range output.RelatedTopics {
		if topic.Icon.URL != "" && !isAbsoluteURL(topic.Icon.URL) {
			t.Errorf("RelatedTopic[%d] Icon URL is not absolute: %s", i, topic.Icon.URL)
		}
		if i < 3 && topic.Icon.URL != "" {
			t.Logf("RelatedTopic[%d] Icon URL: %s", i, topic.Icon.URL)
		}
	}

	// Test results icon URLs are absolute
	for i, result := range output.Results {
		if result.Icon.URL != "" && !isAbsoluteURL(result.Icon.URL) {
			t.Errorf("Result[%d] Icon URL is not absolute: %s", i, result.Icon.URL)
		}
	}
}

func isAbsoluteURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}
