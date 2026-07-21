package appconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"aw/internal/domain"
)

type Config struct {
	VaultDir        string `json:"vaultDir"`
	AutoLockMinutes *int   `json:"autoLockMinutes,omitempty"`
	AppZoomPercent  int    `json:"appZoomPercent,omitempty"`
	Wallpaper       string `json:"wallpaper,omitempty"`
	// WallpaperGlass is the glass slider value (0–100). Pointer so that 0
	// ("fully solid") round-trips through config correctly — a nil means
	// "not configured yet" and the default of 20 is used instead.
	WallpaperGlass *int                  `json:"wallpaperGlass,omitempty"`
	AddedModules   []string              `json:"addedModules,omitempty"`
	HiddenModules  []string              `json:"hiddenModules,omitempty"`
	ModuleOrder    []string              `json:"moduleOrder,omitempty"`
	Browser        BrowserConfig         `json:"browser,omitempty"`
	Obsidian       domain.ObsidianConfig `json:"obsidian,omitempty"`
	SelfDev        SelfDevConfig         `json:"selfDev,omitempty"`
	// DesktopNotificationsEnabled controls whether transient OS notifications
	// are shown (Settings toggle "Desktop notifications"). Pointer so that
	// true (the default-on behavior) is distinguishable from "never written".
	DesktopNotificationsEnabled *bool `json:"desktopNotificationsEnabled,omitempty"`
	// Session restore: last active view and selected chat ID, saved on
	// navigation and restored after unlock. Empty means home / newest chat.
	LastView   string `json:"lastView,omitempty"`
	LastChatID string `json:"lastChatId,omitempty"`
	// Theme studio: the active theme id (built-in or "custom:slug") and the
	// user's saved custom themes. Kept in appconfig (not vault) because theme
	// selection is cosmetic state, not a secret.
	ActiveTheme  string                 `json:"activeTheme,omitempty"`
	CustomThemes map[string]interface{} `json:"customThemes,omitempty"`
	// ProviderFallbackOrder is the user-defined priority list of provider IDs
	// for the LLM fallback chain (index 0 = primary / #1). Not a secret — just
	// provider IDs — so it lives here next to ModuleOrder, not in the vault.
	ProviderFallbackOrder []string `json:"providerFallbackOrder,omitempty"`
	// ProviderCooldownMinutes is how long a provider that failed with a
	// failover-class error is benched by the in-memory circuit breaker before
	// being retried. Pointer so 0 ("never bench") round-trips. nil = default 30.
	ProviderCooldownMinutes *int `json:"providerCooldownMinutes,omitempty"`
	// SubagentMode controls automatic generic-subagent guidance injected into
	// the agent prompt. Empty/default is "balanced".
	SubagentMode string `json:"subagentMode,omitempty"`
	// WebServer holds the web-access settings that must be readable pre-unlock
	// (the headless awd boots its login screen with the vault still locked, and
	// the GUI starts the server on startup). These are not secrets — only the
	// session signing key lives in the vault. Pointer so an absent block is
	// distinguishable from an all-defaults one.
	WebServer *domain.WebServerConfig `json:"webServer,omitempty"`
	// ServerTLS is the shared TLS profile (mode selector) for the web/MCP/REST
	// servers. Non-secret and readable pre-unlock; the certificate material lives
	// on disk under server-tls/, never here. Pointer so an absent block is
	// distinguishable from an all-defaults one.
	ServerTLS *domain.ServerTLSConfig `json:"serverTls,omitempty"`
	// Firewall is the Agent Firewall policy (ordered PERMIT/DENY ACL) governing
	// which peers may reach the MCP/REST/Web listeners. Non-secret network
	// policy, readable pre-unlock so the composition root can decide how to bind
	// each server. Pointer so an absent block (deny-all default) is
	// distinguishable from an empty configured one.
	Firewall *domain.FirewallConfig `json:"firewall,omitempty"`
}

// BrowserConfig overrides the Agent Browser module defaults (binary paths and
// CDP ports). Zero values mean "use the per-OS default".
type BrowserConfig struct {
	ChromePath string `json:"chromePath,omitempty"`
	EdgePath   string `json:"edgePath,omitempty"`
	ChromePort int    `json:"chromePort,omitempty"`
	EdgePort   int    `json:"edgePort,omitempty"`
}

func (config BrowserConfig) IsZero() bool {
	return config == BrowserConfig{}
}

