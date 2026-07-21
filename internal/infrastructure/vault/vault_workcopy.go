package vault

// The working-copy model (spec: docs/specs/vault-working-copy.md): the folder
// the user picks (v.dir — possibly inside OneDrive/iCloud) is the MASTER and
// only ever holds the cold form of the vault: a vault.db snapshot plus the
// salt/recovery sibling files. When a work dir is configured, the live SQLite
// database runs there instead — a local, ephemeral copy the app hydrates from
// the master on unlock and writes back as cold snapshots (VACUUM INTO). The
// master always wins: an external change (another machine's snapshot arriving
// via sync) freezes write-back until Resync re-hydrates. With no work dir
// configured the vault behaves exactly as before (live DB in v.dir).

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"aw/internal/domain"
	"aw/internal/infrastructure/securemem"
)

const masterStampFile = "master.stamp"

// masterStamp records the master vault.db's size+mtime as of our own last
// write (snapshot) or read (hydration). A later mismatch means another
// machine's snapshot landed — master manda, write-back must stop.
type masterStamp struct {
	Size    int64 `json:"size"`
	ModTime int64 `json:"modTimeUnixNano"`
}

// TickResult is one write-back tick's outcome (the ports-level type — the
// appcore loop consumes it through ports.VaultWriteBacker).
type TickResult = domain.VaultWriteBackResult

// SetWorkDir configures the working-copy directory; empty disables (legacy
// behavior: live DB in the master folder). Locked-only, like SetDir.
func (v *Vault) SetWorkDir(dir string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db != nil {
		return errors.New("cannot switch work dir while unlocked")
	}
	v.workDir = strings.TrimSpace(dir)
	v.stamp = nil
	return nil
}

func (v *Vault) masterDBPath() string { return filepath.Join(v.dir, dbName) }

func (v *Vault) stampPath() string { return filepath.Join(v.workDir, masterStampFile) }

// hydrateLocked prepares the working copy before the live DB opens. The
// pending-snapshot corner: a leftover working copy whose stamp still matches
// the master carries changes the final snapshot failed to persist — keep it
// and report pending=true so the first tick writes it back. Any other
// leftover is discarded: master manda.
func (v *Vault) hydrateLocked() (pending bool, err error) {
	if v.workDir == "" {
		return false, nil
	}
	if err := os.MkdirAll(v.workDir, 0o700); err != nil {
		return false, err
	}
	workDB := filepath.Join(v.workDir, dbName)
	if fileExists(workDB) && v.masterMatchesStampLocked() && v.stampExistsLocked() {
		return true, nil
	}
	// Copy first, replace after: a failed copy must leave any previous working
	// copy intact (Resync falls back to it when the master is unreachable).
	if err := copyFileAtomic(v.masterDBPath(), workDB); err != nil {
		// Reading the master is what makes OneDrive download a Files
		// On-Demand placeholder; if even that fails the master is genuinely
		// unreachable and a stale open would silently fork history.
		return false, fmt.Errorf("cannot read the vault master at %s — if the folder lives in the cloud, go online or mark it \"Always keep on this device\": %w", v.masterDBPath(), err)
	}
	// The old copy's WAL/SHM must not pair with the fresh database file.
	removeSQLiteSidecars(workDB)
	if err := v.writeStampLocked(); err != nil {
		return false, err
	}
	return false, nil
}

func (v *Vault) stampExistsLocked() bool {
	if v.stamp != nil {
		return true
	}
	return fileExists(v.stampPath())
}

func (v *Vault) readStampLocked() (masterStamp, bool) {
	if v.stamp != nil {
		return *v.stamp, true
	}
	data, err := os.ReadFile(v.stampPath())
	if err != nil {
		return masterStamp{}, false
	}
	var stamp masterStamp
	if err := json.Unmarshal(data, &stamp); err != nil {
		return masterStamp{}, false
	}
	v.stamp = &stamp
	return stamp, true
}

// writeStampLocked records the master's current identity as "ours".
func (v *Vault) writeStampLocked() error {
	info, err := os.Stat(v.masterDBPath())
	if err != nil {
		return err
	}
	stamp := masterStamp{Size: info.Size(), ModTime: info.ModTime().UnixNano()}
	v.stamp = &stamp
	data, err := json.Marshal(stamp)
	if err != nil {
		return err
	}
	return os.WriteFile(v.stampPath(), data, 0o600)
}

