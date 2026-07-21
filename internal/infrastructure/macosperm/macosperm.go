// Package macosperm provides the v1 macOS TCC permission inventory for Agent
// Workspace 3. Both the Settings security card and the SELFCODE agent-docs
// section render from this single canonical list — they cannot drift (drift
// fence enforced by macosperm_test.go).
package macosperm

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Permission is one row in the macOS TCC permission table.
type Permission struct {
	// ID is a stable machine-readable key used by the probe API.
	ID string
	// Name is the human-readable display label shown in the card header.
	Name string
	// Why is one sentence describing what breaks without this permission.
	Why string
	// SettingsPane is the breadcrumb in System Settings (human-readable).
	SettingsPane string
	// DeepLink is the x-apple.systempreferences: URL that opens the pane
	// directly. Never empty; must start with the systempreferences scheme.
	DeepLink string
	// HasProbe reports whether a cheap status probe is available for this row.
	HasProbe bool
}

// all is the canonical v1 permission inventory. Called on every GOOS so the
// drift-fence and deep-link tests do not require a darwin host.
func all() []Permission {
	return []Permission{
		{
			ID:           "microphone",
			Name:         "Microphone",
			Why:          "Voice capture (ffmpeg :default)",
			SettingsPane: "Privacy & Security › Microphone",
			DeepLink:     "x-apple.systempreferences:com.apple.preference.security?Privacy_Microphone",
			HasProbe:     true,
		},
		{
			ID:           "automation",
			Name:         "Automation (Apple Events)",
			Why:          "osascript driving other apps via Apple Events (agent shell scripts)",
			SettingsPane: "Privacy & Security › Automation",
			DeepLink:     "x-apple.systempreferences:com.apple.preference.security?Privacy_Automation",
			HasProbe:     true,
		},
		{
			ID:           "files",
			Name:         "Files and Folders / Full Disk Access",
			Why:          "fs.* file actions on ~/Desktop, ~/Documents, ~/Downloads (TCC dirs) even when the sandbox allows them",
			SettingsPane: "Privacy & Security › Files and Folders",
			DeepLink:     "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles",
			HasProbe:     true,
		},
		{
			ID:           "app_management",
			Name:         "App Management",
			Why:          "Updating/replacing app bundles (self-dev rebuilding Agent Workspace.app, anything touching other apps)",
			SettingsPane: "Privacy & Security › App Management",
			DeepLink:     "x-apple.systempreferences:com.apple.preference.security?Privacy_AppManagement",
			HasProbe:     false,
		},
	}
}

// All returns the full permission inventory on macOS; nil on all other
// platforms so callers can hide the card automatically.
func All() []Permission {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return all()
}

// AllForBuild returns the full list regardless of GOOS. Intended only for
// tests and the drift-fence check so the suite can run on any host platform.
func AllForBuild() []Permission {
	return all()
}

// fallbackPane is opened when the deep link is not found in the known list.
const fallbackPane = "x-apple.systempreferences:com.apple.preference.security"

// OpenPane opens a macOS System Settings pane by deep link. The link is
// validated against the known inventory as an injection guard. Unknown links
// degrade to the Privacy & Security root pane. No-op on non-macOS platforms.
func OpenPane(deepLink string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	link := deepLink
	known := false
	for _, p := range all() {
		if p.DeepLink == deepLink {
			known = true
			break
		}
	}
	if !known {
		if deepLink == fallbackPane {
			known = true
		}
	}
	if !known {
		// Degrade gracefully: unknown link opens the root pane rather than
		// returning an error that surfaces to the user.
		link = fallbackPane
	}
	return exec.Command("open", link).Run() //nolint:gosec
}

// PermHint is the standard short hint appended (never replacing) to
// permission-shaped errors so the user knows where to look.
const PermHint = "This looks like a macOS permission — see Settings › Security › macOS permissions."

// IsMacosPermError reports whether the error string has a shape that suggests
// a missing TCC permission on macOS. Matches the known error signatures:
//   - avfoundation/capture device errors (Microphone)
//   - osascript -1743 (Automation)
//   - "operation not permitted" on ~/Desktop, ~/Documents, ~/Downloads (Files)
func IsMacosPermError(errMsg string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	lower := toLower(errMsg)
	if contains(lower, "unable to get capture device") ||
		contains(lower, "avfoundation") && contains(lower, "capture device") ||
		contains(lower, "no audio was captured") {
		return true
	}
	if contains(lower, "-1743") ||
		contains(lower, "not allowed to send keystrokes") ||
		contains(lower, "not allowed to interact with") {
		return true
	}
	if contains(lower, "operation not permitted") {
		if containsAny(lower, []string{"desktop", "documents", "downloads"}) {
			return true
		}
	}
	return false
}

// AppendHint appends PermHint to msg when IsMacosPermError reports true.
// The original message is never replaced — only extended with a separator.
func AppendHint(msg string) string {
	if !IsMacosPermError(msg) {
		return msg
	}
	return fmt.Sprintf("%s — %s", msg, PermHint)
}

// toLower is a stdlib-only wrapper so the package stays free of external deps.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, sub string) bool {
	return len(sub) <= len(s) && (sub == "" || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}
