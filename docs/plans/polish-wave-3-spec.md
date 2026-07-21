# Spec — Polish wave 3: chat layout bug, sidebar veto, Theme parity, nav bug, servers to Apps

> **Status:** implemented (2026-06-12, commits: 848210e, f1a8368).

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **BUG — chat layout broken (the screenshot):** the composer renders at the TOP of the content area with the message list collapsed and a large glass-veil area below. Required end state: message list fills the available height, composer pinned at the BOTTOM, exactly as before the glass change. Root-cause first — prime suspect is the `.aw-wallpaper-glass-veil` wrapper (commit 9ff63ee) altering the chat view's flex chain; the fix must keep the veil working. Add a regression test asserting the composer is the LAST element of the chat column and the list container carries the flex-grow. |
| 2 | **REMOVE the "Show desktop" sidebar button** — explicit user veto ("não pedi isso"). SUPERSEDES `wallpaper-desktop-session-spec.md` Decision 4. The desktop appears naturally when the user closes every module (the existing per-item X). KEEP `HideAllModules` and the `app.desktop.show` agent action (aw-coverage Decision 3) — only the human-facing button goes. Update/remove its tests. |
| 3 | **Settings › Theme gains the Apps-screen controls it was specced to have**: search bar, "Order by" select and the icon/card size control — same components, same look, same behavior as the Apps screen (reuse the components; if Apps' controls are not yet extracted as shared, extract them now and use in both). This was `theme-studio-spec.md` Decision 1, not delivered — close it for real, with a vitest asserting the three controls render and filter/sort/resize the grid. |
| 4 | **BUG — navigation from a module view to a chat does nothing** (reported from the Wallpaper view; assume any module view). Root-cause first — suspects: the session-restore effect or the hidden-module unhide effect re-routing/looping after `setView('chat')`, or a stale `selectedChatId` guard. Required end state: clicking any sidebar chat from ANY view lands on that chat. Regression test: render with view = a module id, click a ChatNavItem, assert chat view + selected id. |
| 5 | **MCP Server and REST API move OUT of Settings into Apps**: the two pages leave the Settings navigation; their full config UI (port, token, autostart, status) opens from the Apps service cards (card click → the server panel, same components relocated, rendered as the card's detail view in the aw-consistent pattern). The toggles on the cards stay. Update SELFCODE references ("settings under Settings → MCP Server" → the Apps location) and any deep link that pointed at the old pages. |
| 6 | Where decisions 2 and 5 touch implemented specs, leave those spec files untouched except a one-line `> Superseded in part by polish-wave-3-spec.md (date)` under their Status header. History stays honest. |

## 3. What already exists — reuse, don't reinvent

- The glass veil CSS (`globals.css` `.aw-wallpaper-glass-veil`) — fix the
  layout around it, don't fork it.
- The Apps screen controls (search/order/size) and `ServiceCardDef`
  plumbing in `HomeModule.tsx`; the two server page components
  (`McpServerPage.tsx`, `RestApiPage.tsx`) — relocate, don't rewrite.
- `ApiServerSettingsCards.tsx` shared settings logic.
- The AppShell view/effect plumbing for the navigation bug.

## 4. Phases

### Phase 1 — The two bugs (Decisions 1, 4)

Root-cause notes go in the commit messages. Regression tests for both.

### Phase 2 — The three changes (Decisions 2, 3, 5, 6)

Button removal, Theme controls parity, servers relocation, spec
supersession notes.

## 5. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. Decision 1's fix must be tested with the glass at 0 AND at 100 — the
   bug may hide at one extreme.
2. Decision 5 must not break `app.server.set` or the Home toggles — the
   backend surface is untouched, only the page location moves.
3. Decision 4's root cause may also explain other navigation oddities —
   if the fix is in a shared effect, run the full AppShell suite and
   click-test chat/module/settings navigation manually before committing.
4. **No `git add -A`**; small commits, one per phase. Backend changes (none
   expected) need an app restart — say so in summaries.
