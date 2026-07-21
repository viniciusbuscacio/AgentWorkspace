# Spec — Sidebar & module windows (port of the AW2 model)

> **Status:** implemented (phases 1-4: commits a776588 "Sidebar phase 1",
> 06da644 "Notes parity", 786db68 "Backlog parity", 998cbf8 "Notepad module",
> mais merges 89691d3/a066827/56ea7f1 e follow-up c36950a). Kept as the design
> record. Fourth spec in the series (`workspace-modules-spec.md`,
> `prompt-base-draft.md`, `permissions-spec.md`) — same conventions, same gates.
> This was a **port of AW2's sidebar instance model and of three modules**, not a
> new design. The reference implementation lives in
> `AgentWorkspace2`:
>
> - `src/renderer/app/Sidebar.tsx` — the generic item renderer; `closeModule`
>   (lines ~212-242), per-item context menu (~511-522), hover close button
>   (~662-670). Note: NO section headings — a flat ordered list.
> - `src/renderer/app/sidebar-module-policy.ts` — `shouldHideModuleOnClose`
>   (singleton → hide, never delete)
> - `src/shared/domain/module-catalog.ts` — the `singleton?: boolean` flag
> - `src/renderer/modules/registry.tsx` — `ModuleDefinition`
>   (`id/title/icon/layout/component`)
> - `src/renderer/modules/notes/` — NotesModule + notes.service (multi-note,
>   split pane, 500ms-debounce autosave, pinned, archived)
> - `src/renderer/modules/backlog/` — BacklogModule + backlog.types
>   (`BacklogStatus = open | in-progress | needs-validation | completed`,
>   body, detail view, image attachments, auto-spec via chat)
> - `src/renderer/modules/notepad/` — NotepadModule (single-file filesystem
>   editor, fullscreen, menu bar, 5 recents, manual save)
>
> When in doubt about semantics or visuals, open the AW2 file and copy the
> behavior.

## 1. Objective

