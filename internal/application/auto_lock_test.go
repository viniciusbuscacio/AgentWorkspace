package application

import (
	"errors"
	"testing"
	"time"
)

type fakeAutoLockPolicy struct {
	touchedAt time.Time
	minutes   int
	expired   bool
}

func (policy *fakeAutoLockPolicy) Touch(now time.Time) {
	policy.touchedAt = now
}

func (policy *fakeAutoLockPolicy) SetTimeoutMinutes(minutes int) {
	policy.minutes = minutes
}

func (policy *fakeAutoLockPolicy) TimeoutMinutes() int {
	return policy.minutes
}

func (policy *fakeAutoLockPolicy) Expired(time.Time) bool {
	return policy.expired
}

type fakeAutoLockVault struct {
	unlocked bool
	locked   bool
	err      error
}

func (vault *fakeAutoLockVault) IsUnlocked() bool {
	return vault.unlocked
}

func (vault *fakeAutoLockVault) Lock() error {
	vault.locked = true
	return vault.err
}

type fakeAutoLockConfig struct {
	minutes int
}

func (config *fakeAutoLockConfig) SaveVaultDir(string) error {
	return nil
}

func (config *fakeAutoLockConfig) SaveAutoLockMinutes(minutes int) error {
	config.minutes = minutes
	return nil
}

func TestAutoLockUseCases(t *testing.T) {
	now := time.Date(2026, 6, 8, 20, 0, 0, 0, time.UTC)
	policy := &fakeAutoLockPolicy{minutes: 15}
	RecordAutoLockActivity(policy, now)
	if !policy.touchedAt.Equal(now) {
		t.Fatalf("Touch recorded %v, want %v", policy.touchedAt, now)
	}

	config := &fakeAutoLockConfig{}
	if err := SetAutoLockMinutes(policy, config, 30); err != nil {
		t.Fatalf("SetAutoLockMinutes() error = %v", err)
	}
	if policy.minutes != 30 || config.minutes != 30 {
		t.Fatalf("minutes policy/config = %d/%d, want 30/30", policy.minutes, config.minutes)
	}
}

func TestLockVaultIfInactive(t *testing.T) {
	policy := &fakeAutoLockPolicy{expired: true}
	vault := &fakeAutoLockVault{unlocked: true}
	locked, err := LockVaultIfInactive(policy, vault, time.Now())
	if err != nil || !locked || !vault.locked {
		t.Fatalf("LockVaultIfInactive() = %v, %v, locked=%v; want locked", locked, err, vault.locked)
	}

	want := errors.New("boom")
	vault = &fakeAutoLockVault{unlocked: true, err: want}
	locked, err = LockVaultIfInactive(policy, vault, time.Now())
	if !errors.Is(err, want) || locked {
		t.Fatalf("LockVaultIfInactive() = %v, %v; want propagated error without locked=true", locked, err)
	}
}
