package application

import (
	"testing"
	"time"
)

func newTestCooldown(d time.Duration) (*ProviderCooldown, *time.Time) {
	c := NewProviderCooldown(d)
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return clock }
	return c, &clock
}

func TestProviderCooldownPenalizeAndExpire(t *testing.T) {
	c, clock := newTestCooldown(30 * time.Minute)

	if c.Active("openrouter") {
		t.Fatal("provider should not start benched")
	}
	c.Penalize("openrouter")
	if !c.Active("openrouter") {
		t.Fatal("provider should be benched right after Penalize")
	}
	if got := c.Remaining("openrouter"); got != 30*time.Minute {
		t.Fatalf("Remaining = %v, want 30m", got)
	}

	// Advance 29 minutes: still benched.
	*clock = clock.Add(29 * time.Minute)
	if !c.Active("openrouter") {
		t.Fatal("provider should still be benched at 29m")
	}

	// Advance past 30 minutes: cooldown expired, retried automatically.
	*clock = clock.Add(2 * time.Minute)
	if c.Active("openrouter") {
		t.Fatal("provider should leave cooldown after 30m")
	}
	if got := c.Remaining("openrouter"); got != 0 {
		t.Fatalf("Remaining after expiry = %v, want 0", got)
	}
}

func TestProviderCooldownDisabledWhenZero(t *testing.T) {
	c, _ := newTestCooldown(0)
	c.Penalize("openrouter")
	if c.Active("openrouter") {
		t.Fatal("duration 0 must disable benching")
	}
}

func TestProviderCooldownNegativeBenchesUntilRestart(t *testing.T) {
	c, clock := newTestCooldown(-1)
	c.Penalize("openrouter")
	if !c.Active("openrouter") {
		t.Fatal("provider should be benched right after Penalize")
	}
	// Even a year later (same process), the provider stays benched: only an
	// app restart (fresh in-memory breaker) retries it.
	*clock = clock.Add(365 * 24 * time.Hour)
	if !c.Active("openrouter") {
		t.Fatal("until-restart bench must not expire within the process lifetime")
	}
	// The user reconfiguring the provider still clears it explicitly.
	c.Clear("openrouter")
	if c.Active("openrouter") {
		t.Fatal("Clear must lift an until-restart bench")
	}
}

func TestProviderCooldownClearAndReset(t *testing.T) {
	c, _ := newTestCooldown(30 * time.Minute)
	c.Penalize("a")
	c.Penalize("b")
	c.Clear("a")
	if c.Active("a") {
		t.Fatal("Clear should remove bench for a")
	}
	if !c.Active("b") {
		t.Fatal("Clear(a) must not affect b")
	}
	c.Reset()
	if c.Active("b") {
		t.Fatal("Reset should clear all benches")
	}
}

func TestProviderCooldownBenchedListSortedAndPruned(t *testing.T) {
	c, clock := newTestCooldown(30 * time.Minute)
	c.Penalize("openrouter")
	c.Penalize("github-copilot")
	got := c.Benched()
	if len(got) != 2 || got[0] != "github-copilot" || got[1] != "openrouter" {
		t.Fatalf("Benched = %v, want sorted [github-copilot openrouter]", got)
	}
	*clock = clock.Add(31 * time.Minute)
	if got := c.Benched(); len(got) != 0 {
		t.Fatalf("Benched after expiry = %v, want empty", got)
	}
}

func TestProviderCooldownSetDurationKeepsExistingExpiry(t *testing.T) {
	c, clock := newTestCooldown(30 * time.Minute)
	c.Penalize("openrouter")
	c.SetDuration(60 * time.Minute)
	// Existing bench keeps its original 30m expiry.
	*clock = clock.Add(31 * time.Minute)
	if c.Active("openrouter") {
		t.Fatal("existing bench should keep original 30m expiry")
	}
	// New penalty uses the new 60m duration.
	c.Penalize("openrouter")
	if got := c.Remaining("openrouter"); got != 60*time.Minute {
		t.Fatalf("Remaining after SetDuration = %v, want 60m", got)
	}
}
