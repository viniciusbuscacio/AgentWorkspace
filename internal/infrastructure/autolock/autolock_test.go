package autolock

import (
	"testing"
	"time"
)

func TestExpiredFalseBeforeFirstTouch(t *testing.T) {
	l := New(15)
	if l.Expired(time.Now()) {
		t.Fatal("Expired should be false before any activity baseline")
	}
}

func TestExpiredAfterTimeout(t *testing.T) {
	l := New(15)
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	l.Touch(base)

	if l.Expired(base.Add(14 * time.Minute)) {
		t.Fatal("should not be expired before timeout")
	}
	if !l.Expired(base.Add(15 * time.Minute)) {
		t.Fatal("should be expired at timeout")
	}
	if !l.Expired(base.Add(30 * time.Minute)) {
		t.Fatal("should remain expired after timeout")
	}
}

func TestTouchResetsWindow(t *testing.T) {
	l := New(10)
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	l.Touch(base)
	l.Touch(base.Add(9 * time.Minute))
	if l.Expired(base.Add(10 * time.Minute)) {
		t.Fatal("touch should reset the inactivity window")
	}
	if !l.Expired(base.Add(19 * time.Minute)) {
		t.Fatal("should expire 10 minutes after the latest touch")
	}
}

func TestZeroAndNegativeYieldDisabled(t *testing.T) {
	if got := New(0).TimeoutMinutes(); got != 0 {
		t.Fatalf("TimeoutMinutes() = %d, want 0 (disabled)", got)
	}
	if got := New(-5).TimeoutMinutes(); got != 0 {
		t.Fatalf("negative TimeoutMinutes() = %d, want 0 (disabled)", got)
	}
}

func TestNeverDisablesAutoLock(t *testing.T) {
	l := New(0) // Never
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	l.Touch(base)
	if l.Expired(base.Add(999 * time.Hour)) {
		t.Fatal("Expired should always be false when timeout is 0 (Never)")
	}
}

func TestSetTimeoutMinutes(t *testing.T) {
	l := New(15)
	l.SetTimeoutMinutes(2)
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	l.Touch(base)
	if !l.Expired(base.Add(2 * time.Minute)) {
		t.Fatal("should expire after updated 2-minute timeout")
	}
}
