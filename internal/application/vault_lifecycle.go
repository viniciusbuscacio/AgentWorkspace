package application

import (
	"fmt"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

func VaultStatus(vault ports.VaultLifecycle) (domain.VaultStatus, error) {
	if vault == nil {
		return domain.VaultStatus{}, fmt.Errorf("vault is required")
	}
	return vault.Status(), nil
}

// IsVaultUnlocked reports whether the vault is currently unlocked, via the
// status reader so the interface layer never calls the vault adapter directly.
func IsVaultUnlocked(vault ports.VaultLifecycle) bool {
	return vault != nil && vault.Status().Unlocked
}

// VaultExists reports whether a vault exists on disk.
func VaultExists(vault ports.VaultLifecycle) bool {
	return vault != nil && vault.Status().Exists
}

func CreateVault(vault ports.VaultLifecycle, password string) (string, error) {
	if vault == nil {
		return "", fmt.Errorf("vault is required")
	}
	return vault.Create(password)
}

func UnlockVault(vault ports.VaultLifecycle, password string) error {
	if vault == nil {
		return fmt.Errorf("vault is required")
	}
	return vault.Unlock(password)
}

func LockVault(vault ports.VaultLifecycle) error {
	if vault == nil {
		return fmt.Errorf("vault is required")
	}
	return vault.Lock()
}

// VaultPasswordVerifier checks a password against an unlocked vault without
// changing its lock state. Web mode authenticates sessions through it.
type VaultPasswordVerifier interface {
	VerifyPassword(password string) error
}

// VerifyVaultPassword authenticates password against an already-unlocked vault.
// It is the explicit-verification half of the web login contract: when the
// vault is unlocked, /login verifies the password here rather than calling
// Unlock (which returns nil for any password once open).
func VerifyVaultPassword(vault VaultPasswordVerifier, password string) error {
	if vault == nil {
		return fmt.Errorf("vault is required")
	}
	return vault.VerifyPassword(password)
}
