package application

import (
	"fmt"
	"path/filepath"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

const DefaultProfileAvatar = "database_upload"

type ProfileSelectionResult struct {
	Profile          domain.ProfileInfo
	CurrentProfileID string
}

type VaultCreationResult struct {
	CurrentProfileID string
	RecoveryKey      string
}

type ProfileBootstrapInput struct {
	ConfigVaultDir  string
	DefaultVaultDir string
}

type ProfileBootstrapResult struct {
	VaultDir         string
	CurrentProfileID string
}

type vaultCreateStore interface {
	ports.VaultLifecycle
	ports.VaultDirectoryStore
}

func BootstrapProfiles(profiles ports.ProfileBootstrapStore, input ProfileBootstrapInput) ProfileBootstrapResult {
	result := ProfileBootstrapResult{VaultDir: input.DefaultVaultDir}
	if profiles == nil {
		return result
	}
	if input.ConfigVaultDir != "" {
		_ = profiles.MigrateExistingVault(input.ConfigVaultDir)
	}
	if input.DefaultVaultDir != "" {
		_ = profiles.MigrateExistingVault(input.DefaultVaultDir)
	}
	if currentProfile, ok := profiles.GetDefaultInfo(); ok {
		result.VaultDir = currentProfile.VaultDir
		result.CurrentProfileID = currentProfile.ID
	}
	return result
}

func ListProfiles(profiles ports.ProfileReader) []domain.ProfileInfo {
	if profiles == nil {
		return []domain.ProfileInfo{}
	}
	return profiles.ListInfo()
}

// ForgetAllProfiles removes every profile entry from the store — the "clear
// recent vaults" action on the start screen. It only forgets the list entries
// (profiles.json metadata); the vault directories on disk are never touched,
// so any vault can be reopened later via "Open existing vault".
func ForgetAllProfiles(profiles ports.ProfileStore) error {
	if profiles == nil {
		return fmt.Errorf("profile store is required")
	}
	for _, info := range profiles.ListInfo() {
		if err := profiles.Delete(info.ID); err != nil {
			return err
		}
	}
	return nil
}

// EnsureProfileByName returns the first profile with the given name, creating a
// new one (with an auto-generated vault directory) when none exists. It is the
// use-case entry point the dev auto-unlock binary uses to bootstrap a
// throwaway profile without reaching into the profile adapter directly.
func EnsureProfileByName(profiles ports.ProfileStore, name string) (domain.ProfileInfo, error) {
	if profiles == nil {
		return domain.ProfileInfo{}, fmt.Errorf("profile store is required")
	}
	for _, info := range profiles.ListInfo() {
		if info.Name == name {
			return info, nil
		}
	}
	return profiles.Create(name, DefaultProfileAvatar, "")
}

func SelectProfile(profiles ports.ProfileStore, vault ports.VaultProfileStore, config ports.AppConfigStore, currentProfileID string, id string) (ProfileSelectionResult, error) {
	if profiles == nil {
		return ProfileSelectionResult{}, fmt.Errorf("profile store is required")
	}
	info, err := profiles.Touch(id)
	if err != nil {
		return ProfileSelectionResult{}, err
	}
	return ApplyProfile(vault, config, currentProfileID, info)
}

func CreateProfileAtLocation(profiles ports.ProfileStore, vault ports.VaultProfileStore, config ports.AppConfigStore, currentProfileID string, parentDir string, folderName string) (ProfileSelectionResult, error) {
	if profiles == nil {
		return ProfileSelectionResult{}, fmt.Errorf("profile store is required")
	}
	info, err := profiles.CreateAtLocation(parentDir, folderName, DefaultProfileAvatar)
	if err != nil {
		return ProfileSelectionResult{}, err
	}
	return ApplyProfile(vault, config, currentProfileID, info)
}

func ImportVaultProfile(profiles ports.ProfileStore, vault ports.VaultProfileStore, config ports.AppConfigStore, currentProfileID string, vaultDir string) (ProfileSelectionResult, error) {
	if profiles == nil {
		return ProfileSelectionResult{}, fmt.Errorf("profile store is required")
	}
	info, err := profiles.Import(filepath.Base(vaultDir), DefaultProfileAvatar, vaultDir)
	if err != nil {
		return ProfileSelectionResult{}, err
	}
	return ApplyProfile(vault, config, currentProfileID, info)
}

func ApplyProfile(vault ports.VaultProfileStore, config ports.AppConfigStore, currentProfileID string, info domain.ProfileInfo) (ProfileSelectionResult, error) {
	result := ProfileSelectionResult{Profile: info, CurrentProfileID: currentProfileID}
	if info.ID == "" {
		return result, fmt.Errorf("profile is required")
	}
	if vault == nil {
		return result, fmt.Errorf("vault is required")
	}
	if vault.IsUnlocked() {
		if currentProfileID == info.ID {
			return result, nil
		}
		if err := vault.Lock(); err != nil {
			return result, err
		}
	}
	if err := vault.SetDir(info.VaultDir); err != nil {
		return result, err
	}
	if config != nil {
		if err := config.SaveVaultDir(info.VaultDir); err != nil {
			return result, err
		}
	}
	result.CurrentProfileID = info.ID
	return result, nil
}

func CreateVaultForProfile(vault vaultCreateStore, profiles ports.ProfileStore, currentProfileID string, password string) (VaultCreationResult, error) {
	result := VaultCreationResult{CurrentProfileID: currentProfileID}
	if vault == nil {
		return result, fmt.Errorf("vault is required")
	}
	if profiles == nil {
		return result, fmt.Errorf("profile store is required")
	}
	if result.CurrentProfileID == "" {
		info, err := profiles.Create(filepath.Base(vault.Dir()), DefaultProfileAvatar, vault.Dir())
		if err != nil {
			return result, err
		}
		result.CurrentProfileID = info.ID
	}
	recoveryKey, err := vault.Create(password)
	if err != nil {
		return result, err
	}
	result.RecoveryKey = recoveryKey
	TouchProfile(profiles, result.CurrentProfileID)
	return result, nil
}

func UnlockVaultForProfile(vault ports.VaultLifecycle, profiles ports.ProfileStore, currentProfileID string, password string) error {
	if err := UnlockVault(vault, password); err != nil {
		return err
	}
	TouchProfile(profiles, currentProfileID)
	return nil
}

func RecoverVaultForProfile(vault ports.VaultRecoveryStore, profiles ports.ProfileStore, currentProfileID string, recoveryKey string, newPassword string) (string, error) {
	newRecoveryKey, err := RecoverVault(vault, recoveryKey, newPassword)
	if err != nil {
		return "", err
	}
	TouchProfile(profiles, currentProfileID)
	return newRecoveryKey, nil
}

func TouchProfile(profiles ports.ProfileStore, currentProfileID string) {
	if profiles != nil && currentProfileID != "" {
		_, _ = profiles.Touch(currentProfileID)
	}
}

func ChooseVaultDir(vault ports.VaultDirectoryStore, config ports.AppConfigStore, dir string) error {
	if vault == nil {
		return fmt.Errorf("vault is required")
	}
	if err := vault.SetDir(dir); err != nil {
		return err
	}
	if config == nil {
		return nil
	}
	return config.SaveVaultDir(dir)
}
