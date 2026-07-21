package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aw/internal/domain"
	"aw/internal/infrastructure/sandbox"
)

// Phase 2 acceptance: every mode enforced at the two call sites — resolve()
// for fs.* and runShell() for shell.exec/git.exec.

func sandboxWorkspace(root string, policy sandbox.Policy) *workspace {
	return &workspace{
		root:            root,
		autoApprove:     true,
		allowShell:      true,
		selfManage:      true,
		sandboxPolicyFn: func() sandbox.Policy { return policy },
	}
}

func TestPermitListBlocksOutsidePathsWithClearReason(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "x.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := sandboxWorkspace(root, sandbox.Policy{
		Config:        domain.DefaultSandboxConfig(),
		WorkspaceRoot: root,
	})

	if _, err := ws.readFile(context.TODO(), readFileArgs{Path: secret}); !sandbox.IsDenied(err) {
		t.Fatalf("readFile outside permit_list = %v, want sandbox denial", err)
	}
	if _, err := ws.writeFile(context.TODO(), writeFileArgs{Path: secret, Content: "y"}); !sandbox.IsDenied(err) {
		t.Fatalf("writeFile outside permit_list = %v, want sandbox denial", err)
	}

	// cat of an outside path comes back blocked with the reason, not an error.
	result, err := ws.runShell(context.TODO(), runShellArgs{Command: "cat " + secret})
	if err != nil {
		t.Fatalf("runShell error = %v", err)
	}
	if !result.Blocked || !strings.Contains(result.Reason, "Access denied") {
		t.Fatalf("runShell result = %+v, want blocked with clear reason", result)
	}

	// Safe no-path command still runs.
	echo, err := ws.runShell(context.TODO(), runShellArgs{Command: "echo hi"})
	if err != nil || echo.Blocked || !strings.Contains(echo.Stdout, "hi") {
		t.Fatalf("echo hi = %+v, err = %v, want it to run", echo, err)
	}

	// Workspace-relative operations keep working.
	if _, err := ws.writeFile(context.TODO(), writeFileArgs{Path: "notes.txt", Content: "ok"}); err != nil {
		t.Fatalf("writeFile inside workspace error = %v", err)
	}
	if got, err := ws.readFile(context.TODO(), readFileArgs{Path: "notes.txt"}); err != nil || got.Content != "ok" {
		t.Fatalf("readFile inside workspace = %+v, err = %v", got, err)
	}
}

