package patterns

import (
	"context"
)

type Pattern interface {
	Execute(ctx context.Context, prompt string) (*Overview, error)
}
