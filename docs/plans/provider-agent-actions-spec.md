# Spec — Full provider management as `aw` actions for the agent

> **Status:** implemented (2026-07-01) — 7 new `provider.*` actions added; docs/prompt mirrors + drift tests updated; browser auth made non-blocking. Go + frontend gates green.

## 1. Objective

Let the agent fully manage LLM providers through the token-authenticated `aw`
channel: create, rename, delete, reorder (priority), change API key/model/base
URL, activate, start web sign-in, and read balance. Previously the agent had
only `status`/`switch`/`test`/`config.set`/`credential.delete`.

## 2. Principle (authorization model)

The `aw` channel is authenticated (bearer token + unlocked vault) and is a
separate surface from the UI. Following the skill-write-actions precedent and the
existing `provider.credential.delete`, provider write actions run through the
**application layer directly, with no native dialog** (the UI's native dialogs
guard the DOM-driving automation path, not this token-authenticated one). This is
an explicit decision by the owner: the agent owns its own LLM config.

**Invariants kept:** no action returns key material; action args are never logged
by value (the dispatcher logs only key names + byte counts, `aw_registry.go`).

## 3. New actions

| Action | Args | Backend |
|---|---|---|
| `provider.create` | `{ name? }` | `App.CreateCustomProvider` → `application.CreateCustomProvider` |
| `provider.rename` | `{ provider, name }` | `App.RenameCustomProvider` |
| `provider.delete` | `{ provider }` | `App.DeleteCustomProviderConfirmed` (no dialog; drops from fallback order) |
| `provider.order.set` | `{ order: [id,...] }` | `App.SetProviderFallbackOrder` |
| `provider.order.move` | `{ provider, direction: up\|down }` | read+swap+`SetProviderFallbackOrder` (adapter) |
| `provider.auth.start` | `{ provider, model? }` | `App.StartProviderBrowserAuthAsync` (new, non-blocking) |
| `provider.balance` | `{ provider }` | `App.GetProviderBalance` |

Existing (now documented): `provider.config.set` (= change API key/model/base
URL + activate), `provider.credential.delete`.

### Non-blocking browser auth
`App.StartProviderBrowserAuth` blocks up to 10 min. New
`App.StartProviderBrowserAuthAsync` (appcore/app.go) runs the flow in a goroutine
and returns as soon as the first user-facing detail is ready: `{ started,
deviceCode?, verificationUri?, message }` (GitHub device code / OpenAI browser
URL), or a generic "started" after a 30s cap. The agent relays the code/URL to
the user and polls `provider.status` (`connected`) for completion.

### Order semantics
The real failover priority is `ProviderFallbackOrder` (config.json), not the
Settings UI "configured order" (browser localStorage only). The agent controls
the former. `provider.order.move` appends an unlisted provider before moving it.

## 4. Implementation

- Handlers: `internal/infrastructure/tools/aw_app.go` (`registerAppActions`), args
  via `awRequiredStringArg`/`awStringArg`/`awStringSliceArg` (reused from
  aw_diagnostics.go).
- Port: `AppControl` interface (aw_app.go) — 7 new methods.
- Adapter: `internal/appcore/app_control.go` (`appControl`), delegating to App
  bindings / `application.*`, never the native-dialog variant.
- `internal/appcore/app.go`: new `StartProviderBrowserAuthAsync`.
- Docs/mirrors: `docs/SELFCODE.md` (catalog + cage), prompt strings in
  `internal/infrastructure/tools/tools.go` and
  `internal/infrastructure/agent/runtime.go`.

## 5. Testing

- `internal/infrastructure/tools/aw_app_test.go`: `fakeControl` gains the 7
  methods; `TestAwProviderActionsDoNotIncludeSecretReaders` allowlist updated;
  new `TestAwProviderManagementActionsRoute` (routing + required-arg + bad
  direction + non-string order validation).
- Application-layer create/rename/delete already covered by
  `internal/application/custom_provider_test.go`.
- Gate: `go run ./tools/buildgate` + `cd frontend && npm run build:frontend`.

## 6. Verification (REST, read/write)
`POST http://127.0.0.1:9301/api/aw` with bearer: `provider.create` →
`provider.config.set` → `provider.rename` → `provider.order.set`/`move` →
`provider.switch` → `provider.auth.start` (complete in browser) → `provider.delete`.
