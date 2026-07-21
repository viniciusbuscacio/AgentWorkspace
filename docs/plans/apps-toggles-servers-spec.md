# Spec — Apps screen: runtime toggles + servers as cards

> **Status:** implemented (2026-06-12, commits: 948fa49, 2574d9d)

## 1. Objective

The Apps screen stops being only a catalog and starts showing **live
state**: things that can be ON or OFF (browsers, local servers) get a
visible toggle right on their card.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **The toggle controls RUNTIME state, not workspace membership.** ON/OFF on a browser card starts/stops the browser process; on a server card it starts/stops the server. Adding/removing a module from the workspace stays where it is (card click / sidebar context menu). The two states are independent and both visible. |
| 2 | **Browser cards (Agent Browser Chrome, Agent Browser Edge)** get the toggle: ON = `browser.start`-equivalent via the existing Wails browser surface (default profile rules unchanged — aw isolated), OFF = stop. The toggle reflects the real CDP-endpoint state (the existing Status check), not a local boolean; while starting/stopping it shows a busy state. The personal-profile restart flow is NOT reachable from the toggle — it stays in the module view where the warning UX lives. |
| 3 | **MCP Server and REST API become cards on the Apps screen** (a "Services" row in the same grid). They are NOT workspace modules (no sidebar item, no module contract) — a new lightweight service-card type rendered by Home. Toggle = the existing enable/start logic from `application/api_servers.go` (same semantics as the Settings pages); clicking the card body opens the corresponding Settings page. |
| 4 | Toggle state updates live: reuse the existing status results/events; poll only where an event does not exist already (browsers poll status the way the Browser module view does). |
| 5 | No new capability is minted: everything the toggles do is already reachable through Settings pages or the module view — this is surfacing, not new power. The `ui.click` exposure is therefore unchanged. |

## 3. What already exists — reuse, don't reinvent

- Browser start/stop/status Wails surface (`app_browser.go`) and the
  status-polling pattern in `BrowserModule.tsx`.
- Server enable/start logic and settings shape:
  `internal/application/api_servers.go`, the two Settings pages and
  `ApiServerSettingsCards.tsx`.
- The Home grid card components — extend with a toggle slot and a service
  card variant; keep the card visuals consistent (this lands near the
  polish-wave changes to the same screen — coordinate, see Risks).
- A switch/toggle primitive from `components/ui/` if one exists; otherwise
  add the Radix Switch once, in `components/ui/`, and use it in both places.

## 4. Phases

### Phase 1 — Browser card toggles

Toggle slot on module cards, wired for the two browser cards (status
reflect + start/stop + busy state). Vitest: toggle calls the service,
busy while pending, reflects status result.

### Phase 2 — Service cards (MCP + REST)

Service-card variant in the Home grid ("Services" row), toggle wired to the
existing server enable/start logic, card click navigates to the Settings
page. Vitest: ON/OFF round-trip with mocked service; navigation works.

## 5. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. **This spec and the polish wave both edit the Apps screen**
   (`apps-notes-polish-spec.md` changes the heading and removes the check
   badge). Land polish Phase 1 first, or coordinate in one branch — do not
   create a merge fight over `HomeModule.tsx`.
2. A browser toggle that lies (shows ON when CDP is dead) erodes trust in
   the whole screen — always reflect the endpoint check, never the last
   action.
3. Stopping the MCP/REST server while an external client is mid-call is
   already handled by the existing stop path — do not add new shutdown
   logic, just call it.
4. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart — say so in summaries.
