# aw self-test battery

Dogfoods two things at once:

1. **Self-management** — can the *internal* aw agent do real work when asked
   in plain language? (one key operation per module)
2. **Module health** — does each module actually work? Every effect is verified
   **externally** (REST / disk / UI tree) with a per-test random nonce, so the
   harness never trusts the agent's "done".

The external tester (this script, or a `pi` worker driving it) is a *separate*
process from aw: it talks to the internal agent over the chat REST surface and
checks the result out-of-band.

## Prerequisites

- Agent Workspace **dev build** running, REST on `:9311`, dev vault `1234` auto-unlocked.
  See the `aw-dev` skill. Dev is a **separate** app bundle (`Agent Workspace-DEV.app`) that
  never overwrites the production `Agent Workspace.app`. Build + launch:

  ```sh
  cd ~/aw
  ./scripts/build-dev.sh   # builds build/bin/Agent Workspace-DEV.app (auto-unlock vault 1234)
  ./scripts/run-dev.sh     # launches it with aw_DATA_DIR=~/aw-dev-data, REST :9311
  ```

- A provider connected in the dev vault (GitHub Copilot, `claude-sonnet-4.6`)
  via **Settings → Providers** (device flow, once).

The script runs a hard preflight and fails loud if either is missing.

## Usage

```sh
# all 11 modules
~/venv/bin/python3 scripts/automation/selftest/run_battery.py

# one module
~/venv/bin/python3 scripts/automation/selftest/run_battery.py --only notes

# write a markdown report (used by the loop spec)
~/venv/bin/python3 scripts/automation/selftest/run_battery.py \
    --report docs/plans/reports/selftest-$(date +%F).md
```

### Deep coverage (full CRUD/lifecycle)

`run_battery.py` does one key action per module. `run_coverage.py` exercises the
full **create -> read -> update -> delete / lifecycle** of each module (~31 action
checks) — so a broken *edit* or *delete* path is caught before real use:

```sh
~/venv/bin/python3 scripts/automation/selftest/run_coverage.py \
    --report docs/plans/reports/coverage-$(date +%F).md
# one scenario: --only notes,tasks,chat
```

Dangerous/irreversible actions are excluded by design (it says so at the top of
the file): `app.lock` (would lock the vault and kill REST),
`provider.credential.delete` (would drop the connected provider),
`provider.test` (paid call), `chat.delete_permanent` / `chat.clear` (mass
destructive). Note `chat.delete` is a **soft** delete (trash-bin = archive,
recoverable); `chat.delete_permanent` is the hard delete.

Exit code is `0` only if **every** module passed, `1` otherwise — so it composes
with the spec-queue loop.

## How a test works

For each module: create an isolated chat → ask the internal agent (with a unique
`SMK…` nonce embedded) → poll `chat.messages` until the turn settles → verify the
effect externally → classify PASS/FAIL → clean up.

| Module | Asked to… | Verified by |
|--------|-----------|-------------|
| notes | create a note == nonce | `notes.list` |
| tasks | add an item == nonce | `tasks.list` |
| fs | write `/tmp/aw-selftest-<nonce>.txt` | disk read |
| chat | create a chat titled nonce | `chat.list` |
| settings | set zoom 115% | `app.state.zoomPercent` |
| logs | report log count | `logs.list` |
| wallpaper | set glass 45% | `app.state.wallpaperGlass` |
| home | navigate Home | `app.state.ui.view` |
| passwords | create entry named nonce | `ui.snapshot` tree |
| browser | open example.com | `browser.status` |

The agent adds modules to the workspace on its own when they aren't loaded yet —
that resilience is part of what's under test.

## Triage when a module FAILs

- **FAIL-APP** — the tool/app is broken → fix the code (becomes a normal spec).
- **FAIL-AGENT** — the tool works but the LLM didn't do it / hallucinated → fix
  the prompt (`internal/infrastructure/agent/prompts/base.md`) or the tool
  description.
- **FAIL-VERIFY** — the harness checked the wrong place → fix this script.

Always confirm on disk/config before blaming the agent (e.g. the glass blur was
really set in `config.json`; the gap was that `app.state` didn't surface it).
