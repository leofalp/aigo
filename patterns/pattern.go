package patterns

import (
	"context"

	"github.com/leofalp/aigo/providers/ai"
)

type Pattern interface {
	Execute(ctx context.Context, prompt string) (*ai.Overview, error)
}
