package utils

import "time"

// Timer measures elapsed wall-clock time between a start and stop event.
// Create one with [NewTimer], which starts the timer immediately. Call
// [Timer.Stop] to capture the elapsed duration, then retrieve it with
// [Timer.GetDuration].
type Timer struct {
	startTime time.Time
	duration  time.Duration
}

// NewTimer creates a new Timer and immediately starts it by recording the
// current time as the start instant.
func NewTimer() *Timer {
	return &Timer{startTime: time.Now()}
}

// Start resets the timer's start time to now, beginning a fresh measurement.
// It can be called multiple times to restart the timer without allocating a
// new instance.
func (t *Timer) Start() {
	t.startTime = time.Now()
}

// Stop records the elapsed time since the last call to [Timer.Start] (or since
// construction via [NewTimer]). The captured duration is available via
// [Timer.GetDuration].
func (t *Timer) Stop() {
	t.duration = time.Since(t.startTime)
}

// GetDuration returns the duration captured by the most recent call to
// [Timer.Stop]. If Stop has not been called yet, it returns zero.
func (t *Timer) GetDuration() time.Duration {
	return t.duration
}
