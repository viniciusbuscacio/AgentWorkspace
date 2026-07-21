# Spec — File Explorer module (port of the AW2 model)

> **Status:** implemented (2026-06-12, commit 927ffaf + de672ab — salvaged). Workspace-module
> spec in the established series — same conventions, same gates. This is a
> **port of AW2's sandbox-aware File Explorer**. Reference implementation in
> `AgentWorkspace2`:
>
> - `src/renderer/modules/file-explorer/` — `FileExplorerModule.tsx` (root),
>   `FileExplorerChrome.tsx` (toolbar/breadcrumb/search),
>   `FileExplorerSidebar.tsx` (tree + favorites), `FileExplorerMainPane.tsx`,
>   `FileDetailsList.tsx`, `FilePreviewPane.tsx`, `FileExplorerTree.tsx`,
>   plus `file-explorer.service.ts` / `.tree.ts` / `.actions.ts` /
>   `.tree-actions.ts` / `.favorites.ts` / `.paths.ts`
> - `src/desktop/handlers/fs-handlers.ts` — every IPC op validates through
>   the sandbox before touching disk
> - `src/shared/infrastructure/aw-actions/file-explorer-actions.ts` —
>   `file_explorer.open` / `reveal`
> - `src/shared/domain/module-catalog.ts:32` — catalog entry (icon `folder`,
>   singleton, "Browse and manage files visible to the agent sandbox.")
>
> When in doubt about semantics or visuals, open the AW2 file and copy.

## 1. Objective