type SelfDevConfig struct {
	Enabled     bool   `json:"enabled,omitempty"`
	RepoRoot    string `json:"repoRoot,omitempty"`
	AllowShell  *bool  `json:"allowShell,omitempty"`
	AutoApprove *bool  `json:"autoApprove,omitempty"`
}

type Store struct{}

// ProviderCooldownUntilRestart is the sentinel meaning a provider that failed
// with a failover-class error is benched until the app restarts (the default:
// the breaker state is in-memory, so a restart is what clears it). Positive
// values bench for that many minutes instead; 0 disables benching.
const ProviderCooldownUntilRestart = -1

func (Store) LoadAppZoomPercent() (int, error) {
	return Load().AppZoomPercent, nil
}

func (Store) SaveAppZoomPercent(percent int) error {
	return Save(Config{AppZoomPercent: percent})
}

// LoadObsidianConfig returns the Obsidian module setup (zero value = not
// configured). SaveObsidianConfig replaces it wholesale; note the Save merge
// keeps the previous block when handed a zero value.
func (Store) LoadObsidianConfig() domain.ObsidianConfig {
	return Load().Obsidian
}

func (Store) SaveObsidianConfig(cfg domain.ObsidianConfig) error {
	return Save(Config{Obsidian: cfg})
}

const DefaultWallpaperGlass = 20

// LoadDesktopNotificationsEnabled returns the persisted preference.
// Default is true (notifications on) when the field has never been written.
func (Store) LoadDesktopNotificationsEnabled() bool {
	v := Load().DesktopNotificationsEnabled
	if v == nil {
		return true
	}
	return *v
}

func (Store) SaveDesktopNotificationsEnabled(enabled bool) error {
	return Save(Config{DesktopNotificationsEnabled: &enabled})
}

func (Store) SaveVaultDir(dir string) error {
	return Save(Config{VaultDir: dir})
}

func (Store) SaveAutoLockMinutes(minutes int) error {
	return Save(Config{AutoLockMinutes: &minutes})
}

// LoadAutoLockMinutes returns the persisted auto-lock timeout in minutes.
// Returns 0 (Never / disabled) when the field has never been written.
func (Store) LoadAutoLockMinutes() int {
	v := Load().AutoLockMinutes
	if v == nil {
		return 0
	}
	return *v
}

// LoadProviderCooldownMinutes returns the persisted circuit-breaker bench time
// in minutes, or ProviderCooldownUntilRestart (bench until the app restarts)
// when never written.
func (Store) LoadProviderCooldownMinutes() int {
	v := Load().ProviderCooldownMinutes
	if v == nil {
		return ProviderCooldownUntilRestart
	}
	return *v
}

func (Store) SaveProviderCooldownMinutes(minutes int) error {
	if minutes < 0 {
		minutes = ProviderCooldownUntilRestart
	}
	return Save(Config{ProviderCooldownMinutes: &minutes})
}

const (
	SubagentModeOff        = "off"
	SubagentModeBalanced   = "balanced"
	SubagentModeAggressive = "aggressive"
)

func NormalizeSubagentMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case SubagentModeOff:
		return SubagentModeOff
	case SubagentModeAggressive:
		return SubagentModeAggressive
	default:
		return SubagentModeBalanced
	}
}

func (Store) LoadSubagentMode() string {
	return NormalizeSubagentMode(Load().SubagentMode)
}

func (Store) SaveSubagentMode(mode string) error {
	return Save(Config{SubagentMode: NormalizeSubagentMode(mode)})
}

func (Store) LoadBrowserExecutable(moduleID string) (string, error) {
	browser := Load().Browser
	switch strings.TrimSpace(moduleID) {
	case domain.BrowserModuleChrome:
		return browser.ChromePath, nil
	case domain.BrowserModuleEdge:
		return browser.EdgePath, nil
	default:
		return "", nil
	}
}

func (Store) SaveBrowserExecutable(moduleID string, path string) error {
	browser := Load().Browser
	switch strings.TrimSpace(moduleID) {
	case domain.BrowserModuleChrome:
		browser.ChromePath = strings.TrimSpace(path)
	case domain.BrowserModuleEdge:
		browser.EdgePath = strings.TrimSpace(path)
	default:
		return nil
	}
	return Save(Config{Browser: browser})
}

// LoadSelfDev returns the persisted self-dev posture mapped onto the domain
// type so callers need not import appconfig.
func (Store) LoadSelfDev() domain.SelfDevConfig {
	c := Load().SelfDev
	return domain.SelfDevConfig{
		Enabled:     c.Enabled,
		RepoRoot:    c.RepoRoot,
		AllowShell:  c.AllowShell,
		AutoApprove: c.AutoApprove,
	}
}

