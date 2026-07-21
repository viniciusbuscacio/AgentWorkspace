# Spec — LLM Providers page: test, balance, honest buttons, quick toggle

> **Status:** implemented (2026-06-12, commits: 9785272, dacb2b3)

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Provider detail buttons become:** `SaveCancelActions` (Save + Cancel), an **Active toggle** (ON = this provider becomes the active one), and **Remove credential** (destructive variant, native-dialog confirmation — it deletes a secret). "Save & activate" dies: saving a credential and choosing the active provider are different decisions. |
| 2 | **Active is radio-semantics**: exactly one provider is active. Turning a provider ON deactivates the previous one (the existing `provider.switch` semantics). Turning the active provider OFF directly is refused with the hint "Activate another provider instead" — chat always has a provider. |
| 3 | **The provider LIST gets the same Active toggle per row** (item 6) — one component, used in both places. Rows of unconfigured providers show the toggle disabled with a "Configure first" tooltip. |
| 4 | **"Test" button on the provider detail**: one-shot minimal completion ("Reply with OK") through the EXISTING runtime config resolution for that provider (not the active one), 15s timeout, result inline: latency + model echoed, or the provider's error verbatim. No new adapter — reuse what `chat` uses. |
| 5 | **Balance for OpenRouter — with the key the user already has**: OpenRouter exposes credit/usage to the inference key (`GET /api/v1/credits`, fallback `GET /api/v1/auth/key`). Show "Balance: $X.XX used / $Y limit" on the OpenRouter detail; HIDE the row on any error and for providers without such an endpoint. **No management/provisioning key** — never ask the user for a second, more powerful credential just to render a number (it can create/delete keys; the risk is not worth a label). |
| 6 | Balance/test calls go through the backend (Go) like every provider call — the key never reaches the frontend; results carry no key material (the provider-secrets anti-regression tests stay green). |

## 3. What already exists — reuse, don't reinvent

- `provider.switch` / activation logic and the `ProvidersPage.tsx` state.
- Runtime config resolution per provider (`internal/infrastructure/providers`)
  for the Test call.
- Native-dialog pattern for the destructive Remove (`app_permissions.go`).
- The Radix `Select`/Switch primitives; `SaveCancelActions` from its spec.

## 4. Phases

### Phase 1 — Buttons + toggles (items 5b, 6)

Detail buttons (Decision 1), radio toggle in detail + list, Remove with
native confirm. Vitest: toggle switches active; OFF on active refused with
hint; Remove asks natively (mocked binding).

### Phase 2 — Test + balance (items 5a)

Backend test endpoint (per-provider one-shot, timeout) + OpenRouter
balance fetch; UI rows. Go tests: test uses the named provider's config;
balance hidden on non-200; no key in any result payload.

## 5. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. The Test button fires a real (paid) completion — keep the prompt tiny,
   one call per click, busy state while pending.
2. Verify the OpenRouter endpoints against current docs at implementation
   time; if neither works with the inference key anymore, drop the row (do
   NOT fall back to asking for a management key — Decision 5).
3. Removing the active provider's credential leaves chat without a working
   provider — after Remove, if it was active, activate nothing and surface
   the existing "configure a provider" empty state, not a crash.
4. **No `git add -A`**; small commits, one per phase. Backend changes need
   an app restart.
