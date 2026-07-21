# Spec — Workspace presentation: wallpaper layers, desktop & session restore

> **Status:** implemented (2026-06-12, commits: 9ff63ee, 9f05aa0)
> Superseded in part (2026-06-12): slider 100 = the crisp photo, no veil floor
> (user request); the Show-desktop button was removed by polish-wave-3.
> Superseded in part by polish-wave-3-spec.md (2026-06-12): Decision 4 (Show-desktop sidebar button) removed per user veto.

## 1. Objective

Make the wallpaper behave like a desktop: fully visible when nothing is
open, a controllable frosted-glass presence behind apps, a one-click "show
desktop", and an app that reopens exactly as it was closed.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Two wallpaper layers:** (a) the desktop — Home/Apps view — shows the wallpaper fully (dark overlay from the module spec); (b) behind module views and chat the wallpaper shows through a **frosted-glass veil** (blur + the app background color at partial opacity), so module content stays perfectly readable. |
| 2 | **The glass intensity is user-controlled**: a slider in the Wallpaper module — "Background visibility behind apps", 0–100. **0 = solid app background (no wallpaper behind modules at all)**; default **20** (subtle). The slider value persists in `config.json` next to the wallpaper id. This single control resolves "should it show behind modules" both ways — dial to taste. |
| 3 | Implementation: one CSS variable pair (e.g. `--wallpaper-glass-opacity`, fixed blur) applied by the same surface class everywhere — NOT per-module styling. A module never opts out or restyles the veil (same centralization rule as the sidebar items). Text contrast at slider 100 is still protected by the veil's minimum opacity floor (whatever keeps WCAG-ish readability on the brightest bundled wallpaper — pick once, document the floor in the CSS). |
| 4 | **"Show desktop"**: a button in the sidebar footer (icon `wallpaper`, tooltip "Show desktop") that **closes (hides) every open module view at once** — the existing hide mechanism, modules stay added — and navigates Home. Item 15 verbatim. |
| 5 | **Session restore**: the app reopens in the state it was closed — the open/hidden modules and order already persist (`hiddenModules`/`moduleOrder`); ADD: last view and last selected chat id, persisted in `config.json` (merge-on-save), restored after unlock. A no-longer-valid view (module removed) falls back to Home; a deleted chat falls back to the newest. |
| 6 | Window geometry restore is OUT OF SCOPE here (Wails window state is a separate concern; note it as a possible follow-up, do not implement). |

## 3. What already exists — reuse, don't reinvent

- The Wallpaper module's CSS var + surface class — extend, don't fork.
- The hide mechanism and its persistence (sidebar phase 1) — "Show desktop"
  is a loop over `HideModule`, plus one new use case `HideAllModules` so it
  is one config write, not N.
- `config.json` merge-on-save for the new fields (`wallpaperGlass`,
  `lastView`, `lastChatId`).
- The `ui:navigate` / view plumbing in AppShell for the restore path.

## 4. Phases

### Phase 1 — Wallpaper layers + glass slider (items 1, 3)

Surface class behind modules/chat, the slider in the Wallpaper module,
config persistence, the opacity floor. Vitest: slider writes config; 0
yields the solid class path.

### Phase 2 — Show desktop + session restore (items 15, 2)

`HideAllModules` use case + sidebar footer button; `lastView`/`lastChatId`
persistence (written on change, debounced) and restore-on-unlock with the
fallbacks. Go tests: hide-all is one write and skips core; restore
validates view against navigable views. Vitest: button hides all and lands
on Home.

## 5. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. **Readability beats beauty**: the veil floor (Decision 3) is not
   negotiable — test against the brightest bundled wallpaper, not the dark
   ones.
2. Restore must not race the unlock flow: read config after the vault
   unlocks, before the first render decision — a flash of Home then jump
   is acceptable, a crash on a stale chat id is not.
3. The glass effect uses `backdrop-filter` — verify WKWebView (Wails on
   macOS) renders it acceptably; if it janks, fall back to a pre-blurred
   image variant rather than shipping a slow app.
4. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart — say so in summaries.
