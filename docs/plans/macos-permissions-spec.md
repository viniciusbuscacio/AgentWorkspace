# Spec — macOS permissions: the app and the agent know about TCC

> **Status:** implemented (2026-06-12, commits: ed6f408, 796971a)

## 1. Objective

macOS gates several aw features behind TCC permissions the app cannot
grant itself. Today a missing permission looks like a mysterious failure.
After this spec: the user has one screen that lists what to enable and a
button that opens the right System Settings pane, and the AGENT recognizes
permission-shaped failures and says exactly what to click.

## 2. The permission inventory (v1, macOS only)

| Permission | System Settings pane | What breaks without it |
|---|---|---|
| Microphone | Privacy & Security › Microphone | Voice capture (ffmpeg `:default`) |
| Automation (Apple Events) | Privacy & Security › Automation | Browser graceful quit/restart (osascript telling Chrome/Edge to quit) |
| Files and Folders / Full Disk Access | Privacy & Security › Files and Folders | fs/Explorer/Notepad on `~/Desktop`, `~/Documents`, `~/Downloads` (TCC dirs) even when the sandbox allows them |
| App Management | Privacy & Security › App Management | Updating/replacing app bundles (self-dev rebuilding Agent Workspace.app, anything touching other apps) |
| Notifications (osascript) | Notifications | Desktop notifications may not display |

## 3. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Settings › Security gains a "macOS permissions" card** listing the §2 table: name, one-line why, and an **"Open System Settings" button per row** using the `x-apple.systempreferences:` deep links (e.g. `...security?Privacy_Microphone`, `?Privacy_AppManagement`, `?Privacy_Automation`, `?Privacy_AllFiles`) via an infra port (`exec open` — interface layer stays I/O-free). Non-macOS builds hide the card. |
| 2 | **Status detection is best-effort, honest about unknowns:** show "unknown — test it" rather than guessing. Where a cheap probe exists, offer a Test button per row: microphone = the existing voice-capture probe; Automation = a harmless osascript query against System Events; Files = stat `~/Desktop` through the existing checker path. App Management has no cheap probe — the row says "enable if app updates fail". No new TCC prompts fire without the user clicking Test. |
| 3 | **The agent learns to diagnose:** SELFCODE gains a "macOS permissions" section with the §2 table PLUS the failure signatures ("Operation not permitted" on a TCC dir the sandbox allows; osascript error `-1743`; ffmpeg device open failure) → which permission → the exact System Settings path to tell the user. The agent CANNOT enable these (System Settings is outside the app — outside even yolo scope by nature); its job is a precise instruction, not an attempt. |
| 4 | **Error-site hints:** the few places that already surface these failures (voice capture start, browser graceful quit, sandbox-allowed fs ops returning permission errors on TCC dirs, notify) append one short hint to the error string: "This looks like a macOS permission — see Settings › Security › macOS permissions." Match on the specific error shapes, never blanket-append. |
| 5 | Copy lives in one constants file; the table in SELFCODE and the Settings card render from the same data (Go-side list exposed via the existing settings surface) so they cannot drift. |

## 4. What already exists — reuse, don't reinvent

- Voice capture probe (`agent.NewFfmpegVoiceCapture`), the osascript
  invocation patterns (browser gracefulQuit, notifier), the sandbox checker
  (to distinguish "sandbox denied" from "macOS denied").
- The Security page card layout; `SaveCancelActions` not needed (no state
  to save — this card is links + tests).
- The infra-port pattern for `open` (notepad/notify precedents).

## 5. Phases

### Phase 1 — Card + deep links + agent docs

The Security card from the Go-side permission list, deep-link buttons,
SELFCODE section (Decision 3). Go test: the list renders into SELFCODE
content (drift fence); deep links are well-formed.

### Phase 2 — Probes + error hints

The per-row Test buttons (Decision 2) and the error-site hints
(Decision 4). Go tests: hint appended only on the matching error shapes;
probes never fire without explicit invocation.

## 6. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 7. Risks / attention

1. TCC behavior varies across macOS versions — deep links and error codes
   are checked against the current machine (macOS 25.x/Darwin 25.5);
   degrade to opening the Privacy & Security root pane when a sub-pane
   link fails.
2. Probes must not spam permission prompts: each fires only on the user's
   explicit Test click (Decision 2), never on page load.
3. The hint text (Decision 4) must not mask the real error — append,
   never replace.
4. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart — say so in summaries.
