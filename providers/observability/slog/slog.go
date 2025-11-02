package slog

import (
	"aigo/providers/observability"
	"context"
	"log/slog"
	"sync"
	"time"
)

// Observer implements observability.Provider using Go's standard library slog
type Observer struct {
	logger  *slog.Logger
	metrics *metricsStore
}

// New creates a new slog-based observer
func New(logger *slog.Logger) *Observer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Observer{
		logger:  logger,
		metrics: newMetricsStore(),
	}
}

// Ensure Observer implements observability.Provider
var _ observability.Provider = (*Observer)(nil)

// --- TRACING ---

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
	// Use Info level for span end to make it visible at INFO level
	s.logger.LogAttrs(context.Background(), slog.LevelInfo, "Span ended", logAttrs...)
}

func (s *slogSpan) SetAttributes(attrs ...observability.Attribute) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attrs = append(s.attrs, attrs...)
}

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

func (o *Observer) Counter(name string) observability.Counter {
	return o.metrics.getCounter(name, o.logger)
}

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

func (o *Observer) Trace(ctx context.Context, msg string, attrs ...observability.Attribute) {
	// Trace is more verbose than Debug, use Debug-4 (which is typically filtered out unless explicitly enabled)
	o.log(ctx, slog.LevelDebug-4, msg, attrs...)
}

func (o *Observer) Debug(ctx context.Context, msg string, attrs ...observability.Attribute) {
	o.log(ctx, slog.LevelDebug, msg, attrs...)
}

func (o *Observer) Info(ctx context.Context, msg string, attrs ...observability.Attribute) {
	o.log(ctx, slog.LevelInfo, msg, attrs...)
}

func (o *Observer) Warn(ctx context.Context, msg string, attrs ...observability.Attribute) {
	o.log(ctx, slog.LevelWarn, msg, attrs...)
}

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