Make the sidebar behave like AW2's: module items that **close** (today they
open but never close — the only operation is removing the module from the
workspace), no category headings, an ordered flat list, a context menu, and a
typed registry that makes it impossible to compile a module that skips the
sidebar rules. On top of that, bring **Notes** and **Backlog** to 100% AW2
feature parity and port the **Notepad** module.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Close ≠ Remove.** The sidebar X **closes** the module: the item disappears from the sidebar but the module **stays added** — its `aw` actions and prompt block are untouched. This ports AW2's `closeModule` + `shouldHideModuleOnClose` (singleton → hide). "Remove from workspace" (`module.remove`, drops actions) stays in the right-click menu as the capability fence. The X must NEVER call `removeModule`. |
| 2 | **Every sidebar module item gets the hover X and the context menu — guaranteed by architecture:** items are rendered by ONE generic renderer in `AppShell.tsx`; modules never render their own sidebar item. (This is also how AW2 guarantees it — there is no per-module opt-out to forget.) |
| 3 | Open/hidden is **UI state → `config.json`** (e.g. `hiddenModules: []` next to `addedModules`), with the item order (`moduleOrder`). NOT the vault — no security impact (sandbox config stays in the vault per the permissions spec, Decision 5). |
| 4 | **Reopen from Apps (home):** clicking the card of an added-but-hidden module reopens it (no re-add). `module.add` on an already-added module unhides it. `app.navigate` to a hidden module unhides it — the agent must never get "view not found" for an added module. |
| 5 | **Drop the section headings** "Modules" and "Chats" (`nav-heading` in `AppShell.tsx`): one flat scroll area, modules then chats, like AW2. The "Archived" disclosure stays — it is a function (collapsed search), not a category label. |
| 6 | **Module context menu ports AW2's:** Close, Move Up, Move Down, separator, Remove from workspace (destructive). Move Up/Down persists `moduleOrder`. Deviation from AW2: "Rename" is omitted — aw modules have no per-instance titles (names come from the Go catalog). |
| 7 | **`singleton` is declared per module in the TS registry.** Every current module (notes, backlog, notepad, browser-chrome, browser-edge) is `singleton: true` → close hides. Multi-instance windows (one id, many windows) are OUT OF SCOPE until a module needs them; chats stay their own multi-by-nature list. |
| 8 | **Compile-time enforcement:** replace `MODULE_VIEWS: Record<string, ComponentType>` with a `ModuleId` union derived from one `MODULE_IDS` const and a registry typed `satisfies Record<ModuleId, ModuleDefinition>` where `ModuleDefinition = { id, component, singleton }` with **no optional fields**. A new module that is missing from the registry, has an unknown id, or omits `singleton` does not compile. A vitest drift test keeps `MODULE_IDS` in sync with the Go `ModuleCatalog()` (same pattern as the Go-side action drift test). |
| 9 | **Notes = 100% AW2:** split pane (note list left, editor right), create/delete, autosave with 500ms debounce, `pinned` and `archived` flags. Vault `notes` table gains the two columns (additive migration); `notes.list`/`notes.update` actions expose them. |
| 10 | **Backlog = 100% AW2:** `status ∈ open / in-progress / needs-validation / completed` (replaces `todo/done`; migrate `todo→open`, `done→completed`), `body`, a detail view with editor, **image attachments**, and auto-spec adapted to aw (the AW2 "auto-spec" streams a spec for the item through chat — wire it to aw's chat instead). Attachments live in a new vault table (BLOBs); `backlog.*` actions updated to the new statuses. |
| 11 | **Notepad = new module ported from AW2:** single-file filesystem editor, fullscreen layout, classic menu bar (New / Open / Save / Save As / Open Recent ×5), manual save. **Every file path goes through `sandbox.ResolveAndCheck`** — the permissions spec (Decision 7 / Risk 7) makes any module touching disk a checker call site, even when the path came from a native file dialog. Open/Save pickers are native (Wails runtime dialogs). |
| 12 | The **in-flight uncommitted change** in `AppShell.tsx` (X button wired to `removeModule`, present in the tree 2026-06-11) **must be reconciled to Decision 1** before landing: same button, close semantics. |

## 3. What already exists — reuse, don't reinvent

- The chat items' hover close button (`.nav-chat-close`, `aw-sidebar.css`)
  — reuse the class and hover pattern for module items.
- The context-menu plumbing in `AppShell.tsx` (`openModuleMenu`,
  `openChatMenu`, `measureFixedScale`) — extend the module menu, don't fork.
- `config.json` persistence with merge-on-save (`appconfig.Save` keeps
  existing sections) — add `hiddenModules` / `moduleOrder` the same way
  `addedModules` works.
- Vault migrations: the `CREATE TABLE IF NOT EXISTS` block
  (`vault.go:~1304`) plus additive `ALTER TABLE` guards — follow the existing
  pattern for new columns and the attachments table.
- The Go drift test pattern (`TestBrowserSpecActionsMatchRegistry`) for the
  TS↔Go catalog drift test.
- The sandbox checker (`internal/infrastructure/sandbox`) — Notepad calls
  it like `resolve()` does; no second path-validation code path.

## 4. Architecture by layer

- `internal/domain/module.go`: catalog unchanged except the Notepad entry.
- `internal/infrastructure/appconfig`: `HiddenModules []string`,
  `ModuleOrder []string` (merge-on-save like `AddedModules`).
- `internal/application/modules.go`: `HideModule` / `ShowModule` /
  `ReorderModule` use cases; `module.add` unhides; list result gains
  `hidden` so the frontend filters without a second source of truth.
- `frontend/src/modules/module-views.ts` → becomes the typed registry of
  Decision 8 (rename to `registry.ts` if cleaner; keep one file).
- `frontend/src/app/AppShell.tsx`: the single sidebar item renderer
  (Decision 2) — X, context menu, order, no headings.
- Notes/Backlog/Notepad UI under `frontend/src/modules/<id>/`, services in
  `frontend/src/services/`.

## 5. Phases

### Phase 1 — Sidebar mechanics (close/hide, order, flat list, typed registry)

Backend hidden/order state + use cases; frontend typed registry with
`singleton`; X = close (Decision 1, reconcile the in-flight diff); context
menu (Close / Move Up / Move Down / Remove); headings removed; reopen paths
(Apps card, `module.add`, `app.navigate`). Tests: vitest registry/AppShell
behavior; Go tests for hide/show/reorder; drift test TS↔Go.

### Phase 2 — Notes parity

Vault columns `pinned`, `archived`; `notes.*` actions updated; UI ported
from AW2 (split pane, debounce autosave, pin/archive affordances). Port the
AW2 look — open `NotesModule.tsx` side by side.

### Phase 3 — Backlog parity

Status migration (`todo→open`, `done→completed`) + `needs-validation` +
`in-progress`; `body` column; detail view with editor; image attachments
(vault BLOB table, paste/drag like AW2); auto-spec wired to aw chat
(`chat.send`); `backlog.*` actions updated.

### Phase 4 — Notepad

New catalog entry + typed-registry entry (`singleton: true`); fullscreen
module ported from AW2; native open/save dialogs; **every path through the
sandbox checker** with an explicit test (save into `~/.ssh` blocked even in
`permit_all`); recents in `config.json`. `Actions: []` in v1 — a UI-only
module is valid; the agent reaches files via `fs.*` anyway.

## 6. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate            # lint + go test + wails build
cd frontend && npm run build:frontend
```

`internal/architecture` tests stay green. Update `docs/SELFCODE.md` whenever
actions or module behavior change (phases 2-4).

## 7. Risks / attention

1. **Close vs remove regressions:** the X must never unregister actions.
   Test explicitly: close Notes → `notes.list` still dispatches; remove
   Notes → it does not. The uncommitted X-as-remove diff (Decision 12) is
   exactly this bug.
2. **Vault migrations are additive only** — never rewrite or drop columns on
   existing tables; map old backlog statuses once, idempotently.
3. **Attachments size:** copy AW2's limits; BLOBs live in the vault, so a
   runaway paste bloats the encrypted DB — cap and surface errors.
4. **Notepad is a disk surface** (permissions spec Risk 7 applies): paths
   from native dialogs still go through `ResolveAndCheck`; built-in denies
   hold even in `permit_all` + self-dev.
5. **Agent navigation to hidden modules** must unhide, not error — the agent
   cannot see the hidden state and should not need to.
6. **No `git add -A`** (shared working copy); small commits, one per phase.
   Filenames use a hyphen, never an em dash. Backend changes need an app
   restart to load — say so in summaries.
