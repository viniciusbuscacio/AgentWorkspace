# Spec: preserve partial assistant output on interrupted runs

**Status:** implemented 2026-07-20 — validated live (stop button preserved the partial with its marker)
**Date:** 2026-07-20
**Origin:** bug report — kimi k2 via OpenRouter thought for a long time, hit
`429 Too Many Requests: Provider returned error`, and everything it had
streamed vanished from the chat.

## Problem

Two layers discard streamed content when a run fails mid-stream:

1. **Frontend** — `ChatModule.tsx` `dropPendingBubbles()` (line ~222) removes
   ANY assistant bubble flagged `pending || streaming` on `chat:error`,
   including one holding thousands of streamed tokens.
2. **Backend** — `chat_run.go` `RunChatMessage` (line ~203) returns the error
   without persisting anything. The partial text is never stored, so it also
   never enters the history the next turn (possibly on another provider) sees.

Note: the adapter does not separate thinking from answer text — reasoning
arrives inside `delta.content` (`openai_compatible.go` delta struct has no
`reasoning_content` field). Preserving deltas therefore preserves thinking
automatically.

## Decided behavior (from the owner's answers, 2026-07-20)

| # | Decision |
|---|----------|
| 1 | Preserve the partial whenever it has **≥ 10 words**; below that, behave as today. |
| 2-3 | The preserved message keeps the partial text, then a blank line, then the raw error line (e.g. `429 Too Many Requests: Provider returned error`). The existing error banner stays as-is. |
| 4 | Error with **0 deltas** emitted: unchanged — silent failover to the next provider. |
| 5-6 | **User stop preserves too**, with marker `Interrupted by user (stop button)`. |
| 7-10 | No continuation UI, no auto-failover mid-stream, no special "continue" prompt. The persisted partial simply enters the normal history window; the interruption marker in the text is what tells the next model the reply was cut. |
| 11-12 | Thinking is preserved and enters context like any message (automatic, see note above). |
| 14 | No editing/deleting features — it is a normal, immutable message. |
| 15 | Compaction treats it as a normal message. |
| 16 | Crash/process-kill recovery is **out of scope** (would need incremental persistence during the stream). |
| 19 | Fix applies to **all adapters** — implemented at the `chat_run.go` layer (adapter-agnostic), not inside any single adapter. |

## Decisions Claude made on the "não sei" answers (veto if wrong)

- **Q17 — retry on 429:** no retry in v1. A mid-stream 429 after a long
  generation would re-bill the whole generation on retry, and OpenRouter 429s
  are usually upstream capacity, not the user sending too many messages.
  (Answering the owner's question: no — one long generation is a single request;
  sending fewer chat messages per minute would not have prevented this.)
  Logged as a possible future improvement.
- **Q18 — bench on mid-stream 429:** yes. Today benching is coupled to the
  failover decision (`failover := !midStream && IsFailoverError`). Decouple:
  penalize the provider whenever the error is failover-class, even mid-stream;
  only the *advance to next provider* keeps requiring `!midStream`.
- **Q20 — validation:** automated tests (below) plus one manual repro through
  the dev mock (`frontend/src/dev/wails-mock.ts` can emit deltas then
  `chat:error`) so the behavior can be seen with real eyes before merge.

## Changes

### 1. Backend — `internal/application/chat_run.go`

- Wrap the `OnDelta` callback to accumulate streamed text into a
  `strings.Builder` (per attempt; reset at each provider attempt start).
- In the `!succeeded` path, if the accumulated text has ≥ 10 words
  (`len(strings.Fields(s)) >= 10`), persist an assistant message:

  ```
  <partial text>

  <marker line>
  ```

  Marker line:
  - provider error: the raw error string, e.g.
    `429 Too Many Requests: Provider returned error`
  - user stop (detected via `context.Canceled` /
    `errors.Is(lastErr, context.Canceled)`): `Interrupted by user (stop button)`
- Emit `chat:refresh` (or reuse `OnDone`-adjacent plumbing) after persisting so
  mounted views reload. Implementation detail: a new optional callback
  `OnPartialSaved(chatID, messageID)` may be cleaner than overloading OnDone —
  implementer's choice, but the UI must repaint without a chat switch.
- The `midStream` no-failover rule is **unchanged** (still based on
  `attemptDeltas > 0`, regardless of the 10-word threshold).
- Benching change (Q18): `bench := domain.IsFailoverError(lastErr)` applies
  even when `midStream` is true; the `continue` to the next provider keeps the
  `!midStream` guard.

### 2. Backend — ADK session consistency (`runtime.go`)

The ADK session service is in-memory and parallel to the vault. After a failed
run, verify whether the ADK session already holds the partial content:

- If the ADK runner did NOT append the partial: append a session event with the
  persisted partial (or reseed via the existing `ResetSessionWithHistory`,
  runtime.go:194) so the next turn in the same process sees the same history
  the vault has.
- If it DID append something: make sure we do not duplicate content on the
  next turn. Investigate first, then pick append vs reseed; document the
  finding in the PR.

(After an app restart this is moot — sessions are reseeded from the vault,
which now contains the partial.)

### 3. Frontend — `ChatModule.tsx`

- `dropPendingBubbles()` on a non-cancel `chat:error`: only drop **empty**
  bubbles; a bubble with content stays until the reload replaces it with the
  persisted message.
- `chat:error` handler: also call `loadChat()` (today it never reloads, so the
  persisted partial would only appear after switching chats).
- Stop path ("context canceled" branch): same treatment — keep non-empty
  bubble, reload; the persisted "Interrupted by user" message replaces it.

### 4. Tests

- `chat_run_test.go`:
  - mid-stream 429 with ≥10 words streamed → assistant message persisted with
    text + blank line + error marker; provider benched; no failover.
  - mid-stream 429 with <10 words → nothing persisted (today's behavior).
  - 0 deltas + 429 → silent failover unchanged, nothing persisted.
  - user cancel mid-stream → persisted with `Interrupted by user (stop button)`.
- Frontend (`ChatModule.test.tsx` / hook tests): `chat:error` with a non-empty
  streaming bubble keeps the content and triggers reload; empty bubble is
  dropped as today.
- Manual: dev mock streams N deltas then emits `chat:error`; verify visually.

## Out of scope (explicitly)

- Crash/process-kill recovery (incremental persistence).
- Auto-retry on 429; auto-failover mid-stream with continuation.
- "Continue with next provider" button.
- Distinguishing thinking vs answer in the UI (depends on adapter-level
  `reasoning_content` support — separate future spec if wanted).

## Rollout

1. The owner reviews this spec.
2. Implement backend → frontend → tests (single branch, one logical commit).
3. `go run ./tools/buildgate` green.
4. The owner reopens the app (backend change → old binary until reopen) and
   runs the manual repro.
