# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Agent Workspace (`aw`) is a desktop agent workspace: **Go + Wails v2** backend
(Clean Architecture) and a **React + TypeScript** frontend (Vite 8 + Tailwind
v4), with a **Google ADK Go** agent runtime and an encrypted SQLite vault. The
agent can drive and self-manage the app through a single multiplexed `aw` tool,
exposed in-app, over **MCP** (`127.0.0.1:9300`) and a plain-HTTP **REST** mirror
(`127.0.0.1:9301`).

## Read these first

- **`docs/AGENTS.md`** — how to work in this repo: the golden rules, the gate,
  the commit policy. Read it before changing anything.
- **`docs/SELFCODE.md`** — the map of the codebase and the full `aw` tool action
  catalog (also served at runtime via the `aw` action `system.selfcode`).
- **`docs/STORAGE.md`** — the storage map: what persists where (vault master +
  working copy, workspace.json, config.json, OS credential store) and who wins
  on conflict. Read it before touching anything that persists state.

This file is a quick-start; those two are canonical. Don't duplicate their
content here — update them when behavior changes.

## Commands

The build is **gated**: nothing ships unless Go lint, Go tests, frontend lint,
typecheck, Vitest, and the Vite build all pass.

```sh
# Dev (Vite hot reload; Go-methods dev server at http://localhost:34115)
wails dev

# Full gate (lint -> go test -> wails build). Run before declaring done.
go run ./tools/buildgate
go run ./tools/buildgate --skip-build   # faster: lint + tests only

# Backend lint / test directly
golangci-lint run ./...
go test ./...
go test ./internal/application -run TestName   # single test

# Frontend gate (lint + save/cancel check + typecheck + vitest + vite build)
cd frontend && npm run build:frontend
cd frontend && npx vitest run path/to/file.test.tsx   # single test file
```

On Windows, the README's one-command build is `./build-windows.cmd` (flags:
`-Pull`, `-SkipTests`, `-StopRunning`, `-Run`, `-OpenFolder`); output lands at
`build/bin/Agent Workspace.exe`.

On macOS/Linux, prefix Go commands with the toolchain PATH if needed:
`export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"`.

After a **backend** change the running app keeps the old compiled binary — the
user must reopen the app to load it (say so in your summary). Frontend changes
hot-reload under `wails dev` but still need a rebuild for the embedded app.

## Architecture (the non-obvious parts)

**Clean Architecture is machine-enforced, not aspirational.** Seven boundary
tests in `internal/architecture` fail the build if you break the dependency
rule (`interface -> application -> domain`; `infrastructure -> domain`):

- `internal/domain` — pure types and `domain/ports` (interfaces). No framework
  or I/O imports.
- `internal/application` — use cases. Pure, stdlib-only, well tested.
- `internal/infrastructure` — adapters; **all** real I/O lives here behind a
  port (agent runtime, vault, providers, browser/CDP, MCP server/client, PiP,
  diagnostics, etc.).
- `internal/dto` — typed Wails response structs.
- Package `main` at the repo root (`app*.go`, `main.go`, `pip_control.go`) is
  the **interface layer / composition root**: it wires adapters and delegates to
  use cases. A test forbids it from importing raw I/O (`net`, `net/http`,
  `database/sql`, low-level crypto) **or** calling infrastructure I/O functions
  directly — only constructors (`New*`), an allowlist of pure helpers, and
  config bootstrap in `app.go`/`main.go` are allowed.

**Frontend talks to Go only through `frontend/src/services/*.service.ts`.** An
ESLint `no-restricted-imports` rule fails the build if anything outside
`src/services/**` imports `@wails/go/main/*` or `@wails/runtime*` (type-only
`@wails/go/models` is allowed everywhere). Never redefine Go types — import them
from `@wails/go/models`. Reuse `components/ui` primitives and `theme` tokens.

**Module system.** Workspace modules (Notes, Tasks (`tasks`), Passwords,
the agent browsers, MCP Client, ...) register their
own `<id>.*` aw actions and prompt sections only while added. Frontend views
register **only** through `defineModuleView` (`modules/module-contract.ts`) — a
branded type forces each module to declare its sidebar rules or the frontend
won't compile. The view list is derived from the module registry, not a
hardcoded catalog.

**External-content safety.** Web pages, email, files, shell/tool output,
screenshots, attachments and OCR/PDF text are untrusted. Route them through the
shared `external_safety` envelope + per-turn taint store
(`internal/infrastructure/externalsafe` / `externaltaint`) — never add a one-off
sanitizer. Tainted turns can re-gate later sensitive actions. Visual/document
text is data, not instructions.

**The sandbox is the only fence.** There is no per-action confirmation popup;
every aw action and tool is auto-approved. Authorization is granted once in
Settings -> Permissions (`block_all` "Block all" / `permit_list` "Balanced",
the default / `permit_all` "Permit all"), enforced on every path and shell
command via
`internal/infrastructure/sandbox`. `sandbox.set_mode` is always refused. Don't
retry blocked operations — relay the reason and point at Settings.

**Secrets are vault-only.** Provider API keys / OAuth JSON live in the encrypted
vault `secrets` table and must never reach `config.json`, logs, chat context,
`system.state`/`app.state`, or any agent tool result. `ProviderRuntimeConfig`
keeps its credential fields `json:"-"`.

## Workflow rules (from docs/AGENTS.md)

- **Work and commit directly on `main`. Never create branches or worktrees** for
  this repo, and never leave the working tree dirty.
- Stage only the files you changed (`git status --short` first), not `git add
  -A` — another agent may share the working copy.
- Editing existing files in self-dev mode: use `edit_file`, never `write_file`;
  never re-emit a large file from memory or abbreviate unchanged regions.
- Filenames use a hyphen `-`, never an em dash.