// masterMatchesStampLocked reports whether the master is still the version we
// last wrote/read. No stamp yet (fresh create) or an unreadable master count
// as unchanged — real write errors surface through the snapshot itself.
func (v *Vault) masterMatchesStampLocked() bool {
	stamp, ok := v.readStampLocked()
	if !ok {
		return true
	}
	info, err := os.Stat(v.masterDBPath())
	if err != nil {
		return true
	}
	return info.Size() == stamp.Size && info.ModTime().UnixNano() == stamp.ModTime
}

// liveStateLocked fingerprints the live DB files (size+mtime of db/wal/shm).
// Comparing fingerprints instead of wall-clock times sidesteps filesystem
// mtime granularity: a commit within the same mtime granule still changes the
// WAL size. False positives only cost an extra snapshot.
func (v *Vault) liveStateLocked() string {
	path := v.dbPath()
	var b strings.Builder
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if info, err := os.Stat(p); err == nil {
			fmt.Fprintf(&b, "%d:%d;", info.Size(), info.ModTime().UnixNano())
		} else {
			b.WriteString("-;")
		}
	}
	return b.String()
}

// liveChangedSinceLocked: did any live DB file change after our last
// write-back?
func (v *Vault) liveChangedSinceLocked() bool {
	return v.liveStateLocked() != v.lastWriteBack
}

// removeWorkFilesLocked deletes the ephemeral working copy (db + sidecars +
// stamp). The directory itself stays.
func (v *Vault) removeWorkFilesLocked() {
	if v.workDir == "" {
		return
	}
	workDB := filepath.Join(v.workDir, dbName)
	removeSQLiteSidecars(workDB)
	_ = os.Remove(workDB)
	_ = os.Remove(v.stampPath())
	v.stamp = nil
}

// WriteBackTick runs one write-back cycle: skip when nothing changed, freeze
// when the master changed elsewhere, otherwise snapshot the working copy over
// the master. Called every minute by the app while unlocked.
func (v *Vault) WriteBackTick() TickResult {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil || v.workDir == "" {
		return TickResult{Skipped: true}
	}
	if v.masterChanged || !v.masterMatchesStampLocked() {
		v.masterChanged = true
		return TickResult{MasterChanged: true}
	}
	if !v.pendingWriteBack && !v.liveChangedSinceLocked() {
		return TickResult{Skipped: true}
	}
	if err := v.snapshotLocked(v.masterDBPath()); err != nil {
		return TickResult{Err: err}
	}
	v.pendingWriteBack = false
	v.lastWriteBack = v.liveStateLocked()
	if err := v.writeStampLocked(); err != nil {
		return TickResult{Err: err}
	}
	return TickResult{Wrote: true}
}

// MasterChanged reports whether write-back is frozen because the master was
// updated outside this session.
func (v *Vault) MasterChanged() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.masterChanged
}

// Resync adopts the changed master mid-session: close the live DB (keeping
// the key in memory), re-hydrate, reopen — an invisible lock+unlock with no
// password prompt. Local changes since the last successful snapshot are
// discarded (master manda). The caller picks an idle moment (no active chat
// run) so no in-flight write is severed.
func (v *Vault) Resync() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil || v.workDir == "" {
		return nil
	}
	key, err := v.unwrapKeyLocked()
	if err != nil {
		return err
	}
	defer securemem.Zero(key)
	old := v.db
	v.db = nil
	_, _ = old.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	_ = old.Close()
	// The stale stamp is what flags the master as changed — drop it so
	// hydration discards the old working copy and copies the new master.
	v.stamp = nil
	_ = os.Remove(v.stampPath())
	if _, err := v.hydrateLocked(); err != nil {
		// Master unreadable mid-resync: fall back to the stale working copy —
		// still frozen (masterChanged stays true), so it can never overwrite
		// the newer master.
		v.reopenLocked(key)
		return err
	}
	db, err := openEncrypted(v.dbPath(), key)
	if err != nil {
		return err
	}
	v.db = db
	v.masterChanged = false
	v.pendingWriteBack = false
	v.lastWriteBack = v.liveStateLocked()
	return nil
}

// copyFileAtomic copies src over dst via a temp file + rename so a crash
// mid-copy never leaves a half-written working copy behind.
func copyFileAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp-hydrate"
	_ = os.Remove(tmp)
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
