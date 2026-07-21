package domain

// WebServerConfig is the non-secret web-access configuration (persisted in
// config.json so it is readable before the vault is unlocked). Enabled and
// SessionTTLMinutes are pointers so that false / 0 round-trip correctly through
// a load-merge — a bool/int zero-value would otherwise fail to persist
// "disabled" or "no expiry".
type WebServerConfig struct {
	Enabled           *bool    `json:"enabled,omitempty"`
	Port              int      `json:"port,omitempty"`
	BindMode          string   `json:"bindMode,omitempty"`
	BindAddr          string   `json:"bindAddr,omitempty"`
	AllowedCIDRs      []string `json:"allowedCIDRs,omitempty"`
	SessionTTLMinutes *int     `json:"sessionTTLMinutes,omitempty"`
	// TLS toggles HTTPS for web access using the shared server-TLS bundle.
	// Pointer (like Enabled) so false round-trips through a load-merge. Default
	// false: web access stays plaintext until the user creates a certificate in
	// the TLS manager and enables this toggle.
	TLS *bool `json:"tls,omitempty"`
}

const (
	// DefaultWebServerPort is the default web-mode listen port (after MCP 9300 /
	// REST 9301).
	DefaultWebServerPort = 9302
	// WebBindModeTailscale binds the tailnet IP and refuses non-tailnet peers.
	WebBindModeTailscale = "tailscale"
	// WebBindModeManual binds a user-chosen address with no tailnet enforcement.
	WebBindModeManual = "manual"
)

// EnabledOrDefault reports whether web access is enabled (default false).
func (c WebServerConfig) EnabledOrDefault() bool {
	return c.Enabled != nil && *c.Enabled
}

// TLSEnabledOrDefault reports whether HTTPS is enabled for web access (default
// false). It does not imply a certificate exists — that is checked separately.
func (c WebServerConfig) TLSEnabledOrDefault() bool {
	return c.TLS != nil && *c.TLS
}

// PortOrDefault returns the configured port or the default.
func (c WebServerConfig) PortOrDefault() int {
	if c.Port <= 0 || c.Port > 65535 {
		return DefaultWebServerPort
	}
	return c.Port
}

// BindModeOrDefault returns the configured bind mode or the Tailscale default.
func (c WebServerConfig) BindModeOrDefault() string {
	if c.BindMode == "" {
		return WebBindModeTailscale
	}
	return c.BindMode
}

// SessionTTLOrZero returns the session TTL in minutes (0 = no time expiry).
func (c WebServerConfig) SessionTTLOrZero() int {
	if c.SessionTTLMinutes == nil {
		return 0
	}
	return *c.SessionTTLMinutes
}
