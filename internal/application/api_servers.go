package application

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"aw/internal/domain/ports"
)

// APIServerSettings groups the vault-held settings of one local API server
// (MCP, REST). They live in the vault so they are encrypted at rest and only
// readable while unlocked — the servers themselves only run while unlocked.
type APIServerSettings struct {
	enabledSecret    string
	tokenSecret      string
	portSecret       string
	tlsEnabledSecret string
	defaultPort      int
}

var (
	McpServerSettings = APIServerSettings{
		enabledSecret:    "_mcp_server_enabled",
		tokenSecret:      "_mcp_server_token",
		portSecret:       "_mcp_server_port",
		tlsEnabledSecret: "_mcp_server_tls_enabled",
		defaultPort:      9300,
	}
	RestServerSettings = APIServerSettings{
		enabledSecret:    "_rest_server_enabled",
		tokenSecret:      "_rest_server_token",
		portSecret:       "_rest_server_port",
		tlsEnabledSecret: "_rest_server_tls_enabled",
		defaultPort:      9301,
	}
	// WebServerSettings holds only the web session signing key in the vault
	// (enabled/port/bind/TTL live in config.json so they are readable
	// pre-unlock). EnsureToken/RegenerateToken manage the HMAC key used to mint
	// and verify web session tokens; regenerating it invalidates every session.
	WebServerSettings = APIServerSettings{
		enabledSecret: "_web_server_enabled",
		tokenSecret:   "_web_session_key",
		portSecret:    "_web_server_port",
		defaultPort:   9302,
	}
)

// Autostart reports whether the server should start automatically when the
// vault unlocks. Read errors (for example a locked vault) read as off.
func (s APIServerSettings) Autostart(store ports.SecretStore) bool {
	secret, err := GetSecret(store, s.enabledSecret)
	if err != nil {
		return false
	}
	return secret.Exists && secret.Value == "1"
}

// SetAutostart persists the auto-start toggle. It does not start or stop a
// running server; the composition root owns that.
func (s APIServerSettings) SetAutostart(store ports.SecretStore, enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	return SetSecret(store, s.enabledSecret, value)
}

// EnsureToken returns the persisted bearer token, generating and storing a
// fresh one on first use.
func (s APIServerSettings) EnsureToken(store ports.SecretStore) (string, error) {
	secret, err := GetSecret(store, s.tokenSecret)
	if err != nil {
		return "", err
	}
	if secret.Exists && strings.TrimSpace(secret.Value) != "" {
		return secret.Value, nil
	}
	return s.RegenerateToken(store)
}

// SetToken pins the bearer token to a caller-supplied value. Dev tooling uses
// this to give a throwaway instance a known token; the settings page always
// goes through EnsureToken/RegenerateToken instead.
func (s APIServerSettings) SetToken(store ports.SecretStore, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required")
	}
	return SetSecret(store, s.tokenSecret, token)
}

// RegenerateToken replaces the bearer token with a new 256-bit value and
// returns it. Existing clients are invalidated.
func (s APIServerSettings) RegenerateToken(store ports.SecretStore) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	if err := SetSecret(store, s.tokenSecret, token); err != nil {
		return "", err
	}
	return token, nil
}

// Port returns the configured listen port, falling back to the server's
// default on missing or invalid values.
func (s APIServerSettings) Port(store ports.SecretStore) int {
	secret, err := GetSecret(store, s.portSecret)
	if err != nil || !secret.Exists {
		return s.defaultPort
	}
	port, err := strconv.Atoi(strings.TrimSpace(secret.Value))
	if err != nil || port < 1 || port > 65535 {
		return s.defaultPort
	}
	return port
}

// SetPort validates and persists the listen port.
func (s APIServerSettings) SetPort(store ports.SecretStore, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return SetSecret(store, s.portSecret, strconv.Itoa(port))
}

// TLSEnabled reports whether HTTPS is enabled for this server (default false).
// Read errors (for example a locked vault) read as off.
func (s APIServerSettings) TLSEnabled(store ports.SecretStore) bool {
	secret, err := GetSecret(store, s.tlsEnabledSecret)
	if err != nil {
		return false
	}
	return secret.Exists && secret.Value == "1"
}

// SetTLSEnabled persists the HTTPS toggle. It does not start or stop a running
// server; the composition root owns that.
func (s APIServerSettings) SetTLSEnabled(store ports.SecretStore, enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	return SetSecret(store, s.tlsEnabledSecret, value)
}
