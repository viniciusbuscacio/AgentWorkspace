package appcore

import (
	"reflect"
	"testing"

	vaultpkg "aw/internal/infrastructure/vault"
	"aw/internal/infrastructure/webserver"
)

// nativeDialogMethods is the audited set of *App methods that open a native
// dialog / window / OS pane (grep of wailsruntime.*Dialog / Window* /
// OpenFileDialog / OpenDirectoryDialog / BrowserOpenURL / ClipboardSetText /
// OpenSystemSettings, mapped to the enclosing exported method). Every entry
// must be on the web denylist OR have a tested no-dialog web alternative. This
// test fails if a native-dialog method is renamed/added and the denylist drifts.
var nativeDialogMethods = map[string]string{
	"SelectFolder":           "denied",
	"ChooseVaultDir":         "denied",
	"SelectSkillFile":        "denied",
	"SelectImportFolder":     "denied",
	"PickBrowserExecutable":  "denied",
	"PickAndUploadWallpaper": "denied",
	"SaveSandboxSettings":    "denied",
	"OpenPipWindow":          "denied",
	"OpenSystemSettingsPane": "denied",
	"VaultTouchIDEnroll":     "denied",
	"VaultTouchIDUnlock":     "denied",
	"VaultTouchIDRemove":     "denied",
	// Native-dialog deletes: denied, but a *Confirmed no-dialog web variant exists.
	"DeleteProviderCredential": "alt:DeleteProviderCredentialConfirmed",
	"DeleteCustomProvider":     "alt:DeleteCustomProviderConfirmed",
}

func appMethodSet() map[string]bool {
	set := map[string]bool{}
	t := reflect.TypeOf(&App{})
	for i := 0; i < t.NumMethod(); i++ {
		set[t.Method(i).Name] = true
	}
	return set
}

func TestDenylistCoversNativeDialogMethods(t *testing.T) {
	deny := webserver.Denylist()
	methods := appMethodSet()
	for name, disposition := range nativeDialogMethods {
		if !methods[name] {
			t.Errorf("audited native-dialog method %q no longer exists on *App (rename?)", name)
			continue
		}
		if !deny[name] {
			t.Errorf("native-dialog method %q is not on the web denylist", name)
		}
		if alt, ok := trimAlt(disposition); ok {
			if !methods[alt] {
				t.Errorf("method %q promises web alternative %q, which does not exist on *App", name, alt)
			}
		}
	}
}

// TestPasswordMethodsAreWebDenied pins the security posture: the password
// manager's bindings carry credential PLAINTEXT in their results, and the
// module promises "local-only" — a remote web client must never reach them.
func TestPasswordMethodsAreWebDenied(t *testing.T) {
	deny := webserver.Denylist()
	methods := appMethodSet()
	for _, name := range []string{"ListPasswords", "SavePassword", "DeletePassword"} {
		if !methods[name] {
			t.Errorf("password method %q no longer exists on *App (rename?)", name)
			continue
		}
		if !deny[name] {
			t.Errorf("password method %q is not on the web denylist; it would return credential plaintext to a remote client", name)
		}
	}
}

func TestDenylistEntriesAreRealMethods(t *testing.T) {
	methods := appMethodSet()
	for name := range webserver.Denylist() {
		if !methods[name] {
			t.Errorf("denylist references %q, which is not a method on *App (typo or removed)", name)
		}
	}
}

func trimAlt(disposition string) (string, bool) {
	const prefix = "alt:"
	if len(disposition) > len(prefix) && disposition[:len(prefix)] == prefix {
		return disposition[len(prefix):], true
	}
	return "", false
}

func TestWebSessionMintVerify(t *testing.T) {
	v := vaultpkg.New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() { _ = v.Lock() }()
	app := &App{vault: v}

	token, err := app.mintWebSession()
	if err != nil {
		t.Fatalf("mintWebSession() error = %v", err)
	}
	if !app.validWebSession(token) {
		t.Fatal("freshly minted session should validate")
	}

	// A tampered token must not validate.
	if app.validWebSession(token + "x") {
		t.Fatal("tampered token must not validate")
	}

	// Invalidation (lock/autolock/regenerate) must kill existing tokens.
	app.InvalidateWebSessions("lock")
	if app.validWebSession(token) {
		t.Fatal("token must die after InvalidateWebSessions")
	}

	// New tokens minted after invalidation validate again.
	token2, err := app.mintWebSession()
	if err != nil {
		t.Fatalf("mintWebSession() #2 error = %v", err)
	}
	if !app.validWebSession(token2) {
		t.Fatal("post-invalidation token should validate")
	}
}
