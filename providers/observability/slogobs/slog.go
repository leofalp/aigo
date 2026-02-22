package slogobs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/leofalp/aigo/providers/observability"
)

// Observer implements observability.Provider using Go's standard library slog.
// It routes tracing, metrics, and log events through a structured slog.Logger,
// making it suitable for lightweight observability without external dependencies.
type Observer struct {
	logger  *slog.Logger
	metrics *metricsStore
}

// New creates a new slog-based observer with functional options.
// If no options are provided, it uses environment variables for configuration
// (AIGO_LOG_FORMAT and AIGO_LOG_LEVEL), defaulting to compact format and INFO level.
//
// Example usage:
//
//	// Use defaults from environment
//	observer := slogobs.New()
//
//	// Explicit configuration
//	observer := slogobs.New(
//	    slogobs.WithFormat(slogobs.FormatCompact),
//	    slogobs.WithLevel(slog.LevelDebug),
//	)
//
//	// Use existing logger
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	observer := slogobs.New(slogobs.WithLogger(logger))
func New(opts ...Option) *Observer {
	cfg := applyOptions(opts...)

	var logger *slog.Logger
	if cfg.logger != nil {
		// Use provided logger
		logger = cfg.logger
	} else {
		// Create custom handler with specified format
		handler := NewHandler(&HandlerOptions{
			Format: cfg.format,
			Level:  cfg.level,
			Output: cfg.output,
			Colors: cfg.colors,
		})
		logger = slog.New(handler)
	}

	return &Observer{
		logger:  logger,
		metrics: newMetricsStore(),
	}
}

// Ensure Observer implements observability.Provider
var _ observability.Provider = (*Observer)(nil)

// --- TRACING ---

// StartSpan begins a new named span and emits a debug log event at its start.
// It attaches the provided attributes to the span for the duration of its lifetime.
// The returned context is unchanged; the returned Span's End method logs the
// elapsed duration. Use SetAttributes, SetStatus, RecordError, and AddEvent on
// the Span to enrich it before calling End.
func (o *Observer) StartSpan(ctx context.Context, name string, attrs ...observability.Attribute) (context.Context, observability.Span) {
	span := &slogSpan{
		name:      name,
		startTime: time.Now(),
		logger:    o.logger,
		attrs:     attrs,
	}

	// Log span start
	logAttrs := []slog.Attr{
		slog.String("span", name),
		slog.String("event", "span.start"),
	}
	for _, attr := range attrs {
		logAttrs = append(logAttrs, slog.Any(attr.Key, attr.Value))
	}
	o.logger.LogAttrs(ctx, slog.LevelDebug, "Span started", logAttrs...)

	return ctx, span
}

type slogSpan struct {
	name      string
	startTime time.Time
	logger    *slog.Logger
	attrs     []observability.Attribute
	mu        sync.Mutex
}

// End completes the span by recording the elapsed time and any accumulated attributes,
// then logging the span end event at debug level.
func (s *slogSpan) End() {
	s.mu.Lock()
	defer s.mu.Unlock()

	duration := time.Since(s.startTime)
	logAttrs := []slog.Attr{
		slog.String("span", s.name),
		slog.String("event", "span.end"),
		slog.Duration("duration", duration),
	}
	for _, attr := range s.attrs {
		logAttrs = append(logAttrs, slog.Any(attr.Key, attr.Value))
	}
	// Use Debug level for span end to reduce log verbosity
	s.logger.LogAttrs(context.Background(), slog.LevelDebug, "Span ended", logAttrs...)
}

// SetAttributes appends the provided attributes to the span's attribute list.
func (s *slogSpan) SetAttributes(attrs ...observability.Attribute) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attrs = append(s.attrs, attrs...)
}

// SetStatus records the final status of the span using the provided code and optional description.
func (s *slogSpan) SetStatus(code observability.StatusCode, description string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var statusStr string
	switch code {
	case observability.StatusOK:
		statusStr = "ok"
	case observability.StatusError:
		statusStr = "error"
	default:
		statusStr = "unset"
	}

	s.attrs = append(s.attrs, observability.String(observability.AttrStatus, statusStr))
	if description != "" {
		s.attrs = append(s.attrs, observability.String(observability.AttrStatusDescription, description))
	}
}

// RecordError records the provided error as an exception event on the span and logs it at error level.
func (s *slogSpan) RecordError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.attrs = append(s.attrs, observability.Error(err))

	logAttrs := []slog.Attr{
		slog.String("span", s.name),
		slog.String("event", "error"),
		slog.String("error", err.Error()),
	}
	s.logger.LogAttrs(context.Background(), slog.LevelError, "Span error", logAttrs...)
}

// AddEvent appends a named event with optional attributes to the span's timeline by logging it at debug level.
func (s *slogSpan) AddEvent(name string, attrs ...observability.Attribute) {
	s.mu.Lock()
	defer s.mu.Unlock()

	logAttrs := []slog.Attr{
		slog.String("span", s.name),
		slog.String("event", name),
	}
	for _, attr := range attrs {
		logAttrs = append(logAttrs, slog.Any(attr.Key, attr.Value))
	}
	s.logger.LogAttrs(context.Background(), slog.LevelDebug, "Span event", logAttrs...)
}

// --- METRICS ---