The module that makes the Permissions fence *visible*: the user browses and
manages exactly the files the agent can reach. Roots, blocks and errors all
come from the same sandbox the agent lives in — one fence, now with a UI.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **One checker, no second validation path.** Every operation goes through `sandbox.ResolveAndCheck` with the current policy, in an infrastructure adapter behind a domain port — copy the Notepad pattern (`internal/domain/ports/notepad.go` + `internal/infrastructure/notepad/files.go`) exactly. The explorer never re-implements path rules. |
| 2 | **Roots = the sandbox's visible top folders**, computed by the SAME code that fills `dto.SandboxSettings.Roots` for the Permissions page — single source of truth. Mode changes apply on the next call (no caching across Settings saves, same rule as the sandbox). Denied roots are never listed; navigating into a denied path shows the AW2-style error bar with the sandbox reason. |
| 3 | **v1 feature set (ported from AW2):** dual pane (sidebar tree with lazy loading + main grid Name/Modified/Type/Size), clickable breadcrumb, Up/Refresh, live search with count, sort by name/size/mtime (folders first), create file, create folder, delete, copy/paste, favorites (localStorage, Home locked like AW2), per-extension icons, `formatSize`/date formatting, empty states and selection styling copied from AW2. |
| 4 | **Text preview ≤ 1MB with inline edit + Save/Cancel** (AW2's extension list ported verbatim); binary files show metadata cards. An "Open" button opens the file with the OS default app via the infra port (path checked first). |
| 5 | **Cut from AW2 in v1 (documented deviations):** the two hardcoded tabs (favorites + last-path cover the need; revisit on demand), Google Drive virtual FS (no Drive in aw), move/rename (AW2's UI had neither — copy/paste + delete match the reference). |
| 6 | **Delete confirms via a NATIVE Wails dialog, not `window.confirm`.** Reason: `fs.*` gives the agent read/write/list but NOT delete — a DOM confirm would let `ui.click` mint a brand-new destructive capability through this module. Same precedent as the Permissions save dialog. Create/save/copy keep plain UI flows: they grant nothing `fs.write` does not already grant inside the same fence. Deletion is best-effort `os.Remove`-level (no system trash in v1 — say so in the confirm text); non-empty directories are refused with a clear error. |
| 7 | **One agent action: `explorer.open` `{path}`** — resolves the path through the checker, unhides/navigates the module to the directory (or the file's parent, selecting it). Port of AW2's `file_explorer.open`/`reveal` collapsed into one. Registered in the ModuleSpec (drift test covers it). |
| 8 | Module id `file-explorer`, name "File Explorer", icon `folder`, AW2's description, singleton, standard sidebar contract via `defineModuleView`. |
| 9 | **`readDir` caps at 2000 entries** (sorted first, then truncated) and reports the truncation in the result — a `node_modules` must not freeze the UI. Log the cap hit through the Logs module's writer if it has landed. |

## 3. What already exists — reuse, don't reinvent

- The sandbox checker and policy plumbing (`internal/infrastructure/sandbox`,
  `internal/application/sandbox.go`) — Decision 1's only dependency.
- The Notepad fs port pattern (`NotepadFileAccess`) — same shape, wider
  surface: `Roots, ReadDir, Stat, ReadFile(maxBytes), WriteFile, Mkdir,
  Delete, Copy, OpenWithDefaultApp`.
- The Roots computation behind the Permissions page (Decision 2).
- Native dialog precedent: `app_permissions.go` (`wailsruntime.MessageDialog`).
- Module plumbing: catalog entry, `defineModuleView` (singleton,
  compile-time contract), drift tests, hand-edited wailsjs stubs.
- `components/ui/` primitives and the split-pane/list styling conventions
  the other modules already use (keep the dropdowns consistent with
  whatever the polish wave standardizes).

## 4. Architecture by layer

- `internal/domain/ports/explorer.go`: the fs access port (Decision 3
  surface) — no framework imports.
- `internal/infrastructure/explorer/`: the adapter; every method resolves +
  checks before touching disk (mirror `notepad/files.go`, including the
  fail-closed default policy when none is wired).
- `internal/application/explorer.go`: use cases (validation, the 2000-entry
  cap, text-extension gate for preview).
- `app_explorer.go`: Wails surface incl. the native delete confirmation;
  `internal/dto`: `ExplorerListResult` etc. following the
  `{Success, Error, data}` shape.
- `frontend/src/modules/file-explorer/`: the AW2 component split (Module,
  Chrome, Sidebar, MainPane, DetailsList, PreviewPane, Tree) + utils files;
  service in `frontend/src/services/explorer.service.ts`.
- `internal/infrastructure/tools/aw_explorer.go`: `explorer.open`.

## 5. Phases

### Phase 1 — Backend (port + adapter + use cases)

Port, adapter with per-op `ResolveAndCheck`, use cases, Wails surface,
native delete dialog. Go tests: every op against a denied path is blocked
even in `permit_all` (the `~/.ssh` test, one per op — follow
`notepad/files_test.go`); roots match the Permissions Roots; readDir cap;
non-empty dir delete refused.

### Phase 2 — Frontend module

Component split ported from AW2 (tree lazy-load, breadcrumb, search, sort,
favorites, preview/edit, icons/formatting/empty states); catalog + registry
entries. Vitest following the neighbors' mock-service pattern, including:
denied navigation shows the error bar; delete button calls the native-
confirm Wails binding (mocked), never `window.confirm`.

### Phase 3 — Agent action + docs

`explorer.open` (checker-validated, unhide + navigate + reveal), SELFCODE
update (module list + the "delete confirms natively" note), drift tests
green.

## 6. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 7. Risks / attention

1. **A second path-validation path is the failure mode** — if any explorer
   code calls `os.*` without going through the adapter's checked methods,
   the fence has a hole. Keep ALL disk I/O in the one adapter; the
   architecture tests (no raw I/O in the interface layer) help but do not
   cover infrastructure — review for it explicitly.
2. **Delete via DOM would be a capability escalation for `ui.click`**
   (Decision 6 is the fence). Never "simplify" the native dialog away.
3. **Copy can recurse into something huge** — v1 copies files and
   directories like AW2, but refuse when the source tree exceeds a sane
   bound (e.g. 512MB or 10k files) with a clear error instead of hanging.
4. Preview edit + agent writes can race on the same file — last write wins
   is acceptable in v1; reload after save (AW2 behavior) so the user sees
   the truth.
5. Symlinks: `ResolveAndCheck` already realpaths — make sure directory
   listings show the entry but operations resolve through the checker (a
   symlink into `~/.ssh` must die at the op, port the AW2 test cases).
6. **No `git add -A`** (shared working copy); small commits, one per phase.
   Backend changes need an app restart — say so in summaries.
