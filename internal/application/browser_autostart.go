package application

import "aw/internal/domain/ports"

// browserAutostartSecret is the vault key holding a per-browser auto-start
// toggle. Like the API-server settings it lives in the vault, so it is only
// readable while unlocked — which is also the only moment a browser may start.
func browserAutostartSecret(moduleID string) string {
	return "_browser_autostart_" + moduleID
}

// BrowserAutostart reports whether the given Agent Browser module should start
// automatically when the vault unlocks. Read errors (for example a locked
// vault) read as off.
func BrowserAutostart(store ports.SecretStore, moduleID string) bool {
	secret, err := GetSecret(store, browserAutostartSecret(moduleID))
	if err != nil {
		return false
	}
	return secret.Exists && secret.Value == "1"
}

// SetBrowserAutostart persists the auto-start toggle for one Agent Browser
// module. It does not start or stop a running browser; the composition root
// owns that.
func SetBrowserAutostart(store ports.SecretStore, moduleID string, enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	return SetSecret(store, browserAutostartSecret(moduleID), value)
}
