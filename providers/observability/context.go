package observability

import "context"

// contextKey is a private type for context keys to avoid collisions
type contextKey string

const (
	spanContextKey     contextKey = "span"
	observerContextKey contextKey = "observer"
)

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

// ObserverFromContext extracts an Observer (Provider) from the context.
// Returns nil if no observer is present.
func ObserverFromContext(ctx context.Context) Provider {
	if ctx == nil {
		return nil
	}
	observer, _ := ctx.Value(observerContextKey).(Provider)
	return observer
}

// ContextWithObserver returns a new context with the given observer attached.
func ContextWithObserver(ctx context.Context, observer Provider) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, observerContextKey, observer)
}
