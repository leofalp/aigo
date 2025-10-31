package patterns

import (
	"aigo/providers/ai"
)

type Pattern interface {
	Execute(llmProvider ai.Provider, options ...func(tool *PatternOptions)) (*ai.ChatResponse, error)
}
