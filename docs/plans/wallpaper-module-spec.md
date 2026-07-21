# Spec — Wallpaper module (port of the AW2 model)

> **Status:** implemented (2026-06-12, commit 6c78e5c + de672ab — salvaged). Workspace-module
> spec in the established series — same conventions, same gates. This is a
> **port of AW2's Wallpaper module**. Reference implementation in
> `AgentWorkspace2`:
>
> - `src/renderer/modules/wallpaper/WallpaperModule.tsx` — the gallery grid
> - `src/renderer/domain/wallpaper-data.ts` — the 24-wallpaper catalog +
>   `getWallpaperBackgroundImage` (default = CSS gradient, no file)
> - `src/renderer/app/PageLayout.tsx` + `theme/globals.css`
>   (`.aw-wallpaper-surface`) — where/how the background applies (CSS var +
>   dark overlay gradient)
> - `src/shared/domain/module-catalog.ts:29` — catalog entry (icon
>   `wallpaper`, singleton)
> - Image assets: AW2's `wallpapers/` public directory — copy the files.
>
> When in doubt about semantics or visuals, open the AW2 file and copy.

## 1. Objective

Personality for the workspace: a gallery of bundled wallpapers applied to
the Home/Apps view background. Cheap, visible, pure user-delight.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Gallery = AW2's bundled set** (ids/names ported from `wallpaper-data.ts`; image files copied into `frontend/public/wallpapers/`). `default` stays the CSS gradient with no file. No upload/custom images in v1 (same as AW2). |
| 2 | **Applied to the Home/Apps view** (aw's equivalent of AW2's empty workspace) and to the Wallpaper module view itself — NOT behind other modules or the chat. Port the dark overlay gradient (`globals.css` `.aw-wallpaper-surface`) so text stays readable on any image. |
| 3 | **Persistence in `config.json`** (`wallpaper` field, merge-on-save like the module fields). Deviation from AW2 (vault `_config_wallpaper`): a wallpaper id is cosmetic UI state, and aw's convention keeps that in appconfig — the vault is for secrets. |
| 4 | **Agent action `app.wallpaper.set` `{id}`** in the always-on app group, validated against the gallery ids (unknown id → error listing valid ids, same shape as `app.theme.set`); current id exposed in `app.state`. The user can say "muda meu wallpaper" and it just works. |
| 5 | Module id `wallpaper`, icon `wallpaper`, singleton, standard sidebar contract via `defineModuleView` — the module view is the gallery picker, selection applies immediately (no save button), AW2-style hover/active states. |

## 3. What already exists — reuse, don't reinvent

- `app.theme.set` (`internal/infrastructure/tools/aw_app.go:70`) — copy its
  validation/persistence/event shape for `app.wallpaper.set`.
- `appconfig` merge-on-save pattern for the new `wallpaper` field.
- Module plumbing: catalog entry, `defineModuleView`, drift tests.
- Vite serves `frontend/public/` as-is — no asset pipeline work.

## 4. Phases

### Phase 1 — Everything (this is a one-phase module)

Config field + Wails get/set; gallery data file (ids/names ported) + image
files copied from AW2; Home/Apps background via CSS var + overlay;
`WallpaperModule` gallery UI; catalog + registry entries;
`app.wallpaper.set` + `app.state` field + SELFCODE update. Tests: Go for the
action validation and config round-trip; vitest for the gallery selection.

## 5. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. **Asset weight**: 23 JPGs go into the repo and the built app — keep the
   AW2 files as-is (already web-sized); do not source new heavier images.
2. Missing image file for a valid id must fall back to the default gradient
   (AW2 behavior), never a broken background.
3. Keep the Go-side gallery id list and the frontend data file in sync — a
   drift test (the theme list already lives duplicated Go↔frontend; follow
   whatever convention exists there).
4. **No `git add -A`**; small commits. Backend changes need an app restart
   — say so in summaries.
