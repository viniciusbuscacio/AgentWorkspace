# Spec — Chat identity: the view never shows another chat's messages

> **Status:** implemented (2026-06-12, by the planner directly — critical path). From the project owner's bug
> batch: deleted chat stays rendered on the right; after a restart, closing
> a chat leaves it rendered; a NEW chat loaded the OLD chat's messages.
> Data layer already verified by the planner: `messages.session_id NOT NULL`
> + FK to sessions with `ON DELETE CASCADE` (`vault.go:1572-1580`) — nothing
> is cross-linked in the vault; every leak below is view-layer identity.
> Runs AFTER `chat-inflight-state-spec.md` (same files; serial queue).

## 1. Root causes (verified before writing)

1. `ChatModule`'s load effect (`ChatModule.tsx:46`) bails on `!chatId`
   WITHOUT clearing `baseMessages` — a deleted/deselected chat keeps its
   messages rendered.
2. The load effect has NO staleness guard: switching A→B fires two async
   loads; a slow A response landing after B's paints A's messages inside
   B. This is the "new chat loaded the old chat" bug.
3. `removeChat` deselects, but `refreshChats` auto-selects `next[0]` when
   `selectedChatId` is empty (`AppShell.tsx:70`) — fighting the explicit
   deselection; and the session-restore path can re-select a stale
   `lastChatId` after a restart.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Stale async loads are discarded**: every chat-scoped fetch carries the chatId it was issued for and its result is IGNORED if the displayed chat changed meanwhile (effect-cancellation guard). No fetch result ever paints into a different chat. |
| 2 | **`chatId` undefined/changed clears the pane immediately**: the moment the displayed chat is deselected or switched, `baseMessages` resets (empty state or the new chat's loading state) — the previous chat's content never outlives its selection, deleted or not. |
| 3 | **Delete means gone, everywhere, instantly**: deleting the displayed chat clears the right side in the same interaction (the existing deselect + home navigation), and `refreshChats`' auto-select runs ONLY on first load — never right after an explicit deselection. |
| 4 | **Session restore validates `lastChatId`**: a stale id (deleted chat) falls back to the newest active chat, or to no selection when none exist — the wallpaper-desktop-session spec said this; verify it was actually implemented and pin it with a test. |
| 5 | **The data-layer guarantee gets its regression fence**: a Go test asserting messages load strictly by `session_id` and that deleting a session cascades its messages (the schema already does this — the test keeps it true). |
| 6 | Regression tests (vitest): fast A→B switch with a slow A response → B never shows A's content; delete the displayed chat → pane clears; new chat → always empty; restart with a stale lastChatId → newest/empty, never a ghost. |

## 3. What already exists — reuse, don't reinvent

- The chatIdRef pattern already in `ChatModule` (AW2's moduleIdRef) — the
  staleness guard extends it, no new machinery.
- `ValidateRestoreView` shows the restore-validation shape for Decision 4.
- The vault schema needs NO change (verified) — Decision 5 is a test only.

## 4. Phases

### Phase 1 — The guards (Decisions 1, 2, 3)

### Phase 2 — Restore validation + fences (Decisions 4, 5, 6)

## 5. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. This spec and `chat-inflight-state-spec.md` touch the same files — the
   queue runs them serially; read the landed inflight changes FIRST and
   build the guards on top of them, not around them.
2. The auto-select change (Decision 3) must not break the first-launch
   flow (open app → newest chat selected) — that behavior stays.
3. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart — say so in summaries.
