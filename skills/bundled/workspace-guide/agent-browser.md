# Google Chrome / Microsoft Edge modules — user guide

One-line: the agent's own real browser (Chrome or Edge), driven over CDP, in
an isolated profile fully separate from the user's browser.

## Purpose and trust model

- Two thin modules over one CDP core: "Google Chrome" and "Microsoft Edge".
  Adding one gives the agent a real browser it can start, navigate, read,
  click and fill.
- **Always an isolated profile.** The browser launches with its own profile
  dir under the app's data (`browser-profiles/<module>`) and an aw-owned
  debugging port. It never touches the user's own browser, tabs, or logins.
  There is no way to attach to the user's personal profile: Chromium 136+
  refuses remote debugging on the default user data dir outright.
- **Logins**: if a site needs an account (e.g. Gmail), the user signs in once
  INSIDE the agent's browser window; the session persists in the isolated
  profile. Turning on browser sync there can bring favorites/passwords.
- **Downloads are disabled** in the agent browser (CDP deny, fail-closed) so
  pages cannot write to disk around the Permissions sandbox.
- Everything read from pages returns sanitized, size-capped and marked as
  untrusted external content, and taints the turn (sensitive actions re-gate).

## What the user sees

- **Status card**: Connected/Stopped dot, CDP port, binary path,
  Connect/Disconnect, Refresh, and Screenshot (while connected).
- **Browser executable**: custom path + Browse/Reset for non-default installs.
- **Connect automatically when the app starts**: opens the agent's browser
  when the vault unlocks (default off).
- **Tabs card**: live tab list of the agent's browser, plus the latest
  screenshot when taken.
- The Apps grid card has the same start/stop toggle inline.

## How the agent accesses it

Actions exist only while the browser's module is added:

- Lifecycle: `browser.start { profile? (aw|inprivate), headless? }`,
  `browser.stop`, `browser.status`. `inprivate` is ephemeral (nothing
  persists).
- Reading: `browser.tabs`, `browser.snapshot` (accessibility tree with [eN]
  refs — structure, not prose), `browser.cdp` (Runtime.evaluate for body
  text), `browser.screenshot`. Reads are direct and sanitized.
- Acting: `browser.navigate`, `browser.new_tab`, `browser.click`,
  `browser.fill`, `browser.close_tab`. Mutating CDP calls require
  confirmation; navigation is fenced (local/private addresses blocked,
  `file://` goes through the Permissions sandbox).
- Gmail helpers (`gmail_web.*`) work on an open Gmail tab in this browser and
  are re-gated on tainted turns.

The `browser` arg (chrome|edge) is only needed when both modules are added.
