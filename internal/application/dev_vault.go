package application

import (
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

const DefaultDevVaultProfileName = "1234"
const DefaultDevVaultPassword = "1234"

type DevVaultUnlockInput struct {
	ProfileName      string
	Password         string
	CurrentProfileID string
}

type DevVaultUnlockResult struct {
	Profile          domain.ProfileInfo
	CurrentProfileID string
	Created          bool
}

type devVaultStore interface {
	ports.VaultLifecycle
	ports.VaultProfileStore
}

// EnsureDevVaultUnlocked bootstraps and unlocks a throwaway development vault.
// Pass a profile manager and vault rooted in an isolated data directory; this
// use case intentionally does not write app config, so it cannot repoint a
// production profile.
func EnsureDevVaultUnlocked(vault devVaultStore, profiles ports.ProfileStore, input DevVaultUnlockInput) (DevVaultUnlockResult, error) {
	if vault == nil {
		return DevVaultUnlockResult{}, fmt.Errorf("vault is required")
	}
	if profiles == nil {
		return DevVaultUnlockResult{}, fmt.Errorf("profile store is required")
	}
	name := strings.TrimSpace(input.ProfileName)
	if name == "" {
		name = DefaultDevVaultProfileName
	}
	password := strings.TrimSpace(input.Password)
	if password == "" {
		password = DefaultDevVaultPassword
	}
	info, err := EnsureProfileByName(profiles, name)
	if err != nil {
		return DevVaultUnlockResult{}, err
	}
	selection, err := ApplyProfile(vault, nil, input.CurrentProfileID, info)
	if err != nil {
		return DevVaultUnlockResult{}, err
	}
	created := false
	if !vault.Status().Exists {
		if _, err := CreateVaultForProfile(vault, profiles, selection.CurrentProfileID, password); err != nil {
			return DevVaultUnlockResult{}, err
		}
		created = true
	}
	if err := UnlockVaultForProfile(vault, profiles, selection.CurrentProfileID, password); err != nil {
		return DevVaultUnlockResult{}, err
	}
	info, _ = profiles.GetInfo(selection.CurrentProfileID)
	return DevVaultUnlockResult{
		Profile:          info,
		CurrentProfileID: selection.CurrentProfileID,
		Created:          created,
	}, nil
}
