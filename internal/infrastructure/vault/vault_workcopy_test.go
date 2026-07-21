package vault

import (
	"os"
	"path/filepath"
	"testing"
)

// The working-copy model: master folder holds only the cold form; the live DB
// runs in the work dir; master always wins. Spec:
// docs/specs/vault-working-copy.md (2026-07-20).

func newWorkVault(t *testing.T, master string) *Vault {
	t.Helper()
	v := New(master)
	if err := v.SetWorkDir(filepath.Join(t.TempDir(), "work")); err != nil {
		t.Fatalf("SetWorkDir: %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	return v
}

func TestWorkingCopyLifecycle(t *testing.T) {
	master := t.TempDir()
	v := newWorkVault(t, master)
	if _, err := v.Create("senha-123"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Master holds the cold form only; the live DB (and its WAL) runs in the
	// work dir.
	if !fileExists(filepath.Join(master, "vault.db")) {
		t.Fatal("master vault.db missing after create")
	}
	for _, sidecar := range []string{"vault.db-wal", "vault.db-shm"} {
		if fileExists(filepath.Join(master, sidecar)) {
			t.Fatalf("master must never hold live sidecar %s", sidecar)
		}
	}
	workDB := v.dbPath()
	if filepath.Dir(workDB) == master {
		t.Fatal("live DB must not run in the master folder")
	}
	if !fileExists(workDB) {
		t.Fatal("working copy missing while unlocked")
	}

	// Tick: a change writes back; no change skips.
	if err := v.SetSecret("probe", "v1"); err != nil {
		t.Fatalf("secret: %v", err)
	}
	if result := v.WriteBackTick(); !result.Wrote {
		t.Fatalf("tick after change = %+v, want Wrote", result)
	}
	if result := v.WriteBackTick(); !result.Skipped {
		t.Fatalf("tick without change = %+v, want Skipped", result)
	}

	// Lock: final snapshot reaches the master, the ephemeral copy is deleted.
	if err := v.SetSecret("probe", "v2"); err != nil {
		t.Fatalf("secret: %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if fileExists(workDB) {
		t.Fatal("working copy must be deleted after a clean lock")
	}

	// A fresh vault (new work dir) hydrates from the master and sees v2.
	v2 := newWorkVault(t, master)
	if err := v2.Unlock("senha-123"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if got, ok, err := v2.GetSecret("probe"); err != nil || !ok || got != "v2" {
		t.Fatalf("secret after rehydration = %q ok=%v err=%v, want v2", got, ok, err)
	}
}

// Alternating machines: whoever locked last owns the master; the next unlock
// adopts it unconditionally.
func TestWorkingCopyMasterWinsAcrossMachines(t *testing.T) {
	master := t.TempDir()
	a := newWorkVault(t, master)
	if _, err := a.Create("senha-123"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := a.SetSecret("k", "from-A"); err != nil {
		t.Fatalf("secret: %v", err)
	}
	if err := a.Lock(); err != nil {
		t.Fatalf("lock A: %v", err)
	}

	// "Machine B": own work dir, same master.
	b := newWorkVault(t, master)
	if err := b.Unlock("senha-123"); err != nil {
		t.Fatalf("unlock B: %v", err)
	}
	if err := b.SetSecret("k", "from-B"); err != nil {
		t.Fatalf("secret B: %v", err)
	}
	if err := b.Lock(); err != nil {
		t.Fatalf("lock B: %v", err)
	}

	if err := a.Unlock("senha-123"); err != nil {
		t.Fatalf("unlock A again: %v", err)
	}
	if got, _, _ := a.GetSecret("k"); got != "from-B" {
		t.Fatalf("A sees %q, want from-B (master manda)", got)
	}
}

// Simultaneous use: the session that detects a newer master freezes write-back
// and Resync adopts the master, discarding local unsaved changes.
func TestWorkingCopyExternalChangeFreezesThenResyncs(t *testing.T) {
	master := t.TempDir()
	a := newWorkVault(t, master)
	if _, err := a.Create("senha-123"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// While A stays unlocked, "machine B" writes the master.
	b := newWorkVault(t, master)
	if err := b.Unlock("senha-123"); err != nil {
		t.Fatalf("unlock B: %v", err)
	}
	if err := b.SetSecret("winner", "B"); err != nil {
		t.Fatalf("secret B: %v", err)
	}
	if err := b.Lock(); err != nil {
		t.Fatalf("lock B: %v", err)
	}

	// A writes locally — this change is doomed (master manda).
	if err := a.SetSecret("loser", "A"); err != nil {
		t.Fatalf("secret A: %v", err)
	}
	result := a.WriteBackTick()
	if !result.MasterChanged {
		t.Fatalf("tick = %+v, want MasterChanged (B rewrote the master)", result)
	}
	if !a.MasterChanged() {
		t.Fatal("MasterChanged() should stay true until resync")
	}

	if err := a.Resync(); err != nil {
		t.Fatalf("resync: %v", err)
	}
	if got, _, _ := a.GetSecret("winner"); got != "B" {
		t.Fatalf("after resync winner = %q, want B", got)
	}
	if _, ok, _ := a.GetSecret("loser"); ok {
		t.Fatal("local change must be discarded by resync (master manda)")
	}
	if a.MasterChanged() {
		t.Fatal("resync must clear the frozen state")
	}
	if result := a.WriteBackTick(); result.MasterChanged || result.Err != nil {
		t.Fatalf("tick after resync = %+v, want normal operation", result)
	}
}

// A failed final snapshot keeps the working copy; the next unlock writes it
// back instead of losing the session (pending-snapshot rule) — as long as the
// master is still ours.
func TestWorkingCopyPendingSnapshotAfterFailedLock(t *testing.T) {
	master := t.TempDir()
	v := newWorkVault(t, master)
	if _, err := v.Create("senha-123"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := v.SetSecret("probe", "unsaved"); err != nil {
		t.Fatalf("secret: %v", err)
	}

	// Sabotage the snapshot: a non-empty directory squatting on the temp path
	// makes VACUUM INTO/rename fail.
	obstruction := filepath.Join(master, "vault.db.tmp-snapshot")
	if err := os.MkdirAll(filepath.Join(obstruction, "x"), 0o700); err != nil {
		t.Fatalf("obstruction: %v", err)
	}
	workDB := v.dbPath()
	_ = v.Lock()
	if !fileExists(workDB) {
		t.Fatal("working copy must survive a failed final snapshot")
	}
	if err := os.RemoveAll(obstruction); err != nil {
		t.Fatalf("cleanup obstruction: %v", err)
	}

	// Next unlock keeps the leftover (stamp still matches the master) and the
	// first tick writes the pending change back.
	if err := v.Unlock("senha-123"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if got, ok, _ := v.GetSecret("probe"); !ok || got != "unsaved" {
		t.Fatalf("pending change lost: got %q ok=%v", got, ok)
	}
	if result := v.WriteBackTick(); !result.Wrote {
		t.Fatalf("first tick = %+v, want Wrote (pending write-back)", result)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("final lock: %v", err)
	}

	// The master now carries the once-unsaved change.
	fresh := newWorkVault(t, master)
	if err := fresh.Unlock("senha-123"); err != nil {
		t.Fatalf("fresh unlock: %v", err)
	}
	if got, ok, _ := fresh.GetSecret("probe"); !ok || got != "unsaved" {
		t.Fatalf("master missing the pending change: got %q ok=%v", got, ok)
	}
}
