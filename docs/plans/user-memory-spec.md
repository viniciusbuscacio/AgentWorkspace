# Spec — User Memory (long-term facts about the user, in the vault)

> **Status:** implemented (2026-06-10, phases 1–4, one commit each:
> da499f9, 3748379, 23de6e6, 9fd9a03). Kept as the design record. The compose
> step landed as `application.RefreshAgentContext` (option (a) of §6 Phase 2),
> replacing `RefreshChatMemoryContext`.

## 1. Objective

Give aw a **long-term memory about the user** (preferences, recurring context,
facts the user states or the agent learns) that:

- persists across chats and app restarts,
- is **per-vault / per-profile** and **encrypted** (lives in the vault, never on
  the filesystem, never in git),
- is **readable and editable by the user** (a Settings page),
- is **available to the agent** in every chat: injected as a compact block in
  the prompt, plus a tool to record/update a fact mid-conversation.

This is the user-facts sibling of the existing **chat memory** (`llm_turns`,
`sessions.summary`, `messages_fts`). Reuse that design; do not invent a new one.

Decision context: user facts are *instance state* (runtime, per-user, personal),
not *project docs* — so SQLite-in-vault, not a Markdown file. Rule of thumb:
*"does this go in a commit, or does the app write it during use?"* — app writes
it → vault.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | Storage = a table in the encrypted vault (`internal/infrastructure/vault`), not a file. |
| 2 | A fact = `{ key, category, content, source, updated_at }`. `content` is free text (Markdown allowed); the row is structured, the body is rich. |
| 3 | `key` is a stable slug; writing the same key **upserts** (no duplicates). |
| 4 | Categories mirror the agent-memory model: `profile` (who they are), `preference` (how they want things done), `context` (ongoing projects/goals), `reference` (links/resources). |
| 5 | `source` ∈ `agent` \| `user` — who last wrote the fact. |
| 6 | Agent reads via a prompt-injected block **and** can write via a dedicated always-on tool; the user curates via a Settings page. |
| 7 | No vector search. Memory is small and fully injected (with a cap, see §5). If it ever outgrows the cap, add lexical search later — out of scope here. |
| 8 | Agent writes do **not** require confirmation (memory is non-destructive). Deleting a fact from the UI is a normal user action. |

## 3. What already exists — reuse, don't reinvent

- **Prompt injection:** `ports.MemoryContextSetter` (`internal/domain/ports/vault.go`)
  with `SetMemoryContext(extra string)`, implemented by the agent runtime
  (`internal/infrastructure/agent/runtime.go:86`). Today **chat memory** drives it
  via `application.RefreshChatMemoryContext(...)`, called from `app.go:405`.
  ⚠️ `SetMemoryContext` takes **one** block. User memory must **compose** with the
  chat-memory catalog, not overwrite it (see §6, Phase 2).
- **Always-on dedicated tools pattern:** `chat_memory.go` registers
  `search_chat_history` / `open_chat_session` / `list_recent_chat_sessions` via
  `functiontool`, wired through injected funcs on `Options`
  (`ChatSearchFn`/`ChatHistoryFn`/`ChatCatalogFn` in `tools.go`). Copy this for the
  user-memory write/read tool — do **not** access the vault from infra/tools
  directly; pass an injected function from the composition root.
- **Vault migrations:** `CREATE TABLE IF NOT EXISTS` and `ensureColumn(...)` in
  `internal/infrastructure/vault/vault.go` (~line 1180). Add the new table there.
- **Settings page pattern:** `frontend/src/modules/settings/` + a typed
  `services/*.service.ts` wrapper; `SecurityPage.tsx` (list + delete rows) is the
  closest visual analog.
- **Boundary tests:** `internal/architecture` (6 tests). Any new infra/port field
  on `App` is auto-guarded — access it through `internal/application`, never call
  the adapter directly from package `main`.

## 4. Architecture by layer (respect the dependency rule)

`interface → application → domain`, `infrastructure → domain`.

- `internal/domain`: `UserMemoryFact` type; a `UserMemoryStore` port (or extend an
  existing vault port) with `UpsertUserFact`, `ListUserFacts`, `DeleteUserFact`.
- `internal/application`: use cases — `RecordUserFact`, `ListUserFacts`,
  `DeleteUserFact`, `UserMemoryInstruction(facts) string` (formats the prompt
  block), and the compose step in §6.
- `internal/infrastructure/vault`: schema + repository methods implementing the port.
- `internal/infrastructure/tools`: the dedicated `remember_user_fact` tool, wired
  by an injected func on `Options` (mirror `ChatSearchFn`).
- `internal/dto`: response struct(s) for the Wails surface.
- package `main` (`app*.go`): Wails methods + wiring; compose user-memory into the
  context refresh.

