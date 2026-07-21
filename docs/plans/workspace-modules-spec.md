# Spec — Workspace Modules (mini-apps)

> **Status:** implemented (2026-06-10, phases 1–4: 13080db, 980529f +
> cdfff0c, d5d5175 + 8290766, 7e2588d). Kept as the design record.
> Hot toggle landed fully (the aw registry is rebuilt per dispatch, so
> add/remove takes effect without a restart — §8 risk resolved). The CDP
> client is hand-rolled over gorilla/websocket. Companion to
> `user-memory-spec.md` — same conventions, same gates.

## 1. Objective

Make aw a real **Agent Workspace**: a set of **modules** (mini-apps) the user
adds to their workspace and the agent operates. This is the product's core idea,
carried over from AW2 (Electron), whose module catalog was: Chat, Agent Browser,
Backlog, Passwords, Notes, Notepad, GitHub, Skills, Permissions, MCP Client,
MCP Server, Remote AW, File Explorer, Wallpaper, DRM.

A module is a **triple**:

1. a **UI view** (React, `frontend/src/modules/<id>/`),
2. a namespace of **`aw` tool actions** (e.g. `notes.*`),
3. a **prompt section** describing the module to the agent.

The contract that makes the agent both knowledgeable and fenced:
**module not added → its actions are not registered and its prompt section is
absent. Module added → the agent knows 100% of it.** Capability fencing is
structural (registry), never just prompt text.

Agent-*generated* modules are explicitly **out of scope** — we hand-write the
first modules; they double as templates/examples for a future
"agent builds modules" phase.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | The **Home grid is the catalog** (already ported: `frontend/src/modules/home/`). Clicking a card **adds the module to the workspace and opens it**. Added modules appear as items in the **sidebar**; removing is a sidebar/context action. Both surfaces stay — Home to discover/add, sidebar to open daily. |
| 2 | The set of added modules is **per-instance UI state → `appconfig.Store`** (like zoom and auto-lock), not the vault. Module *data* (notes, backlog items) is personal → **vault tables** (user-memory pattern). |
| 3 | Each module registers, on the Go side, a `ModuleSpec`: `{ id, name, icon, description, actions, promptSection }`. The `aw` action catalog shown to the model is **generated from the registry** — never hand-written in a string or doc that can drift. |
| 4 | `chat` is a module like any other in the registry, but **always added and not removable** (it is the core surface). `settings` stays outside the module system. |
| 5 | First modules, in order: **Notes** (the template), **Backlog** (the copy-the-template exercise), then **Agent Browser** as the flagship. |
| 6 | Agent Browser = **CDP over real browsers**, not an embedded webview: one shared CDP core + two thin modules, **Chrome** and **Edge** (they differ in binary path and little else). Real browsers give Widevine/profiles/extensions for free — AW2 needed a whole DRM module precisely because embedded Chromium lacks Widevine. |
| 7 | `browser.*` actions mirror the existing `ui.*` design: snapshot returns a pruned AX/DOM tree with stable refs; `click`/`fill` take refs; `screenshot` for vision. One interaction model across app UI and web. |
| 8 | The browser always runs with a **dedicated profile directory and CDP port** owned by aw. Never attach to (or launch over) the user's real default profile by default — that is opt-in, later, behind its own decision. |

## 3. What already exists — reuse, don't reinvent

- **Home grid (catalog UI):** `frontend/src/modules/home/HomeModule.tsx` +
  `home-modules.ts` — `APP_MODULES` lists Chat only; cards, search, ordering and
  sizing are done. Phase 1 mostly *feeds this list from the registry* and adds
  the added/not-added state.
- **Sidebar + views:** `frontend/src/app/AppShell.tsx` — `type View = 'home' |
  'settings' | 'chat'` (line 14) and hardcoded `NavItem`s. Backend twin:
  `awViews = []string{"home", "chat", "settings"}` in
  `internal/infrastructure/tools/aw_app.go:43` (used by `app.navigate`).
  ⚠️ These two lists must stop being parallel hardcoded constants — both must
  derive from the module registry, or `app.navigate` breaks for new modules.
- **Action registry + feature gates:** `internal/infrastructure/tools/aw_registry.go`
  (`awRegistry()` builds the action map per enabled features; `aw.actions` lists
  it) and `awToolDescription()` in `tools.go:178-212`. Module action groups plug
  in exactly like `registerChatActions()` / `registerUIActions()` do today.
