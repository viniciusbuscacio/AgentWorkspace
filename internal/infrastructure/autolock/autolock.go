// Package autolock tracks user activity and decides when an unlocked vault
// should be automatically locked after a period of inactivity.
package autolock

import (
	"sync"
	"time"
)

// DefaultMinutes is the fallback inactivity timeout when none is configured.
const DefaultMinutes = 15

// Locker tracks the last activity timestamp and reports when the configured
// inactivity timeout has elapsed. It is safe for concurrent use.
type Locker struct {
	mu      sync.Mutex
	timeout time.Duration
	last    time.Time
}

// New creates a Locker. Zero or negative minutes means auto-lock is disabled
// (Expired always returns false). Use 0 for the "Never" option.
func New(minutes int) *Locker {
	return &Locker{timeout: minutesToDuration(minutes)}
}

func minutesToDuration(minutes int) time.Duration {
	if minutes <= 0 {
		return 0 // 0 = disabled (Never)
	}
	return time.Duration(minutes) * time.Minute
}

// Touch records activity at the given time, resetting the inactivity window.
func (l *Locker) Touch(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.last = now
}

// SetTimeoutMinutes updates the inactivity timeout.
func (l *Locker) SetTimeoutMinutes(minutes int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.timeout = minutesToDuration(minutes)
}

// TimeoutMinutes returns the current timeout in whole minutes.
func (l *Locker) TimeoutMinutes() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return int(l.timeout / time.Minute)
}

// Expired reports whether the inactivity window has elapsed by the given time.
// Returns false when timeout is 0 (disabled/Never) or before the first Touch.
func (l *Locker) Expired(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.timeout == 0 || l.last.IsZero() {
		return false
	}
	return now.Sub(l.last) >= l.timeout
}
