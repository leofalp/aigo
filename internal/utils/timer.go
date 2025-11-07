package utils

import "time"

type Timer struct {
	startTime time.Time
	duration  time.Duration
}

func NewTimer() *Timer {
	return &Timer{startTime: time.Now()}
}

func (t *Timer) Start() {
	t.startTime = time.Now()
}

func (t *Timer) Stop() {
	t.duration = time.Since(t.startTime)
}

func (t *Timer) GetDuration() time.Duration {
	return t.duration
}