func TestUserAllowedFolderReachableInPermitList(t *testing.T) {
	root := t.TempDir()
	allowed := t.TempDir()
	target := filepath.Join(allowed, "doc.txt")
	if err := os.WriteFile(target, []byte("doc"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := sandboxWorkspace(root, sandbox.Policy{
		Config: domain.SandboxConfig{
			Mode:           domain.SandboxPermitList,
			AllowedFolders: []string{allowed},
		},
		WorkspaceRoot: root,
	})
	if got, err := ws.readFile(context.TODO(), readFileArgs{Path: target}); err != nil || got.Content != "doc" {
		t.Fatalf("readFile in user-allowed folder = %+v, err = %v", got, err)
	}
}

func TestBlockAllDisablesShellEntirely(t *testing.T) {
	root := t.TempDir()
	ws := sandboxWorkspace(root, sandbox.Policy{
		Config:        domain.SandboxConfig{Mode: domain.SandboxBlockAll},
		WorkspaceRoot: root,
	})
	result, err := ws.runShell(context.TODO(), runShellArgs{Command: "echo hi"})
	if err != nil {
		t.Fatalf("runShell error = %v", err)
	}
	if !result.Blocked || !strings.Contains(result.Reason, "BLOCK_ALL") {
		t.Fatalf("block_all shell = %+v, want blocked with BLOCK_ALL reason", result)
	}
	// Reads/writes still work in the workspace folder.
	if _, err := ws.writeFile(context.TODO(), writeFileArgs{Path: "x.txt", Content: "x"}); err != nil {
		t.Fatalf("writeFile in workspace under block_all error = %v", err)
	}
	if _, err := ws.readFile(context.TODO(), readFileArgs{Path: filepath.Join(t.TempDir(), "y.txt")}); !sandbox.IsDenied(err) {
		t.Fatalf("readFile outside workspace under block_all = %v, want denial", err)
	}
}

func TestBuiltinDeniesHoldInEveryMode(t *testing.T) {
	root := t.TempDir()
	for _, mode := range domain.SandboxModes() {
		ws := sandboxWorkspace(root, sandbox.Policy{
			Config:        domain.SandboxConfig{Mode: mode},
			WorkspaceRoot: root,
		})
		if _, err := ws.readFile(context.TODO(), readFileArgs{Path: "~/.ssh/id_rsa"}); !sandbox.IsDenied(err) {
			t.Errorf("mode %s: readFile ~/.ssh/id_rsa = %v, want sandbox denial", mode, err)
		}
		if mode == domain.SandboxBlockAll {
			continue // shell is refused outright there
		}
		result, err := ws.runShell(context.TODO(), runShellArgs{Command: "cat ~/.ssh/id_rsa"})
		if err != nil {
			t.Fatalf("mode %s: runShell error = %v", mode, err)
		}
		if !result.Blocked {
			t.Errorf("mode %s: cat ~/.ssh/id_rsa not blocked: %+v", mode, result)
		}
	}
}

// Risk 1: an fs.write to the aw data dir is blocked even in permit_all with
// the repo as workspace (the self-dev shape) — the fence protects itself.
func TestDataDirProtectedEvenInSelfDevPermitAll(t *testing.T) {
	repo := t.TempDir()
	dataDir := t.TempDir()
	vaultDir := filepath.Join(dataDir, "AgentWorkspace")
	ws := sandboxWorkspace(repo, sandbox.Policy{
		Config:        domain.SandboxConfig{Mode: domain.SandboxPermitAll},
		WorkspaceRoot: repo,
		DataDir:       dataDir,
		Protected:     []string{vaultDir},
	})
	configPath := filepath.Join(dataDir, "config.json")
	if _, err := ws.writeFile(context.TODO(), writeFileArgs{Path: configPath, Content: "{}"}); !sandbox.IsDenied(err) {
		t.Fatalf("writeFile config.json in permit_all = %v, want sandbox denial", err)
	}
	if _, err := ws.writeFile(context.TODO(), writeFileArgs{Path: filepath.Join(vaultDir, "vault.db"), Content: "x"}); !sandbox.IsDenied(err) {
		t.Fatalf("writeFile vault file in permit_all = %v, want sandbox denial", err)
	}
	result, err := ws.runShell(context.TODO(), runShellArgs{Command: "echo pwned > " + configPath})
	if err != nil {
		t.Fatalf("runShell error = %v", err)
	}
	if !result.Blocked {
		t.Fatalf("shell redirect into config.json not blocked: %+v", result)
	}
	// The repo itself stays fully writable — self-dev keeps working.
	if _, err := ws.writeFile(context.TODO(), writeFileArgs{Path: "main.go", Content: "package main"}); err != nil {
		t.Fatalf("writeFile in repo error = %v", err)
	}
}

// The aw actions share the same enforcement: fs.* goes through resolve() and
// shell.exec through runShell() — no second code path.
func TestAwActionsShareSandboxEnforcement(t *testing.T) {
	root := t.TempDir()
	ws := sandboxWorkspace(root, sandbox.Policy{
		Config:        domain.DefaultSandboxConfig(),
		WorkspaceRoot: root,
	})
	registry := ws.awRegistry()

	if _, err := registry["fs.read"](context.TODO(), map[string]any{"path": "~/.ssh/id_rsa"}, ws); !sandbox.IsDenied(err) {
		t.Fatalf("fs.read ~/.ssh = %v, want sandbox denial", err)
	}
	out, err := registry["shell.exec"](context.TODO(), map[string]any{"command": "cat /etc/hosts"}, ws)
	if err != nil {
		t.Fatalf("shell.exec error = %v", err)
	}
	if !strings.Contains(out, `"blocked": true`) {
		t.Fatalf("shell.exec result = %s, want structured blocked result", out)
	}
}

func TestRunShellBlockedDirComesBackStructured(t *testing.T) {
	root := t.TempDir()
	ws := sandboxWorkspace(root, sandbox.Policy{
		Config:        domain.DefaultSandboxConfig(),
		WorkspaceRoot: root,
	})
	result, err := ws.runShell(context.TODO(), runShellArgs{Command: "echo hi", Dir: "~/.ssh"})
	if err != nil {
		t.Fatalf("runShell error = %v", err)
	}
	if !result.Blocked || len(result.BlockedPaths) == 0 {
		t.Fatalf("blocked dir result = %+v, want blocked with paths", result)
	}
}