// Counter returns a named observability.Counter backed by the in-memory metrics
// store. Multiple calls with the same name return the same counter instance,
// so callers can safely fetch it on every use without caching.
// Each Add call emits a debug log entry reporting the delta and cumulative value.
func (o *Observer) Counter(name string) observability.Counter {
	return o.metrics.getCounter(name, o.logger)
}

// Histogram returns a named observability.Histogram backed by the in-memory
// metrics store. Multiple calls with the same name return the same histogram
// instance. Each Record call emits a debug log entry with the observed value.
func (o *Observer) Histogram(name string) observability.Histogram {
	return o.metrics.getHistogram(name, o.logger)
}

// metricsStore holds metrics in memory (thread-safe)
type metricsStore struct {
	mu         sync.RWMutex
	counters   map[string]*slogCounter
	histograms map[string]*slogHistogram
}

func newMetricsStore() *metricsStore {
	return &metricsStore{
		counters:   make(map[string]*slogCounter),
		histograms: make(map[string]*slogHistogram),
	}
}

func (m *metricsStore) getCounter(name string, logger *slog.Logger) *slogCounter {
	m.mu.RLock()
	counter, exists := m.counters[name]
	m.mu.RUnlock()

	if exists {
		return counter
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if counter, exists := m.counters[name]; exists {
		return counter
	}

	counter = &slogCounter{name: name, logger: logger}
	m.counters[name] = counter
	return counter
}

func (m *metricsStore) getHistogram(name string, logger *slog.Logger) *slogHistogram {
	m.mu.RLock()
	histogram, exists := m.histograms[name]
	m.mu.RUnlock()

	if exists {
		return histogram
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if histogram, exists := m.histograms[name]; exists {
		return histogram
	}

	histogram = &slogHistogram{name: name, logger: logger}
	m.histograms[name] = histogram
	return histogram
}

type slogCounter struct {
	name   string
	logger *slog.Logger
	mu     sync.Mutex
	value  int64
}

// Add increments the counter by value and logs the updated total at DEBUG level.
// It implements [observability.Counter].
func (c *slogCounter) Add(ctx context.Context, value int64, attrs ...observability.Attribute) {
	c.mu.Lock()
	c.value += value
	currentValue := c.value
	c.mu.Unlock()

	logAttrs := []slog.Attr{
		slog.String("metric", c.name),
		slog.String("type", "counter"),
		slog.Int64("value", currentValue),
		slog.Int64("delta", value),
	}
	for _, attr := range attrs {
		logAttrs = append(logAttrs, slog.Any(attr.Key, attr.Value))
	}
	c.logger.LogAttrs(ctx, slog.LevelDebug, "Counter", logAttrs...)
}

type slogHistogram struct {
	name   string
	logger *slog.Logger
	mu     sync.Mutex
}

// Record logs a histogram observation at DEBUG level.
// It implements [observability.Histogram].
func (h *slogHistogram) Record(ctx context.Context, value float64, attrs ...observability.Attribute) {
	h.mu.Lock()
	defer h.mu.Unlock()

	logAttrs := []slog.Attr{
		slog.String("metric", h.name),
		slog.String("type", "histogram"),
		slog.Float64("value", value),
	}
	for _, attr := range attrs {
		logAttrs = append(logAttrs, slog.Any(attr.Key, attr.Value))
	}
	h.logger.LogAttrs(ctx, slog.LevelDebug, "Histogram", logAttrs...)
}

// --- LOGGING ---

// Trace logs a message at TRACE level (below DEBUG) with optional structured attributes.
// TRACE is the most granular level; it is typically filtered out unless the log
// level is explicitly set to TRACE via [WithLevel] or the AIGO_LOG_LEVEL env var.
func (o *Observer) Trace(ctx context.Context, msg string, attrs ...observability.Attribute) {
	// Trace is more verbose than Debug, use Debug-4 (which is typically filtered out unless explicitly enabled)
	o.log(ctx, slog.LevelDebug-4, msg, attrs...)
}

// Debug logs a message at DEBUG level with optional structured attributes.
// Use this for detailed diagnostic information useful during development.
func (o *Observer) Debug(ctx context.Context, msg string, attrs ...observability.Attribute) {
	o.log(ctx, slog.LevelDebug, msg, attrs...)
}

// Info logs a message at INFO level with optional structured attributes.
// Use this for general operational events that confirm normal behavior.
func (o *Observer) Info(ctx context.Context, msg string, attrs ...observability.Attribute) {
	o.log(ctx, slog.LevelInfo, msg, attrs...)
}

// Warn logs a message at WARN level with optional structured attributes.
// Use this for unexpected situations that are recoverable but worth investigating.
func (o *Observer) Warn(ctx context.Context, msg string, attrs ...observability.Attribute) {
	o.log(ctx, slog.LevelWarn, msg, attrs...)
}

// Error logs a message at ERROR level with optional structured attributes.
// Use this for failures that affect the current operation and require attention.
func (o *Observer) Error(ctx context.Context, msg string, attrs ...observability.Attribute) {
	o.log(ctx, slog.LevelError, msg, attrs...)
}

func (o *Observer) log(ctx context.Context, level slog.Level, msg string, attrs ...observability.Attribute) {
	logAttrs := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		logAttrs = append(logAttrs, slog.Any(attr.Key, attr.Value))
	}
	o.logger.LogAttrs(ctx, level, msg, logAttrs...)
}
