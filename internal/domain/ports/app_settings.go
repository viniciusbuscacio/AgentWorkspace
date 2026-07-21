package ports

import "aw/internal/domain"

// ThemeConfigStore persists the theme studio state: the active theme id and the
// user's custom theme map. Cosmetic state (appconfig), never vault data.
type ThemeConfigStore interface {
	LoadActiveTheme() string
	SaveActiveTheme(id string) error
	LoadCustomThemes() map[string]interface{}
	SaveCustomThemes(themes map[string]interface{}) error
}

// SelfDevConfigStore reads the persisted self-development posture.
type SelfDevConfigStore interface {
	LoadSelfDev() domain.SelfDevConfig
}

// CustomWallpaperStore manages user-uploaded wallpaper files on disk.
type CustomWallpaperStore interface {
	CustomWallpaperExists(filename string) bool
	CustomWallpaperDataURI(filename string) (string, error)
	DeleteCustomWallpaper(filename string) error
	ImportWallpaper(srcPath string) (string, error)
	ListCustomWallpapers() ([]string, error)
}

// WallpaperGlassStore persists the wallpaper glass/blur slider value.
type WallpaperGlassStore interface {
	LoadWallpaperGlass() (int, error)
	SaveWallpaperGlass(opacity int) error
}

// SessionRestoreStore persists the last active view and chat for restore on
// unlock.
type SessionRestoreStore interface {
	LoadLastView() (string, error)
	LoadLastChatID() (string, error)
	SaveLastSession(view string, chatID string) error
}

// ProviderFallbackStore persists the LLM fallback chain priority and the
// circuit-breaker cooldown.
type ProviderFallbackStore interface {
	LoadProviderFallbackOrder() ([]string, error)
	SaveProviderFallbackOrder(ids []string) error
	LoadProviderCooldownMinutes() int
	SaveProviderCooldownMinutes(minutes int) error
}

// WorkspaceStateInitializer seeds a brand-new vault's per-vault workspace
// state so it starts clean instead of inheriting the previous vault's look.
type WorkspaceStateInitializer interface {
	InitCleanWorkspace() error
}

// DesktopNotificationPrefStore reads the user's desktop-notifications toggle.
type DesktopNotificationPrefStore interface {
	LoadDesktopNotificationsEnabled() bool
}

// WebServerSettingsStore persists the non-secret web-access settings in
// config.json (readable pre-unlock). The session signing key is NOT here — it
// lives in the vault.
type WebServerSettingsStore interface {
	LoadWebServerConfig() domain.WebServerConfig
	IsWebServerEnabled() bool
	SaveWebServerEnabled(enabled bool) error
	SaveWebServerPort(port int) error
	SaveWebServerBind(mode string, addr string, allowedCIDRs []string) error
	SaveWebServerSessionTTL(minutes int) error
	SaveWebServerTLSEnabled(enabled bool) error
}

// FirewallSettingsStore persists the Agent Firewall policy in config.json
// (readable pre-unlock so the composition root can bind each server). The rules
// are non-secret network policy.
type FirewallSettingsStore interface {
	LoadFirewallConfig() domain.FirewallConfig
	SaveFirewallRules(rules []domain.FirewallRule) error
}

// NetworkInterfaceLister enumerates the local network interfaces the firewall
// can bind, classified by reachability. Implemented in infrastructure (it reads
// the host's interfaces); the composition root reaches it through an
// application use case.
type NetworkInterfaceLister interface {
	ListInterfaces() ([]domain.NetworkInterface, error)
}

// FirewallBindResolver resolves the concrete listen addresses (host:port) a
// governed service must bind under Option A, given the current rules. An empty
// result means no rule permits the service (blocked by the firewall).
// Implemented in infrastructure (it enumerates host interfaces); reached
// through an application use case.
type FirewallBindResolver interface {
	ResolveBindAddrs(rules []domain.FirewallRule, service string, port int) ([]string, error)
}
