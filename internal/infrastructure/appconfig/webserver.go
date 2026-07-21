package appconfig

import "aw/internal/domain"

// Web-access settings live in config.json (not the vault) because they must be
// readable before the vault is unlocked: the headless awd boots its login
// screen with the vault locked, and the GUI starts the web server on startup.
// None of these are secrets — the session signing key is the only web datum
// kept in the vault. The Save merge replaces the WebServer block wholesale, so
// these setters do load-modify-write of the whole sub-struct (avoiding the
// bool/zero-value pitfall that would stop "disabled" from persisting).
//
// Store implements ports.WebServerSettingsStore; the composition root reaches
// these through the application layer, never directly.

// LoadWebServerConfig returns the persisted web settings with defaults applied
// (port and bind mode), so callers get a ready-to-use value pre-unlock.
func (Store) LoadWebServerConfig() domain.WebServerConfig {
	cfg := domain.WebServerConfig{}
	if existing := Load().WebServer; existing != nil {
		cfg = *existing
	}
	cfg.Port = cfg.PortOrDefault()
	cfg.BindMode = cfg.BindModeOrDefault()
	return cfg
}

// IsWebServerEnabled reports whether web access is on. Readable pre-unlock.
func (Store) IsWebServerEnabled() bool {
	cfg := Load().WebServer
	return cfg != nil && cfg.EnabledOrDefault()
}

func saveWeb(modify func(*domain.WebServerConfig)) error {
	current := domain.WebServerConfig{}
	if existing := Load().WebServer; existing != nil {
		current = *existing
	}
	modify(&current)
	return Save(Config{WebServer: &current})
}

// SaveWebServerEnabled persists the on/off toggle (false round-trips correctly).
func (Store) SaveWebServerEnabled(enabled bool) error {
	return saveWeb(func(c *domain.WebServerConfig) { c.Enabled = &enabled })
}

// SaveWebServerPort persists the listen port.
func (Store) SaveWebServerPort(port int) error {
	return saveWeb(func(c *domain.WebServerConfig) { c.Port = port })
}

// SaveWebServerBind persists the bind mode plus its address and CIDR policy.
func (Store) SaveWebServerBind(mode, addr string, allowedCIDRs []string) error {
	return saveWeb(func(c *domain.WebServerConfig) {
		c.BindMode = mode
		c.BindAddr = addr
		c.AllowedCIDRs = allowedCIDRs
	})
}

// SaveWebServerSessionTTL persists the session lifetime in minutes. 0 means
// "no time-based expiry" (lock/autolock/regenerate still invalidate sessions).
func (Store) SaveWebServerSessionTTL(minutes int) error {
	return saveWeb(func(c *domain.WebServerConfig) { c.SessionTTLMinutes = &minutes })
}

// SaveWebServerTLSEnabled persists the HTTPS toggle (false round-trips
// correctly via the pointer field).
func (Store) SaveWebServerTLSEnabled(enabled bool) error {
	return saveWeb(func(c *domain.WebServerConfig) { c.TLS = &enabled })
}

// LoadServerTLSConfig returns the shared TLS profile with the mode defaulted.
// Readable pre-unlock; the certificate material lives on disk, not here.
func (Store) LoadServerTLSConfig() domain.ServerTLSConfig {
	cfg := domain.ServerTLSConfig{}
	if existing := Load().ServerTLS; existing != nil {
		cfg = *existing
	}
	cfg.Mode = cfg.ModeOrDefault()
	return cfg
}

// SaveServerTLSMode persists the active TLS mode (self_signed / custom).
func (Store) SaveServerTLSMode(mode string) error {
	current := domain.ServerTLSConfig{}
	if existing := Load().ServerTLS; existing != nil {
		current = *existing
	}
	current.Mode = mode
	return Save(Config{ServerTLS: &current})
}
