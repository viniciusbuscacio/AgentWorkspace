package domain

// VaultWriteBackResult is one working-copy write-back tick's outcome (spec:
// docs/specs/vault-working-copy.md).
type VaultWriteBackResult struct {
	// Wrote: a fresh snapshot reached the master.
	Wrote bool
	// Skipped: nothing to do (locked, no work dir, or no change since the
	// last snapshot).
	Skipped bool
	// MasterChanged: the master changed outside this session — snapshots are
	// frozen; the caller should resync at a safe (idle) moment.
	MasterChanged bool
	Err           error
}
