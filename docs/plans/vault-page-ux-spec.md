# Spec — Settings › Vault: honest labels + Danger zone

> **Status:** implemented (2026-06-12, commits: 1b9135c)

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Tell the truth about "Choose vault folder":** the action points the app at a different folder (an existing vault there is opened; an empty folder hosts a new vault) — it does NOT move or copy data (`application.ChooseVaultDir` just re-targets config). The UI becomes a "Vault location" block: current path displayed read-only + a "Change location…" button whose NATIVE confirmation dialog states exactly that ("This switches which vault the app opens. Your current vault stays where it is."). |
| 2 | **Danger zone:** Change password, Recovery key and Change location move into a visually distinct bottom section (destructive-tinted border + title "Danger zone", the GitHub-settings pattern), each with a one-line consequence description. The top of the page keeps the calm facts: current profile, path, lock state. |
| 3 | Destructive/irreversible confirmations are NATIVE dialogs (the Permissions-save precedent); Save/Cancel pairs use `SaveCancelActions`. |
| 4 | Copy lives in one constants block in the page file — refine wording there, never inline. |

## 3. What already exists — reuse, don't reinvent

- `ChooseVaultDir`/`ChangePassword`/recovery flows — untouched; this spec
  re-presents them.
- Native dialog pattern (`app_permissions.go`); `SaveCancelActions`.
- The `VaultPage.tsx` logic/state — keep, restyle.

## 4. Phases

### Phase 1 — Full restyle (one page)

Location block + Danger zone + native confirms + copy. Vitest: danger
actions render inside the danger section; Change location triggers the
native-dialog binding (mocked), never `window.confirm`.

## 5. Gates

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./... && go test ./... && go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 6. Risks / attention

1. UI-only: if any backend change feels needed, stop and flag — switching
   vaults is lock/profile-sensitive territory.
2. **No `git add -A`**; small commits. Backend changes (none expected) need
   an app restart.
