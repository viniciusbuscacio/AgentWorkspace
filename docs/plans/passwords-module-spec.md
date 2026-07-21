# Spec — Passwords module (port of the AW2 model)

> **Status:** implemented (2026-06-12, commit f2ee048 + de672ab — salvaged). Workspace-module
> spec in the established series — same conventions, same gates. This is a
> **port of AW2's Passwords module** on top of aw's existing vault secrets.
> Reference implementation in `AgentWorkspace2`:
>
> - `src/renderer/modules/passwords/PasswordsModule.tsx` — the UI (list +
>   editor, search, reveal/hide, copy)
> - `src/renderer/modules/passwords/passwords.service.ts` — the
>   `_password:` prefix and the JSON item shape
> - `src/shared/domain/module-catalog.ts:33` — catalog entry (icon `key`,
>   singleton)
>
> When in doubt about semantics or visuals, open the AW2 file and copy.

## 1. Objective

Give the user a local, encrypted password manager inside the workspace —
entries live in the vault the app already has. The AW2 pitch carries over:
"Encrypted, local-only, never in chat history."

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Storage = existing vault secrets** with AW2's `_password:` key prefix; the value is one JSON document per entry: `{name, username, password, url, notes, updatedAt}`. No new table — reuse `ports.SecretStore` through application use cases. |
| 2 | **UI ports AW2**: list with live search (name/username/url) + editor pane; password masked by default (`<input type="password">`), Show/Hide toggle, **Copy without revealing**, create/edit/delete. Module id `passwords`, icon `key`, singleton, standard sidebar contract (`defineModuleView`). |
| 3 | **UI-only module in v1: `Actions: []`.** Deviation from AW2 (where the agent could `vault.get('_password:…')`): the aw agent has NO secrets actions today and this spec does not add any. The agent gets a credential only when the user deliberately pastes it. Revisit only with a concrete use case — never as a side effect. |
| 4 | **`_password:*` keys are excluded from every agent-visible and generic listing**: the Settings → Security secrets list keeps its own behavior, but `system.state`, prompt blocks and any agent-facing surface treat `_password:*` like the `_config_*` internal convention. Test pinned. |
| 5 | **Snapshot hardening (aw-specific):** `ui.snapshot` already masks `input[type=password]` (`frontend/src/lib/ui-automation.ts:183`). The Reveal toggle switches the input to `type=text`, which would leak through a snapshot — so the module marks the revealed field with `data-sensitive`, and `ui-automation` learns to mask ANY element carrying `data-sensitive` (`••••••`). Test pinned in the ui-automation suite. This closes a gap AW2 shipped with. |
| 6 | Password values never enter chat history, events, logs or `aw` results — same posture as the provider secrets. |

## 3. What already exists — reuse, don't reinvent

- Vault secrets CRUD: `internal/application/secrets.go`, `ports.SecretStore`,
  the Wails secret surface in `app.go` (`SecretResult`). Add list/get/set
  use cases that speak the `_password:` prefix; do not fork the store.
- The internal-config listing convention (`_config_*` keys hidden) — extend
  it, don't duplicate it.
- Module plumbing: catalog entry in `internal/domain/module.go`, typed
  registry entry via `defineModuleView` (singleton, compile-time contract),
  drift tests.
- Clipboard: `navigator.clipboard.writeText` as in AW2.

## 4. Phases

### Phase 1 — Backend (use cases + listing exclusion)

Password entry use cases over SecretStore (list parses only `_password:*`
keys; set/delete validate the prefix); `_password:*` excluded from agent-
facing listings (Decision 4) with tests; Wails surface (`app_passwords.go`
following `app_notes.go`), hand-edited wailsjs stubs.

### Phase 2 — UI module

Catalog + registry entries; `PasswordsModule.tsx` ported from AW2 (split
list/editor, search, masked + reveal + copy); `data-sensitive` masking in
`ui-automation.ts` with its test (Decision 5). Vitest for the component
following the neighbors' pattern.

## 5. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

Update `docs/SELFCODE.md` in Phase 2 (module list + the explicit note that
the agent has no access to password values).

## 6. Risks / attention

1. **The agent must not gain read access by accident**: any future secrets
   action must keep the `_password:*` exclusion; the Decision 4 test is the
   regression fence.
2. Reveal + `ui.snapshot`: Decision 5 is mandatory, not cosmetic — without
   it the agent can read a revealed password through the snapshot.
3. JSON-in-secret parsing must tolerate corrupt/legacy values (skip, never
   crash the list).
4. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart — say so in summaries.