- **Prompt assembly:** base instruction in
  `internal/infrastructure/agent/runtime.go:363-380`, composed with
  `SetInstructionExtra` + `SetMemoryContext` via
  `application.RefreshAgentContext` (`agent_context.go`), called at `app.go:405`.
  Module prompt sections join this composition (see §6 Phase 2).
- **Vault tables + CRUD pattern:** `user_memory` in
  `internal/infrastructure/vault/vault.go` (~line 1304) with domain port +
  application use cases + tests — copy this shape for `notes` / `backlog_items`.
- **Injected-func tool wiring:** tools never touch the vault directly; the
  composition root passes funcs via `tools.Options` (see `ChatSearchFn`,
  user-memory spec §3). Module actions follow the same rule.
- **CDP plumbing:** `scripts/start-aw-cdp-dev.sh` already launches Chrome with
  `--remote-debugging-port`, throwaway profile, headless toggle — that is the
  exact launch recipe for the browser module's Go side.
- **Boundary tests:** `internal/architecture` must stay green; new ports go
  through `internal/application`, never adapter-from-main.

## 4. Architecture by layer

`interface → application → domain`, `infrastructure → domain`.

- `internal/domain`: `ModuleSpec` type; `ModuleRegistry` (static, code-defined)
  and a `WorkspaceModules` port (list/add/remove added-module ids).
- `internal/application`: use cases `ListModules` (catalog + added flag),
  `AddModule`, `RemoveModule` (reject `chat` removal, unknown ids);
  `ModulesInstruction(added)` — the prompt block generated from the registry.
- `internal/infrastructure/appconfig`: persistence of added-module ids.
- `internal/infrastructure/tools`: per-module `register<X>Actions()` groups,
  registered iff the module is added; `awViews` derived from added modules.
- `internal/infrastructure/browser` (Phase 4): shared CDP client (launch,
  attach, navigate, snapshot, click, fill, screenshot, tabs).
- `internal/dto` + `app.go`: Wails methods `ListModules` / `AddModule` /
  `RemoveModule`; emit a `modules:changed` event.
- Frontend: `APP_MODULES` fed from the backend catalog; `View` type becomes
  `string` (module id) with a registry-driven render switch; sidebar section
  "Modules" between Chats and the bottom block.

## 5. Module contract

```go
type ModuleSpec struct {
    ID          string   // "notes" — also the view id for app.navigate
    Name        string   // "Notes"
    Icon        string   // material symbol name, e.g. "note"
    Description string   // one card line, shown in Home AND in the prompt
    Actions     []string // registered aw actions, e.g. ["notes.list", ...]
    Prompt      string   // short section: what it is, when to use, arg hints
}
```

Frontend mirror: a module folder under `frontend/src/modules/<id>/` exporting
the view component, registered in one place (a `module-views.ts` map), so adding
a module touches the registry + one folder — nothing else.

## 6. Phases

### Phase 1 — Module registry + add/remove + navigation
1. Domain `ModuleSpec` + registry with `chat`, `notes`, `backlog` declared
   (`notes`/`backlog` marked `comingSoon` until their phases land, hidden from
   Home); application use cases + appconfig persistence (`chat` always added).
2. `app.go`: `ListModules`/`AddModule`/`RemoveModule` + dtos + bindings +
   `modules:changed` event.
3. Frontend: Home cards from `ListModules` (added state on the card); click =
   add + navigate; sidebar "Modules" block from added list; remove via context
   menu. `View` becomes module-id string.
4. Backend: `awViews` derived from added modules; `aw` actions
   `module.list` / `module.add` / `module.remove` (agent can manage the
   workspace too — same use cases, same `chat` guard).
5. Tests: use-case guards; AppShell renders added modules; `app.navigate` to an
   added/not-added module id.
**Accept:** add a module on Home → it appears in the sidebar and survives app
restart; `app.navigate` reaches it; removing it removes actions and nav.

