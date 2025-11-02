package observability

import "context"

// contextKey is a private type for context keys to avoid collisions
type contextKey struct{}

var spanContextKey = contextKey{}

// SpanFromContext extracts a Span from the context.
// Returns nil if no span is present.
func SpanFromContext(ctx context.Context) Span {
	if ctx == nil {
		return nil
	}
	span, _ := ctx.Value(spanContextKey).(Span)
	return span
}

// ContextWithSpan returns a new context with the given span attached.
func ContextWithSpan(ctx context.Context, span Span) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, spanContextKey, span)
}
