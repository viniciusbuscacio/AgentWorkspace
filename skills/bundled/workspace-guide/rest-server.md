# REST API Server module — user guide

One-line: the plain-HTTP mirror of the MCP Server, for scripts and clients
that do not speak MCP.

## Purpose and trust model

- Same capabilities as the MCP Server over simple HTTP:
  - `POST /api/aw` runs any `aw` action
    (`{"action":"app.theme.set","args":{"theme":"ocean"}}`).
  - `GET /api/actions` lists every available action.
- Same rules as MCP: bearer token on every request, reachability governed by
  the Agent Firewall (deny-all default), runs only while the vault is
  unlocked, optional HTTPS via the shared TLS certificate.

## What the user sees

Identical layout to the MCP Server module (they share the same settings UI):

- **Server card**: Running/Stopped, endpoint (`http://127.0.0.1:<port>/api/aw`)
  with Copy, Start/Stop, Port draft with Save/Cancel, auto-start and HTTPS
  toggles.
- **Bearer token card**: masked token, Copy, Regenerate (disconnects clients).
- The Apps grid card has the same start/stop toggle inline.

## Example call

```
curl -X POST http://127.0.0.1:<port>/api/aw \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"action":"system.state","args":{}}'
```

## Relationship to other pieces

- Default port differs from MCP (9301 vs 9300 in production); both are
  configurable and both bind only interfaces the Agent Firewall permits.
- Native-dialog Wails methods are denylisted over remote bridges by design —
  a remote client can never pop a file picker on the desktop.
