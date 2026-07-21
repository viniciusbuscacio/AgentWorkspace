package application

import (
	"sort"
	"sync"
	"time"
)

// ProviderCooldown is an in-memory circuit breaker for the LLM fallback chain.
// When a provider fails with a failover-class error (see domain.IsFailoverError)
// it is benched so subsequent turns skip it instead of hammering a
// rate-limited, out-of-credit or down account. A negative duration (the
// default) benches until the app restarts; a positive one benches for that
// long; zero disables benching.
//
// State is per-process and intentionally NOT persisted: closing the app clears
// every cooldown (a deliberate product decision — restart is the reset lever).
// The bench duration itself is configurable and persisted separately in
// appconfig.
type ProviderCooldown struct {
	mu       sync.Mutex
	until    map[string]time.Time
	duration time.Duration
	now      func() time.Time
}

// NewProviderCooldown builds a circuit breaker with the given bench duration.
// Negative benches until restart, zero disables benching (Penalize no-ops).
func NewProviderCooldown(duration time.Duration) *ProviderCooldown {
	return &ProviderCooldown{
		until:    make(map[string]time.Time),
		duration: duration,
		now:      time.Now,
	}
}

// Duration returns the current bench duration.
func (c *ProviderCooldown) Duration() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.duration
}

// SetDuration updates the bench duration applied to future penalties. Existing
// benches keep their original expiry.
func (c *ProviderCooldown) SetDuration(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.duration = duration
}

// Penalize benches a provider until now()+duration. A negative duration
// benches until the app restarts (the breaker is in-memory, so a far-future
// deadline is exactly that); zero is a no-op so an operator can disable the
// breaker entirely.
func (c *ProviderCooldown) Penalize(providerID string) {
	if providerID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.duration == 0 {
		return
	}
	if c.duration < 0 {
		c.until[providerID] = c.now().Add(100 * 365 * 24 * time.Hour)
		return
	}
	c.until[providerID] = c.now().Add(c.duration)
}

// Active reports whether the provider is currently benched. Expired entries are
// cleaned up lazily on read.
func (c *ProviderCooldown) Active(providerID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	deadline, ok := c.until[providerID]
	if !ok {
		return false
	}
	if !c.now().Before(deadline) {
		delete(c.until, providerID)
		return false
	}
	return true
}

// Remaining returns how long until the provider leaves cooldown, or 0 if it is
// not benched.
func (c *ProviderCooldown) Remaining(providerID string) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	deadline, ok := c.until[providerID]
	if !ok {
		return 0
	}
	remaining := deadline.Sub(c.now())
	if remaining <= 0 {
		delete(c.until, providerID)
		return 0
	}
	return remaining
}

// Clear removes a provider's bench (e.g. after the user reconfigures it).
func (c *ProviderCooldown) Clear(providerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.until, providerID)
}

// Reset clears every bench.
func (c *ProviderCooldown) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.until = make(map[string]time.Time)
}

// Benched returns the provider IDs currently in cooldown, sorted for stable
// output. Useful for status/UI surfaces.
func (c *ProviderCooldown) Benched() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	ids := make([]string, 0, len(c.until))
	for id, deadline := range c.until {
		if now.Before(deadline) {
			ids = append(ids, id)
		} else {
			delete(c.until, id)
		}
	}
	sort.Strings(ids)
	return ids
}
