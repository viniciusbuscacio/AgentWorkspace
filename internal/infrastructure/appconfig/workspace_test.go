package appconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func workspaceFor(dir string) WorkspaceStore {
	return WorkspaceStore{VaultDir: func() string { return dir }}
}

// A pre-existing vault (no workspace.json yet) inherits the global look once;
// a freshly created vault (InitCleanWorkspace) starts clean; each vault keeps
// its own state afterwards.
func TestWorkspaceStoreSeedingAndIsolation(t *testing.T) {
	t.Setenv("aw_DATA_DIR", t.TempDir())
	if err := Save(Config{AddedModules: []string{"chat", "notes"}, ActiveTheme: "midnight", Wallpaper: "northern-lights"}); err != nil {
		t.Fatalf("seed global: %v", err)
	}

	// Pre-existing vault: inherits the global state on first read.
	oldVault := t.TempDir()
	old := workspaceFor(oldVault)
	if mods, _ := old.LoadAddedModules(); len(mods) != 2 || mods[0] != "chat" {
		t.Fatalf("old vault modules = %v, want inherited [chat notes]", mods)
	}
	if old.LoadActiveTheme() != "midnight" {
		t.Fatal("old vault should inherit the global theme")
	}
	if _, err := os.Stat(filepath.Join(oldVault, "workspace.json")); err != nil {
		t.Fatal("seed must persist workspace.json next to the vault")
	}

	// New vault: InitCleanWorkspace first → defaults, no inheritance.
	newVault := t.TempDir()
	fresh := workspaceFor(newVault)
	if err := fresh.InitCleanWorkspace(); err != nil {
		t.Fatalf("init clean: %v", err)
	}
	if mods, _ := fresh.LoadAddedModules(); len(mods) != 0 {
		t.Fatalf("new vault modules = %v, want clean", mods)
	}
	if fresh.LoadActiveTheme() != "" {
		t.Fatal("new vault must not inherit the previous vault's theme")
	}

	// Isolation: writes in one vault never leak into the other.
	if err := fresh.SaveActiveTheme("ocean"); err != nil {
		t.Fatalf("save theme: %v", err)
	}
	if err := fresh.SaveAddedModules([]string{"passwords"}); err != nil {
		t.Fatalf("save modules: %v", err)
	}
	if old.LoadActiveTheme() != "midnight" {
		t.Fatal("old vault theme must be untouched by the new vault's writes")
	}
	if mods, _ := fresh.LoadAddedModules(); len(mods) != 1 || mods[0] != "passwords" {
		t.Fatalf("new vault modules = %v, want [passwords]", mods)
	}

	// InitCleanWorkspace is a no-op when the file exists — never wipes state.
	if err := fresh.InitCleanWorkspace(); err != nil {
		t.Fatalf("re-init: %v", err)
	}
	if fresh.LoadActiveTheme() != "ocean" {
		t.Fatal("re-init must not wipe existing workspace state")
	}
}

// Session restore + provider fallback follow the vault too.
func TestWorkspaceStoreSessionAndFallback(t *testing.T) {
	t.Setenv("aw_DATA_DIR", t.TempDir())
	store := workspaceFor(t.TempDir())
	if err := store.SaveLastSession("chat", "chat-123"); err != nil {
		t.Fatalf("save session: %v", err)
	}
	view, _ := store.LoadLastView()
	chatID, _ := store.LoadLastChatID()
	if view != "chat" || chatID != "chat-123" {
		t.Fatalf("session = %q/%q, want chat/chat-123", view, chatID)
	}
	if err := store.SaveProviderFallbackOrder([]string{"openrouter", "openai"}); err != nil {
		t.Fatalf("save order: %v", err)
	}
	if order, _ := store.LoadProviderFallbackOrder(); len(order) != 2 || order[0] != "openrouter" {
		t.Fatalf("order = %v", order)
	}
}
