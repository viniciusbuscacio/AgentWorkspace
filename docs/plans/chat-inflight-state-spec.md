# Spec — Chat in-flight state survives navigation; sends always queue

> **Status:** implemented (2026-06-12, by the planner directly — critical path). From the project owner's bug
> report: send → agent thinking → leave the chat → return → the thinking
> indicator is gone, the user message shows duplicated, and a new send
> errors "chat is already sending". Expected (his words, now the rule): "ele
> deveria sempre aceitar uma mensagem adicional, e ficar na fila de espera".
> Conventions and gates as the series.

## 1. Root cause (verified before writing)

The in-flight truth lives in TWO places that disagree after a remount:
the backend's `chatRuns` map (`app.go:~1190`, the real truth) and
`sendingByChatRef` — a React ref INSIDE `ChatModule` (`ChatModule.tsx:128`)
that dies on unmount (AppShell renders the chat conditionally). A fresh
mount knows nothing: no indicator, and `performSend` claims the chat,
calls the backend and surfaces its rejection instead of enqueueing. The
queue state itself (`queueRef`) is also mount-local and dies the same way.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **The backend is the single source of truth for in-flight state.** On ChatModule mount, rehydrate from the existing session info (`ChatSessionInfo.Streaming` + run identity): if a run is in flight for this chat, the thinking indicator shows and sends route to the queue — exactly as if the user had never left. |
| 2 | **"chat is already sending" NEVER reaches the user.** Two fences: (a) the rehydrated state routes to the queue before any backend call; (b) if the backend still rejects with in-flight (race), the frontend silently enqueues — the error string becomes an internal signal, not UI. |
| 3 | **The queue survives navigation**: queued messages and the flush logic move out of mount-local refs into state that outlives the view (module-scope store or AppShell — implementer picks the aw-consistent spot). The flush listens to the run-completion event globally, so a queued message dispatches even while the user is on another view. |
| 4 | **Streamed content is not lost on remount**: minimum bar — the indicator shows during the run and the completed reply lands on `chat:done` (reload path). If the partial streamed text can be replayed cheaply (the events already carry chatId+runId), render it; do not build new replay infrastructure for v1. |
| 5 | **No duplicated user message.** Root-cause where the duplication comes from (optimistic append + reload, or a persisted message from a rejected send) and fix at the source; messages dedupe by id as the safety net. |
| 6 | Regression tests, all five: mount-rehydrate shows the indicator; send-while-inflight enqueues (no error shown); queue flushes after done even with the chat view closed; returning mid-run then receiving done shows exactly one user message and one reply; the literal string "chat is already sending" never renders. |

## 3. What already exists — reuse, don't reinvent

- The queue mechanics (`QueuePanel`, `enqueueMessage`, AW2's queueId
  semantics) — relocate, don't rewrite.
- `ChatSessionInfo.Streaming`, the runId on every chat event (the
  bidirectional chatId+runId work, commit 60981fa) — this is the
  rehydration substrate; no new backend state should be needed.
- `useStreamingMessages` (`chat-streaming.hooks.ts`) for the event side.

## 4. Phases

### Phase 1 — Truth + queue lift (Decisions 1, 2, 3)

Rehydrate-on-mount, the two enqueue fences, the queue store that outlives
the view with a global flush listener.

### Phase 2 — Remount fidelity (Decisions 4, 5, 6)

Indicator/partial handling, the duplicate fix at its source, the full
regression suite.

## 5. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. The send path is the most user-critical code in the app — small
   commits, and run the existing chat test suites between each.
2. The race in Decision 2(b) is real (two windows, agent-driven
   `chat.send`): the silent-enqueue fallback must also cover sends
   arriving via the `aw` action path.
3. Queue flush ordering: one message at a time, FIFO, never two backend
   runs for the same chat (the backend guard stays as the last fence).
4. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart — say so in summaries.
