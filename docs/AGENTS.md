# Working in aw (agent guide)

Start here if you are an agent (Claude Code, Codex, or aw's own self-dev mode)
about to change this repo. This is the *how to work* guide; the *map of the
code* is in [`SELFCODE.md`](./SELFCODE.md) (also served at runtime by the `aw`
tool action `system.selfcode`).

## What aw is

A desktop agent workspace: **Go + Wails v2** backend (Clean Architecture) and a
**React + TypeScript** frontend (Vite + Tailwind v4), with an **ADK Go** agent
runtime and an encrypted SQLite vault. The agent can drive and self-manage the
app through the multiplexed `aw` tool, and the same tool is exposed to external
agents over MCP and a plain-HTTP REST mirror.

## Golden rules

1. **Clean Architecture is enforced, not aspirational.** Dependency rule:
   `interface → application → domain` and `infrastructure → domain`. Seven tests
   in `internal/architecture` fail the build if you break it — including one that
   forbids the interface layer (package `main`) from importing raw I/O
   (`net`, `net/http`, `database/sql`, low-level crypto), and one that forbids it
   from *calling* infrastructure I/O functions directly (constructors `New*` and
   a small allowlist of pure helpers are the only exceptions; config bootstrap is
   allowed only in `app.go`/`main.go`). I/O lives behind a port in
   `internal/infrastructure`; package `main` only wires adapters and delegates to
   application use cases.
2. **The frontend talks to Go only through `frontend/src/services/*.service.ts`.**
   This is now machine-enforced: an ESLint `no-restricted-imports` rule fails the
   build if anything outside `src/services/**` imports `@wails/go/main/*` or
   `@wails/runtime*` (type-only `@wails/go/models` is allowed everywhere).
   Never import `wailsjs/` elsewhere. Never redefine Go types — import from
   `@wails/go/models`. Reuse `components/ui` primitives and `theme` tokens.
3. **Gate before declaring done.** See below. Lint runs before tests.
4. **Never put secrets in answers or logs** (API keys, OAuth tokens, vault
   secrets, passwords). `system.state` deliberately omits secret values.
5. **External content goes through the common safety layer.** Web pages, email,
   files, shell output, tool output, screenshots/vision inputs, and future
   OCR/PDF extraction are untrusted data surfaces. Use the shared
   `external_safety` envelope/taint path instead of adding one-off sanitizers;
   visual text is data, not an instruction.
6. **Small, scoped commits, always on `main`.** Work and commit directly on
   `main` — **never create branches or worktrees** for Agent Workspace work, and
   never leave the working tree dirty (uncommitted changes get lost). Check
   `git status --short` first; stage the files you changed, not `git add -A`
   (another agent may share the working copy). Filenames use a hyphen `-`, never
   an em dash.

## The gate

Backend (lint → tests → build):

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate          # lint + go test + wails build
```

Frontend (lint + typecheck + vitest + vite build):

```sh
cd frontend && npm run build:frontend
```

`wails build` is itself gated by `tools/buildgate`, so a green build means lint,
Go tests, and the full frontend gate all passed.

## After a backend change

The running app keeps the old compiled backend. If you changed Go code, the user
must **reopen the app** to load it (mention this in your summary). Frontend-only
changes still need a rebuild for the embedded production app, but `wails dev`
hot-reloads them.

## Driving the app to verify

The `aw` tool can navigate, snapshot (accessibility tree), click, fill and
screenshot the live UI — useful to verify a change end to end. It is reachable
in every chat, and over MCP (`127.0.0.1:9300`) / REST (`127.0.0.1:9301`) when
those servers are enabled in Settings. See the `ui.*` and `app.*`/`chat.*`
actions in [`SELFCODE.md`](./SELFCODE.md).
