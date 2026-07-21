# Spec — Permissions / Sandbox (port of the AW2 model)

> **Status:** implemented (2026-06-10, phases 1-4: commits 59d6143, 4355806,
> 4957945, bbe173b; later hardened in `permissions-sandbox-hardening-spec.md`,
> commit ab218ce). Kept as the design record. Third spec in
> the series (`workspace-modules-spec.md`, `prompt-base-draft.md`) — same
> conventions, same gates. This was a **port of AW2's battle-tested sandbox**,
> not a new design. The reference implementation lives in
> `AgentWorkspace2`:
>
> - `src/shared/infrastructure/path-validator.ts` — modes, built-in lists, `isPathAllowed`, `extractPaths`
> - `src/shared/infrastructure/sandbox-shell.ts` — `sandboxExec` flow, safe no-path commands
> - `src/shared/infrastructure/sandbox-fs.ts` — symlink/traversal-safe fs ops (`assertSandboxResolvedPath`)
> - `src/shared/infrastructure/aw-actions/sandbox-actions.ts` — `sandbox.status/test/set_mode`
> - `src/shared/application/build-state-context.ts:477-546` — prompt injection per mode
> - `src/renderer/modules/permissions/` — the Permissions UI
> - tests: `path-validator.test.ts`, `sandbox-shell.test.ts`, `sandbox-fs.test.ts` (port the cases)
>
> When in doubt about semantics, open the AW2 file and copy the behavior.
>
> Implementation note: aw now enforces the sandbox through the Go checker
> before filesystem and shell operations, with the app data dir, vault and
> `config.json` protected in every mode. Follow-up hardening in
> `permissions-sandbox-hardening-spec.md` added Windows path extraction,
> platform-specific built-in denies and an explicit self-dev shell warning.
> Covered chokepoints today: always-on `read_file` / `write_file` /
> `list_directory` / `run_shell`; `aw` `fs.*`, `shell.exec` and `git.exec`;
> `browser.navigate` / `file://` navigation; Notepad file open/save/recents.

## 1. Objective

