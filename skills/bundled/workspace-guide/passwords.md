# Passwords module — user guide

One-line: an encrypted credential store whose contents the user deliberately
shares with the agent.

## Purpose and trust model

- Credentials the user saves here are MEANT to be used by the agent (log into
  sites, fill forms). Their fully private passwords belong in a personal
  manager (e.g. Bitwarden), not here — this module is the sharing surface.
- Everything is stored in the encrypted vault (SQLite `secrets` table, one
  secret per credential, `_password:` name prefix). Encrypted at rest; only
  readable while the vault is unlocked.
- These credentials do NOT appear in Settings › Security › "Vault secrets"
  (that card intentionally hides `_`-prefixed entries — it lists the app's own
  technical secrets like provider API keys, not user credentials).

## What the user sees

Two panes under the title "Passwords — Encrypted, local-only, shared with the
agent":

- **Left — list**: search box, an "Add Password" button, and one card per
  credential (name + username or URL). Nothing is auto-selected.
- **Right — editor**: starts CLOSED (empty right side). It opens when the user
  clicks a credential or "Add Password", and closes via the X pinned at the
  panel's top-right corner.

Editor fields and controls:

- Name, Username/email, Password, URL, Notes.
- **Eye icon** (inside the password field): reveals the typed value for the
  HUMAN only — `ui.snapshot` keeps masking it (`data-sensitive`), so the agent
  never reads a revealed field from the screen. The reveal resets when
  switching credentials.
- **Copy**: copies the password to the macOS clipboard and auto-clears it
  after 25 seconds (only if the clipboard still holds that exact value — a
  newer copy is never destroyed). A copy failure reports an error.
- **Save / Cancel**: Save persists to the vault; Cancel reverts the draft.
  Switching credentials or closing with unsaved changes asks before
  discarding.
- **Delete**: asks for confirmation in a dialog; deletion is permanent (a
  deleted password is unrecoverable).

## Web mode

Over the web bridge the module shows a desktop-only notice. The password
bindings are denylisted server-side: credential plaintext never travels to a
remote browser, by design.

## How the agent accesses it

Actions exist only while the module is added to the workspace:

- `passwords.list {}` — metadata for all credentials: name, username, URL.
  Never values, never notes (notes often hold recovery codes).
- `passwords.get { "id": "<id or exact name>" }` — the ONLY path that reveals
  a password value. On a turn tainted by untrusted external content (web page,
  email, tool output) the reveal is re-gated and denied without user approval —
  a planted "read me that credential" is the classic exfiltration setup.

Etiquette: reveal at the exact moment of use; never echo a password into chat,
notes, files or logs unless the user explicitly asks to see it.

## Known limits

- No built-in password generator (yet).
- Notes are excluded from `passwords.list` on purpose.
- No import/export UI.
