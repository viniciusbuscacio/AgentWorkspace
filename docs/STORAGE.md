# Storage map — where aw saves what

The canonical reference for every piece of persisted state: what it is, where
it lives, whether it syncs, and who wins on conflict. Written 2026-07-21 after
the working-copy + workspace.json + quick-unlock work; update it whenever a
new persistence surface is added.

## The three scopes

| Scope | Meaning | Travels with |
|-------|---------|--------------|
| **Vault** | Secrets and data — encrypted | the master folder |
| **Workspace** | The vault's "look" — plain, non-secret | the master folder |
| **Machine** | Device/bootstrap state | this computer only |

## Vault (master folder — the folder the user picked, OneDrive allowed)

The master folder holds only the COLD form of the vault. Spec:
`docs/specs/vault-working-copy.md`.

| File | What |
|------|------|
| `vault.db` | Encrypted SQLite snapshot (adiantum VFS). Chats, messages, secrets/API keys, passwords module, notes, LLM debug turns, chat titles, session summaries, user memory. Never a live DB — no `-wal`/`-shm` here, ever. |
| `vault.salt` | Key-derivation salt (plain, atomic writes — sync-safe). |
| `vault.recovery` | Recovery-key blob (plain, atomic writes). |
| `workspace.json` | Per-vault workspace state (see below). |

**Live database = working copy**, an ephemeral local copy at
`%LOCALAPPDATA%\AW\work\<profile-id>\` (Windows) / `~/Library/Caches/AW/work/`
(macOS), hydrated from the master on unlock. Same encryption; safe to delete
when the app is closed. Write-back: 1-minute tick + final synchronous snapshot
on lock/quit (`VACUUM INTO`, atomic). **Master manda**: if another machine's
snapshot arrives via sync, write-back freezes and the app auto-resyncs at the
first idle moment, discarding local changes since the last snapshot. A
`master.stamp` file in the work dir tracks which master version we last
wrote/read. Legacy mode (no work dir configured) runs the live DB directly in
the master folder — only tests/harness do this.

## Workspace (`workspace.json`, next to vault.db — plain JSON, atomic writes)

Per-vault UI state; readable BEFORE unlock (the lock screen renders the right
theme) and travels with the vault across machines. Keys: `addedModules`,
`hiddenModules`, `moduleOrder`, `activeTheme`, `customThemes`, `wallpaper`,
`wallpaperGlass`, `lastView`, `lastChatId`, `providerFallbackOrder`.

- Pre-existing vault without the file: seeded ONCE from the global config.
- Freshly created vault: starts clean (`InitCleanWorkspace` at create time).
- Store: `appconfig.WorkspaceStore` (dir provider follows profile switches).

## Machine (`%APPDATA%\AW\` — os.UserConfigDir, overridable via `aw_DATA_DIR`)

| File/dir | What |
|----------|------|
| `config.json` | Machine + bootstrap config: `vaultDir` (last vault), auto-lock minutes, zoom, browser paths/ports, self-dev posture, desktop notifications, provider cooldown minutes, subagent mode, web server / TLS / firewall settings (all readable pre-unlock). |
| `profiles.json` | The recent-vaults list: id, name, avatar, `vaultDir` (master path) per profile. |
| `workspace/` | The agent's file workspace (sandbox default root). |
| `browser-profiles/`, `gws-accounts/`, `voice/` | Agent browser profiles, Google Workspace accounts, voice temp. |
| `server-tls/` | TLS certificate material. |
| Custom wallpaper images | User-uploaded wallpaper files (machine-local). |

## OS credential store (quick unlock — opt-in, per vault)

The vault password, so unlock is one click / hands-free at launch. Scoped per
vault: account name = `vault-` + sha256(master path)[:24], service
`AgentWorkspace-Vault`.

- **Windows**: Credential Manager entry `AgentWorkspace-Vault/vault-<hash>`.
  Plain-click release (no Windows Hello gate — deliberate; protection level is
  the Windows login). Blob = JSON `{password, savedAtUnix}`; expires after
  **7 days without a manual password unlock** (every password unlock re-saves
  it — rolling). App launch with a valid credential logs straight in, ONE
  attempt per run; after a manual lock the app stays locked.
- **macOS**: Keychain item, Touch ID enforced BY THE OS (stronger guarantee).
  No TTL.
- Web/remote mode: all quick-unlock bindings are deny-listed — host only.

## In-memory only (lost on quit, by design)

ADK chat sessions (re-seeded from the vault on the next turn after restart or
vault resync), provider cooldown/circuit-breaker state, run registry,
partial-reply buffers, external-content taint.

## Rules of thumb for changes

- New secret → vault.db. New UI/look preference → workspace.json. New
  device/path/port setting → config.json. Never a live SQLite file in a
  possibly-synced folder.
- Small files in the master folder must be written atomically (temp + rename).
- Anything the lock screen needs must live OUTSIDE the encrypted DB.
