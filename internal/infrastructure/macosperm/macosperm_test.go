package macosperm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot resolves the aw project root from this test file's path.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// macosperm_test.go is 4 levels inside the repo root:
	// internal/infrastructure/macosperm/macosperm_test.go
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

// TestDeepLinksWellFormed verifies that every permission entry has a non-empty
// deep link that starts with the x-apple.systempreferences: scheme.
func TestDeepLinksWellFormed(t *testing.T) {
	const scheme = "x-apple.systempreferences:"
	for _, p := range AllForBuild() {
		if p.DeepLink == "" {
			t.Errorf("permission %q: DeepLink is empty", p.ID)
			continue
		}
		if !strings.HasPrefix(p.DeepLink, scheme) {
			t.Errorf("permission %q: DeepLink %q does not start with %q",
				p.ID, p.DeepLink, scheme)
		}
	}
}

// TestAllIDsUnique verifies that each permission has a unique, non-empty ID.
func TestAllIDsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range AllForBuild() {
		if p.ID == "" {
			t.Errorf("permission has empty ID: %+v", p)
			continue
		}
		if seen[p.ID] {
			t.Errorf("duplicate permission ID %q", p.ID)
		}
		seen[p.ID] = true
	}
}

// TestAllNonMacOSReturnsNil verifies that All() returns nil on non-darwin
// platforms so the frontend can hide the card automatically.
func TestAllNonMacOSReturnsNil(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("running on darwin — All() returns non-nil on the host platform")
	}
	if got := All(); got != nil {
		t.Fatalf("All() = %v on non-darwin, want nil", got)
	}
}

// TestSELFCODEDriftFence verifies that docs/SELFCODE.md contains the name
// of every permission in the canonical Go list. If a permission is added or
// renamed in Go without updating SELFCODE.md this test fails — that is the
// correct behavior (update the doc, then re-run).
func TestSELFCODEDriftFence(t *testing.T) {
	root := repoRoot(t)
	selfcodePath := filepath.Join(root, "docs", "SELFCODE.md")
	data, err := os.ReadFile(selfcodePath)
	if err != nil {
		t.Skipf("SELFCODE.md not found at %s (CI without workspace?): %v", selfcodePath, err)
	}
	content := string(data)
	for _, p := range AllForBuild() {
		if !strings.Contains(content, p.Name) {
			t.Errorf("SELFCODE.md missing permission name %q — update docs/SELFCODE.md to stay in sync with macosperm.AllForBuild()", p.Name)
		}
	}
	// The failure-signature section header must be present.
	if !strings.Contains(content, "macOS Permissions (TCC)") {
		t.Error(`SELFCODE.md missing "macOS Permissions (TCC)" section header`)
	}
}

// TestOpenPaneUnknownLinkDegradesSilently verifies that OpenPane never returns
// an error for an unknown link (it degrades to the fallback pane without
// executing on non-darwin hosts).
func TestOpenPaneUnknownLinkDegradesSilently(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skip on darwin — would actually open System Settings")
	}
	// On non-darwin the function is a no-op.
	if err := OpenPane("x-apple.systempreferences:unknown?Query"); err != nil {
		t.Fatalf("OpenPane unexpected error on non-darwin: %v", err)
	}
}

// TestIsMacosPermErrorMatches verifies the known TCC error patterns.
func TestIsMacosPermErrorMatches(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("IsMacosPermError is always false on non-darwin")
	}
	positives := []string{
		"voice recording failed: Unable to get capture device",
		"avfoundation: capture device error",
		"no audio was captured",
		"osascript: execution error: -1743",
		"not allowed to send keystrokes",
		"open /Users/test/Desktop/file.txt: operation not permitted",
		"open /Users/test/Documents/x: operation not permitted",
	}
	for _, msg := range positives {
		if !IsMacosPermError(msg) {
			t.Errorf("IsMacosPermError(%q) = false, want true", msg)
		}
	}
	negatives := []string{
		"",
		"file not found",
		"permission denied on /tmp/x",
		"operation not permitted on /etc/hosts",
	}
	for _, msg := range negatives {
		if IsMacosPermError(msg) {
			t.Errorf("IsMacosPermError(%q) = true, want false", msg)
		}
	}
}

// TestAppendHintOnlyOnMatch ensures that AppendHint appends PermHint only
// when IsMacosPermError is true, and never replaces the original message.
func TestAppendHintOnlyOnMatch(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("AppendHint is a no-op on non-darwin")
	}
	original := "voice recording failed: Unable to get capture device"
	out := AppendHint(original)
	if !strings.HasPrefix(out, original) {
		t.Fatalf("AppendHint replaced the original message: %q", out)
	}
	if !strings.Contains(out, PermHint) {
		t.Fatalf("AppendHint did not append PermHint: %q", out)
	}

	noMatch := "file not found"
	if got := AppendHint(noMatch); got != noMatch {
		t.Fatalf("AppendHint changed a non-matching message: %q → %q", noMatch, got)
	}
}

// TestProbeNonDarwinIsUnknown ensures probes return ProbeUnknown on non-darwin
// hosts (no TCC on non-macOS) and do not panic.
func TestProbeNonDarwinIsUnknown(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("running on darwin — probes may do real checks")
	}
	for _, id := range []string{"microphone", "automation", "files", "app_management", "unknown"} {
		res := Probe(id)
		if res.Status != ProbeUnknown {
			t.Errorf("Probe(%q).Status = %q on non-darwin, want %q", id, res.Status, ProbeUnknown)
		}
	}
}
