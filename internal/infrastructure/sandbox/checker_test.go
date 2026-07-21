package sandbox

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aw/internal/domain"
)

// The mode-table cases are ported from AW2's path-validator.test.ts. AW2's
// "~/AgentWorkspace" app folder maps to Policy.WorkspaceRoot here.

const testWorkspace = "/aw-test/workspace"

func permitListPolicy(allowedFolders ...string) Policy {
	return Policy{
		Config:        domain.SandboxConfig{Mode: domain.SandboxPermitList, AllowedFolders: allowedFolders},
		WorkspaceRoot: testWorkspace,
	}
}

func blockAllPolicy() Policy {
	return Policy{
		Config:        domain.SandboxConfig{Mode: domain.SandboxBlockAll, AllowedFolders: []string{}},
		WorkspaceRoot: testWorkspace,
	}
}

func permitAllPolicy() Policy {
	return Policy{
		Config:        domain.SandboxConfig{Mode: domain.SandboxPermitAll, AllowedFolders: []string{}},
		WorkspaceRoot: testWorkspace,
	}
}

func home(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	return home
}

func TestExpandPath(t *testing.T) {
	homeDir := home(t)
	if got := ExpandPath("~"); got != homeDir {
		t.Errorf("ExpandPath(~) = %q, want %q", got, homeDir)
	}
	if got, want := ExpandPath("~/foo"), filepath.Join(homeDir, "foo"); got != want {
		t.Errorf("ExpandPath(~/foo) = %q, want %q", got, want)
	}
	if got := ExpandPath("./foo"); !filepath.IsAbs(got) {
		t.Errorf("ExpandPath(./foo) = %q, want absolute", got)
	}
	if got := ExpandPath("/usr/local/../bin"); runtime.GOOS == "windows" {
		if !filepath.IsAbs(got) || !strings.HasSuffix(got, filepath.Join("usr", "bin")) {
			t.Errorf("ExpandPath(/usr/local/../bin) = %q, want absolute path ending in usr/bin", got)
		}
	} else if got != "/usr/bin" {
		t.Errorf("ExpandPath(/usr/local/../bin) = %q, want /usr/bin", got)
	}
}

