package vault

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aw/internal/infrastructure/securemem"
)

// SnapshotTo writes a complete, consistent, still-encrypted copy of the open
// vault database to destPath using SQLite's VACUUM INTO — safe to run while
// the vault is in use (SQLite assembles the snapshot, WAL included, into a
// single cold file). The write is atomic: VACUUM INTO a temp file in the
// destination folder, then rename over destPath.
func (v *Vault) SnapshotTo(destPath string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.snapshotLocked(destPath)
}

// snapshotLocked is SnapshotTo with v.mu already held — the write-back tick
// and the final on-lock snapshot call it mid-flow.
func (v *Vault) snapshotLocked(destPath string) error {
	if v.db == nil {
		return fmt.Errorf("vault is locked")
	}
	destPath = strings.TrimSpace(destPath)
	if destPath == "" {
		return fmt.Errorf("destination path is required")
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	tmp := destPath + ".tmp-snapshot"
	_ = os.Remove(tmp)
	if v.key == nil {
		return fmt.Errorf("vault key unavailable")
	}
	// The target must go through the same encrypting VFS with the same key —
	// a bare path would create the copy via the DEFAULT VFS (plaintext at
	// best; with adiantum in the URI stack it comes out empty). VACUUM INTO
	// refuses to overwrite; the temp name is ours alone.
	err := v.key.Use(func(key []byte) error {
		keyHex := make([]byte, hex.EncodedLen(len(key)))
		hex.Encode(keyHex, key)
		defer securemem.Zero(keyHex)
		uri := "file:" + strings.ReplaceAll(filepath.ToSlash(tmp), " ", "%20") +
			"?vfs=adiantum&hexkey=" + string(keyHex)
		_, execErr := v.db.Exec("VACUUM INTO ?", uri)
		return execErr
	})
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("vault snapshot: %w", err)
	}
	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	// The DB alone cannot be unlocked: the key derivation salt (and the
	// recovery blob) live in sibling files. Mirror them too — tiny, static.
	destDir := filepath.Dir(destPath)
	for _, name := range []string{"vault.salt", "vault.recovery"} {
		data, err := os.ReadFile(filepath.Join(v.dir, name))
		if err != nil {
			continue // recovery may not exist yet; salt always does post-create
		}
		_ = os.WriteFile(filepath.Join(destDir, name), data, 0o600)
	}
	return nil
}

// DataVersion returns SQLite's data_version counter, which changes whenever
// another commit lands — the cheap "did anything change since the last
// mirror?" check. Returns 0 while locked.
func (v *Vault) DataVersion() int64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return 0
	}
	var version int64
	if err := v.db.QueryRow("PRAGMA data_version").Scan(&version); err != nil {
		return 0
	}
	return version
}

// LastChangeTime returns the newest mtime among the live database files (db,
// WAL, SHM) — the cheap "did anything change since the last mirror?" signal.
// Zero time while locked or when the path is unknown.
func (v *Vault) LastChangeTime() time.Time {
	v.mu.Lock()
	path := v.dbPath()
	locked := v.db == nil
	v.mu.Unlock()
	if locked || path == "" {
		return time.Time{}
	}
	var newest time.Time
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if info, err := os.Stat(p); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}
