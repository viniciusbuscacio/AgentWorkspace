package appcore

import (
	"fmt"

	"aw/internal/application"
)

const touchIDKeychainService = "AgentWorkspace-Vault"

func (a *App) touchIDAccount() string {
	view := application.BuildVaultStatusView(a.vault, a.profileManager, a.currentProfileID)
	return application.TouchIDAccountForVaultDir(view.Status.VaultDir)
}

// VaultTouchIDAvailable reports whether macOS biometric auth can be prompted.
// It is intentionally optional: password unlock remains the canonical path.
func (a *App) VaultTouchIDAvailable() bool {
	return touchIDAvailableNative()
}

// VaultTouchIDHasPassword reports whether the currently selected vault/profile
// has a stored credential. Credentials are scoped by the resolved vault path.
func (a *App) VaultTouchIDHasPassword() bool {
	if !touchIDAvailableNative() {
		return false
	}
	return touchIDKeychainHas(touchIDKeychainService, a.touchIDAccount())
}

// VaultTouchIDEnroll stores the successfully-entered vault password in the
// local Keychain for this vault only. The frontend calls this after a normal
// password unlock/create, matching AW2's optional Touch ID flow.
func (a *App) VaultTouchIDEnroll(password string) OperationResult {
	if password == "" {
		return a.basicOperationResult(fmt.Errorf("password is required"))
	}
	if !touchIDAvailableNative() {
		return a.basicOperationResult(fmt.Errorf("Touch ID not available"))
	}
	err := touchIDKeychainSet(touchIDKeychainService, a.touchIDAccount(), password)
	return a.basicOperationResult(err)
}

// VaultTouchIDUnlock prompts Touch ID, reads the scoped password from Keychain,
// and then routes through the same unlock lifecycle as a normal password unlock.
func (a *App) VaultTouchIDUnlock() OperationResult {
	if !touchIDAvailableNative() {
		return a.basicOperationResult(fmt.Errorf("Touch ID not available"))
	}
	account := a.touchIDAccount()
	if !touchIDKeychainHas(touchIDKeychainService, account) {
		return a.basicOperationResult(fmt.Errorf("No stored Touch ID credential for this vault"))
	}
	if err := touchIDPromptNative("unlock Agent Workspace vault"); err != nil {
		return a.basicOperationResult(err)
	}
	password, err := touchIDKeychainGet(touchIDKeychainService, account)
	if err != nil {
		return a.basicOperationResult(err)
	}
	return a.UnlockVault(password)
}

// VaultTouchIDRemove removes the scoped credential for the selected vault.
func (a *App) VaultTouchIDRemove() OperationResult {
	err := touchIDKeychainDelete(touchIDKeychainService, a.touchIDAccount())
	return a.basicOperationResult(err)
}

// touchIDAutoRepair re-owns the stored credential with the current app binary
// after a successful unlock. macOS pins silent Keychain access to the exact
// binary (cdhash partition ID) that created the item — a locally signed dev
// build gets a new cdhash on every rebuild, so without this rewrite macOS asks
// for the login-keychain password on every unlock after a rebuild, and
// "Always Allow" never sticks. Rewriting (delete + add) needs no read, so it is
// prompt-free; it also keeps the stored password current if it changed.
func (a *App) touchIDAutoRepair(password string) {
	if password == "" || !touchIDAvailableNative() {
		return
	}
	account := a.touchIDAccount()
	if !touchIDKeychainHas(touchIDKeychainService, account) {
		return
	}
	_ = touchIDKeychainSet(touchIDKeychainService, account, password)
}

// VaultTouchIDRepair re-writes the stored credential as a fresh Keychain item
// owned by the CURRENT app binary. Items created before the signing
// certificate was trusted carry access grants that never validate, so macOS
// re-prompts on every unlock; deleting and re-adding the item makes the
// running (trusted) app its owner, which reads without prompting from then
// on. Reading the old item may trigger one final macOS permission dialog.
func (a *App) VaultTouchIDRepair() OperationResult {
	account := a.touchIDAccount()
	if !touchIDKeychainHas(touchIDKeychainService, account) {
		return a.basicOperationResult(fmt.Errorf("No stored Touch ID credential for this vault"))
	}
	password, err := touchIDKeychainGet(touchIDKeychainService, account)
	if err != nil {
		return a.basicOperationResult(err)
	}
	return a.basicOperationResult(touchIDKeychainSet(touchIDKeychainService, account, password))
}
