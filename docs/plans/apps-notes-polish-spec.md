# Spec — Polish wave 2: Apps screen + Providers + Notes

> **Status:** implemented (2026-06-12, commits: 70830fe, 2ecc10a)

## 1. Objective

Small user-visible fixes on three screens (Apps, Settings › LLM Providers,
Notes) plus one real feature: notes that feed the agent prompt.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | Apps screen heading: "Open module" → **"Open Apps and Modules"**. |
| 2 | **Remove the blue check badge** on added cards (Notes, Backlog, …). The sidebar already shows what is added; the card stays clean. No replacement indicator in v1. |
| 3 | Settings › LLM Providers: the **`#1`, `#2` order badges use the same color token as the word "Active"**. |
| 4 | Settings › LLM Providers: **"Configured" uses the Active color, dimmed** (same hue, reduced opacity ~70% — pick the closest existing token treatment, do not invent a new color). |
| 5 | Notes: with "Show Archived" on, **archived notes render dimmed** (muted/faded text + icon, e.g. `opacity-60` or the `text-muted-foreground` treatment — match how AW2 fades archived). |
| 6 | Notes: a **Search box next to/below "Show Archived"**, filtering the list live by title AND content, case-insensitive, with the same look as the chat/archived search in the sidebar. |
| 7 | **"Insert into Agent prompt" checkbox per note**, default CHECKED, placed beside Save. Backed by a new `in_prompt` column on the vault `notes` table (additive migration, default 1). Notes with the flag on are composed into the agent's system context as a "User notes" block. |
| 8 | The notes prompt block **composes inside the single `SetMemoryContext` call** alongside user-memory and the chat catalog — NEVER a second context call (`RefreshChatMemoryContext` was removed for exactly this; do not reintroduce the clobber). |
| 9 | **Caps on the notes block:** per-note 4000 runes, whole-block 12000 runes; truncation appends `[truncated]` and longer notes lose content, never crash. Order: pinned first, then most recently updated. Archived notes are NEVER injected regardless of the flag. |
| 10 | `notes.list`/`notes.update` actions expose `inPrompt`; SELFCODE documents that flagged notes reach the prompt (the agent should know its own context sources). |

## 3. What already exists — reuse, don't reinvent

- The additive-migration pattern for `notes` columns (pinned/archived just
  landed — copy it for `in_prompt`).
- The prompt composition in `application.RefreshAgentContext` — extend the
  existing composition, mind Decision 8.
- The sidebar search input styling (`chat-search` in `aw-sidebar.css`).
- Theme tokens for the Providers colors — reuse the Active token.

## 4. Phases

### Phase 1 — Frontend cosmetics (items 1-6)

Apps heading, check removal, Providers colors, Notes dimming + search.
Vitest where the neighbors have component tests (Notes search filters,
archived dimmed class present).

### Phase 2 — Notes into the prompt (items 7-10)

Migration + flag in actions/Wails surface + checkbox UI + the composed
block with caps. Go tests: flag round-trip, block respects caps and skips
archived, composition keeps user-memory AND chat catalog intact
(regression for Decision 8).

## 5. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. **Decision 8 is the one that bites**: the composed context block has a
   history (a second caller used to clobber it). The regression test in
   Phase 2 is mandatory.
2. The agent can write notes (`notes.create/update`) and flagged notes
   reach its own prompt — self-amplification is bounded by the caps
   (Decision 9); keep the block framed as the user's notes data, and keep
   the caps conservative.
3. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart — say so in summaries.
