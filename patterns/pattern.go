package patterns

import (
	"aigo/providers/ai"
	"context"
)

type Pattern interface {
	Execute(ctx context.Context, prompt string) (*ai.ChatResponse, error)
}
