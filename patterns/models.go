package patterns

import (
	"aigo/internal/jsonschema"
	"aigo/providers/ai"
	"aigo/providers/tool"
)

type PatternOptions struct {
	Model            string
	SystemPrompt     string
	ToolCatalog      *tool.Catalog
	ToolDescriptions []ai.ToolDescription
	OutputSchema     *jsonschema.Schema
}

func WithModel(model string) func(options *PatternOptions) {
	return func(options *PatternOptions) {
		options.Model = model
	}
}

func WithSystemPrompt(systemPrompt string) func(options *PatternOptions) {
	return func(options *PatternOptions) {
		options.SystemPrompt = systemPrompt
	}
}

func WithDescriptions(toolDescriptions []ai.ToolDescription) func(options *PatternOptions) {
	return func(options *PatternOptions) {
		options.ToolDescriptions = toolDescriptions
	}
}

func WithToolCatalog(toolCatalog *tool.Catalog) func(options *PatternOptions) {
	return func(options *PatternOptions) {
		options.ToolCatalog = toolCatalog
	}
}

func WithOutputSchema(outputSchema *jsonschema.Schema) func(options *PatternOptions) {
	return func(options *PatternOptions) {
		options.OutputSchema = outputSchema
	}
}
