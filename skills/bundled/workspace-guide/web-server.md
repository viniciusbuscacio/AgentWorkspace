# Web Access module — user guide

One-line: open the full app UI in a remote browser, riding Tailscale's
encrypted network.

## Purpose and trust model

- Serves the complete frontend over HTTP so the user can drive their
  workspace from another machine (laptop, phone).
- **Tailscale bind (recommended)**: the server binds only the Tailscale
  interface (100.64.0.0/10) — transport is WireGuard-encrypted and only the
  user's tailnet can reach it. Without a Tailscale interface up, the server
  refuses to start in this mode.
- **Manual bind** exists but serves plain HTTP with no Tailscale enforcement —
  the UI warns strongly; anyone who can reach the port can try the login.
  HTTPS via the shared TLS certificate (Settings → TLS) is available.
- Login: the web session authenticates against the vault password; the
  signing key for sessions lives in the vault. Sessions ALWAYS end when the
  vault locks or auto-locks; a session timeout is configurable.
- Sensitive native operations are denylisted on the web bridge: password
  reveals, native file pickers and native dialogs never travel to a remote
  browser.

## What the user sees

- **Server card**: Running/Stopped, the URL to open remotely, Start/Stop,
  bind mode (Tailscale / manual), port, auto-start, HTTPS toggle.
- **Sessions card**: active remote sessions, a session-timeout setting, and
  "Reset signing key & sign out all" — invalidates every session immediately
  (takes effect at once, like Start/Stop).
- The Apps grid card has the same start/stop toggle inline.

## Notes

- The web login screen is served pre-unlock (the server boots with the vault
  still locked); everything else requires unlocking.
- Web Access exposes the UI; the MCP/REST servers expose the agent API — three
  separate doors, all behind the Agent Firewall.
