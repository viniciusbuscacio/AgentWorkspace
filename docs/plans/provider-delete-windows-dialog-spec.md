# Spec — Native confirmation dialogs are always canceled on Windows

> **Status:** implemented (2026-07-01) — `dialogAffirmative` helper added; the 4 affected native dialogs now accept the Windows "Yes"; provider delete confirms with an in-app toast and returns to the list. Go + frontend gates green.

## 1. Objective

Fix "Delete provider" (and the other native confirmation dialogs) on Windows,
where confirming did nothing: the provider was not deleted, the detail panel did
not close, and no feedback appeared. Also give the delete a visible in-app
confirmation ("<name> deleted") via the new toast foundation.

## 2. Root cause

On Windows, Wails v2.12.0 `wailsruntime.MessageDialog` **ignores the custom
`Buttons`** and maps a `QuestionDialog` to the Win32 `MB_YESNO`, returning the
mapped strings **"Yes"/"No"** — not the custom label.

- `…/wails/v2@v2.12.0/internal/frontend/desktop/windows/dialog.go:168` → `flags = windows.MB_YESNO`
- `:196` → `responses := []string{"", "Ok", "Cancel", …, "Yes", "No", …}` (clicking the affirmative returns `"Yes"`)

The backend compared the return against the custom label, e.g.:

```go
if strings.ToLower(strings.TrimSpace(choice)) != "delete" { // choice is "Yes" on Windows
    return application.ProviderOperationResult{Canceled: true}
}
```

So on Windows every confirm returned `Canceled: true`. The frontend
(`ProvidersPage.tsx` `afterDeleteSlot`) does `if (result.canceled) return;` → no
delete, no navigation, no alert. Confirmed live via `provider.status`: the
built-in hideable `custom-openai` slot stayed in the list after the user's
delete attempt.

The permissions dialog already had the correct Windows fallback
(`app_permissions.go` accepted `"yes"`/`"ok"`); the other dialogs did not.

## 3. Affected dialogs (all `QuestionDialog`, all broken on Windows)

| Handler | File | Label |
|---|---|---|
| Delete provider (reported) | `internal/appcore/app.go` `DeleteCustomProvider` | `"delete"` |
| Remove credential | `internal/appcore/app.go` `DeleteProviderCredential` | `"remove"` |
| Change vault location | `internal/appcore/app.go` `ChooseVaultDir` | `"change location"` |
| Delete file (File Explorer) | `internal/appcore/app_explorer.go` `ExplorerDelete` | `"delete"` |

## 4. Fix

### Backend
New helper (composition root, string-only, no I/O):

```go
func dialogAffirmative(choice, label string) bool {
	answer := strings.ToLower(strings.TrimSpace(choice))
	return answer == strings.ToLower(label) || answer == "yes"
}
```

The 4 sites call `if !dialogAffirmative(choice, "<Label>") { …declined… }`.
`app_explorer.go` dropped its now-unused `strings` import. Covered by
`TestDialogAffirmative` in `internal/appcore/app_test.go`.

### Frontend (`frontend/src/modules/settings/pages/ProvidersPage.tsx`)
`afterDeleteSlot(result, provider)` now:
- On error: keep the panel open and show the error in-panel (`setMessage`).
- On success: clear editing state (returns to the list) and
  `toast.success(`${provider.name} deleted`)` — the previous in-panel notice
  was never visible because the detail panel unmounts on navigation.

Uses the Sonner toast foundation (see `in-app-toast-framework-spec.md`).

### Follow-up decision: provider-slot delete uses the themed modal on all platforms
Deleting a custom provider slot is low-stakes and something the agent may
legitimately do, so it is **not** a "cage" action. `deleteCustomProviderSlot`
now always opens the themed in-app confirmation (Radix `AlertDialog`, follows the
theme) and `confirmDelete` calls the no-dialog `DeleteCustomProviderConfirmed`
variant — on desktop **and** web. This also sidesteps the Windows native-dialog
bug for provider delete entirely. (`pendingWebDelete`/`confirmWebDelete` were
renamed to `pendingDelete`/`confirmDelete` since they are no longer web-only.)

The remaining native confirmations keep the native dialog **and** the
`dialogAffirmative` fix, because they are documented cage items (agent must not
self-approve): **Remove credential** (desktop), **Change vault location**,
**Delete file** (File Explorer). See `docs/SELFCODE.md` "the cage".

## 5. Testing

- Go: `TestDialogAffirmative` — custom label, whitespace/case, Windows `"Yes"`,
  and declines (`"No"`/`"Cancel"`/empty).
- Frontend: `ProvidersPage.test.tsx` "Delete provider … returns to list" now
  asserts the detail panel unmounts and `toast.success('Maritaca deleted')`
  fires.
- Gate: `go run ./tools/buildgate --skip-build` and `cd frontend && npm run build:frontend`.

## 6. Notes / future

- UX nicety (not fixed): on Windows the native dialog defaults focus to the
  affirmative button. Wails only honors `DefaultButton` when it is `"No"`
  (`dialog.go:170`), so setting `DefaultButton: "No"` would make Cancel the
  default. Deferred.
- Backend change requires reopening the app to load the new binary.
- Remaining `setMessage`/`notifyService` sites can migrate to toasts over time.
