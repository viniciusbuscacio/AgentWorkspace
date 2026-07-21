package application

import (
	"errors"
	"fmt"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// ErrObsidianWriteDisabled is returned by write/append while the module's
// Write toggle is off — the fence is user-controlled and read live.
var ErrObsidianWriteDisabled = errors.New(`writing to the Obsidian vault is disabled — enable "Write/Update notes" in the Obsidian module`)

// ErrObsidianDeleteDisabled is returned by obsidian.delete while the
// module's Delete toggle is off.
var ErrObsidianDeleteDisabled = errors.New(`deleting in the Obsidian vault is disabled — enable "Delete Notes/Folders" in the Obsidian module`)

// ErrObsidianDisabled is returned by every obsidian.* action while the
// module's master toggle is off.
var ErrObsidianDisabled = errors.New("Obsidian access is disabled — turn it on in the Obsidian module")

func obsidianEnabled(store ports.ObsidianConfigStore) (domain.ObsidianConfig, error) {
	cfg := store.LoadObsidianConfig()
	if !cfg.Enabled {
		return cfg, ErrObsidianDisabled
	}
	return cfg, nil
}

// ObsidianList lists the entries under folder (vault root when empty).
func ObsidianList(store ports.ObsidianConfigStore, vault ports.ObsidianVault, folder string) ([]domain.ObsidianEntry, error) {
	cfg, err := obsidianEnabled(store)
	if err != nil {
		return nil, err
	}
	return vault.List(cfg.VaultDir, folder)
}

// ObsidianSearch searches note names and contents.
func ObsidianSearch(store ports.ObsidianConfigStore, vault ports.ObsidianVault, query string, limit int) ([]domain.ObsidianMatch, error) {
	cfg, err := obsidianEnabled(store)
	if err != nil {
		return nil, err
	}
	return vault.Search(cfg.VaultDir, query, limit)
}

// ObsidianRead returns one note's content (capped by the adapter).
func ObsidianRead(store ports.ObsidianConfigStore, vault ports.ObsidianVault, rel string) (string, error) {
	cfg, err := obsidianEnabled(store)
	if err != nil {
		return "", err
	}
	return vault.Read(cfg.VaultDir, rel)
}

// ObsidianWrite creates or overwrites a note; requires the Write toggle.
func ObsidianWrite(store ports.ObsidianConfigStore, vault ports.ObsidianVault, rel, content string) error {
	cfg, err := obsidianEnabled(store)
	if err != nil {
		return err
	}
	if !cfg.WriteEnabled {
		return ErrObsidianWriteDisabled
	}
	return vault.Write(cfg.VaultDir, rel, content)
}

// ObsidianAppend appends to a note (creating it); requires the Write toggle.
func ObsidianAppend(store ports.ObsidianConfigStore, vault ports.ObsidianVault, rel, content string) error {
	cfg, err := obsidianEnabled(store)
	if err != nil {
		return err
	}
	if !cfg.WriteEnabled {
		return ErrObsidianWriteDisabled
	}
	return vault.Append(cfg.VaultDir, rel, content)
}

// ObsidianContextBlock renders the always-read notes for the agent context.
// Empty unless the module is added, a vault folder is set and at least one
// always-read note is configured.
func ObsidianContextBlock(
	moduleStore ports.WorkspaceModuleStore,
	catalog []domain.ModuleSpec,
	store ports.ObsidianConfigStore,
	vault ports.ObsidianVault,
) string {
	added, err := AddedModuleIDs(moduleStore, catalog)
	if err != nil {
		return ""
	}
	found := false
	for _, id := range added {
		if id == "obsidian" {
			found = true
			break
		}
	}
	if !found {
		return ""
	}
	cfg := store.LoadObsidianConfig()
	if !cfg.Enabled {
		return ""
	}
	return vault.AlwaysReadBlock(cfg.VaultDir, cfg.AlwaysRead)
}

// GetObsidianSettings returns the module setup for the module page.
func GetObsidianSettings(store ports.ObsidianConfigStore) domain.ObsidianConfig {
	return store.LoadObsidianConfig()
}

// SetObsidianSettings normalizes and persists the module setup.
func SetObsidianSettings(store ports.ObsidianConfigStore, cfg domain.ObsidianConfig) error {
	files := make([]string, 0, len(cfg.AlwaysRead))
	for _, f := range cfg.AlwaysRead {
		if f = strings.TrimSpace(f); f != "" {
			files = append(files, f)
		}
	}
	cfg.AlwaysRead = files
	cfg.VaultDir = strings.TrimSpace(cfg.VaultDir)
	return store.SaveObsidianConfig(cfg)
}

// ObsidianTestAccess verifies the configured vault folder is listable and
// every always-read note is readable. Returns a human summary; the first
// failure aborts with a pointed error.
func ObsidianTestAccess(store ports.ObsidianConfigStore, vault ports.ObsidianVault) (string, error) {
	cfg := store.LoadObsidianConfig()
	if strings.TrimSpace(cfg.VaultDir) == "" {
		return "", errors.New("no vault folder is configured")
	}
	entries, err := vault.List(cfg.VaultDir, "")
	if err != nil {
		return "", fmt.Errorf("cannot open the vault folder: %w", err)
	}
	for _, f := range cfg.AlwaysRead {
		if _, err := vault.Read(cfg.VaultDir, f); err != nil {
			return "", fmt.Errorf("always-read note %q is not readable: %w", f, err)
		}
	}
	summary := fmt.Sprintf("Vault OK — %d entries at the root", len(entries))
	if n := len(cfg.AlwaysRead); n > 0 {
		summary += fmt.Sprintf("; %d always-read note(s) readable", n)
	}
	return summary, nil
}

// ObsidianDelete removes a note or folder; requires the Delete toggle.
func ObsidianDelete(store ports.ObsidianConfigStore, vault ports.ObsidianVault, rel string) error {
	cfg, err := obsidianEnabled(store)
	if err != nil {
		return err
	}
	if !cfg.DeleteEnabled {
		return ErrObsidianDeleteDisabled
	}
	return vault.Delete(cfg.VaultDir, rel)
}

// ObsidianAlwaysReadSize returns the total rune count the always-read notes
// inject into every prompt — the page warns above the adapter's threshold.
func ObsidianAlwaysReadSize(store ports.ObsidianConfigStore, vault ports.ObsidianVault) int {
	cfg := store.LoadObsidianConfig()
	if len(cfg.AlwaysRead) == 0 {
		return 0
	}
	return vault.AlwaysReadSize(cfg.VaultDir, cfg.AlwaysRead)
}
