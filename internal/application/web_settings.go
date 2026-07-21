package application

import (
	"fmt"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// Web-access settings use cases. They wrap the WebServerSettingsStore (backed by
// config.json) so the composition root never calls the infrastructure store
// directly — mirroring how the rest of the app routes appconfig access through
// the application layer.

// LoadWebServerConfig returns the persisted web settings with defaults applied.
func LoadWebServerConfig(store ports.WebServerSettingsStore) domain.WebServerConfig {
	if store == nil {
		return domain.WebServerConfig{}
	}
	return store.LoadWebServerConfig()
}

// IsWebServerEnabled reports whether web access is on (readable pre-unlock).
func IsWebServerEnabled(store ports.WebServerSettingsStore) bool {
	return store != nil && store.IsWebServerEnabled()
}

// SetWebServerEnabled persists the on/off toggle.
func SetWebServerEnabled(store ports.WebServerSettingsStore, enabled bool) error {
	if store == nil {
		return fmt.Errorf("web settings store is required")
	}
	return store.SaveWebServerEnabled(enabled)
}

// SetWebServerPort validates and persists the listen port.
func SetWebServerPort(store ports.WebServerSettingsStore, port int) error {
	if store == nil {
		return fmt.Errorf("web settings store is required")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return store.SaveWebServerPort(port)
}

// SetWebServerBind validates and persists the bind mode/addr/CIDRs.
func SetWebServerBind(store ports.WebServerSettingsStore, mode, addr string, allowedCIDRs []string) error {
	if store == nil {
		return fmt.Errorf("web settings store is required")
	}
	if mode != domain.WebBindModeTailscale && mode != domain.WebBindModeManual {
		return fmt.Errorf("unknown bind mode %q", mode)
	}
	if mode == domain.WebBindModeManual && addr == "" {
		return fmt.Errorf("manual bind mode requires a bind address")
	}
	return store.SaveWebServerBind(mode, addr, allowedCIDRs)
}

// SetWebServerSessionTTL persists the session lifetime in minutes (0 = no time
// expiry; lock/autolock/regenerate still invalidate sessions).
func SetWebServerSessionTTL(store ports.WebServerSettingsStore, minutes int) error {
	if store == nil {
		return fmt.Errorf("web settings store is required")
	}
	if minutes < 0 {
		minutes = 0
	}
	return store.SaveWebServerSessionTTL(minutes)
}

// SetWebServerTLSEnabled persists the HTTPS toggle for web access.
func SetWebServerTLSEnabled(store ports.WebServerSettingsStore, enabled bool) error {
	if store == nil {
		return fmt.Errorf("web settings store is required")
	}
	return store.SaveWebServerTLSEnabled(enabled)
}
