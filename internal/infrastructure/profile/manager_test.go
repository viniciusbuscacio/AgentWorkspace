package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerCreateAtLocationDedupeAndSort(t *testing.T) {
	base := t.TempDir()
	manager := NewManager(base)
	parent := t.TempDir()

	first, err := manager.CreateAtLocation(parent, "AgentWorkspace", "database_upload")
	if err != nil {
		t.Fatalf("CreateAtLocation() error = %v", err)
	}
	if first.Name != "AgentWorkspace" {
		t.Fatalf("CreateAtLocation() name = %q", first.Name)
	}
	if !dirExists(first.VaultDir) {
		t.Fatalf("CreateAtLocation() did not create dir %s", first.VaultDir)
	}

	second, err := manager.CreateAtLocation(parent, "AgentWorkspace", "database_upload")
	if err != nil {
		t.Fatalf("second CreateAtLocation() error = %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate vault dir created a new profile: %q != %q", second.ID, first.ID)
	}

	list := manager.ListInfo()
	if len(list) != 1 {
		t.Fatalf("ListInfo() len = %d; want 1", len(list))
	}
}

func TestManagerImportAndMigrateExistingVault(t *testing.T) {
	base := t.TempDir()
	vaultDir := filepath.Join(t.TempDir(), "AgentWorkspace")
	if err := os.MkdirAll(vaultDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(vaultDir, "vault.db"), []byte("fake"), 0o600); err != nil {
		t.Fatalf("WriteFile(vault.db) error = %v", err)
	}

	manager := NewManager(base)
	imported, err := manager.Import("Imported", "database_upload", vaultDir)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if !imported.HasVault {
		t.Fatal("Import() profile HasVault is false")
	}

	selected, err := manager.Touch(imported.ID)
	if err != nil {
		t.Fatalf("Touch() error = %v", err)
	}
	if selected.ID != imported.ID {
		t.Fatalf("Touch() selected %q; want %q", selected.ID, imported.ID)
	}

	migratedBase := t.TempDir()
	migrated := NewManager(migratedBase)
	if err := migrated.MigrateExistingVault(vaultDir); err != nil {
		t.Fatalf("MigrateExistingVault() error = %v", err)
	}
	list := migrated.ListInfo()
	if len(list) != 1 {
		t.Fatalf("migrated ListInfo() len = %d; want 1", len(list))
	}
	if list[0].VaultDir != normalizeDirBestEffort(vaultDir) {
		t.Fatalf("migrated vault dir = %q; want %q", list[0].VaultDir, normalizeDirBestEffort(vaultDir))
	}
}

func TestManagerRejectsBadCreateAndImport(t *testing.T) {
	manager := NewManager(t.TempDir())

	if _, err := manager.CreateAtLocation("", "AgentWorkspace", "database_upload"); err == nil {
		t.Fatal("CreateAtLocation() with empty parent succeeded")
	}
	if _, err := manager.CreateAtLocation(t.TempDir(), "..", "database_upload"); err == nil {
		t.Fatal("CreateAtLocation() with traversal folder succeeded")
	}
	if _, err := manager.Import("Missing", "database_upload", t.TempDir()); err == nil {
		t.Fatal("Import() without vault.db succeeded")
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
