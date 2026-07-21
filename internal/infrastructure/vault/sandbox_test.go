package vault

import (
	"strings"
	"testing"

	"aw/internal/domain"
)

func TestSandboxConfigDefaultsWhenUnset(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	config, err := v.LoadSandboxConfig()
	if err != nil {
		t.Fatalf("LoadSandboxConfig() error = %v", err)
	}
	if config.Mode != domain.SandboxPermitList {
		t.Errorf("default mode = %q, want permit_list", config.Mode)
	}
	if len(config.AllowedFolders) != 0 {
		t.Errorf("default folder list not empty: %+v", config)
	}
}

func TestSandboxConfigRoundTripSurvivesLockUnlock(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	saved := domain.SandboxConfig{
		Mode:           domain.SandboxPermitAll,
		AllowedFolders: []string{"~/Documents"},
	}
	if err := v.SaveSandboxConfig(saved); err != nil {
		t.Fatalf("SaveSandboxConfig() error = %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if _, err := v.LoadSandboxConfig(); err == nil {
		t.Fatal("LoadSandboxConfig() on locked vault succeeded")
	}
	if err := v.Unlock("senha1234"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	config, err := v.LoadSandboxConfig()
	if err != nil {
		t.Fatalf("LoadSandboxConfig() after unlock error = %v", err)
	}
	if config.Mode != saved.Mode {
		t.Errorf("mode = %q, want %q", config.Mode, saved.Mode)
	}
	if len(config.AllowedFolders) != 1 || config.AllowedFolders[0] != "~/Documents" {
		t.Errorf("allowed folders = %v", config.AllowedFolders)
	}
}

// A vault still storing the removed deny_list mode fails safe to permit_list.
func TestSandboxConfigDenyListMigratesToPermitList(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	if err := v.SetSecret(sandboxModeSecret, "deny_list"); err != nil {
		t.Fatal(err)
	}
	config, err := v.LoadSandboxConfig()
	if err != nil {
		t.Fatalf("LoadSandboxConfig() error = %v", err)
	}
	if config.Mode != domain.SandboxPermitList {
		t.Errorf("stored deny_list resolved to %q, want permit_list", config.Mode)
	}
}

func TestSandboxConfigToleratesCorruptEntries(t *testing.T) {
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	if err := v.SetSecret(sandboxModeSecret, "warp-speed"); err != nil {
		t.Fatal(err)
	}
	if err := v.SetSecret(sandboxAllowedFoldersSecret, "{not json"); err != nil {
		t.Fatal(err)
	}
	config, err := v.LoadSandboxConfig()
	if err != nil {
		t.Fatalf("LoadSandboxConfig() error = %v", err)
	}
	if config.Mode != domain.SandboxPermitList {
		t.Errorf("invalid stored mode resolved to %q, want default permit_list", config.Mode)
	}
	if len(config.AllowedFolders) != 0 {
		t.Errorf("malformed list resolved to %v, want empty", config.AllowedFolders)
	}
}

// The sandbox keys must stay on the internal "_" prefix: the Settings secret
// list hides those, so the agent (and the UI) never surface or edit them as
// ordinary secrets.
func TestSandboxSecretsUseInternalPrefix(t *testing.T) {
	for _, name := range []string{sandboxModeSecret, sandboxAllowedFoldersSecret} {
		if !strings.HasPrefix(name, "_") {
			t.Errorf("sandbox secret %q must use the internal _ prefix", name)
		}
	}
}
