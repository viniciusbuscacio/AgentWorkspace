package appconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// WorkspaceConfig is the per-vault workspace state: the "look" of a vault —
// sidebar modules, theme, wallpaper, session restore, provider priority. It
// lives in workspace.json NEXT TO the vault master (same folder as vault.db),
// so it travels with the vault across machines and is readable BEFORE unlock
// (it is plain, non-secret JSON — the lock screen renders the right theme).
// Sync-safe: small file, written atomically (temp + rename), never held open.
type WorkspaceConfig struct {
	AddedModules          []string               `json:"addedModules,omitempty"`
	HiddenModules         []string               `json:"hiddenModules,omitempty"`
	ModuleOrder           []string               `json:"moduleOrder,omitempty"`
	Wallpaper             string                 `json:"wallpaper,omitempty"`
	WallpaperGlass        *int                   `json:"wallpaperGlass,omitempty"`
	ActiveTheme           string                 `json:"activeTheme,omitempty"`
	CustomThemes          map[string]interface{} `json:"customThemes,omitempty"`
	LastView              string                 `json:"lastView,omitempty"`
	LastChatID            string                 `json:"lastChatId,omitempty"`
	ProviderFallbackOrder []string               `json:"providerFallbackOrder,omitempty"`
}

const workspaceFile = "workspace.json"

// workspaceMu serializes read-modify-write cycles on workspace.json (the file
// is tiny; a single process-wide lock is plenty).
var workspaceMu sync.Mutex

// WorkspaceStore persists WorkspaceConfig for the CURRENT vault. VaultDir is
// a provider (not a fixed path) so a profile switch is picked up on the next
// call. Behavior on load when workspace.json is absent: seed it ONCE from the
// global config.json — a pre-existing vault keeps its familiar look. A newly
// created vault calls InitCleanWorkspace first, so it starts with defaults
// instead of inheriting another vault's face.
type WorkspaceStore struct {
	VaultDir func() string
}

func (s WorkspaceStore) filePath() string {
	if s.VaultDir == nil {
		return ""
	}
	dir := strings.TrimSpace(s.VaultDir())
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, workspaceFile)
}

// workspaceFromGlobal projects the workspace-scoped keys out of the global
// config — the one-time seed for vaults that predate workspace.json.
func workspaceFromGlobal(cfg Config) WorkspaceConfig {
	return WorkspaceConfig{
		AddedModules:          cfg.AddedModules,
		HiddenModules:         cfg.HiddenModules,
		ModuleOrder:           cfg.ModuleOrder,
		Wallpaper:             cfg.Wallpaper,
		WallpaperGlass:        cfg.WallpaperGlass,
		ActiveTheme:           cfg.ActiveTheme,
		CustomThemes:          cfg.CustomThemes,
		LastView:              cfg.LastView,
		LastChatID:            cfg.LastChatID,
		ProviderFallbackOrder: cfg.ProviderFallbackOrder,
	}
}

func (s WorkspaceStore) loadLocked() WorkspaceConfig {
	path := s.filePath()
	if path == "" {
		// No vault selected (should not happen post-bootstrap): read-only view
		// of the global values, never persisted.
		return workspaceFromGlobal(Load())
	}
	if data, err := os.ReadFile(path); err == nil {
		var cfg WorkspaceConfig
		if json.Unmarshal(data, &cfg) == nil {
			return cfg
		}
	}
	seeded := workspaceFromGlobal(Load())
	_ = s.writeLocked(seeded)
	return seeded
}

