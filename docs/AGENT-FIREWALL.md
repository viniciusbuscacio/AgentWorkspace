# Agent Firewall

The Agent Firewall is a single, ordered network access-control list (ACL) that
governs which peers may reach the app's local servers — MCP, REST, and (planned)
Web — and any future network module. It replaces the divergent per-server peer
checks with one audited mechanism.

## Model

A rule is `(action, service, interface, origin)`:

| field | values |
|-------|--------|
| action | `PERMIT` \| `DENY` |
| service | `mcp` \| `rest` \| `web` \| `all` (wildcard) |
| interface | `loopback` \| `tailscale` \| `lan` \| `public` \| `all` (wildcard) |
| origin | a CIDR (`10.10.0.0/16`), a single IP (`10.40.45.23`), or `any` |

```
PERMIT  mcp   tailscale  10.10.0.0/16
DENY    web   lan        10.40.45.99/32
PERMIT  web   lan        10.40.0.0/16
(implicit) DENY ALL ALL          ← always last, immutable
```

Rules evaluate **top-down, first-match wins**. Anything that falls through hits
the implicit, immutable `DENY ALL ALL`.

### Two invariants

1. **Deny by default — loopback included.** An empty ruleset blocks everything.
   `127.0.0.1` is not special; to reach a server locally you add an explicit
   `PERMIT <svc> loopback 127.0.0.1/32`.
2. **No wide-open non-loopback PERMITs.** A `PERMIT` on any interface other than
   `loopback` must name a specific CIDR or IP — never `any` / `0.0.0.0/0`.
   Opening a non-loopback interface to every source is exactly the footgun the
   firewall exists to prevent (the REST/MCP surface can run `browser.cdp`,
   delete credentials, etc.). `DENY` rules and loopback PERMITs may use `any`.
   Enforced in `application.ValidateFirewallRules`.

### Bind behavior — Option A

A server binds a listener **only on the interfaces a PERMIT rule opens for it**
(`agentfw.BindKinds` → `agentfw.ResolveBindIPs`). An interface with no PERMIT is
never bound — the socket does not exist there, so there is nothing to attack. If
no rule permits a service, it does not bind at all and the settings page shows it
blocked by the firewall.

Enforcement still happens per request (`agentfw.Guard`): the local address a
connection arrived on is classified and matched against the rules, so
per-interface DENY exceptions work even when one `http.Server` fronts several
per-interface listeners. A request whose local or remote address does not parse
as an IP is denied before the rules are consulted — a wildcard-interface rule
can never match a connection whose arrival interface is unknown.

### Network changes — the rebind loop

Bind IPs are resolved when a server starts, but interfaces come and go
(Tailscale up/down, Wi-Fi DHCP renews). A background reconciler
(`appcore.firewallRebindLoop`, every 15s) re-resolves each governed server's
desired bind set from the rules and bounces the server when the set drifted —
including **starting a blocked server whose permitted interface just
appeared**, and re-blocking one whose only permitted interface vanished.

Relatedly, a server whose start the firewall refused (auto-start *or* a manual
Start click) is remembered as *blocked-but-wanted*: saving rules that permit it
brings it back without another click. Stopping a server withdraws the want.

## Layers (where the code lives)

| layer | package | role |
|-------|---------|------|
| domain | `internal/domain/firewall.go` | `FirewallRule`, `FirewallConfig`, `NetworkInterface`, selectors/validators (pure data) |
| infrastructure | `internal/infrastructure/agentfw` | pure evaluator (`Decide`, `MatchOrigin`, `BindKinds`), interface enumeration (`ListInterfaces`, `ResolveBindIPs`), the HTTP `Guard`, and `Lister` (port impl) |
| application | `internal/application/firewall.go` | `LoadFirewallRules`, `SaveFirewallRules` (+ `ValidateFirewallRules`), `ListNetworkInterfaces` |
| ports | `internal/domain/ports/app_settings.go` | `FirewallSettingsStore`, `NetworkInterfaceLister` |
| config | `internal/infrastructure/appconfig/firewall.go` | rules persisted in `config.json` (readable pre-unlock) |
| interface | `internal/appcore/app_firewall.go` | Wails methods `GetAgentFirewall` / `SetAgentFirewallRules`; `reapplyFirewall` bounces the governed servers |
| frontend | `services/agentfw.service.ts`, `settings/pages/AgentFirewallPage.tsx` | the settings UI |

The pure evaluator is exhaustively unit-tested in
`agentfw/firewall_test.go`; the HTTP guard (including the deny on unresolvable
addresses) in `agentfw/guard_test.go`; the safety invariants in
`application/firewall_test.go`.

## Security notes

- **The firewall is a network fence (L3/L4); the bearer token is a separate
  app-layer auth.** Both apply — a request must pass the firewall *and* present
  the token. Defense in depth.
- **There is no `aw` action to read or change the firewall.** Like
  `sandbox.set_mode`, network policy is UI-only so a compromised or
  prompt-injected agent cannot open its own fence.
- **Rotate a leaked token.** The firewall limits *who can reach* a server; the
  token limits *who is authorized*. If the token leaks, rotate it in the
  server's settings page.

## Adding a governed service (future network modules)

1. Add the service id to `domain.FirewallServices` and a `FirewallService*`
   const in `domain/firewall.go`.
2. In the server's `NewServer`, take `Service` + `Rules` + `FirewallPort` and
   resolve binds via `agentfw.BindAddrs`, gate with `agentfw.Guard` (mirror
   `restserver`/`mcpserver`).
3. In the composition root, pass the loaded rules and handle
   `agentfw.ErrNoPermittedInterface` as "blocked by firewall", and add the
   server to `reapplyFirewall`.

## Not yet migrated

The **Web** server still uses its own bind mode (`WebServerConfig`, Tailscale
detection, sessions). Folding it fully into the ACL — including migrating its
`AllowedCIDRs` into firewall rules — is the remaining follow-up; the UI and
service registry already list `web` so the wiring is the only gap.
