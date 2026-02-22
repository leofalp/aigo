package observability

import (
	"context"
	"time"
)

// Provider is the unified observability interface that composes distributed
// tracing, metrics collection, and structured logging into a single
// injectable dependency. Implementations are passed through a
// [context.Context] via [ContextWithObserver].
type Provider interface {
	Tracer
	Metrics
	Logger
}

// --- TRACING (Distributed Tracing) ---

// Tracer provides distributed tracing capabilities, allowing callers to
// create and manage spans that represent units of work within a trace.
type Tracer interface {
	// StartSpan starts a new span with the given name and optional attributes,
	// returning a derived context that carries the span and the span itself.
	StartSpan(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span)
}

// Span represents a single unit of work within a distributed trace.
// It must be ended exactly once by calling [Span.End].
type Span interface {
	// End completes the span and flushes it to the underlying backend.
	End()
	// SetAttributes attaches key-value metadata to the span.
	SetAttributes(attrs ...Attribute)
	// SetStatus records the final status of the span using a [StatusCode]
	// and an optional human-readable description.
	SetStatus(code StatusCode, description string)
	// RecordError records err as an exception event on the span.
	RecordError(err error)
	// AddEvent appends a named event with optional attributes to the span timeline.
	AddEvent(name string, attrs ...Attribute)
}

// StatusCode represents the completion status of a [Span].
type StatusCode int

const (
	// StatusUnset is the default status, indicating no explicit outcome has been set.
	StatusUnset StatusCode = iota
	// StatusOK indicates the span completed successfully.
	StatusOK
	// StatusError indicates the span completed with an error.
	StatusError
)

// --- METRICS ---

// Metrics provides metrics collection capabilities for recording counts
// and value distributions tied to named instruments.
type Metrics interface {
	// Counter returns a [Counter] instrument for the given name,
	// creating it if it does not already exist.
	Counter(name string) Counter
	// Histogram returns a [Histogram] instrument for the given name,
	// creating it if it does not already exist.
	Histogram(name string) Histogram
}

// Counter is a monotonically increasing metric instrument used to record
// cumulative values such as request counts or error totals.
type Counter interface {
	// Add increments the counter by value, tagging the measurement with
	// the provided context and optional attributes.
	Add(ctx context.Context, value int64, attrs ...Attribute)
}

// Histogram records the statistical distribution of values such as
// latencies or payload sizes.
type Histogram interface {
	// Record observes value for the histogram, tagging the measurement
	// with the provided context and optional attributes.
	Record(ctx context.Context, value float64, attrs ...Attribute)
}

// --- LOGGING (Structured Logging) ---

// Logger provides structured logging capabilities at multiple severity levels.
// Each method accepts a context so that active span information can be
// automatically correlated with log entries by the underlying implementation.
type Logger interface {
	// Trace logs a message at the finest-grained trace level.
	Trace(ctx context.Context, msg string, attrs ...Attribute)
	// Debug logs a message useful for debugging during development.
	Debug(ctx context.Context, msg string, attrs ...Attribute)
	// Info logs a general informational message about normal operation.
	Info(ctx context.Context, msg string, attrs ...Attribute)
	// Warn logs a message indicating a potentially harmful situation.
	Warn(ctx context.Context, msg string, attrs ...Attribute)
	// Error logs a message indicating a failure that should be investigated.
	Error(ctx context.Context, msg string, attrs ...Attribute)
}

// --- ATTRIBUTES (Key-Value pairs) ---

// Attribute represents a key-value pair used to annotate spans, metrics,
// and log entries with structured metadata.
type Attribute struct {
	// Key is the attribute name, typically following dot-separated semantic
	// conventions defined in semconv.go (e.g., "llm.model").
	Key string
	// Value holds the attribute value; supported types include string, int,
	// int64, float64, bool, time.Duration, and []string.
	Value interface{}
}

// String returns an [Attribute] with a string value.
func String(key, value string) Attribute {
	return Attribute{Key: key, Value: value}
}

// Int returns an [Attribute] with an int value.
func Int(key string, value int) Attribute {
	return Attribute{Key: key, Value: value}
}

// Int64 returns an [Attribute] with an int64 value.
func Int64(key string, value int64) Attribute {
	return Attribute{Key: key, Value: value}
}

// Float64 returns an [Attribute] with a float64 value.
func Float64(key string, value float64) Attribute {
	return Attribute{Key: key, Value: value}
}

// Bool returns an [Attribute] with a bool value.
func Bool(key string, value bool) Attribute {
	return Attribute{Key: key, Value: value}
}

// Duration returns an [Attribute] with a [time.Duration] value.
func Duration(key string, value time.Duration) Attribute {
	return Attribute{Key: key, Value: value}
}

// StringSlice returns an [Attribute] with a []string value.
func StringSlice(key string, value []string) Attribute {
	return Attribute{Key: key, Value: value}
}

// Error returns an [Attribute] with key "error" whose value is the result of
// err.Error(). If err is nil, the value is set to an empty string.
func Error(err error) Attribute {
	if err == nil {
		return Attribute{Key: "error", Value: ""}
	}
	return Attribute{Key: "error", Value: err.Error()}
}
