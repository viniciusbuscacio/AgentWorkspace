package webserver

// webDenylist is the set of *App methods that must not be dispatched over the
// web bridge. Two jobs, one list:
//
//  1. UX — these methods open a native dialog / window / OS pane on the *host*
//     machine. Over a remote browser that dialog is invisible, so the call
//     would hang or silently act on the wrong machine. Denying returns a clear
//     error instead of a frozen UI.
//  2. Headless safety (Phase 2) — these are exactly the sites that touch
//     a.ctx (the Wails runtime) without a nil-guard, so they would panic under
//     awd where a.ctx == nil.
//
// The list is derived by auditing every wailsruntime.*Dialog / Window* /
// OpenFileDialog / OpenDirectoryDialog / BrowserOpenURL / ClipboardSetText /
// OpenSystemSettings call site and recording the exported method that wraps it
// (see denylist_test.go, which fails if a dialog-wrapping method is missing
// here without a tested web alternative). It is shared verbatim between web
// mode (Phase 1) and awd (Phase 2).
var webDenylist = map[string]bool{
	// Native folder/file pickers (app.go, app_skills.go, app_browser.go).
	"SelectFolder":           true,
	"ChooseVaultDir":         true,
	"SelectSkillFile":        true,
	"SelectImportFolder":     true,
	"PickBrowserExecutable":  true,
	"PickAndUploadWallpaper": true,

	// Native confirm / message dialogs (app_permissions.go).
	"SaveSandboxSettings": true,

	// Provider delete via native MessageDialog (app.go). The web UI confirms
	// with a React modal and calls the *Confirmed no-dialog variants instead.
	"DeleteProviderCredential": true,
	"DeleteCustomProvider":     true,

	// Picture-in-picture spawns a native Wails window — meaningless remotely.
	"OpenPipWindow": true,

	// Lifecycle / host wiring. These became exported when the App was extracted
	// to internal/appcore (so main and awd can drive it), which also exposes them
	// to the reflection bridge. A remote client must never start/stop the app or
	// re-inject assets, so they are denied (Shutdown/Startup take only a context
	// the bridge would inject, so they WOULD otherwise run).
	"Startup":       true,
	"OnDomReady":    true,
	"Shutdown":      true,
	"StartHeadless": true,
	"SetWebAssets":  true,

	// Password-manager entries carry credential PLAINTEXT in their results.
	// The module promises "local-only": a remote web client must never
	// receive or mutate them (the web UI shows a desktop-only notice instead).
	"ListPasswords":  true,
	"SavePassword":   true,
	"DeletePassword": true,

	// macOS System Settings deep links and Touch ID — OS-local, no remote sense.
	"OpenSystemSettingsPane":  true,
	"VaultTouchIDAvailable":   true,
	"VaultTouchIDHasPassword": true,
	"VaultTouchIDEnroll":      true,
	"VaultTouchIDUnlock":      true,
	"VaultTouchIDRemove":      true,
	"VaultTouchIDRepair":      true,

	// Code-signing trust settings mutate the HOST's certificate trust store —
	// strictly a local, human-at-the-Mac action.
	"CodesignTrustStatus": true,
	"CodesignTrustGrant":  true,
}

// IsDenied reports whether method is blocked in web/headless mode.
func IsDenied(method string) bool { return webDenylist[method] }

// Denylist returns a copy of the shared denylist for tests and the awd host.
func Denylist() map[string]bool {
	out := make(map[string]bool, len(webDenylist))
	for k, v := range webDenylist {
		out[k] = v
	}
	return out
}
