# Spec — Settings consistency: Memory visual, breadcrumb affordance, auto-lock Never

> **Status:** implemented (2026-06-12, commits: d702c87)

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Settings › Memory adopts the Notes module visual**: same split-pane proportions, same list-item styling, same editor pane treatment, same Save styling (via `SaveCancelActions` once it lands). The page keeps its own data/logic — this is presentation only. Where the two screens genuinely differ (facts have no pin/archive), omit, don't invent. |
| 2 | **Breadcrumb hover affordance**: in `Settings › X › Y` headers, hovering a clickable segment shows `text-decoration: underline` + `cursor: pointer`, **no color change** (verbatim request). Applies to every breadcrumb-like header in the app (one CSS class, not per-page styles). Non-clickable segments (the current page) get neither. |
| 3 | **Auto-lock gains "Never", and "Never" is the default**: the option maps to the existing `0 = disabled` semantics (`app.autolock.set` already treats 0 as off — reuse, no new state). Default for fresh installs is Never; EXISTING configured values are preserved untouched. The Select shows "Never" first, then the minute options. |
| 4 | The security trade-off of Decision 3 is the user's explicit call; the page keeps one quiet hint line ("The vault stays unlocked until you lock it manually.") — informative, not nagging. |

## 3. What already exists — reuse, don't reinvent

- `NotesModule.tsx` layout/classes for the Memory page (copy the structure,
  share CSS where trivial; do not fork Notes itself).
- The `0 = off` auto-lock path through `settingsService.setAutoLockMinutes`.
- One shared breadcrumb header component if it exists; if each page
  hand-rolls its header, extract it first (this spec is the excuse).

## 4. Phases

### Phase 1 — all three items (frontend + the default flip)

Memory restyle; breadcrumb class + extraction if needed; "Never" option +
default. Vitest: SecurityPage shows Never selected on fresh state and maps
to 0; breadcrumb segment has the affordance class. Go: default auto-lock
minutes for a fresh config is 0 (adjust the appconfig default + test;
existing non-zero values load unchanged).

## 5. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. Flipping the auto-lock default must not relock/break existing installs —
   only the absent-value default changes; a stored 15 stays 15.
2. **No `git add -A`**; small commits. Backend changes need an app restart.
