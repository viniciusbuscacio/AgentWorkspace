package application

import (
	"os"
	"path/filepath"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// VaultWorkDirFor is where a profile's working copy lives: the platform-local,
// never-synced cache dir (%LOCALAPPDATA% on Windows, ~/Library/Caches on
// macOS). Spec: docs/specs/vault-working-copy.md.
func VaultWorkDirFor(profileID string) string {
	base, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(base) == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "AW", "work", profileID)
}

// VaultDirOf exposes the vault's master directory for composition wiring
// (e.g. the per-vault workspace.json store needs to know where the vault
// lives at call time, across profile switches).
func VaultDirOf(vault ports.VaultDirectoryStore) string {
	if vault == nil {
		return ""
	}
	return vault.Dir()
}

// InitCleanWorkspace seeds a brand-new vault's workspace file so it starts
// with defaults instead of inheriting the previous vault's look. Best-effort.
func InitCleanWorkspace(store ports.WorkspaceStateInitializer) {
	if store != nil {
		_ = store.InitCleanWorkspace()
	}
}

// ConfigureVaultWorkDir points the vault's live database at the profile's
// local working copy (never a possibly-synced master folder). Locked-only by
// design; with the vault already unlocked the profile did not actually
// change, so the work dir in effect is already the right one.
func ConfigureVaultWorkDir(vault ports.VaultWorkDirConfigurer, profileID string) {
	if vault == nil || strings.TrimSpace(profileID) == "" {
		return
	}
	_ = vault.SetWorkDir(VaultWorkDirFor(profileID))
}

// VaultWriteBackTick runs one write-back cycle on the working copy.
func VaultWriteBackTick(vault ports.VaultWriteBacker) domain.VaultWriteBackResult {
	if vault == nil {
		return domain.VaultWriteBackResult{Skipped: true}
	}
	return vault.WriteBackTick()
}

// ResyncVaultFromMaster adopts a master updated outside this session (master
// manda) and drops the in-memory chat sessions so each chat's next turn
// re-seeds from the vault, exactly like after an app restart.
func ResyncVaultFromMaster(vault ports.VaultWriteBacker, sessions ports.ChatSessionResetter) error {
	if vault == nil {
		return nil
	}
	if err := vault.Resync(); err != nil {
		return err
	}
	if sessions != nil {
		sessions.ResetAllSessions()
	}
	return nil
}

// RemoveProfileWorkDirs deletes the ephemeral working copies of forgotten
// profiles (spec Q6). keepID skips the profile whose live DB is still open —
// Windows would refuse the delete anyway.
func RemoveProfileWorkDirs(infos []domain.ProfileInfo, keepID string) {
	for _, info := range infos {
		if info.ID == "" || info.ID == keepID {
			continue
		}
		_ = os.RemoveAll(VaultWorkDirFor(info.ID))
	}
}

// ForgetProfilesAndWorkDirs clears the start screen's recent-vaults list and
// deletes the forgotten profiles' working copies. The masters on disk are
// never touched; the currently unlocked profile keeps its live working copy.
func ForgetProfilesAndWorkDirs(profiles ports.ProfileStore, vault ports.VaultUnlockState, currentProfileID string) error {
	infos := ListProfiles(profiles)
	if err := ForgetAllProfiles(profiles); err != nil {
		return err
	}
	keepID := ""
	if vault != nil && vault.IsUnlocked() {
		keepID = currentProfileID
	}
	RemoveProfileWorkDirs(infos, keepID)
	return nil
}
