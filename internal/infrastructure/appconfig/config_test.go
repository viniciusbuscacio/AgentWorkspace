package appconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func intPtr(v int) *int { return &v }

func TestSavePreservesAppZoomPercent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("aw_DATA_DIR", t.TempDir())

	if err := Save(Config{
		VaultDir:        "/tmp/aw-vault",
		AutoLockMinutes: intPtr(15),
		AppZoomPercent:  140,
		SelfDev:         SelfDevConfig{Enabled: true, RepoRoot: "/tmp/aw"},
	}); err != nil {
		t.Fatalf("Save(initial) error = %v", err)
	}
	if err := Save(Config{AppZoomPercent: 180}); err != nil {
		t.Fatalf("Save(zoom) error = %v", err)
	}

	config := Load()
	if config.VaultDir != "/tmp/aw-vault" {
		t.Fatalf("VaultDir = %q, want preserved vault dir", config.VaultDir)
	}
	if config.AutoLockMinutes == nil || *config.AutoLockMinutes != 15 {
		t.Fatalf("AutoLockMinutes = %v, want pointer to 15", config.AutoLockMinutes)
	}
	if config.AppZoomPercent != 180 {
		t.Fatalf("AppZoomPercent = %d, want 180", config.AppZoomPercent)
	}
	if !config.SelfDev.Enabled || config.SelfDev.RepoRoot != "/tmp/aw" {
		t.Fatalf("SelfDev = %+v, want preserved config", config.SelfDev)
	}
}

func TestSaveAutoLockNever(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("aw_DATA_DIR", t.TempDir())

	// Set a non-zero value first.
	if err := Save(Config{AutoLockMinutes: intPtr(30)}); err != nil {
		t.Fatalf("Save(30) error = %v", err)
	}
	// Then explicitly save 0 (Never) — must not be swallowed by the merge.
	if err := Save(Config{AutoLockMinutes: intPtr(0)}); err != nil {
		t.Fatalf("Save(0) error = %v", err)
	}
	got := Store{}.LoadAutoLockMinutes()
	if got != 0 {
		t.Fatalf("LoadAutoLockMinutes() = %d, want 0 (Never)", got)
	}
}

func TestSubagentModeDefaultsNormalizesAndPreserves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("aw_DATA_DIR", t.TempDir())

	store := Store{}
	if got := store.LoadSubagentMode(); got != SubagentModeBalanced {
		t.Fatalf("default LoadSubagentMode() = %q, want balanced", got)
	}
	if err := store.SaveSubagentMode("AGGRESSIVE"); err != nil {
		t.Fatalf("SaveSubagentMode() error = %v", err)
	}
	if err := Save(Config{AppZoomPercent: 150}); err != nil {
		t.Fatalf("Save(zoom) error = %v", err)
	}
	if got := store.LoadSubagentMode(); got != SubagentModeAggressive {
		t.Fatalf("LoadSubagentMode() = %q, want preserved aggressive", got)
	}
	if err := store.SaveSubagentMode("unexpected"); err != nil {
		t.Fatalf("SaveSubagentMode(invalid) error = %v", err)
	}
	if got := store.LoadSubagentMode(); got != SubagentModeBalanced {
		t.Fatalf("invalid mode normalized to %q, want balanced", got)
	}
}

func TestBrowserExecutableSavesPerBrowserAndPreservesPorts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("aw_DATA_DIR", t.TempDir())

	if err := Save(Config{Browser: BrowserConfig{ChromePort: 9444, EdgePort: 9555}}); err != nil {
		t.Fatalf("Save(initial browser) error = %v", err)
	}
	store := Store{}
	if err := store.SaveBrowserExecutable("browser-edge", `C:\Custom\msedge.exe`); err != nil {
		t.Fatalf("SaveBrowserExecutable(edge) error = %v", err)
	}
	if err := store.SaveBrowserExecutable("browser-chrome", `C:\Custom\chrome.exe`); err != nil {
		t.Fatalf("SaveBrowserExecutable(chrome) error = %v", err)
	}

	config := Load()
	if config.Browser.EdgePath != `C:\Custom\msedge.exe` || config.Browser.ChromePath != `C:\Custom\chrome.exe` {
		t.Fatalf("browser paths = %+v", config.Browser)
	}
	if config.Browser.ChromePort != 9444 || config.Browser.EdgePort != 9555 {
		t.Fatalf("browser ports not preserved: %+v", config.Browser)
	}
}

func TestLoadMigratesRenamedSelfDevRepoRoot(t *testing.T) {
	home := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("aw_DATA_DIR", dataDir)
	newRoot := filepath.Join(home, "aw")
	if err := os.MkdirAll(newRoot, 0o755); err != nil {
		t.Fatalf("mkdir aw: %v", err)
	}
	if err := os.WriteFile(filepath.Join(newRoot, "go.mod"), []byte("module aw\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := Save(Config{SelfDev: SelfDevConfig{Enabled: true, RepoRoot: filepath.Join(home, "aw")}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got := Load().SelfDev.RepoRoot
	if got != newRoot {
		t.Fatalf("RepoRoot = %q, want migrated %q", got, newRoot)
	}
}

func TestSelfDevEffectiveDefaults(t *testing.T) {
	disabled := SelfDevConfig{}
	if disabled.EffectiveAllowShell() {
		t.Fatalf("disabled EffectiveAllowShell() = true, want false")
	}
	if disabled.EffectiveAutoApprove() {
		t.Fatalf("disabled EffectiveAutoApprove() = true, want false")
	}

	enabled := SelfDevConfig{Enabled: true}
	if !enabled.EffectiveAllowShell() {
		t.Fatalf("enabled EffectiveAllowShell() = false, want default true")
	}
	if !enabled.EffectiveAutoApprove() {
		t.Fatalf("enabled EffectiveAutoApprove() = false, want default true")
	}

	no := false
	guarded := SelfDevConfig{Enabled: true, AllowShell: &no, AutoApprove: &no}
	if guarded.EffectiveAllowShell() {
		t.Fatalf("guarded EffectiveAllowShell() = true, want false")
	}
	if guarded.EffectiveAutoApprove() {
		t.Fatalf("guarded EffectiveAutoApprove() = true, want false")
	}
}