func TestIsPathAllowedBlockAll(t *testing.T) {
	policy := blockAllPolicy()
	cases := []struct {
		path string
		want bool
	}{
		{"/usr/bin/ls", false},
		{"~/Documents/file.txt", false},
		{testWorkspace + "/notes/todo.md", true}, // agent keeps its own folder
		{"/tmp/test.txt", false},
	}
	for _, tc := range cases {
		if got := IsPathAllowed(tc.path, policy); got != tc.want {
			t.Errorf("block_all IsPathAllowed(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestIsPathAllowedPermitAll(t *testing.T) {
	policy := permitAllPolicy()
	cases := []struct {
		path string
		want bool
	}{
		{"/Users/test/file.txt", true},
		{"~/.ssh/id_rsa", false},
		{"~/.gnupg/keys", false},
		{"~/.aws/credentials", false},
	}
	if runtime.GOOS != "windows" {
		cases = append(cases,
			struct {
				path string
				want bool
			}{"/etc/shadow", false},
			struct {
				path string
				want bool
			}{"/etc/passwd", false},
		)
	}
	for _, tc := range cases {
		if got := IsPathAllowed(tc.path, policy); got != tc.want {
			t.Errorf("permit_all IsPathAllowed(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestIsPathAllowedPermitList(t *testing.T) {
	homeDir := home(t)
	cases := []struct {
		name   string
		policy Policy
		path   string
		want   bool
	}{
		{"workspace folder", permitListPolicy(), testWorkspace + "/scratch.txt", true},
		{"/tmp", permitListPolicy(), "/tmp/test-file.txt", true},
		{"/var/tmp", permitListPolicy(), "/var/tmp/output.log", true},
		{"random home path without allowed folders", permitListPolicy(), "~/Documents/secret.txt", false},
		{"user-configured allowed folder", permitListPolicy("~/Documents"), "~/Documents/notes.md", true},
		{"outside allowed folders", permitListPolicy("~/Documents"), "~/Desktop/file.txt", false},
		{".ssh denied even with broad allowed folders", permitListPolicy(homeDir), "~/.ssh/id_rsa", false},
		{".gnupg denied", permitListPolicy(homeDir), "~/.gnupg/pubring.kbx", false},
		{".aws denied", permitListPolicy(homeDir), "~/.aws/credentials", false},
	}
	if runtime.GOOS != "windows" {
		cases = append(cases,
			struct {
				name   string
				policy Policy
				path   string
				want   bool
			}{"/etc/shadow denied", permitListPolicy("/etc"), "/etc/shadow", false},
			struct {
				name   string
				policy Policy
				path   string
				want   bool
			}{"/etc/passwd denied", permitListPolicy("/etc"), "/etc/passwd", false},
		)
	}
	for _, tc := range cases {
		if got := IsPathAllowed(tc.path, tc.policy); got != tc.want {
			t.Errorf("%s: IsPathAllowed(%q) = %v, want %v", tc.name, tc.path, got, tc.want)
		}
	}
}

// The removed deny_list mode must be treated as unknown: unknown mode → deny.
func TestIsPathAllowedUnknownModeDenies(t *testing.T) {
	policy := Policy{
		Config:        domain.SandboxConfig{Mode: domain.SandboxMode("deny_list")},
		WorkspaceRoot: testWorkspace,
	}
	if IsPathAllowed("~/Documents/work.txt", policy) {
		t.Error("unknown mode (removed deny_list) must deny paths outside the workspace")
	}
	if !IsPathAllowed(testWorkspace+"/scratch.txt", policy) {
		t.Error("workspace must stay reachable even under an unknown mode")
	}
}

func TestIsPathAllowedNormalization(t *testing.T) {
	// ~/Documents/../.ssh/id_rsa resolves to ~/.ssh/id_rsa.
	if IsPathAllowed("~/Documents/../.ssh/id_rsa", permitListPolicy("~/Documents")) {
		t.Error(".. traversal escaped the allowed folder into ~/.ssh")
	}
	if !IsPathAllowed("/tmp///test//file.txt", permitListPolicy()) {
		t.Error("redundant separators broke /tmp matching")
	}
	policy := permitListPolicy("~/Projects")
	absolute := filepath.Join(home(t), "Projects", "app", "index.ts")
	if !IsPathAllowed(absolute, policy) {
		t.Error("tilde in allowed folders did not match absolute path")
	}
}

// aw-specific: the fence protects itself. The data dir is denied in every
// mode (the workspace carve-out aside), and Protected paths have no carve-out.
func TestIsPathAllowedProtectsAWDataDir(t *testing.T) {
	dataDir := "/aw-test/data"
	workspace := filepath.Join(dataDir, "workspace")
	vaultDir := filepath.Join(dataDir, "AgentWorkspace")
	configFile := filepath.Join(dataDir, "config.json")

	for _, mode := range domain.SandboxModes() {
		policy := Policy{
			Config:        domain.SandboxConfig{Mode: mode},
			WorkspaceRoot: workspace,
			DataDir:       dataDir,
			Protected:     []string{vaultDir, configFile},
		}
		if IsPathAllowed(configFile, policy) {
			t.Errorf("mode %s: config.json reachable — agent could widen its own fence", mode)
		}
		if IsPathAllowed(filepath.Join(vaultDir, "vault.db"), policy) {
			t.Errorf("mode %s: vault file reachable", mode)
		}
		if IsPathAllowed(filepath.Join(dataDir, "other.txt"), policy) {
			t.Errorf("mode %s: data dir reachable outside the workspace carve-out", mode)
		}
		if !IsPathAllowed(filepath.Join(workspace, "scratch.txt"), policy) {
			t.Errorf("mode %s: workspace carve-out inside the data dir not honored", mode)
		}
	}
}

// Port of AW2's getSandboxRoots: the visible top-level folders per mode.
func TestRoots(t *testing.T) {
	permitList := permitListPolicy("/tmp/example")
	roots := Roots(permitList)
	if !contains(roots, ExpandPath("/tmp/example")) || !contains(roots, ExpandPath(testWorkspace)) || !contains(roots, ExpandPath("/tmp")) {
		t.Errorf("permit_list roots = %v, want allowed folder, workspace and /tmp", roots)
	}

	blockAll := Roots(blockAllPolicy())
	if len(blockAll) != 1 || blockAll[0] != ExpandPath(testWorkspace) {
		t.Errorf("block_all roots = %v, want only the workspace", blockAll)
	}

	permitAll := Roots(permitAllPolicy())
	if !contains(permitAll, ExpandPath("/")) {
		t.Errorf("permit_all roots = %v, want filesystem root", permitAll)
	}
}

// Self-dev shape: permit_all with the repo as workspace root and the data dir
// fully protected (no carve-out applies because the workspace is elsewhere).
func TestIsPathAllowedSelfDevShape(t *testing.T) {
	policy := Policy{
		Config:        domain.SandboxConfig{Mode: domain.SandboxPermitAll},
		WorkspaceRoot: "/repo/aw",
		DataDir:       "/aw-test/data",
		Protected:     []string{"/aw-test/data/AgentWorkspace", "/aw-test/data/config.json"},
	}
	if !IsPathAllowed("/repo/aw/main.go", policy) {
		t.Error("self-dev repo not reachable in permit_all")
	}
	if IsPathAllowed("/aw-test/data/config.json", policy) {
		t.Error("config.json reachable in self-dev permit_all")
	}
	if IsPathAllowed("/aw-test/data/workspace/x.txt", policy) {
		t.Error("data dir reachable in self-dev (workspace carve-out must not apply: workspace is the repo)")
	}
	if IsPathAllowed("~/.ssh/id_rsa", policy) {
		t.Error(".ssh reachable in self-dev permit_all")
	}
}
