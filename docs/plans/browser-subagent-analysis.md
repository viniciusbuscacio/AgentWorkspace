# Analysis: "Browser Via Subagent" vs. the existing external-safety pipeline

Status: analysis / recommendation. Not a spec to implement as-is.
Audience: the agent that will design/implement browser-read hardening.
Author context: written after reading the current code; claims below carry
`file:line` so you can verify, not trust.

## The question

A draft spec ("Browser Via Subagent") proposes that the **main agent never reads
browser content directly**. Instead it routes every browser content read
(`browser.tabs/snapshot/cdp/screenshot`) through an isolated, low-privilege
**browser subagent** that returns compact/clean content. Goal: reduce indirect
prompt-injection risk from pages, Gmail, DOM, snapshots, CDP output — without
hiding `browser.*` from the main agent.

## TL;DR

Don't build the original spec as a naive block, and don't sell the subagent as
content **purification** — it is not (if it returns the text, injected
instructions ride along). But the security pipeline below already covers browser
reads, so the subagent's real job is a different, legitimate one: a **context
firewall + workflow boundary**. It keeps raw Gmail/DOM/CDP out of the main
agent's context, returning a compact, contracted answer from a per-task
restricted toolset — which the inline distiller does **not** provide. That is a
product/UX/isolation goal, layered on top of (not replacing) the existing
safety pipeline.

Hybrid plan, in order:
1. **Close the dangerous browser guards** (`browser.cdp` / `navigate` /
   `new_tab`). Highest priority, pure policy, no new infrastructure.
2. **Strengthen the instruction** so the main agent *prefers* `subagent.run` for
   Gmail/web/tab reads.
3. **Build a real read-only browser subagent**: fixed allowlist, owns the
   browse loop, compact contracted return — explicitly **not** sold as
   sanitization.
4. **Decide the policy**: are plain reads *required* to go via subagent, or only
   in Equilibrado/Agressivo? Lean toward a **size/noise threshold** rather than
   all-or-nothing (a one-line tab title doesn't need a subagent; a full inbox or
   CDP dump does).

Three invariants to hold while doing the above:
- **Security still comes from the pipeline** (sanitize + taint + clamp), which
  already covers the browser. The subagent does not remove the taint — a compact
  answer can still carry injected text, so the consuming turn stays
  external/tainted. Firewall **stacks**, never substitutes.
- **Enforcement strength must match the goal.** Context/UX → a soft prompt
  preference (phase 2) is fine. Security guarantee → needs runtime enforcement.
  Don't conflate them, or "context firewall" gets re-sold as a security control.
- **`system.spawn`/`subagent.run` does not exist yet** — phases 3–4 require new
  runtime infrastructure (agent construction with a surface tag + per-task
  toolset), not just policy.

## What already exists (verified)

The codebase has a mature, layered external-content safety system. Browser
content is already wired into it.

### 1. Deterministic sanitizer — AI-free, not injectable
`internal/infrastructure/externalsafe/sanitize.go` (`SanitizeContent`):
- Unicode NFKC + strips invisible/bidi/zero-width chars (anti-smuggling) — `sanitize.go:92,139`.
- HTML → text: drops `<script>/<style>/<noscript>`, hidden nodes
  (`display:none`, `color==background`, tracking pixels) — `sanitize.go:233-313`.
- Multilingual injection regex scan (EN/PT/FR/ZH): "ignore previous
  instructions", "envie a senha", line-start role markers, exfiltrate, etc. —
  `sanitize.go:31-82`.
- Rewrites URLs to `[link: domain]`, removes base64 blocks, truncates.
- `classify`: injection hit → `suspicious=true, risk=high`; invisible text →
  `medium` — `sanitize.go:189-207`.
