# Spec: vault working copy (master anywhere, live DB local)

**Status:** draft — awaiting the owner's review
**Date:** 2026-07-20
**Origin:** the live SQLite vault currently runs inside OneDrive (sync client
corrupts open databases — see `synced_path.go`). The owner's design: the chosen
vault folder (OneDrive or not) is the **master**; the app works on a hidden
local **working copy** and snapshots back. Empirical probe confirmed new
vaults today still create a live DB inside OneDrive with no veto.

## Concept

- **Master** = the folder the user picks and sees ("localização do vault").
  Holds only the cold form: `vault.db` (snapshot), `vault.salt`,
  `vault.recovery`. Never `-wal`/`-shm`. Any folder is allowed, synced or not —
  no veto anywhere.
- **Working copy** = internal, invisible, ephemeral live DB at
  `%LOCALAPPDATA%\AW\work\<profile-id>\` (macOS/Linux: the platform-local
  non-synced equivalent, via `os.UserCacheDir()`). Same encryption as the
  master (same adiantum file — hydration is a file copy, write-back is
  `VACUUM INTO` with the same key). Treated like RAM: safe to delete whenever
  the app is closed. **Scope refinement:** only `vault.db` needs the working
  copy — `vault.salt` and `vault.recovery` are small files written atomically
  and rarely (never held open), so they stay master-only for reads and
  writes. Sync clients only corrupt *open* databases.
- **Master always wins.** The working copy never outranks it. Anything not yet
  snapshotted to the master is, by design, forgettable (bounded by the 1-min
  write-back cadence). No conflict files, no `.conflict-*` duplicates, ever.

## Decided behavior (the owner, 2026-07-20)

| # | Decision |
|---|----------|
| 1-2 | Working copy applies to ALL new vaults, synced folder or not. Fixed behavior — no toggle. |
| 3 | The Settings › Vault "Cloud mirror" card and its config are REMOVED (superseded by this model — write-back is now intrinsic). |
| 4 | Working copy path: `%LOCALAPPDATA%\AW\work\<profile-id>\`. |
| 5 | Same master opened by two profiles/installs on one machine → each has its own working copy. |
| 6 | Working copy is deleted on profile forget, and is deletable after app close (ephemeral). |
| 7 | Write-back: 1-minute tick that skips when nothing changed since the last snapshot. |
| 8 | Lock (manual/auto) and app quit do a final synchronous snapshot, even if it costs 1-2s. |
| 9 | Snapshot failures never block work: keep writing to the working copy, keep retrying. |
| 13 | Multi-machine model is alternating use; the master carries state between machines. |
| 14-15, 19 | Master é que manda: no lease file, no conflict copies, no working-copy-wins recovery. Unsaved-to-master changes are forgotten. |
| 16 | UI shows only the master path; the working copy is 100% invisible. |
| 17 | Failures are retried quietly every tick; only warn after ~20 consecutive failures (~20 min). Success clears the counter. |
| 18 | Working copy keeps the identical encryption; nothing extra. |

## Claude's proposals for the open questions (veto if wrong)

- **Q10 — when to check the master:** at every unlock (mandatory hydration)
  and at every snapshot tick, cheaply, via mtime/size recorded after our own
  last write. No extra watcher. Rationale: the tick already touches the file;
  a dedicated monitor adds moving parts without changing the outcome.
- **Q11 — master changed externally while this session is open** (decided
  with the owner, 2026-07-20): **idle auto-resync.** Snapshots freeze
  immediately (never overwrite the newer master — master manda). As soon as
  the app is idle (no chat run active), it re-syncs automatically: close the
  live DB keeping the key in memory, re-hydrate from the master, reopen,
  refresh the UI. No password prompt, no notification in the common case —
  the machine you left open just catches up. Local changes since the last
  successful snapshot are discarded (accepted: master manda). Only if a chat
  run is active does the swap wait for the turn to finish.
- **Q12 — Files On-Demand placeholder / unreachable master at unlock:**
  hydration reads the master file, which makes OneDrive download it
  transparently when online. If the read fails (offline placeholder, folder
  missing): unlock FAILS with a clear message ("the vault master is only in
  the cloud — go online, or mark the folder 'Always keep on this device'").
  No fallback to a stale working copy: master manda, and a stale open would
  silently fork history. The spec recommends surfacing the 'Always keep on
  this device' tip in that error.
- **Q8/Q19 corner — final snapshot fails at close (e.g. OneDrive down):** keep
  the working copy instead of deleting it, notify once. On the NEXT unlock:
  if the master is byte-identical to what we last successfully wrote (mtime
  marker), write the pending snapshot first — nothing is lost; if the master
  changed externally meanwhile, master manda — discard the leftover and
  hydrate. This preserves "no duplicates, master wins" while not throwing
  away a session's work over a transient outage.
- **Q20 — validation:** unit tests (hydration, tick skip-if-unchanged,
  external-change freeze, failure counter, forget-deletes-work-dir) plus a
  dev-mode probe like today's: create a new vault with the master in the real
  OneDrive, confirm the master folder only ever holds the cold form (no
  `-wal`/`-shm`) while the app runs, confirm `%LOCALAPPDATA%\AW\work\<id>\`
  holds the live DB, edit the master with the app closed and confirm the next
  unlock adopts it.

## Lifecycle (consolidated)

1. **Unlock** → copy master `vault.db`+`salt`+`recovery` → work dir
   (unconditional; leftover working copy is discarded unless the
   pending-snapshot corner above applies). Open the live DB from the work dir.
2. **Session** → every write hits the working copy. Each 1-min tick: skip if
   unchanged; else check master mtime → unchanged: `VACUUM INTO` temp in the
   master folder + atomic rename (reuse today's `SnapshotTo`); changed
   externally: enter `master-changed-elsewhere` (freeze snapshots, notify
   once).
3. **Lock/quit** → final synchronous snapshot (skipped in
   `master-changed-elsewhere`), then delete the working copy on success.
4. **Profile forget** → delete `%LOCALAPPDATA%\AW\work\<profile-id>\`.

## Out of scope

- Migration of existing profiles (the owner: "esquece migração" — he will
  create a fresh vault).
- Lease files / simultaneous-open arbitration beyond the freeze+notify above.
- Merge of concurrent changes.
- Any veto on synced folders (explicitly NOT wanted — OneDrive masters are a
  supported, first-class choice).

## Implementation sketch (for sizing, not code yet)

- `vault` package: hydration helper (copy in), `SnapshotTo` reused (copy out),
  work-dir resolution by profile id.
- `appcore`: replace `vaultMirrorLoop` with the write-back loop (tick, skip,
  freeze, failure counter ≥20 → notify); wire final snapshot into Lock and
  shutdown; delete work dir on forget/clean close.
- Settings UI: remove the Cloud mirror card; vault page keeps showing the
  master path only.
- Config: drop `vaultMirrorDir` (dead key stays ignored for old configs).
- Rough size: ~medium — one focused branch, backend-heavy, small UI removal.

## Rollout

1. The owner reviews this spec (especially the four proposal bullets).
2. Implement; buildgate green; dev-mode probe recorded.
3. The owner creates his new vault pointing the master at OneDrive.
