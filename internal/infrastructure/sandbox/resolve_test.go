package sandbox

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aw/internal/domain"
)

func tempPermitListPolicy(allowed ...string) Policy {
	return Policy{
		Config:        domain.SandboxConfig{Mode: domain.SandboxPermitList, AllowedFolders: allowed},
		WorkspaceRoot: "/aw-test/workspace",
	}
}

// Ported from AW2's sandbox-fs.test.ts symlink-escape case.
func TestResolveAndCheckBlocksSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	allowed := filepath.Join(dir, "allowed")
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(allowed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(allowed, "secret-link.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}

	policy := tempPermitListPolicy(allowed)
	if _, err := ResolveAndCheck(link, policy); !IsDenied(err) {
		t.Errorf("symlink escape not denied: err = %v", err)
	}
	// A regular file inside the allowed folder passes.
	regular := filepath.Join(allowed, "ok.txt")
	if err := os.WriteFile(regular, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveAndCheck(regular, policy); err != nil {
		t.Errorf("regular file inside allowed folder denied: %v", err)
	}
}

// Ported from AW2: ".." traversal out of an allowed folder is caught by
// normalization before any disk access.
func TestResolveAndCheckBlocksTraversal(t *testing.T) {
	dir := t.TempDir()
	allowed := filepath.Join(dir, "allowed")
	if err := os.MkdirAll(allowed, 0o755); err != nil {
		t.Fatal(err)
	}
	escape := filepath.Join(allowed, "..", "outside.txt")
	if _, err := ResolveAndCheck(escape, tempPermitListPolicy(allowed)); !IsDenied(err) {
		t.Errorf("traversal escape not denied: err = %v", err)
	}
}

// A path that does not exist yet is validated through its nearest existing
// ancestor (the file is about to be created by fs.write).
func TestResolveAndCheckNonexistentPathUsesAncestor(t *testing.T) {
	dir := t.TempDir()
	policy := tempPermitListPolicy(dir)
	target := filepath.Join(dir, "new", "deep", "file.txt")
	got, err := ResolveAndCheck(target, policy)
	if err != nil {
		t.Fatalf("ResolveAndCheck(%q) error = %v", target, err)
	}
	if got != filepath.Clean(target) {
		t.Errorf("ResolveAndCheck = %q, want %q", got, filepath.Clean(target))
	}
}

// Allowed roots behind symlinks still pass (macOS /tmp → /private/tmp): the
// resolved path is re-checked against the realpath of each allowed root.
func TestResolveAndCheckRootBehindSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	realRoot := filepath.Join(dir, "real-root")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	linkRoot := filepath.Join(dir, "link-root")
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(linkRoot, "file.txt")
	if err := os.WriteFile(filepath.Join(realRoot, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveAndCheck(inside, tempPermitListPolicy(linkRoot)); err != nil {
		t.Errorf("allowed root behind symlink denied: %v", err)
	}
}

func TestResolveAndCheckDeniedError(t *testing.T) {
	_, err := ResolveAndCheck("~/.ssh/id_rsa", permitAllPolicy())
	if !IsDenied(err) {
		t.Fatalf("expected DeniedError, got %v", err)
	}
	if err.Error() == "" {
		t.Error("DeniedError has empty message")
	}
}
