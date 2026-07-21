# MCP Server module — user guide

One-line: expose this app's single `aw` tool to EXTERNAL agents (Claude Code,
other MCP clients) over Streamable HTTP.

## Purpose and trust model

- Turns Agent Workspace into an MCP server: an external agent connects and
  gets one tool, `aw`, that multiplexes every action of this app.
- **Reachability is governed by the Agent Firewall** (Settings → Agent
  Firewall): deny-all by default — nothing is reachable, not even loopback,
  until a PERMIT rule opens an interface.
- Every request needs the bearer token; it lives encrypted in the vault.
- Lifecycle follows the vault: the server only runs while unlocked and stops
  when it locks.

## What the user sees

- **Server card**: explanation text, Running/Stopped dot, the endpoint
  (`http://127.0.0.1:<port>/mcp`) with Copy, a Start/Stop button, Port
  (draft, saved with Save/Cancel), auto-start toggle, and an HTTPS toggle
  (uses the shared TLS certificate from Settings → TLS; enabling without a
  certificate is refused with a pointer to the TLS manager).
- **Bearer token card**: the token masked, with Copy and Regenerate.
  Regenerating disconnects every connected client.
- The Apps grid card has the same start/stop toggle inline.

## How to connect (external client)

Point the MCP client at the endpoint with header
`Authorization: Bearer <token>`. The only tool is `aw`; the client calls
action `aw.actions` to discover every capability.

## Relationship to other pieces

- **REST API Server** is the plain-HTTP mirror of this for non-MCP clients.
- **MCP Client** is the opposite direction (this app consuming other servers).
- The agent can manage server state via the core `app.server.set`
  (`{ server: "mcp"|"rest", enabled }`) and read it in `system.state`.