func (config SelfDevConfig) IsZero() bool {
	return !config.Enabled &&
		config.RepoRoot == "" &&
		config.AllowShell == nil &&
		config.AutoApprove == nil
}

func (config SelfDevConfig) EffectiveAllowShell() bool {
	if !config.Enabled {
		return false
	}
	if config.AllowShell == nil {
		return true
	}
	return *config.AllowShell
}

func (config SelfDevConfig) EffectiveAutoApprove() bool {
	if !config.Enabled {
		return false
	}
	if config.AutoApprove == nil {
		return true
	}
	return *config.AutoApprove
}

func Load() Config {
	data, err := os.ReadFile(path())
	if err != nil {
		return Config{}
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}
	}
	migrateRenamedSelfDevRepoRoot(&config)
	return config
}

func migrateRenamedSelfDevRepoRoot(config *Config) {
	if config == nil || !config.SelfDev.Enabled {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	oldRoot := filepath.Join(home, "aw")
	newRoot := filepath.Join(home, "aw")
	if filepath.Clean(config.SelfDev.RepoRoot) != oldRoot {
		return
	}
	if _, err := os.Stat(filepath.Join(newRoot, "go.mod")); err != nil {
		return
	}
	if _, err := os.Stat(filepath.Join(oldRoot, "go.mod")); err == nil {
		return
	}
	config.SelfDev.RepoRoot = newRoot
}

// Save persists the configuration, preserving any existing fields that are left
// at their zero value in the provided config (load-merge). This lets callers
// update a single field (e.g. VaultDir) without wiping unrelated settings.
func Save(config Config) error {
	existing := Load()
	if config.VaultDir == "" {
		config.VaultDir = existing.VaultDir
	}
	if config.AutoLockMinutes == nil {
		config.AutoLockMinutes = existing.AutoLockMinutes
	}
	if config.AppZoomPercent == 0 {
		config.AppZoomPercent = existing.AppZoomPercent
	}
	if config.Wallpaper == "" {
		config.Wallpaper = existing.Wallpaper
	}
	if config.WallpaperGlass == nil {
		config.WallpaperGlass = existing.WallpaperGlass
	}
	if config.LastView == "" {
		config.LastView = existing.LastView
	}
	if config.LastChatID == "" {
		config.LastChatID = existing.LastChatID
	}
	if config.ActiveTheme == "" {
		config.ActiveTheme = existing.ActiveTheme
	}
	if config.CustomThemes == nil {
		config.CustomThemes = existing.CustomThemes
	}
	if config.ProviderFallbackOrder == nil {
		config.ProviderFallbackOrder = existing.ProviderFallbackOrder
	}
	if config.ProviderCooldownMinutes == nil {
		config.ProviderCooldownMinutes = existing.ProviderCooldownMinutes
	}
	if config.SubagentMode == "" {
		config.SubagentMode = existing.SubagentMode
	} else {
		config.SubagentMode = NormalizeSubagentMode(config.SubagentMode)
	}
	if config.AddedModules == nil {
		config.AddedModules = existing.AddedModules
	}
	if config.HiddenModules == nil {
		config.HiddenModules = existing.HiddenModules
	}
	if config.ModuleOrder == nil {
		config.ModuleOrder = existing.ModuleOrder
	}
	if config.DesktopNotificationsEnabled == nil {
		config.DesktopNotificationsEnabled = existing.DesktopNotificationsEnabled
	}
	if config.Browser.IsZero() {
		config.Browser = existing.Browser
	}
	if config.Obsidian.IsZero() {
		config.Obsidian = existing.Obsidian
	}
	if config.SelfDev.IsZero() {
		config.SelfDev = existing.SelfDev
	}
	if config.WebServer == nil {
		config.WebServer = existing.WebServer
	}
	if config.ServerTLS == nil {
		config.ServerTLS = existing.ServerTLS
	}
	if config.Firewall == nil {
		config.Firewall = existing.Firewall
	}
	return write(config)
}

func write(config Config) error {
	configPath := path()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0o600)
}

func path() string {
	return filepath.Join(BaseDir(), "config.json")
}

// BaseDir returns the Agent Workspace application data directory. The default
// folder name remains "aw" for compatibility with existing vault/config data.
// aw_DATA_DIR overrides it so smoke tests can run against throwaway data.
func BaseDir() string {
	if override := strings.TrimSpace(os.Getenv("aw_DATA_DIR")); override != "" {
		return override
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "aw")
}
