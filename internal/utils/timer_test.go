package utils

import (
	"testing"
	"time"
)

// TestNewTimer verifies that NewTimer starts the timer immediately so that
// Stop captures a non-zero duration.
func TestNewTimer_StartsImmediately(t *testing.T) {
	timer := NewTimer()
	time.Sleep(time.Millisecond)
	timer.Stop()

	if timer.GetDuration() <= 0 {
		t.Errorf("NewTimer + Stop: expected positive duration, got %v", timer.GetDuration())
	}
}

// TestTimer_GetDuration_BeforeStop verifies that GetDuration returns zero when
// Stop has not yet been called (the default zero value of time.Duration).
func TestTimer_GetDuration_BeforeStop(t *testing.T) {
	timer := NewTimer()
	// Do not call Stop.
	if timer.GetDuration() != 0 {
		t.Errorf("GetDuration() before Stop = %v, want 0", timer.GetDuration())
	}
}

// TestTimer_Start_Restart verifies that calling Start resets the measurement
// so a subsequent Stop captures time elapsed since the restart, not since
// construction.
func TestTimer_Start_Restart(t *testing.T) {
	timer := NewTimer()
	time.Sleep(5 * time.Millisecond)
	timer.Stop()
	firstDuration := timer.GetDuration()

	// Restart the timer and capture a much shorter interval.
	timer.Start()
	timer.Stop()
	secondDuration := timer.GetDuration()

	// The second measurement should be strictly shorter than the first because
	// the first included the 5 ms sleep.
	if secondDuration >= firstDuration {
		t.Errorf("after Start() + immediate Stop(), duration %v should be less than %v",
			secondDuration, firstDuration)
	}
}

// TestTimer_Stop_MultipleCalls verifies that calling Stop a second time
// overwrites the duration with the new elapsed time.
func TestTimer_Stop_MultipleCalls(t *testing.T) {
	timer := NewTimer()
	timer.Stop()
	firstDuration := timer.GetDuration()

	time.Sleep(2 * time.Millisecond)
	timer.Stop()
	secondDuration := timer.GetDuration()

	// The second Stop was called after a sleep, so its captured duration should
	// be larger than the first.
	if secondDuration <= firstDuration {
		t.Errorf("second Stop() duration %v should exceed first Stop() duration %v",
			secondDuration, firstDuration)
	}
}
