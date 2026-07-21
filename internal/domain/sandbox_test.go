package domain

import "testing"

func TestSandboxBuiltinDeniedForGOOS(t *testing.T) {
	unix := sandboxBuiltinDeniedForGOOS("darwin")
	for _, want := range []string{"~/.ssh", "~/.gnupg", "~/.aws", "/etc/shadow", "/etc/passwd"} {
		if !containsString(unix, want) {
			t.Fatalf("unix built-in denies = %v, missing %q", unix, want)
		}
	}
	if containsString(unix, `~/AppData/Roaming/Microsoft/Credentials`) {
		t.Fatalf("unix built-in denies unexpectedly include Windows credential path: %v", unix)
	}

	windows := sandboxBuiltinDeniedForGOOS("windows")
	for _, want := range []string{
		"~/.ssh",
		"~/.gnupg",
		"~/.aws",
		`~/AppData/Roaming/Microsoft/Credentials`,
		`~/AppData/Local/Microsoft/Credentials`,
		`~/AppData/Roaming/Microsoft/Windows/PowerShell/PSReadLine`,
		`~/AppData/Roaming/Microsoft/UserSecrets`,
	} {
		if !containsString(windows, want) {
			t.Fatalf("windows built-in denies = %v, missing %q", windows, want)
		}
	}
	if containsString(windows, "/etc/shadow") {
		t.Fatalf("windows built-in denies unexpectedly include Unix shadow path: %v", windows)
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
