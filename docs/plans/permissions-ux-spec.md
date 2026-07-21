# Spec — Permissions page redesign (UI only)

> **Status:** implemented (2026-06-12, commits: 0740a9b)
> second validation pass (item 3: "a experiência está confusa — nem eu
> estou entendendo o que é o que"). **UI-only**: every sandbox semantic
> (modes, lists, vault persistence, native save dialog, set_mode refusal)
> stays EXACTLY as `permissions-spec.md` locked. AW2 reference (its UI was
> stronger than aw's current page):
>
> - `src/renderer/modules/permissions/PermissionsModule.tsx` — mode cards
>   grid with icons + "Active" badge, the "Folders visible to the agent"
>   preview sidebar with Refresh
> - `src/renderer/modules/permissions/PermissionsPathList.tsx` — built-in
>   chips (dimmed, "built-in" badge, no remove) vs custom chips (X),
>   input + Browse + Enter-to-add
> - `src/renderer/modules/permissions/permissions.constants.ts` — the
>   mode labels/descriptions to adapt
> - DEPENDS ON `notifications-save-cancel-spec.md` for `SaveCancelActions`.

## 1. Objective

A non-developer must answer three questions at a glance: **what can the
agent touch right now, how do I change it, and how do I check a specific
path/command** — without learning the words "permit list".

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Layout = three zones, top to bottom:** (1) "What the agent can access" — the live summary; (2) "Access mode" — four selectable cards; (3) the folder lists for the editable modes. Save/Cancel pinned via `SaveCancelActions`; the NATIVE confirmation dialog on save is untouchable (permissions-spec Decision 6). |
| 2 | **Zone 1 (summary) leads the page**: one plain sentence for the current mode (e.g. "The agent can only use the folders below, plus its own workspace.") + the live root chips (the existing `Roots`) + a **"Try a path or command" test box** — the user types `/Users/me/Desktop` or `rm -rf /tmp/x`, the page calls the EXISTING `sandbox.test` dry-run and shows allowed/blocked with the real reason string. This is the page's superpower and AW2 didn't have it; the backend is already there. |
| 3 | **Zone 2 = four cards in a grid** (AW2's pattern: icon + short label + one-line description + "Active" badge), radio-select, NOT a dropdown. Plain-language labels with the technical name as a small subtitle: "Only allowed folders" (permit_list), "Everything except blocked" (deny_list), "No file access" (block_all), "Full access — development" (permit_all, warning icon). Internal mode ids/names DO NOT change anywhere outside this presentation layer. |
| 4 | **Non-editable modes explain why**: selecting block_all/permit_all shows, in place of the lists, one sentence ("This mode has no custom folders — it blocks/allows everything by itself. The protected paths below still apply.") — AW2's identified weak spot, fixed with words, not widgets. |
| 5 | **Zone 3 lists port AW2's chips**: built-ins dimmed with a "protected" badge and a hover hint saying WHY ("keys and credentials live here — always blocked, even in Full access"); custom chips removable; input + native Browse picker + Enter-to-add. |
| 6 | Every string on this page lives in one constants file (the AW2 pattern) — the copy above is the spec's wording; refine freely there, never inline. |

## 3. What already exists — reuse, don't reinvent

- `dto.SandboxSettings` already carries mode, lists, built-ins AND `Roots` —
  the redesign needs NO new backend except a Wails passthrough for the
  test box if `sandbox.test` lacks one (check `app_permissions.go` first).
- The native save dialog flow and dirty-state handling in the current page —
  keep the logic, replace the presentation.
- `SaveCancelActions`, Radix primitives, the Apps-card visual language for
  the mode cards.

## 4. Phases

### Phase 1 — Full redesign

It is one page; ship it whole. Vitest: mode card selection marks dirty;
test box renders allowed and blocked results (mocked service); built-in
chips expose no remove affordance; save path still goes through the
native-dialog binding (mocked) — pin THAT above all.

## 5. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. **The native save dialog is the security boundary** — a redesign that
   "simplifies" it into a DOM modal reopens the self-approval hole. The
   vitest pin in Phase 1 exists for this.
2. The test box must call the same dry-run the agent action uses — two
   evaluators would drift and lie.
3. Plain-language labels must not leak into `sandbox.status`/prompt
   blocks/actions — presentation only (Decision 3).
4. **No `git add -A`**; small commits. Backend changes (if the passthrough
   is needed) need an app restart.
