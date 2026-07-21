# Spec — Polish: the three verified loose ends

> **Status:** implemented (2026-06-12, commit 9540da0 — salvaged). Closes the three gaps
> found by the 2026-06-11 code verification of the implemented specs. Same
> conventions, same gates as the spec series. References:
>
> - `docs/plans/sidebar-modules-aw2-port-spec.md` Decision 10 (the omitted
>   Backlog auto-spec) and Decision 9 (the 500ms debounce).
> - `docs/plans/provider-secrets-hardening-spec.md` documented follow-up
>   ("renovação no adapter OAuth com persistência do credential atualizado
>   no vault por uma porta explícita").
> - AW2 reference for the Backlog chat hand-off:
>   `AgentWorkspace2/src/renderer/modules/backlog/BacklogModule.tsx`
>   (`onSpecChat`) — copy the prompt wording.

## 1. Objective

Close the only three verified divergences between the implemented specs and
the code, all of them user-facing: the missing "Create spec in chat" on
Backlog items, the Notes autosave debounce drift, and the OAuth token expiry
that forces a re-login.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Backlog "Create spec in chat"**: a button on the item detail view that creates a new chat and sends one message built from the item (title + body), using the existing frontend chat service (`createChat` + the normal send path) — the user lands in the chat and watches the reply stream normally. Port the prompt wording from AW2's `onSpecChat`. |
| 2 | AW2's second pattern — the embedded streaming auto-spec (`BacklogAutoSpecChunk`, spec streamed into the Backlog UI) — is **NOT ported**. Documented deviation: aw hands off to the chat it already has; an embedded streaming panel duplicates the chat surface for no user gain. |
| 3 | **Notes autosave debounce: 500ms** (today 700ms in `frontend/src/modules/notes/NotesModule.tsx:93`) — match the sidebar spec and AW2. |
| 4 | **OAuth refresh**: the stored `refresh_token` is used to renew the access token (on expiry or 401) instead of forcing re-login. The renewed credential is persisted to the vault through an explicit port (no direct vault calls from the adapter); the token value never appears in logs, tool results or events. Re-login remains the fallback when refresh fails. |
| 5 | The provider-secrets anti-regression tests (`json:"-"` on APIKey/Credential, no tool exposes values) must stay green — the refresh path adds no new exposure surface. |

## 3. What already exists — reuse, don't reinvent

- Chat creation/send: `frontend/src/services/chat.service.ts` (`createChat`,
  the ChatModule send path) — do not invent a parallel send.
- OAuth flow: `internal/infrastructure/oauth` (PKCE, token exchange) behind
  `ports.ProviderBrowserAuthenticator`; credential persistence pattern in
  `application.AuthenticateProviderViaBrowser`.
- Secret scrubbing and the no-token-in-logs conventions from
  `provider-secrets-hardening-spec.md`.

## 4. Phases

### Phase 1 — Backlog chat hand-off + Notes debounce (frontend only)

Button "Create spec in chat" on the Backlog detail view (Decision 1);
debounce constant to 500ms. Vitest: button creates a chat and navigates;
debounce pinned by a timer test if the harness allows, otherwise assert the
constant.

### Phase 2 — OAuth refresh (backend)

Refresh in the oauth infrastructure adapter, triggered by the runtime config
resolution when the access token is expired (and once on 401); persisted via
an explicit application use case + port. Go tests: expired token → refresh
called → new credential persisted; refresh failure → clear error that asks
for re-login; token value absent from errors/logs.

## 5. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. Phase 2 touches credential handling — run the provider-secrets
   anti-regression tests first and last; any new struct field carrying a
   token gets `json:"-"`.
2. The refresh must be single-flight (two concurrent expired calls must not
   double-refresh and clobber the stored credential).
3. **No `git add -A`** (shared working copy); small commits, one per phase.
   Backend changes need an app restart — say so in summaries.