## 5. Schema (vault)

In `vault.go`, beside the other tables. Use `CREATE TABLE IF NOT EXISTS`.

```sql
CREATE TABLE IF NOT EXISTS user_memory (
  key        TEXT PRIMARY KEY,         -- stable slug, e.g. "preferred-language"
  category   TEXT NOT NULL,            -- profile | preference | context | reference
  content    TEXT NOT NULL,            -- free text / Markdown
  source     TEXT NOT NULL,            -- agent | user
  updated_at TEXT NOT NULL
);
```

Upsert on `key` (`INSERT ... ON CONFLICT(key) DO UPDATE`). Cap the injected block:
at most **~40 facts** or **~2 KB** of text in the prompt (newest/by-category
first); the Settings page shows all. Log the cap if it truncates.

## 6. Phases

### Phase 1 — Schema + CRUD (vault + application + domain)
1. `vault.go`: `user_memory` table (§5) + methods `UpsertUserFact(fact)`,
   `ListUserFacts()` (ordered), `DeleteUserFact(key)`.
2. `internal/domain`: `UserMemoryFact` + `UserMemoryStore` port.
3. `internal/application`: `RecordUserFact`, `ListUserFacts`, `DeleteUserFact`
   (thin use cases; validate category, non-empty key/content, trim).
4. Tests: vault round-trip (insert → update same key → list → delete); application
   validation.
**Accept:** a fact survives a vault lock/unlock and upserts by key.

### Phase 2 — Inject into the prompt (compose with chat memory)
1. `application`: `UserMemoryInstruction(facts) string` — a short, labeled block
   (e.g. `## What I know about <user>` with one bullet per fact by category).
2. **Compose, don't overwrite.** Refactor the context refresh so chat-memory and
   user-memory blocks are concatenated and pushed via `SetMemoryContext` **once**.
   Options: (a) a new `application.RefreshAgentContext(setter, chatStore,
   userStore, sessionID)` that builds both blocks and calls `SetMemoryContext` with
   the joined text, replacing the direct `RefreshChatMemoryContext` call at
   `app.go:405`; or (b) extend the setter to keep named blocks. Prefer (a) — one
   call site, no port change.
3. Tests: instruction formatting; compose includes both blocks; empty user-memory
   yields only the chat block (no regression).
**Accept:** with a fact saved, a new chat's prompt contains the user-memory block
alongside the chat catalog.

### Phase 3 — Agent write tool (always-on)
1. `tools.go` `Options`: add `UserMemoryRecordFn func(ctx, key, category, content) (any, error)`
   (mirror `ChatSearchFn`); wire it in `app.go` `toolOptions(...)` to
   `application.RecordUserFact(a.vault, ...)`.
2. New dedicated tool `remember_user_fact` in a `user_memory.go` (mirror
   `chat_memory.go`): args `{ key, category, content }`. Always available (every
   chat), like the chat-memory tools. Update the tool description.
3. Update `docs/SELFCODE.md` (the AW Tool Actions / always-on tools section).
4. Tests: tool calls the injected func with parsed args; rejects bad category.
**Accept:** the agent can record a fact mid-chat; it appears on next prompt and in
the Settings page.

### Phase 4 — Settings page (user curation)
1. `app.go`: Wails methods `ListUserMemory()` / `DeleteUserMemoryFact(key)` (+
   optionally `SetUserMemoryFact` for manual add/edit); dto structs; regenerate
   bindings (`wails generate module`).
2. Front: `services/user-memory.service.ts` (typed, imports `@wails/go/models`);
   `modules/settings/pages/UserMemoryPage.tsx` — list facts grouped by category
   with delete (and optional add/edit), `source` shown. Register the card in
   `SettingsModule.tsx` (id + title + icon + render branch), mirroring the MCP/REST
   pages.
3. Tests (vitest + happy-dom): service maps the Go return; page renders grouped
   facts and deletes one.
**Accept:** opening Settings → "Memory" shows saved facts; deleting one removes it
from the vault and the next prompt.

## 7. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate            # lint + go test + wails build
cd frontend && npm run build:frontend   # phases 3–4 (frontend touched)
```

The 6 `internal/architecture` tests must stay green.

## 8. Risks / attention

- **Compose, never clobber** the chat-memory block (§6 Phase 2) — the regression
  to avoid.
- **Privacy:** facts are personal. The vault is encrypted, so storage is fine, but
  never log `content` outside the vault, and keep it out of `system.state`.
- **Prompt budget:** cap the injected block (§5); the full set lives in the UI.
- **No `git add -A`** (shared working copy) — stage the files you changed. Small,
  descriptive commits, one per phase. Filenames use a hyphen, never an em dash.
- **Reopen note:** these are backend changes; the running app loads them only
  after a restart — say so in the final summary.