Give the user runtime control over what the agent can touch on disk and run in
the shell — the third fence: **modules** fence capability, the **base prompt**
fences behavior, **permissions** fence reach, under user control, enforced in
Go. Today aw has one all-or-nothing switch (`self-dev.enabled`) configured by
hand in `config.json`, with no UI and no granularity.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **AW2's four modes, same names, same semantics:** `block_all` (no shell; fs limited to the workspace folder), `permit_list` (default-deny; user folders + built-in allows), `deny_list` (default-allow; built-in + user denies), `permit_all` (no path/shell restriction). **Default: `permit_list`.** |
| 2 | **Built-in always-denied paths apply in EVERY mode, including `permit_all`:** `~/.ssh`, `~/.gnupg`, `~/.aws`, `/etc/shadow`, `/etc/passwd` (AW2 list), **plus aw's own data dir** (vault + `config.json`) — see Risk 1. |
| 3 | Built-in always-allowed in `permit_list`: the agent workspace folder (`~/AgentWorkspace`), `/tmp`, `/var/tmp`. (AW2 also allowed `~/.npm` / `~/.node_modules` — Node-runtime legacy, dropped.) |
| 4 | **The agent can see and test the fence, never move it:** `sandbox.status` and `sandbox.test` (dry-run) are available; `sandbox.set_mode` is registered but ALWAYS returns AW2's exact message: `Access denied. Change Permissions in the app.` |
| 5 | **Sandbox config persists in the vault** (like AW2's `_config_sandbox_mode` / `_config_indexed_folders` / `_config_sandbox_deny_list` secrets), NOT in `config.json` — a file the agent could edit to widen its own fence. Keep these keys out of `system.state` and secret listings (internal-config convention from the checkup fixes). |
| 6 | **Saving permissions requires a NATIVE confirmation dialog** (Wails runtime dialog, outside the webview DOM). aw's `ui.click` operates on the DOM — a webview dialog would let the agent approve its own permission change. AW2 used TouchID/native dialog with the message: *"Only approve this if you personally requested a Permissions change."* Port the message. |
| 7 | Enforcement lives in Go, before execution, for **both** surfaces: the always-on workspace tools (`read_file`/`write_file`/`list_directory`, `run_shell`) and the `aw` actions (`fs.*`, `shell.exec`, `git.exec`). One checker, two call sites. The prompt only *informs* the mode (§6). **"Two call sites" is the list today, not a closed definition:** any module action that takes or produces a disk path (e.g. the browser module — screenshot-to-file, downloads, `file://` URLs) must call the same checker. See Risk 7. |
| 8 | The existing per-write **user confirmation flow stays** (`tool:confirm` → `ToolConfirmDialog`). Sandbox decides *whether the path is reachable at all*; confirmation decides *whether this write happens*. Different questions, both kept. |
| 9 | Self-dev integration: self-dev ON keeps behaving as today (repo-rooted, `Unconfined`) but is **reframed as the sandbox in `permit_all`** with the repo root as workspace — and the built-in denies of Decision 2 now apply even there. `Options.Unconfined` is replaced by the sandbox checker (see §5). |
| 10 | UI = a **Permissions page in Settings** (pattern: `UserMemoryPage`/`SecurityPage`), not a workspace module. Deviation from AW2 (which had a Permissions module): aw's modules spec keeps settings outside the module system. Promoting it to a module card later is purely cosmetic. |

## 3. What already exists — reuse, don't reinvent

**In aw:**
- Path confinement: `workspace.resolve()` (`tools.go:451-474`) — rejects
  absolute paths / `..` escapes against `Options.Root` unless `Unconfined`.
  The sandbox checker generalizes this; `resolve()` becomes the place that
  calls it.
- Confirmation plumbing: `ConfirmFunc` / `ConfirmRequest` (`tools.go:24-32`),
  `confirmToolAction` + `ResolveToolConfirmation` (`app.go:512-562`),
  `ToolConfirmDialog.tsx`. Untouched by this spec (Decision 8).
- Gates wiring: `toolOptions()` in `app.go:133-218` maps `SelfDevConfig` →
  `Options{Root, Unconfined, AllowShell, AutoApprove, SelfManage}`. The
  sandbox config joins this mapping.
- Registry rebuilt per dispatch (`awRegistry()`), so sandbox actions and
  mode changes take effect without restart — the modules work already proved
  this path.
- Vault secrets with internal-config convention (hidden from listings);
  `user_memory`-style port + use case + tests as the shape to copy.
- Settings page pattern: `frontend/src/modules/settings/pages/` +
  `services/*.service.ts` + registration in `SettingsModule.tsx`.

**From AW2 (port, with tests):**
- `isPathAllowed(path, config)` — the mode decision table
  (`path-validator.ts:105-133`).
- `extractPaths(command)` — redirect targets (`>`, `>>`, `2>`), `curl -o` /
  `wget -O`, and any token looking like a path (`/`, `~/`, `./`, `../`),
  quote-aware (`path-validator.ts:141-172`).
- `sandboxExec` flow (`sandbox-shell.ts:34-102`): block_all → refuse;
  extract paths; **no paths found in `permit_list` → deny unless the command
  is in the safe list** (`date echo false id printf pwd true uname whoami`,
  and refuse if the command contains shell operators `; | & < > $ ( ) \``);
  any disallowed path → refuse with `blockedPaths` + reason; else execute.
- Symlink/traversal defense (`sandbox-fs.ts:90-116`): normalize, find
  existing ancestor, `realpath` it, re-check the resolved path. Port the
  symlink-escape and `..`-traversal test cases verbatim.
- The per-mode prompt templates (§6) and the UI structure (§7).

## 4. Architecture by layer

`interface → application → domain`, `infrastructure → domain`.

- `internal/domain`: `SandboxMode` + `SandboxConfig{Mode, AllowedFolders,
  DenyList}`; `SandboxConfigStore` port (load/save via vault).
- `internal/application`: use cases `GetSandboxConfig`, `SetSandboxConfig`
  (validate mode, normalize folder entries), `SandboxInstruction(config)`
  (the prompt block, §6), `TestSandboxCommand` (dry-run result struct).
- `internal/infrastructure/sandbox` (new package): the pure checker —
  `IsPathAllowed`, `ExtractPaths`, `ResolveAndCheck` (realpath walk),
  built-in lists. No vault/tools imports; heavily unit-tested (port the AW2
  suites).
- `internal/infrastructure/vault`: the three config entries (internal
  convention, hidden from secret listings).
- `internal/infrastructure/tools`: `Options.Sandbox` (a small interface or
  injected funcs) consulted by `resolve()` and `runShell()`; `sandbox.*`
  actions in a new `aw_sandbox.go`.
- `app.go` + dto: Wails methods `GetSandboxSettings` / `SaveSandboxSettings`
  (native confirm inside Save); compose the sandbox block in
  `RefreshAgentContext`.

## 5. Enforcement points

1. `workspace.resolve()` — after today's relative-path resolution, run
   `sandbox.ResolveAndCheck(absPath, config)`. `Unconfined` is removed as a
   bypass: self-dev passes a `permit_all` config instead (Decision 9), so
   built-in denies hold everywhere.
2. `workspace.runShell()` (used by `run_shell`, `shell.exec`, `git.exec`) —
   the `sandboxExec` flow before spawning: mode gate → `ExtractPaths` →
   per-path check → safe-no-path-command rule. Blocked result returns the
   AW2-style reason string so the model can relay it honestly.
3. `fs.read` / `fs.write` / `fs.list` go through the same `resolve()` — no
   second code path.
4. Blocked operations are not errors to hide: return
   `blocked: true, reason: ...` so `aw` results stay structured.

## 6. Prompt block (compose in `RefreshAgentContext`)

`SandboxInstruction(config)` produces a `### Filesystem Access` block — port
the AW2 templates (`build-state-context.ts:477-546`):

- `block_all`: warn that shell is completely disabled, do NOT retry, tell the
  user to change Permissions in Settings if asked; reads/writes still work in
  the workspace folder.
- `permit_list`: "You can ONLY run shell commands that touch files within the
  allowed folders listed below, ~/AgentWorkspace, and /tmp. Commands accessing
  any other paths will be blocked." + the allowed-folders list.
- `deny_list`: everything allowed except the built-in sensitive paths + the
  user's deny list (name them).
- `permit_all`: one line, "no shell/path restrictions" (built-in denies still
  enforced silently).

Compose-don't-clobber: this is one more block alongside user-memory and
chat-memory in the single `SetMemoryContext` call.

## 7. Phases

### Phase 1 — Checker + config (domain, sandbox pkg, vault)
1. Domain types + `SandboxConfigStore` port; vault persistence (3 internal
   entries, hidden from listings); default `permit_list` with empty lists.
2. `internal/infrastructure/sandbox`: `IsPathAllowed`, `ExtractPaths`,
   `ResolveAndCheck`, built-in lists + safe-command list. Port AW2 test
   cases: mode table, `..` traversal, symlink escape, always-denied wins over
   broad allows, quote-aware extraction, redirect/curl targets.
3. Application use cases (`Get/SetSandboxConfig`, validation).
**Accept:** unit tests prove AW2 parity on the ported cases; config survives
vault lock/unlock; sandbox keys invisible in secret listings.

### Phase 2 — Enforcement (tools)
1. `Options.Sandbox` wired from `toolOptions()`; `resolve()` consults it;
   `Unconfined` removed (self-dev → `permit_all` + repo root, Decision 9).
2. `runShell()` gets the `sandboxExec` flow; blocked results carry reason +
   blocked paths.
3. Tests: each mode against `read_file`/`write_file`/`run_shell`/`fs.*`;
   self-dev still works end-to-end; built-in denies hold in `permit_all`.
**Accept:** in `permit_list` with no folders, `cat ~/Documents/x` is blocked
with a clear reason; `echo hi` runs; `cat ~/.ssh/id_rsa` is blocked in every
mode.

### Phase 3 — Agent surface (actions + prompt)
1. `aw_sandbox.go`: `sandbox.status` (mode + lists, AW2 format),
   `sandbox.test` (dry-run: result/mode/paths detected/blocked/reason),
   `sandbox.set_mode` (always the denied message — register it so the agent
   gets a deterministic refusal instead of "unknown action").
2. `SandboxInstruction` block composed in `RefreshAgentContext` (§6).
3. `docs/SELFCODE.md`: document the three actions + the rule that
   permissions only change in Settings.
**Accept:** agent can answer "what can you touch?" from `sandbox.status`,
predict a block with `sandbox.test`, and a `sandbox.set_mode` attempt returns
the denied message.

### Phase 4 — Permissions page (Settings UI)
1. Wails methods `GetSandboxSettings` / `SaveSandboxSettings`; Save shows a
   **native** dialog first (Decision 6) with the AW2 warning text; dto +
   bindings.
2. `PermissionsPage.tsx`: mode selector with the four AW2 descriptions
   (Block All / Permit List / Deny List / Permit All, `editable` flags as in
   AW2's `permissions.constants.ts`); conditional path-list editor
   (allowed folders in `permit_list`, denied paths in `deny_list`) with
   built-in entries shown as non-removable badges; a "Folders visible to the
   agent" summary (port `getSandboxRoots`). Register in `SettingsModule.tsx`.
3. Tests: service mapping; page renders modes and edits lists; vitest for the
   conditional editors.
**Accept:** user switches mode in Settings → native confirm → next chat's
prompt block and `sandbox.status` reflect it without restart; agent driving
`ui.click` cannot complete a save (native dialog is out of DOM reach).

## 8. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate            # lint + go test + wails build
cd frontend && npm run build:frontend   # phase 4
```

`internal/architecture` tests stay green. Update `docs/SELFCODE.md` in Phase 3.

## 9. Risks / attention

1. **Self-widening:** the fence must protect itself. The agent must not be
   able to edit `config.json`, the vault files, or the aw data dir via
   `fs.*`/shell in ANY mode (Decision 2), and sandbox config lives in the
   vault, not on plaintext disk (Decision 5). Test this explicitly: an
   `fs.write` to the data dir is blocked even in `permit_all` + self-dev.
2. **UI automation reaching the Permissions UI:** `ui.click`/`ui.fill` can
   drive Settings. The native dialog (Decision 6) is the actual barrier —
   never replace it with a DOM modal "for consistency".
3. **`extractPaths` is a heuristic, not a parser.** AW2 accepted this: it
   stops accidents and casual scope creep, not a determined adversary
   (obfuscated paths, `$(...)`, env tricks). Defense in depth = heuristic +
   safe-command operator rejection + built-in denies at the fs layer +
   per-write confirmation. State this honestly in code comments; do not
   advertise the shell sandbox as adversary-proof.
4. **Don't break self-dev:** it is the workflow building aw. Phase 2 must
   keep `wails dev` + self-dev fully working (repo-rooted `permit_all`); the
   only behavior change is the built-in denies.
5. **Mode-change latency:** config is read per dispatch (registry rebuild
   already does this) — no caching that survives a Settings save.
6. **No `git add -A`** (shared working copy); small commits, one per phase.
   Filenames use a hyphen, never an em dash. Backend changes need an app
   restart to load — say so in summaries.
7. **The browser module is a third surface the original two-call-site list
   did not foresee.** CDP gives the agent disk access paths that bypass
   `resolve()`/`runShell()` entirely: `browser.navigate` to a `file://` URL
   reads any file via the renderer (including always-denied ones like
   `~/.ssh/*`); navigating to a download link writes to `~/Downloads`;
   screenshot/export actions may take a destination path. Minimum bar in
   Phase 2: block `file://` (and `view-source:file://`) navigation unless
   the resolved path passes the checker, and run any path argument of a
   browser action through `ResolveAndCheck`. Downloads: deny-by-default is
   acceptable for now; note the gap in `docs/SELFCODE.md` if not closed.
   The same applies to ANY future module action touching disk (Decision 7).
