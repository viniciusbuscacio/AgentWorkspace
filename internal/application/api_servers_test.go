package application

import (
	"fmt"
	"testing"
)

type fakeSecretStore struct {
	secrets map[string]string
	failing bool
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{secrets: map[string]string{}}
}

func (s *fakeSecretStore) SetSecret(name string, value string) error {
	if s.failing {
		return fmt.Errorf("vault is locked")
	}
	s.secrets[name] = value
	return nil
}

func (s *fakeSecretStore) GetSecret(name string) (string, bool, error) {
	if s.failing {
		return "", false, fmt.Errorf("vault is locked")
	}
	value, ok := s.secrets[name]
	return value, ok, nil
}

func (s *fakeSecretStore) HasSecret(name string) (bool, error) {
	_, ok := s.secrets[name]
	return ok, nil
}

func (s *fakeSecretStore) DeleteSecret(name string) error {
	delete(s.secrets, name)
	return nil
}

func (s *fakeSecretStore) ListSecrets() ([]string, error) {
	names := make([]string, 0, len(s.secrets))
	for name := range s.secrets {
		names = append(names, name)
	}
	return names, nil
}

func TestApiServerAutostartDefaultsToFalse(t *testing.T) {
	store := newFakeSecretStore()
	if McpServerSettings.Autostart(store) {
		t.Fatal("expected auto-start to be off by default")
	}
	if McpServerSettings.Autostart(nil) {
		t.Fatal("expected nil store to read as off")
	}
	store.failing = true
	if McpServerSettings.Autostart(store) {
		t.Fatal("expected store errors to read as off")
	}
}

func TestApiServerAutostartRoundTripIsIndependentPerServer(t *testing.T) {
	store := newFakeSecretStore()
	if err := McpServerSettings.SetAutostart(store, true); err != nil {
		t.Fatal(err)
	}
	if !McpServerSettings.Autostart(store) {
		t.Fatal("expected MCP auto-start on")
	}
	if RestServerSettings.Autostart(store) {
		t.Fatal("expected REST auto-start to stay off: settings must not be shared")
	}
	if err := McpServerSettings.SetAutostart(store, false); err != nil {
		t.Fatal(err)
	}
	if McpServerSettings.Autostart(store) {
		t.Fatal("expected MCP auto-start off after SetAutostart(false)")
	}
}

func TestApiServerEnsureTokenGeneratesOncePerServer(t *testing.T) {
	store := newFakeSecretStore()
	first, err := McpServerSettings.EnsureToken(store)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 {
		t.Fatalf("expected 64 hex chars (256 bits), got %d: %q", len(first), first)
	}
	second, err := McpServerSettings.EnsureToken(store)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatal("expected the persisted token to be reused")
	}
	restToken, err := RestServerSettings.EnsureToken(store)
	if err != nil {
		t.Fatal(err)
	}
	if restToken == first {
		t.Fatal("expected each server to have its own token")
	}
}

func TestApiServerRegenerateTokenReplacesValue(t *testing.T) {
	store := newFakeSecretStore()
	first, err := McpServerSettings.EnsureToken(store)
	if err != nil {
		t.Fatal(err)
	}
	next, err := McpServerSettings.RegenerateToken(store)
	if err != nil {
		t.Fatal(err)
	}
	if next == first {
		t.Fatal("expected a fresh token after regeneration")
	}
	if got, _ := McpServerSettings.EnsureToken(store); got != next {
		t.Fatal("expected the new token to be persisted")
	}
}

func TestApiServerPortDefaultsAndValidation(t *testing.T) {
	store := newFakeSecretStore()
	if got := McpServerSettings.Port(store); got != 9300 {
		t.Fatalf("expected MCP default port 9300, got %d", got)
	}
	if got := RestServerSettings.Port(store); got != 9301 {
		t.Fatalf("expected REST default port 9301, got %d", got)
	}
	if err := McpServerSettings.SetPort(store, 9400); err != nil {
		t.Fatal(err)
	}
	if got := McpServerSettings.Port(store); got != 9400 {
		t.Fatalf("expected port 9400, got %d", got)
	}
	if got := RestServerSettings.Port(store); got != 9301 {
		t.Fatal("expected REST port to be independent of the MCP port")
	}
	if err := McpServerSettings.SetPort(store, 0); err == nil {
		t.Fatal("expected an error for port 0")
	}
	if err := McpServerSettings.SetPort(store, 70000); err == nil {
		t.Fatal("expected an error for an out-of-range port")
	}
	store.secrets["_mcp_server_port"] = "not-a-number"
	if got := McpServerSettings.Port(store); got != 9300 {
		t.Fatalf("expected invalid stored port to fall back to default, got %d", got)
	}
}
