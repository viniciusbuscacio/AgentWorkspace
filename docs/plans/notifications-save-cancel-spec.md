# Spec — Notifications + the Save/Cancel contract (port of the AW2 patterns)

> **Status:** implemented (2026-06-12, commits: e61660e da3e051)
>
> - `src/mcp/infrastructure/notify/desktop.ts` — native OS notification
>   (Electron `Notification`, title + body)
> - `src/desktop/handlers/chat-handlers.ts:46-49` — the notifications
>   enabled/disabled preference
> - `src/renderer/components/patterns/SaveCancelActions.tsx` — THE
>   Save/Cancel component: `onSave` AND `onCancel` are required props (this
>   is the "does not compile without Cancel" rule), default labels/icons,
>   saving state, plus the `saveCancelToolbarActions` adapter
>
> When in doubt, open the AW2 file and copy.

## 1. Objective

Two app-wide patterns: transient native notifications ("Config saved", the
macOS top-right popup) and one canonical Save button that cannot exist
without its Cancel.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Native OS notifications, not an in-app toast** (AW2 parity: the popup at the macOS top-right). Behind a `ports.Notifier` domain port; the v1 adapter is macOS via `osascript -e 'display notification ...'` (no new dependency). Windows/Linux: the adapter degrades to a silent no-op and the gap is a **documented follow-up** in the spec footer — do not block the feature on cross-platform. |
| 2 | **Preference**: Settings toggle "Desktop notifications" (default ON), persisted in `config.json` (cosmetic/UX state). The notify use case checks it; callers never do. |
| 3 | **v1 call sites (closed list):** every Settings save that today only flashes an inline message ("Config saved" + which page), the Permissions save, and **chat reply completed while the app window is not focused** (AW2's main use). Nothing else in v1 — notification fatigue kills the feature. |
| 4 | **Agent action `app.notify` `{title?, body}`** (port of AW2's notify tool): lets the agent ping the user at the end of long tasks. Body plain text, 200-rune cap, scrubbed with the existing secret scrubber before display. Registered in the always-on app group, documented in SELFCODE. |
| 5 | **`SaveCancelActions` ported as the app's ONE way to render Save**: `frontend/src/components/patterns/SaveCancelActions.tsx` with **required `onSave` and `onCancel`** (the type-level rule), saving/disabled states, default labels — visual style taken from the current Notes Save button (the approved design). |
| 6 | **Migration + fence:** every existing Save in the app (Notes, Backlog, Notepad Save/Save As stays a menu, Settings pages, Permissions) migrates to the component. The fence against regressions is an ESLint `no-restricted-syntax` rule flagging a `<Button>` whose text is "Save"/"Saving…" outside `SaveCancelActions` — plus the component's required props doing the real work, exactly like AW2. |
| 7 | Cancel semantics: revert the local draft to the last loaded state (each screen already has this state); never a navigation. A screen with nothing to revert does not render a Save at all. |

## 3. What already exists — reuse, don't reinvent

- `ScrubChatSecrets` for the `app.notify` body.
- The inline `message` pattern stays for errors; notifications are for
  successful, transient confirmation only (don't notify failures — those
  stay visible inline until acted on).
- The composition-root port wiring pattern (pip/notepad) for
  `ports.Notifier`; `osascript` invocation pattern exists in the browser
  manager's graceful-quit helper.
- `components/ui/Button` — `SaveCancelActions` composes it, never forks it.
- ESLint config in `frontend/` for the Decision 6 rule.

## 4. Phases

### Phase 1 — Notifier + call sites + action

Port + osascript adapter + preference + the Decision 3 call sites +
`app.notify` with cap/scrub. Go tests: preference off → no-op; cap and
scrub applied; unfocused-only rule for chat (the focus state already
reaches the backend via window events — if it does not, the chat call site
moves to the frontend service layer; decide by reading, not by adding new
plumbing).

### Phase 2 — SaveCancelActions + migration

The component (ported, styled like the Notes button), migration of every
Save site, the ESLint fence. Vitest: component renders both buttons,
saving state disables; the lint rule catches a raw Save `<Button>` (pin
with an eslint-rule test or a fixture).

## 5. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. `osascript` notifications come from "Script Editor" identity-wise on
   some macOS versions — acceptable for v1; if it looks broken, the
   fallback is the Wails events + an in-app toast as a LAST resort, but
   try the native path first (item 10 asked for the OS popup).
2. **Do not notify failures** (Decision 3's footnote) — a transient popup
   is the wrong place for something the user must act on.
3. The Save migration touches many screens — keep it mechanical, one
   commit per screen group, zero behavior change beyond the Cancel button
   appearing where it was missing.
4. `app.notify` is agent-triggerable: the cap + scrub are the fence;
   never include tool output verbatim.
5. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart — say so in summaries.

## Documented follow-up

Windows/Linux notification adapters (toast APIs / notify-send) — out of
scope for v1, the port makes them drop-in later.
