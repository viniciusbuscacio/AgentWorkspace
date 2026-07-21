package domain

import (
	"testing"
	"time"
)

func TestLogRetentionCutoffUsesCalendarDays(t *testing.T) {
	now := time.Date(2026, 6, 19, 15, 30, 0, 0, time.UTC)
	if got, want := LogRetentionCutoffAt(now, 1), "2026-06-19T00:00:00Z"; got != want {
		t.Fatalf("LogRetentionCutoffAt(1) = %q, want %q", got, want)
	}
	if got, want := LogRetentionCutoffAt(now, 7), "2026-06-13T00:00:00Z"; got != want {
		t.Fatalf("LogRetentionCutoffAt(7) = %q, want %q", got, want)
	}
}
