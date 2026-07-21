package application

import (
	"errors"
	"testing"

	"aw/internal/domain"
)

type fakeVaultLifecycle struct {
	status       domain.VaultStatus
	recoveryKey  string
	err          error
	createdWith  string
	unlockedWith string
	locked       bool
}

func (vault *fakeVaultLifecycle) Status() domain.VaultStatus {
	return vault.status
}

func (vault *fakeVaultLifecycle) Create(password string) (string, error) {
	vault.createdWith = password
	return vault.recoveryKey, vault.err
}

func (vault *fakeVaultLifecycle) Unlock(password string) error {
	vault.unlockedWith = password
	return vault.err
}

func (vault *fakeVaultLifecycle) Lock() error {
	vault.locked = true
	return vault.err
}

func TestVaultLifecycleUseCasesCallPort(t *testing.T) {
	vault := &fakeVaultLifecycle{
		status:      domain.VaultStatus{Exists: true, Unlocked: false},
		recoveryKey: "recovery",
	}
	status, err := VaultStatus(vault)
	if err != nil || !status.Exists {
		t.Fatalf("VaultStatus() = %+v, %v; want existing status", status, err)
	}
	key, err := CreateVault(vault, "senha")
	if err != nil || key != "recovery" || vault.createdWith != "senha" {
		t.Fatalf("CreateVault() = %q, %v, createdWith=%q", key, err, vault.createdWith)
	}
	if err := UnlockVault(vault, "senha"); err != nil || vault.unlockedWith != "senha" {
		t.Fatalf("UnlockVault() = %v, unlockedWith=%q", err, vault.unlockedWith)
	}
	if err := LockVault(vault); err != nil || !vault.locked {
		t.Fatalf("LockVault() = %v, locked=%v", err, vault.locked)
	}
}

func TestVaultLifecycleUseCasesRejectMissingVault(t *testing.T) {
	for name, run := range map[string]func() error{
		"status": func() error {
			_, err := VaultStatus(nil)
			return err
		},
		"create": func() error {
			_, err := CreateVault(nil, "senha")
			return err
		},
		"unlock": func() error { return UnlockVault(nil, "senha") },
		"lock":   func() error { return LockVault(nil) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); err == nil {
				t.Fatalf("expected missing vault error")
			}
		})
	}
}

func TestVaultLifecycleUseCasesReturnPortErrors(t *testing.T) {
	want := errors.New("boom")
	vault := &fakeVaultLifecycle{err: want}
	if _, err := CreateVault(vault, "senha"); !errors.Is(err, want) {
		t.Fatalf("CreateVault error = %v, want %v", err, want)
	}
	if err := UnlockVault(vault, "senha"); !errors.Is(err, want) {
		t.Fatalf("UnlockVault error = %v, want %v", err, want)
	}
	if err := LockVault(vault); !errors.Is(err, want) {
		t.Fatalf("LockVault error = %v, want %v", err, want)
	}
}
