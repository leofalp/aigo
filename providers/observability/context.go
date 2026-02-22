package observability

import "context"

// contextKey is a private type for context keys to avoid collisions
type contextKey string

const (
	spanContextKey     contextKey = "span"
	observerContextKey contextKey = "observer"
)

// SpanFromContext extracts the active [Span] stored in ctx.
// Returns nil if ctx is nil or no span has been attached.
func SpanFromContext(ctx context.Context) Span {
	if ctx == nil {
		return nil
	}
	span, _ := ctx.Value(spanContextKey).(Span)
	return span
}

// ContextWithSpan returns a copy of ctx carrying span so that it can be
// retrieved later with [SpanFromContext]. If ctx is nil, context.Background
// is used as the parent.
func ContextWithSpan(ctx context.Context, span Span) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, spanContextKey, span)
}

// ObserverFromContext extracts the [Provider] stored in ctx.
// Returns nil if ctx is nil or no observer has been attached.
func ObserverFromContext(ctx context.Context) Provider {
	if ctx == nil {
		return nil
	}
	observer, _ := ctx.Value(observerContextKey).(Provider)
	return observer
}

// ContextWithObserver returns a copy of ctx carrying observer so that it can
// be retrieved later with [ObserverFromContext]. If ctx is nil,
// context.Background is used as the parent.
func ContextWithObserver(ctx context.Context, observer Provider) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, observerContextKey, observer)
}
