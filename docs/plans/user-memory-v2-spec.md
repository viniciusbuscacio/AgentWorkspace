# Spec — User memory v2: one living document the agent maintains

> **Status:** implemented (2026-06-12, commits: dfb2053, 3a2d0e6)

## 1. Objective

Memory stops being a curated list of rows and becomes **one freeform text
the agent writes and the user can edit** — with growth control so it never
balloons.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **One document replaces the facts list.** Markdown text stored in the vault (`user_memory` storage reused — one document row; the old fact rows are migrated ONCE by concatenating them as bullet lines, then retired). |
| 2 | **Settings › Memory becomes a single textarea** showing the document, editable, saved via `SaveCancelActions`, with a character count against the cap and a "last condensed" timestamp. Manual edits are first-class — the user owns this text. |
| 3 | **The agent writes automatically through the EXISTING always-on tool** (`remember_user_fact` keeps its name — no churn in prompts/history); it now appends a line to the document. The base-prompt Memory section gains two instructions: (a) the explicit trigger — "lembre-se que/de…" or "remember that…" from the user ALWAYS results in a tool call; (b) passive learning — language, tone, vocabulary, recurring preferences — saved sparingly, never per-message. |
| 4 | **Growth control:** hard cap of 8000 runes on what the prompt receives. When the document exceeds 6000 runes, a one-shot LLM condensation rewrites it more concisely (checked at most once per day, on unlock — the same one-shot pattern as transcript cleanup). The previous version is kept as a one-deep backup (undo in Settings); the condensation is logged through the Logs writer. |
| 5 | **Condensation rules:** the rewrite prompt preserves explicit user preferences verbatim wherever possible, keeps the user's language, and is instructed to compress narrative, not delete facts. `ScrubChatSecrets` runs on the document BEFORE it is sent to the LLM and on every agent append BEFORE persist — secrets never live in memory text. |
| 6 | The document is injected into the system context **inside the single `SetMemoryContext` composition** alongside the chat catalog and notes block — NEVER a second context call (the removed-`RefreshChatMemoryContext` rule applies). Framed as background data about the user, not instructions. |

## 3. What already exists — reuse, don't reinvent

- `user_memory` vault storage, the always-on tool, the prompt block in
  `application.RefreshAgentContext` — evolve in place.
- One-shot LLM helper pattern (transcript cleanup) for condensation.
- `ScrubChatSecrets`; the Logs writer; `SaveCancelActions`.

## 4. Phases

### Phase 1 — Document storage + migration + tool append

Document accessor + one-time migration of fact rows; tool appends; prompt
block reads the document (cap applied). Go tests: migration idempotent;
append + cap; scrub on append; composition keeps catalog/notes intact.

### Phase 2 — Settings page + condensation

Textarea page with save/cancel/count/undo; unlock-time condensation
(threshold + once-a-day) with backup + log. Go tests: condensation only
above threshold, once per day, backup written, scrub before LLM call.

## 5. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. **The agent writes its own prompt context** — bounded by the cap, the
   scrub, the data framing and sparing-write instructions. Keep all four.
2. Condensation is lossy by design — the one-deep backup + the user's
   textarea are the recourse; never condense twice without an unlock
   between.
3. A failed LLM condensation must leave the document untouched (no partial
   writes).
4. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart — say so in summaries.