func (s WorkspaceStore) writeLocked(cfg WorkspaceConfig) error {
	path := s.filePath()
	if path == "" {
		// No vault selected (pre-profile/onboarding): workspace state is
		// cosmetic, so writes are ephemeral rather than errors.
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s WorkspaceStore) mutate(apply func(*WorkspaceConfig)) error {
	workspaceMu.Lock()
	defer workspaceMu.Unlock()
	cfg := s.loadLocked()
	apply(&cfg)
	return s.writeLocked(cfg)
}

func (s WorkspaceStore) snapshot() WorkspaceConfig {
	workspaceMu.Lock()
	defer workspaceMu.Unlock()
	return s.loadLocked()
}

// InitCleanWorkspace writes an empty workspace file when none exists yet —
// called right after vault creation so the new vault does NOT inherit the
// global (previous vault's) look. No-op when the file already exists.
func (s WorkspaceStore) InitCleanWorkspace() error {
	workspaceMu.Lock()
	defer workspaceMu.Unlock()
	path := s.filePath()
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return s.writeLocked(WorkspaceConfig{})
}

// ── ports.WorkspaceModuleStore ────────────────────────────────────────────────

func (s WorkspaceStore) LoadAddedModules() ([]string, error) {
	return s.snapshot().AddedModules, nil
}

func (s WorkspaceStore) SaveAddedModules(ids []string) error {
	if ids == nil {
		ids = []string{}
	}
	return s.mutate(func(cfg *WorkspaceConfig) { cfg.AddedModules = ids })
}

func (s WorkspaceStore) LoadHiddenModules() ([]string, error) {
	return s.snapshot().HiddenModules, nil
}

func (s WorkspaceStore) SaveHiddenModules(ids []string) error {
	if ids == nil {
		ids = []string{}
	}
	return s.mutate(func(cfg *WorkspaceConfig) { cfg.HiddenModules = ids })
}

func (s WorkspaceStore) LoadModuleOrder() ([]string, error) {
	return s.snapshot().ModuleOrder, nil
}

func (s WorkspaceStore) SaveModuleOrder(ids []string) error {
	if ids == nil {
		ids = []string{}
	}
	return s.mutate(func(cfg *WorkspaceConfig) { cfg.ModuleOrder = ids })
}

// ── ports.WallpaperConfigStore + ports.WallpaperGlassStore ───────────────────

func (s WorkspaceStore) LoadWallpaper() (string, error) {
	return s.snapshot().Wallpaper, nil
}

func (s WorkspaceStore) SaveWallpaper(id string) error {
	return s.mutate(func(cfg *WorkspaceConfig) { cfg.Wallpaper = id })
}

func (s WorkspaceStore) LoadWallpaperGlass() (int, error) {
	v := s.snapshot().WallpaperGlass
	if v == nil {
		return DefaultWallpaperGlass, nil
	}
	return *v, nil
}

func (s WorkspaceStore) SaveWallpaperGlass(opacity int) error {
	return s.mutate(func(cfg *WorkspaceConfig) { cfg.WallpaperGlass = &opacity })
}

// ── ports.ThemeConfigStore ───────────────────────────────────────────────────

func (s WorkspaceStore) LoadActiveTheme() string {
	return s.snapshot().ActiveTheme
}

func (s WorkspaceStore) SaveActiveTheme(id string) error {
	return s.mutate(func(cfg *WorkspaceConfig) { cfg.ActiveTheme = id })
}

func (s WorkspaceStore) LoadCustomThemes() map[string]interface{} {
	return s.snapshot().CustomThemes
}

func (s WorkspaceStore) SaveCustomThemes(themes map[string]interface{}) error {
	return s.mutate(func(cfg *WorkspaceConfig) { cfg.CustomThemes = themes })
}

// ── ports.SessionRestoreStore ────────────────────────────────────────────────

func (s WorkspaceStore) LoadLastView() (string, error) {
	return s.snapshot().LastView, nil
}

func (s WorkspaceStore) LoadLastChatID() (string, error) {
	return s.snapshot().LastChatID, nil
}

func (s WorkspaceStore) SaveLastSession(view string, chatID string) error {
	return s.mutate(func(cfg *WorkspaceConfig) {
		cfg.LastView = view
		cfg.LastChatID = chatID
	})
}

// ── ports.ProviderFallbackStore ──────────────────────────────────────────────

func (s WorkspaceStore) LoadProviderFallbackOrder() ([]string, error) {
	return s.snapshot().ProviderFallbackOrder, nil
}

func (s WorkspaceStore) SaveProviderFallbackOrder(ids []string) error {
	if ids == nil {
		ids = []string{}
	}
	return s.mutate(func(cfg *WorkspaceConfig) { cfg.ProviderFallbackOrder = ids })
}

// Cooldown minutes stay MACHINE-scoped on purpose: they tune this device's
// circuit breaker and are read at app construction, before any vault is
// selected. The port bundles them with the fallback order, so delegate.
func (s WorkspaceStore) LoadProviderCooldownMinutes() int {
	return Store{}.LoadProviderCooldownMinutes()
}

func (s WorkspaceStore) SaveProviderCooldownMinutes(minutes int) error {
	return Store{}.SaveProviderCooldownMinutes(minutes)
}
