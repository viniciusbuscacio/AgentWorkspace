# Settings — user guide

One-line: the hub of cards where the user configures the app; each card is a
page. What follows is what each page does and the traps worth explaining.

## LLM Providers

- Cards for the built-in providers (GitHub Copilot, OpenRouter, OpenAI API
  Key, OpenAI Subscription, Azure OpenAI, NVIDIA NIM) plus any custom
  OpenAI-compatible providers the user added via "Add custom provider".
  A fresh vault shows NO custom slot — customs are created on demand.
- A provider that was never configured starts **disabled** (grey toggle,
  "Not Configured"); configuring it (key/OAuth) connects and enables it.
- Editing a provider: model picker (live model list when the provider
  answers), API key / credential fields (write-only: stored in the vault,
  never displayed back), base URL for OpenAI-compatible customs.
- Device-flow sign-in (GitHub Copilot) shows the code in a large copyable
  box; browser OAuth (OpenAI Subscription) has no code — it waits for the
  redirect.
- **Global default vs per-chat model**: the provider order IS the chain —
  **#1 is the "Agent Workspace default"** used by new chats (setting a
  provider to #1 activates it; activating a provider moves it to #1, so the
  numbers and the Active badge never diverge). #2 is the first fallback when
  #1 fails, and so on; disabled providers are never used, not even as
  fallback. A chat where the user typed `/model <provider> [model]` keeps
  its own override until `/model default`. The agent can change the global
  default with `provider.switch` or `provider.order.set`; the per-chat lever
  is the `/model` slash command.
- **Automatic failover**: when a provider fails with an endpoint-class error
  (down, timeout, rate-limited, out of credit), the turn automatically moves
  to the next provider in the order, the chat shows a system message naming
  the switch and a desktop notification fires. The failed provider is then
  benched and NOT retried until the app restarts (default; a timed bench in
  minutes is configurable, 0 disables benching). Reconfiguring a provider's
  credentials lifts its bench.

## Security

- **Auto-lock** — re-lock the vault after N minutes of inactivity.
- **Quick unlock** — optional per-vault unlock via the OS credential store,
  opt-in at unlock/create, never silent. On **macOS** the vault password
  lives in the Keychain gated by Touch ID (enforced by the OS, no expiry);
  when the app's local signing certificate is untrusted, the card offers a
  one-click "Trust certificate" fix (macOS asks for the account password) so
  Keychain "Always Allow" persists across rebuilds. On **Windows** it lives
  in the Credential Manager (plain-click release; protection level is the
  Windows login) and expires after 7 days without a manual password unlock —
  each password unlock re-saves it. With a valid credential the app **logs
  straight in at launch** (one attempt per run); after a manual lock it
  stays locked. "Remove" deletes the stored credential.
- **Vault secrets** — names-only inventory of the app's own technical
  secrets (provider API keys, OAuth JSONs, server tokens). One Delete per
  secret. It hides `_`-prefixed internals on purpose: Passwords-module
  credentials, agent memory and config flags do NOT appear here. Values are
  never shown. It is an audit/cleanup panel, not a password manager.
- **macOS permissions** — TCC inventory (Microphone, Automation, Files and
  Folders, App Management…) with per-row "Open System Settings" deep links
  and probes. The app cannot grant these itself.

## Permissions (the sandbox)

The single fence for every file/shell operation — every `fs.*`/`shell.*`
call passes through it.
Three modes, picked as radios with the selected mode's panel below: **Block
all** (`block_all` — no shell, workspace only), **Balanced** (`permit_list`,
the default — only the folders the user lists, edited right in the panel) and
**Permit all** (`permit_all` — everything; built-in protected paths still
blocked). deny_list was removed; a vault still storing it fails safe to
permit_list. Changing the mode is UI-only: the `sandbox.set_mode` action is
always refused, so planted content can never widen the fence.

## API servers (MCP / REST) + Agent Firewall + TLS

- **Servers** (the hub) — one card per network server (REST API, MCP Server,
  Web Access) with live status and start/stop. It is the page the sidebar's
  green exposure dot opens; full configuration lives in each server's own
  module page, one click away.
- MCP server (`127.0.0.1:9300`) and REST mirror (`127.0.0.1:9301`) expose the
  same `aw` tool to external clients, bearer-token protected (tokens shown
  masked; regenerate any time).
- The Agent Firewall is an ordered PERMIT/DENY ACL over (service, interface,
  origin): servers only bind where a PERMIT rule allows, deny-all by default
  (even loopback). PERMIT outside loopback requires a specific CIDR/IP.
- TLS page manages the certificate the servers use on non-loopback binds.
- Web access page serves the app UI to a browser; native-dialog features and
  local-secret features (Passwords, Touch ID, trust settings) are denylisted
  over the web bridge by design.

## Agent pages

- **Agent Instructions** — AGENTS.md (app-managed) and USER.md (the user's
  own standing instructions), both vault-stored and injected into the prompt.
- **Memory** — the durable user-memory document the agent maintains via
  `memory.remember`; the page live-reloads when the agent writes it.
- **Subagents** — delegation mode for generic subagents (Off / Balanced /
  Aggressive); they receive small scoped tasks under the main agent.
- Skills management is NOT here: it is the **Skills app** (Apps → Skills) —
  browse/create/import/edit skills, enable/disable, reset builtins.

## Appearance & misc

- **Theme** (catalog + custom theme editor), **Font** (bundled families +
  size), **Wallpaper** module for Home/Apps backgrounds, zoom.
- **Vault** — vault location, change password, recovery key.
- **Debug** — diagnostics; **About** — the real build version (stamped at
  release time; local builds show "dev") and a link to the project's GitHub
  (viniciusbuscacio/AgentWorkspace), opened in the OS browser.
