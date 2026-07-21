# Spec — aw self-test battery (run + report)

> **Status:** not started. Recurring validation spec, not a feature build. It
> runs the existing battery script and commits a dated report. Re-queue it
> whenever you want a fresh pass. The runner lives at
> `scripts/automation/selftest/run_battery.py` (see its README); this spec only
> orchestrates running it and recording the result.

## 1. Objective

Prove, on demand and repeatably, that (a) the **internal aw agent can manage
itself** and (b) **every module works** — by asking the agent (in plain
language, over chat) to perform one key operation per module and verifying each
effect **externally** with a unique nonce. The deliverable is a committed report
under `docs/plans/reports/`.

This is a *validation* spec: it does not change product code. If it uncovers a
broken module, that becomes a **separate** fix spec (don't fix inline here).

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Drive the internal agent over REST chat**, never call the module APIs directly to "do" the work — the point is to test self-management. Verification, by contrast, is always **external** (REST read / disk / `ui.snapshot`), never the agent's self-report. |
| 2 | **Nonce per test** (`SMK…`): embedded in created content, checked for exact presence, so a prior run can't cause a false positive. |
| 3 | **Runs against the DEV vault** (`1234`, throwaway, `aw_DATA_DIR=$HOME/aw-dev-data`, REST `:9301`). Never the production vault. |
| 4 | **The runner is the single source of truth** (`scripts/automation/selftest/run_battery.py`). This spec must not duplicate the test logic — it calls the script. Improvements to coverage go in the script + README, not here. There are **two runners**: `run_battery.py` (smoke — one key action per module, 11 checks) and `run_coverage.py` (deep — full create→read→update→delete / lifecycle per module, ~31 checks). The spec can run either; deep coverage takes longer (more chat turns). |
| 5 | **No inline fixes.** A `FAIL` is recorded in the report and (if FAIL-APP) noted as a follow-up. Fixing it is a different spec so each agent owns one task. |
| 6 | **Abort clean if preconditions are missing** (dev app not up / no provider). Commit nothing, per the loop contract — do not try to build or relaunch the app from inside this spec (it would collide with the loop's own builds). |

## 3. Prerequisites (the spec checks; the runner enforces)

- Agent Workspace **dev build** running with REST on `:9301`, dev vault auto-unlocked. The
  dev build is now a **separate app bundle** (`build/bin/Agent Workspace-DEV.app`, built by
  `scripts/build-dev.sh`, launched by `scripts/run-dev.sh`) so it never
  overwrites the production `Agent Workspace.app`. See the `aw-dev` skill.
- A provider connected in that vault (GitHub Copilot `claude-sonnet-4.6`).

Both are validated by the runner's hard preflight. If it exits non-zero on
preflight, **abort the spec without committing** and report why.

## 4. Phases

### Phase 1 — Preflight

Run a single module as a smoke check and confirm the runner reaches the app:

```sh
~/venv/bin/python3 scripts/automation/selftest/run_battery.py --only home
```

If this fails on preflight (REST unreachable / no provider), **abort** — commit
nothing, leave the tree clean, and report the missing precondition.

### Phase 2 — Full battery + report

```sh
~/venv/bin/python3 scripts/automation/selftest/run_battery.py \
    --report "docs/plans/reports/selftest-$(date +%F-%H%M).md"
```

The script exercises all 11 modules and writes a markdown table. Capture the
report path it prints.

### Phase 3 — Record the result

- `git add` **only** the new report file (and, if you genuinely improved the
  runner/README during this pass, those explicit files — never `git add -A`).
- Commit: `selftest battery report (<date>): <N>/11 PASS`.
- If any module FAILed, add a short **Follow-ups** section at the bottom of the
  report naming each failing module and the suspected triage bucket (FAIL-APP /
  FAIL-AGENT / FAIL-VERIFY) per the README. Do not fix it here.
- Update this spec's `> **Status:**` to `implemented (<date>, commits: <hashes>)`
  and commit that edit. End with a **clean working tree**.

## 5. Done criteria

- A new `docs/plans/reports/selftest-*.md` exists and is committed.
- Its table shows all 11 modules with PASS/FAIL + the external check used.
- Any FAIL has a triage note; no product code was changed by this spec.
- `git status --porcelain` is empty.

## 6. Re-running

This spec is recurring. To run another pass later: reset its Status to
`not started`, append the spec filename to `docs/plans/queue.txt`, and let the
loop pick it up (with the dev app already running). Each pass produces its own
dated report.
