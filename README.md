# Agent Workspace (aw)

Agent Workspace is a desktop agent workspace built with **Wails v2** (Go backend + web
frontend) and an **ADK Go** agent runtime. An encrypted SQLite vault stores
chats, messages, secrets, profiles and the agent's long-term memory about the
user (curated in Settings → Memory). The agent can drive and self-manage the
app through a single multiplexed `aw` tool, and that same tool is exposed to
external agents over **MCP** and a plain-HTTP **REST** mirror.

- **Backend:** Go, Clean Architecture (`internal/domain`, `internal/application`,
  `internal/infrastructure`, `internal/dto`), with the dependency rule enforced
  by tests in `internal/architecture`.
- **Frontend:** React + TypeScript (Vite 8 + Tailwind v4).
- **Agent surface:** the `aw` tool — app/chat control, self-management, and raw
  UI automation (accessibility-tree snapshot, click, fill, screenshot).

Project settings live in `wails.json`
([Wails project config](https://wails.io/docs/reference/project-config)).

## Develop

```sh
wails dev
```

Runs a Vite dev server with hot reload. A Go-methods dev server is also exposed
at http://localhost:34115 (open it in a browser to call Go from devtools).

## Build

### Windows

From the repo root:

```powershell
.\build-windows.cmd
```

Useful options:

```powershell
.\build-windows.cmd -Pull          # fast-forward main from GitHub before building
.\build-windows.cmd -SkipTests     # quicker local build, skips Go lint/tests
.\build-windows.cmd -StopRunning   # stop a running Agent Workspace before building
.\build-windows.cmd -Run           # start the freshly built app
.\build-windows.cmd -OpenFolder    # open build\bin after the build
```

Output:

```text
build\bin\Agent Workspace.exe
```

### Generic

```sh
wails build
```

Produces a redistributable production package. The build is **gated**: it only
compiles if Go lint, Go tests, frontend lint, typecheck, frontend tests and the
Vite build all pass.

## Regression gate

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
go run ./tools/buildgate            # golangci-lint + go test + wails build -trimpath
go run ./tools/buildgate --skip-build   # faster: lint + tests only
```

The frontend portion runs via `npm run build:frontend` (ESLint, TypeScript,
Vitest, Vite).

## Lint

```sh
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run ./...          # backend
cd frontend && npm run lint      # frontend
```

## Documentation

- [`docs/AGENTS.md`](docs/AGENTS.md) — start here if you are an agent working on
  this repo: the golden rules, the gate, and how to drive the app to verify.
- [`docs/SELFCODE.md`](docs/SELFCODE.md) — the map of the codebase and the `aw`
  tool actions (also served at runtime via `aw` → `system.selfcode`).
