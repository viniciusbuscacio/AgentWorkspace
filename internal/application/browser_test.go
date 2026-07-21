package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aw/internal/domain"
)

func TestRunBrowserCommandAllowsCloseTab(t *testing.T) {
	// close_tab is registered in the dispatcher and routed in the manager;
	// the application allowlist must accept it too (2026-06-12: it didn't,
	// so the agent's close_tab failed with "unknown browser command").
	if !browserCommands["close_tab"] {
		t.Fatal("close_tab must be in the browser command allowlist")
	}
	if !browserCommands["new_tab"] {
		t.Fatal("new_tab must be in the browser command allowlist")
	}
	if !browserCommands["navigationtarget"] || !browserCommands["currenturl"] {
		t.Fatal("internal browser guard probes must survive command normalization")
	}
	for _, expected := range []string{"navigate", "snapshot", "click", "fill", "screenshot", "cdp"} {
		if !browserCommands[expected] {
			t.Fatalf("%q dropped from the allowlist", expected)
		}
	}
}

func TestSaveBrowserExecutableValidatesAndConfiguresBrowser(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "browser.exe")
	if err := os.WriteFile(exe, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write exe: %v", err)
	}
	browser := &recordingBrowserAutomation{}
	store := &recordingBrowserExecutableStore{}
	settings, err := SaveBrowserExecutable(context.Background(), browser, store, domain.BrowserModuleEdge, exe)
	if err != nil {
		t.Fatalf("SaveBrowserExecutable() error = %v", err)
	}
	if store.path != exe || browser.executablePath != exe {
		t.Fatalf("saved/configured paths store=%q browser=%q want %q", store.path, browser.executablePath, exe)
	}
	if settings.Path != exe || settings.DefaultPath == "" || !settings.Custom {
		t.Fatalf("settings = %+v", settings)
	}
}

func TestRunBrowserCommandAllowsInternalGuardCommandsAfterLowercaseNormalization(t *testing.T) {
	browser := &recordingBrowserAutomation{}
	if _, err := RunBrowserCommand(context.Background(), browser, domain.BrowserModuleEdge, "navigationTarget", map[string]any{"ref": "e1"}); err != nil {
		t.Fatalf("RunBrowserCommand(navigationTarget) error = %v", err)
	}
	if browser.command != "navigationtarget" {
		t.Fatalf("command sent to backend = %q, want lowercase normalized navigationtarget", browser.command)
	}
	if _, err := RunBrowserCommand(context.Background(), browser, domain.BrowserModuleEdge, "currentURL", nil); err != nil {
		t.Fatalf("RunBrowserCommand(currentURL) error = %v", err)
	}
	if browser.command != "currenturl" {
		t.Fatalf("command sent to backend = %q, want lowercase normalized currenturl", browser.command)
	}
}

type recordingBrowserAutomation struct {
	command        string
	executablePath string
}

func (b *recordingBrowserAutomation) ConfigureExecutable(_ string, path string) error {
	b.executablePath = path
	return nil
}

func (b *recordingBrowserAutomation) DefaultExecutable(string) (string, error) {
	return "/default/browser", nil
}

func (b *recordingBrowserAutomation) Start(context.Context, string, domain.BrowserStartOptions) (domain.BrowserStatus, error) {
	return domain.BrowserStatus{}, nil
}

func (b *recordingBrowserAutomation) Stop(context.Context, string) (domain.BrowserStatus, error) {
	return domain.BrowserStatus{}, nil
}

func (b *recordingBrowserAutomation) Status(context.Context, string) (domain.BrowserStatus, error) {
	return domain.BrowserStatus{Binary: "/default/browser"}, nil
}

func (b *recordingBrowserAutomation) Tabs(context.Context, string) ([]domain.BrowserTab, error) {
	return nil, nil
}

func (b *recordingBrowserAutomation) Command(_ context.Context, _ string, command string, _ map[string]any) (any, error) {
	b.command = command
	return map[string]any{"ok": true}, nil
}

type recordingBrowserExecutableStore struct {
	path string
}

func (s *recordingBrowserExecutableStore) LoadBrowserExecutable(string) (string, error) {
	return s.path, nil
}

func (s *recordingBrowserExecutableStore) SaveBrowserExecutable(_ string, path string) error {
	s.path = path
	return nil
}