- Output is always marked `Untrusted=true` + `Notice` (the "treat as data, not
  instructions" string) — `sanitize.go:180`, `domain/external_safety.go:271`.

### 2. Optional AI distiller — second layer
`internal/infrastructure/tools/external_pipeline.go` (`processExternalContent` →
`runExternalDistiller`): an LLM rewrites content into `clean_text` + `summary` +
`possible_instructions_found`; if it finds instructions → forces
`suspicious + high`. Hard 30s timeout. **This is, in spirit, the spec's
"subagent that cleans content" — already here.**
- Caveat: it is **optional** — `shouldDistillExternal` returns false when
  `externalDistillFn == nil` — `external_pipeline.go:172-177`.
- Two modes — `domain/external_safety.go:78-81`:
  - `distill`: returns cleaned/summarized text (raw replaced).
  - `preserve_verbatim`: returns the full sanitized body, still marked untrusted.
  **`preserve_verbatim` IS the user's "return the whole cleaned content" ask.**

### 3. Tracked per-turn taint — not self-reported
`domain/external_safety.go:325-366` (`ExternalTaint`) +
`internal/infrastructure/externaltaint/store.go`. Every suspicious/high read
contaminates the turn scope (`WithExternalTaintScope`, set before the LLM turn).
The model under attack **cannot drop it** — `tools/external_safety.go:110-118`.

### 4. Action guard — confirmation on sensitive actions in a tainted turn
`tools/external_safety.go:120-173` (`requireExternalActionGuard`) +
`domain/external_safety.go:245-269` (`DecideExternalAction`): a sensitive action
(`send/execute/persist/upload/delete/...`) in a tainted/suspicious context →
**requires user confirmation**. Read is never gated. It also scans the action's
own args for injection and folds in the tracked taint (so it fires even if the
model omits the safety metadata).

### 5. Sandbox capability clamp — wired and live
`domain/sandbox.go:29-37` (`ClampSandboxModeForUntrusted`) is applied per-call in
`tools/tools.go:717-731` (`effectiveSandboxPolicy`): when the current turn's
taint is `Suspicious`, the sandbox mode is tightened to no looser than
`permit_list` for that call only (never widens, never persists).

### 6. Browser is already plumbed through all of the above
`internal/infrastructure/tools/aw_browser.go`:
- `browser.snapshot` → `processExternalContent` (distill) — `aw_browser.go:157-164`.
- `browser.cdp` → `processExternalContent` (distill) on its **output** — `:165-175`.
- `browser.tabs` → `processExternalContent` (preserve_verbatim) — `:251-256`.
- `browser.screenshot` → `externalVisualSafety` metadata — `:176-180`.

So browser reads are **already** sanitized, taint-tracked, and they already gate
later sensitive actions and clamp the sandbox.

## Assessment of the spec against this reality

- "Route reads through a subagent so injection is laundered" — **weak premise.**
  The subagent is another LLM reading the same untrusted content with the same
  prompt rules; it is equally injectable. Its summary becomes a *new* injection
  surface, minus the "this is untrusted web content" framing.
- "Subagent has fewer tools" — this protects the *subagent* from being
  weaponized; it does **not** protect the *main* agent, which receives the
  content and has the full toolset. Injection just moves up a floor.
- "Return the whole content, cleaned" (the user's chosen #3) — already exists as
  `preserve_verbatim`. But note: returning the full body means **injection text
  comes with it**; "clean" here = markup/script/invisible stripped, **not**
  injection removed (that cannot be done reliably on prose). So with "fewer
  tools" + "return whole content", the subagent is a *low-privilege fetch+extract
  proxy*, not a laundering boundary — the real neutralization still has to be the
  taint-clamp on the consuming turn, which already exists.
- The original spec's read/interact split (snapshot via subagent, click via main
  agent) is **technically unworkable**: element refs (`[e1]`) come from the
  snapshot, so the main agent would have nothing to click with. Resolution: pick
  one side — pure reads need no click (no loop), or the subagent owns the entire
  navigate→snapshot→click loop.

### Where a subagent still earns its place (context firewall ≠ sanitization)

The bullets above debunk the subagent as a *security* mechanism. They do **not**
debunk it as a **product/context** mechanism, which is a separate axis the first
draft of this report under-weighted:

- **Context firewall.** Even fully sanitized, a snapshot/CDP dump/long inbox is
  large, noisy, and lands in the main agent's context — and the main agent then
  *decides* on top of it. A subagent returns one compact, contracted answer
  instead. That is real value for cost, clarity, and "don't put raw external
  content in the planner's head" — a goal the user explicitly cares about.
- **The inline distiller is NOT a substitute for this.** `processExternalContent`
  cleans/summarizes one action's output, but it does not give you: a bounded
  **workflow**, per-task **toolset restriction**, subagent **ownership**, subagent
  **lifecycle logging**, or a collapsible **UI block**. Those are the subagent's
  distinguishing features.
- **Caveat that must travel with the feature:** this benefit is UX/cost/
  isolation, **not** injection removal. The compact answer can still carry
  injected text, so the consuming turn stays external/tainted. Build it as a
  firewall on top of the pipeline, never as a replacement for it.

## The real gaps (verified) — fix these first (Phase 1)

1. **`browser.cdp` and `browser.navigate`/`new_tab` are NOT action-guarded.**
   The guard runs only for `click`/`fill` — `aw_browser.go:140`. `cdp`
   (`Runtime.evaluate` = arbitrary JS in the user's *logged-in* session: read
   cookies/localStorage, navigate, trigger downloads) executes **without
   confirmation**, tainted or not. Only its output is sanitized. This is the
   highest-priority hole. (Note also: attached personal-profile downloads are not
   fenced by Permissions — see `internal/infrastructure/browser/manager.go`.)
2. **The clamp fires on `Suspicious` only** (`tools.go:719`), i.e. on
   regex/distiller detection. Paraphrased/novel injection that no pattern catches
   → no taint → no clamp. Detection is regex-first and the AI distiller is
   optional (gap #4).
3. **The clamp only tightens fs/shell** (sandbox mode). It does **not** remove or
   restrict `email.*`, browser mutations, `git`, or memory writes — those rely on
   per-action *confirmation*, which is vulnerable to confirmation fatigue.
4. **AI distillation is optional.** If `externalDistillFn` is nil, only the
   deterministic regex layer runs. For browser content specifically, consider
   making distillation mandatory.

## Recommended plan (hybrid, phased)

**Phase 1 — close the dangerous browser guards (do first; pure policy).**
Gate `browser.cdp` / `browser.navigate` / `browser.new_tab` the way `click`/
`fill` already are (`aw_browser.go:140`), and/or forbid `cdp` `Runtime.evaluate`
outside an explicit CDP-method allowlist. This is the one item that is a real
security hole today and needs no new infrastructure.

**Phase 2 — bias the main agent toward delegation (prompt-level).**
Strengthen the `browser.*` / instruction copy so the main agent *prefers*
`subagent.run` for Gmail/web/tab reads. Soft enforcement is acceptable here
because the goal is context/UX, not a security guarantee.

**Phase 3 — build a real read-only browser subagent (new infrastructure).**
`subagent.run`/`system.spawn` does not exist yet. Requirements:
- Fixed, host-controlled **allowlist** per `kind`+`mode`; the caller's requested
  `allowedActions` is intersected server-side, never trusted as authoritative.
- A **surface** tag (`SurfaceBrowserSubagent`) set by the host at agent
  construction, **never** read from model output.
- The subagent **owns the browse loop** (navigate→snapshot→click→fill→snapshot)
  and returns only the compact result — because element refs (`[e1]`) live in the
  snapshot, a read-here/act-there split does not work.
- Its output still flows through `processExternalContent`, and the **parent turn
  stays tainted** — the subagent is a context firewall, not a purifier.
- Lifecycle logging + collapsible UI block (the features the distiller lacks).
  Do **not** store raw DOM/email/CDP/screenshot in logs.

**Phase 4 — decide the read policy.**
Are plain reads *required* to go via subagent, or only in Equilibrado/Agressivo?
Recommendation: route by a **size/noise threshold**, not all-or-nothing — small
reads (tab title) stay inline; large/noisy reads (full inbox, CDP dump) go to the
subagent. Whatever the choice, do **not** couple "can read browser at all" to the
generic Subagents=Off switch; browser isolation should be its own setting.

## Open questions / to verify before building

- Should `browser.cdp` be gated as `ExternalActionExecute` (always confirm) or
  removed from the main-agent surface entirely? Today the kind is just
  `ExternalActionKind(command)` = `"cdp"`, which `DecideExternalAction` treats as
  "sensitive but not in the always-confirm set" → confirmed only when already
  tainted (`domain/external_safety.go:250-268`).
- Is regex+optional-distill detection strong enough, or should browser reads
  always force the distiller (cost/latency tradeoff)?
- For the subagent path: does it drive the user's *real* attached browser
  session? If so, even a read-only subagent acts on authenticated state — scope
  the browser allowlist accordingly.

## Project constraints (for whoever implements)

- Work on `main`, never branch, never leave the tree dirty (`docs/AGENTS.md`
  Golden Rule 6). Commit frequently.
- Gate before done: backend `golangci-lint run ./...` + `go test ./...` +
  `go run ./tools/buildgate`; frontend `cd frontend && npm run build:frontend`.
- Clean Architecture is enforced by tests in `internal/architecture`; keep I/O
  behind ports.
