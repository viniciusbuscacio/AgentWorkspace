package application

import (
	"fmt"
	"time"

	"aw/internal/domain/ports"
)

const AutoLockDefaultMinutes = 0 // Never — disabled by default

type autoLockVault interface {
	ports.VaultUnlockState
	Lock() error
}

func RecordAutoLockActivity(policy ports.AutoLockPolicy, now time.Time) {
	if policy != nil {
		policy.Touch(now)
	}
}

func GetAutoLockMinutes(policy ports.AutoLockPolicy) int {
	if policy == nil {
		return AutoLockDefaultMinutes
	}
	return policy.TimeoutMinutes()
}

func SetAutoLockMinutes(policy ports.AutoLockPolicy, config ports.AppConfigStore, minutes int) error {
	if policy == nil {
		return fmt.Errorf("auto-lock is not available")
	}
	policy.SetTimeoutMinutes(minutes)
	if config == nil {
		return nil
	}
	return config.SaveAutoLockMinutes(policy.TimeoutMinutes())
}

func ShouldLockVaultIfInactive(policy ports.AutoLockPolicy, vault autoLockVault, now time.Time) bool {
	return policy != nil && vault != nil && vault.IsUnlocked() && policy.Expired(now)
}

func LockVaultIfInactive(policy ports.AutoLockPolicy, vault autoLockVault, now time.Time) (bool, error) {
	if !ShouldLockVaultIfInactive(policy, vault, now) {
		return false, nil
	}
	if err := vault.Lock(); err != nil {
		return false, err
	}
	return true, nil
}
