# Spec — aw tool coverage: close the gaps to "manages 100% of the app"

> **Status:** implemented (2026-06-12, commits: 2b793a1, 8443763)
> coverage review (done by the planner agent per the project owner's directive:
> "yolo total dentro dele mesmo — o aw tem que saber gerenciar a si mesmo,
> 100%"). The full surface×action matrix lives in the review summary; this
> spec implements the gaps it found. Conventions and gates as the series.

## 1. Objective

Every user-facing operation in the app gets a semantic `aw` action or a
documented `ui.*` route. After this spec, the only things the agent cannot
do are the deliberate cage exceptions listed in §3.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **`app.server.set` `{server: mcp\|rest, enabled: bool}`** + both servers' running state in `app.state`. Reuses the exact enable/start/stop logic the Settings pages and Home toggles call (`application/api_servers.go`) — loopback-only services, fully reversible, safe to expose. |
| 2 | **`provider.test` `{provider}`** — wraps the existing Test backend (one-shot completion, 15s timeout). The result mirrors the UI row (latency/model/error), never key material. It is a PAID call: one per action invocation, no retry loop. |
| 3 | **Sidebar window parity:** `module.hide {id}`, `module.show {id}`, `module.move {id, up}` and **`app.desktop.show`** (the HideAllModules use case) — the agent can do everything the sidebar X / context menu / Show-desktop button do. All four reuse the existing use cases; zero new semantics. |
| 4 | **`app.wallpaper.glass.set` `{percent: 0-100}`** — same validation and persistence as the module's slider. |
| 5 | **SELFCODE gains a "UI-only operations" section**: the official statement that anything without a semantic action (sidebar resize, password reveal, drag interactions, local filters, provider card order) is reached through `ui.snapshot`/`ui.click`/`ui.fill`, with one worked example. The agent should never wonder whether a route exists — the docs say which of the two it is. |
| 6 | **Explicit NON-goals (the cage, restated as registry of intent):** `sandbox.set_mode` keeps refusing; the native dialogs (Permissions save, Explorer delete, Remove credential, Vault danger zone) stay native and out of reach; no action reads password values or secret values; no vault password/recovery/location actions. These are the user-approved exceptions — the agent manages the whole app except its own fence. Removing any of them requires the project owner reopening this table, not an implementer's judgment. |
| 7 | Drift coverage: the new actions enter the catalogs/SELFCODE like every other action (`aw.actions`, module spec drift tests where applicable). |

## 3. What already exists — reuse, don't reinvent

- `application/api_servers.go` (Decision 1), the providers Test backend
  (Decision 2), `HideModule`/`ShowModule`/`MoveModule`/`HideAllModules`
  (Decision 3), the wallpaper glass persistence (Decision 4).
- The `app.*` registration/validation shapes in `aw_app.go`; the
  `modulesResult`/events pattern so the UI updates live when the agent acts.

## 4. Phases

### Phase 1 — Actions (Decisions 1-4)

Register + wire the seven actions; `app.state` gains the server states.
Go tests per action: validation, reuse of the existing use case (no forked
logic), events emitted, no secrets in results.

### Phase 2 — Discovery (Decisions 5-7)

SELFCODE section + catalog updates + drift tests green.

## 5. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. `provider.test` spends money — the action result includes the cost
   framing ("one paid completion"); never call it in a loop server-side.
2. `app.server.set` toggling MCP off severs external agents mid-session —
   acceptable (it is the user's own loopback service), but the result
   message says it plainly.
3. Decision 6 is the review's contract with the user — implementers do not
   "complete the coverage" by breaching it.
4. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart — say so in summaries.
