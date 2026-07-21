# Spec — Logs module (port of the AW2 model)

> **Status:** implemented (2026-06-12, commit 4ed197d + de672ab — salvaged). Workspace-module
> spec in the established series — same conventions, same gates. This is a
> **port of AW2's unified log + Logs module**, scoped to the sources aw
> actually has. Reference implementation in `AgentWorkspace2`:
>
> - `src/shared/infrastructure/unified-log.ts` — the `logs` table schema and
>   query shape
> - `src/shared/logging/log-entry.ts` — entry fields + sensitive-data
>   sanitization before write
> - `src/renderer/modules/logs/` — `LogsModule.tsx`, `LogsToolbar.tsx`,
>   `LogsEntries.tsx`, `LogsRetention.tsx`, `logs.utils.ts` (filters,
>   50/page pagination, retention)
> - `src/shared/domain/module-catalog.ts:39` — catalog entry (icon
>   `clarify`, singleton)
>
> When in doubt about semantics or visuals, open the AW2 file and copy.

## 1. Objective

User-facing transparency: a module that shows what the app and the agent
did — actions dispatched, things the sandbox blocked, errors — without
reading code or asking. ("Focar no user": trust comes from being able to
look.)

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **New vault table `logs`**, porting AW2's schema: `id, timestamp, level (0-7), level_name, source, module_id, module_type, session_id, message, context_json, error_name, error_message, error_stack, created_at`. Append-only: no update/edit path exists; deletion happens ONLY through retention cleanup. |
| 2 | **Writes are best-effort and never block or fail the operation being logged** (same posture as `RecordLLMTurn`): a logging error is swallowed (and at most counted), never propagated. |
| 3 | **Sanitize before write**: message, error fields and context pass the existing chat secret scrubbing (`internal/application/chat_secret_scrub.go`). Additionally, `aw` action logs record the **action name and outcome only — never raw args** (args may carry file contents, secrets, page text). |
| 4 | **v1 sources (closed list):** (a) `aw` action dispatches — name, ok/blocked/error; (b) sandbox blocks — path/command refused + mode; (c) tool confirmations: asked/approved/denied; (d) chat turn errors (the `TurnRecordErrors` path); (e) app lifecycle — unlock/lock, MCP/REST server start/stop, module add/remove. Explicit non-goals in v1: HTTP request logs, per-token streaming logs, frontend console. |
| 5 | **UI ports AW2**: date dropdown (with per-date counts), level filter (Emergency→Debug), source filter, text search, 50/page + Load more, expandable details (context JSON, stack), retention setting (default 7 days, range 1-90) with double-click-confirmed "Clean old logs". Module id `logs`, icon `clarify`, singleton, standard sidebar contract. |
| 6 | **One read-only agent action: `logs.list`** `{date?, level?, source?, search?, limit?}` (cap 200 rows, fields sans context blobs). The agent can inspect what happened — useful for self-debugging — but has no write/delete action. Registered in the module spec (drift test covers it). |
| 7 | Retention cleanup runs on user request (the button) and once per unlock (sweep entries older than the configured days). |

## 3. What already exists — reuse, don't reinvent

- Vault migration pattern (`CREATE TABLE IF NOT EXISTS` block,
  `internal/infrastructure/vault/vault.go:~1300`) — add `logs` + indexes
  (timestamp, level, source).
- Scrubbing: `ScrubChatSecrets` — reuse, do not write a second scrubber.
- Best-effort persistence posture: the `RecordLLMTurn` fix (812b87b).
- Hook points already centralized: the `aw` dispatcher
  (`internal/infrastructure/tools/aw_registry.go` — one choke point logs
  every action), the sandbox blocked results, `requireConfirmation`, the
  module add/remove use cases, lock/unlock in the composition root.
- Module plumbing: catalog + `defineModuleView` + drift tests.

## 4. Phases

### Phase 1 — Log infrastructure (table + writer + hooks)

`logs` table + vault accessors (insert, query with filters, dates-with-
counts, delete-older-than); a `ports.AppLogger` port + application use
cases; hooks at the Decision 4 sources. Tests: schema migration, scrubbing
applied, append-only (no update API), best-effort (vault locked → operation
still succeeds), retention sweep.

### Phase 2 — UI module + agent action

Catalog + registry entries; `LogsModule` ported from AW2 (toolbar filters,
entries list, pagination, retention); `logs.list` action (Decision 6) +
SELFCODE update. Vitest for the component; Go test for the action caps.

## 5. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. **Logging the logger**: `logs.list` dispatches through the `aw`
   dispatcher, which logs dispatches — make the logger skip `logs.*` (or
   the table grows by reading it).
2. **Volume**: per-dispatch logging is fine; anything chattier (streaming,
   per-token) is out of scope by Decision 4 — do not add "just one more"
   source.
3. **No secrets in logs**: Decision 3 is the fence; the test writes an
   action with a secret-looking arg and asserts the stored row has none.
4. Vault size: retention default 7 days keeps it bounded; the sweep must
   use the index, not a table scan per row.
5. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart — say so in summaries.
