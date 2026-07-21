package vault

import (
	"os"
	"path/filepath"
	"testing"
)

// The snapshot must be complete, still encrypted with the vault key, and
// openable as a vault on its own.
func TestSnapshotToProducesUsableEncryptedCopy(t *testing.T) {
	dir := t.TempDir()
	v := New(dir)
	if _, err := v.Create("senha-123"); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Close the sqlite handle before TempDir cleanup — Windows refuses to
	// delete files a process still holds open.
	t.Cleanup(func() { _ = v.Lock() })
	if err := v.SetSecret("probe", "valor-espelhado"); err != nil {
		t.Fatalf("secret: %v", err)
	}
	dest := filepath.Join(dir, "mirror", "vault.db")
	if err := v.SnapshotTo(dest); err != nil {
		t.Fatalf("SnapshotTo: %v", err)
	}
	info, err := os.Stat(dest)
	if err != nil || info.Size() == 0 {
		t.Fatalf("snapshot missing/empty: %v size=%d", err, info.Size())
	}
	// Encrypted: plaintext sqlite header must NOT appear.
	head := make([]byte, 16)
	f, _ := os.Open(dest)
	_, _ = f.Read(head)
	_ = f.Close()
	if string(head) == "SQLite format 3\x00" {
		t.Fatal("snapshot is plaintext — encryption lost")
	}
	// The copy opens as a vault with the same password and carries the data.
	v2 := New(filepath.Dir(dest))
	if err := v2.Unlock("senha-123"); err != nil {
		t.Fatalf("unlock copy: %v", err)
	}
	t.Cleanup(func() { _ = v2.Lock() })
	got, ok, err := v2.GetSecret("probe")
	if err != nil || !ok || got != "valor-espelhado" {
		t.Fatalf("secret in copy = %q ok=%v err=%v", got, ok, err)
	}
}