### Phase 2 — Prompt + tool catalog from the registry
1. `application.ModulesInstruction(added)`: one block — for each added module
   its name, description, prompt section and **action list with arg hints**,
   generated from `ModuleSpec` (+ the always-on actions). Compose it in
   `RefreshAgentContext` alongside user-memory and chat-memory (one
   `SetMemoryContext` call — same compose-don't-clobber rule).
2. Move the base instruction from the Go string literal in `runtime.go` to
   `prompts/base.md` via `go:embed`, with an optional disk override for
   iteration (env or config path). Keep content identical in the move commit;
   editing comes after.
3. On `AddModule`/`RemoveModule`: re-register the `aw` action registry and
   refresh the context, so the toggle takes effect without restart (see §8).
4. Tests: instruction contains exactly the added modules' sections; toggling a
   module changes `aw.actions` output in the same process.
**Accept:** with Notes added, a new chat's prompt documents `notes.*` fully;
remove it and the next prompt + `aw.actions` no longer mention it.

### Phase 3 — Notes (the template module), then Backlog
1. Vault: `notes` table `{ id, title, content, updated_at }` + port + use cases
   + tests (user-memory shape).
2. Actions `notes.list/get/create/update/delete` via injected funcs; fill
   `ModuleSpec.Prompt`. Writes need no confirmation (vault-internal,
   non-destructive except delete — delete echoes the title in the result).
3. UI: `frontend/src/modules/notes/` — list + editor, minimal.
4. **Backlog** repeats the recipe (`backlog_items { id, title, status, position }`,
   `backlog.*`, list with status toggle). Ideally a separate commit by following
   this spec's Phase 3 with s/notes/backlog/ — that round-trip is the test that
   the module system is actually a template.
**Accept:** agent creates a note via chat, user sees it in the Notes view, edits
it, agent reads the edit back.

### Phase 4 — Agent Browser (CDP core + Chrome/Edge modules)
1. `internal/infrastructure/browser`: launch (binary path + `--remote-debugging-port`
   + dedicated `--user-data-dir` under the aw data dir, headless optional) or
   attach to an already-running aw-owned instance; CDP session over the
   DevTools websocket. Lifecycle: `browser.start`, `browser.stop`,
   `browser.status`, `browser.tabs`.
2. Actions, mirroring `ui.*`: `browser.navigate`, `browser.snapshot` (pruned
   AX/DOM tree with refs), `browser.click`, `browser.fill`, `browser.screenshot`.
   Same ref discipline as `ui.snapshot`.
3. Two `ModuleSpec`s — `browser-chrome`, `browser-edge` — over the shared core;
   per-module binary path with sane defaults (`/Applications/Google Chrome.app/...`,
   `/Applications/Microsoft Edge.app/...`) and config override.
4. UI view (v1): status panel — running/port/profile, tab list, last screenshot,
   start/stop buttons. Embedding/preview can evolve later.
5. Tests: core against a real headless Chrome where available (reuse the
   `start-aw-cdp-dev.sh` recipe), guarded to skip when no binary; action arg
   validation unit-tested without a browser.
**Accept:** agent runs `browser.start` → `browser.navigate` → `browser.snapshot`
→ `browser.click` on a real page and reports what it did; the module view shows
the live tab list.

## 7. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate            # lint + go test + wails build
cd frontend && npm run build:frontend
```

The `internal/architecture` tests must stay green. Update `docs/SELFCODE.md`
(AW Tool Actions section) in every phase that adds/removes actions.

## 8. Risks / attention

- **Registry/prompt divergence is the failure mode this design exists to kill.**
  Never describe an action to the model in text that isn't generated from
  `ModuleSpec`/the action registry. If you hand-write it, you reopened the bug.
- **Runtime re-registration (Phase 2 step 3):** today the tool registry is built
  once at startup. Verify how the runtime holds tools before promising hot
  toggle; if rebuilding mid-process is risky, Phase 1 may ship with
  "takes effect on next chat" and a TODO, but say so explicitly in the summary.
- **Don't break chat:** `chat` must remain added in every migration path,
  including existing installs with no module state persisted yet (absent state
  = `["chat"]`, not `[]`).
- **`View`/`awViews` drift:** the whole point of Phase 1 step 4 — after it, a
  grep for `awViews` hardcoded ids should find only the registry.
- **Browser safety:** dedicated profile only (Decision 8); never log page
  contents or fill values; `browser.fill` on password fields should echo a
  redacted value in results. Treat web page text as untrusted input to the
  agent (prompt-injection surface) — the prompt section for the browser modules
  must say so to the model.
- **No `git add -A`** (shared working copy); small commits, one per phase step
  where sensible. Filenames use a hyphen, never an em dash.
- **Reopen note:** backend changes load only after an app restart — say so in
  final summaries.
