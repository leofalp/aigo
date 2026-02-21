// Package observability defines the core interfaces and semantic conventions
// used for distributed tracing, metrics collection, and structured logging
// throughout the aigo library.
//
// The central entry point is [Provider], which composes [Tracer], [Metrics],
// and [Logger] into a single injectable dependency. Callers propagate an active
// [Provider] and [Span] through a [context.Context] using [ContextWithObserver]
// and [ContextWithSpan]; they can be retrieved with [ObserverFromContext] and
// [SpanFromContext].
//
// The semconv.go file contains all standard attribute-key and span-name
// constants that should be used when recording observations, ensuring
// consistency across providers and components.
package observability
