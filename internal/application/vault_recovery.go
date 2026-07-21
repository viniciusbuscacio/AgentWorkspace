package application

import (
	"fmt"

	"aw/internal/domain/ports"
)

func RecoverVault(vault ports.VaultRecoveryStore, recoveryKey string, newPassword string) (string, error) {
	if vault == nil {
		return "", fmt.Errorf("vault recovery store is required")
	}
	return vault.RecoverWithKey(recoveryKey, newPassword)
}

func ChangeVaultPassword(vault ports.VaultRecoveryStore, currentPassword string, newPassword string) (string, error) {
	if vault == nil {
		return "", fmt.Errorf("vault recovery store is required")
	}
	return vault.ChangePassword(currentPassword, newPassword)
}

func GenerateVaultRecoveryKey(vault ports.VaultRecoveryStore) (string, error) {
	if vault == nil {
		return "", fmt.Errorf("vault recovery store is required")
	}
	return vault.GenerateRecoveryKey()
}

func VerifyVaultRecoveryKey(vault ports.VaultRecoveryStore, recoveryKey string) (bool, error) {
	if vault == nil {
		return false, fmt.Errorf("vault recovery store is required")
	}
	return vault.VerifyRecoveryKey(recoveryKey), nil
}
