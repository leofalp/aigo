package patterns

import (
	"aigo/providers/ai"
	"aigo/providers/memory"
)

type Pattern interface {
	Execute(llmProvider ai.Provider, memoryProviderPtr *memory.Provider, options ...func(tool *PatternOptions)) (*ai.ChatResponse, error)
}
