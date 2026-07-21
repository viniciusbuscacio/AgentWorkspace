package tools

import (
	"context"
	"strings"
	"testing"

	"aw/internal/domain"
	"aw/internal/infrastructure/sandbox"
)

func sandboxActionWorkspace(t *testing.T, config domain.SandboxConfig) *workspace {
	t.Helper()
	root := t.TempDir()
	return sandboxWorkspace(root, sandbox.Policy{Config: config, WorkspaceRoot: root})
}

func TestSandboxStatusReportsModeAndLists(t *testing.T) {
	ws := sandboxActionWorkspace(t, domain.SandboxConfig{
		Mode:           domain.SandboxPermitList,
		AllowedFolders: []string{"~/Documents"},
	})
	out, err := ws.awRegistry()["sandbox.status"](context.TODO(), nil, ws)
	if err != nil {
		t.Fatalf("sandbox.status error = %v", err)
	}
	for _, want := range []string{"Mode: permit_list", "Allowed folders: ~/Documents", "Workspace folder: " + ws.root} {
		if !strings.Contains(out, want) {
			t.Errorf("sandbox.status missing %q:\n%s", want, out)
		}
	}
}

func TestSandboxTestDryRunsWithoutExecuting(t *testing.T) {
	ws := sandboxActionWorkspace(t, domain.DefaultSandboxConfig())
	registry := ws.awRegistry()

	blocked, err := registry["sandbox.test"](context.TODO(), map[string]any{"command": "cat ~/.ssh/id_rsa"}, ws)
	if err != nil {
		t.Fatalf("sandbox.test error = %v", err)
	}
	for _, want := range []string{"Result: BLOCKED", "Mode: permit_list", "Blocked paths:", "Access denied to 1 path(s)."} {
		if !strings.Contains(blocked, want) {
			t.Errorf("blocked dry-run missing %q:\n%s", want, blocked)
		}
	}

	allowed, err := registry["sandbox.test"](context.TODO(), map[string]any{"command": "echo hi"}, ws)
	if err != nil {
		t.Fatalf("sandbox.test error = %v", err)
	}
	for _, want := range []string{"Result: ALLOWED", "Paths detected: (none)", "without explicit filesystem paths"} {
		if !strings.Contains(allowed, want) {
			t.Errorf("allowed dry-run missing %q:\n%s", want, allowed)
		}
	}

	if _, err := registry["sandbox.test"](context.TODO(), map[string]any{}, ws); err == nil {
		t.Error("sandbox.test without command did not error")
	}
}

func TestSandboxSetModeAlwaysRefused(t *testing.T) {
	ws := sandboxActionWorkspace(t, domain.DefaultSandboxConfig())
	registry := ws.awRegistry()

	out, err := registry["sandbox.set_mode"](context.TODO(), map[string]any{"mode": "permit_all"}, ws)
	if err != nil {
		t.Fatalf("sandbox.set_mode error = %v", err)
	}
	if out != sandboxConfigDeniedMessage {
		t.Fatalf("sandbox.set_mode = %q, want the AW2 denied message", out)
	}

	invalid, err := registry["sandbox.set_mode"](context.TODO(), map[string]any{"mode": "warp"}, ws)
	if err != nil {
		t.Fatalf("sandbox.set_mode(invalid) error = %v", err)
	}
	if !strings.Contains(invalid, "invalid mode") {
		t.Fatalf("invalid mode reply = %q", invalid)
	}
}

// The policy is read per dispatch: a mode change shows up in sandbox.status
// without rebuilding the workspace.
func TestSandboxStatusReflectsModeChangeWithoutRestart(t *testing.T) {
	root := t.TempDir()
	mode := domain.SandboxPermitList
	ws := &workspace{
		root:        root,
		autoApprove: true,
		sandboxPolicyFn: func() sandbox.Policy {
			return sandbox.Policy{Config: domain.SandboxConfig{Mode: mode}, WorkspaceRoot: root}
		},
	}
	out, _ := ws.awRegistry()["sandbox.status"](context.TODO(), nil, ws)
	if !strings.Contains(out, "Mode: permit_list") {
		t.Fatalf("status before change = %q", out)
	}
	mode = domain.SandboxBlockAll
	out, _ = ws.awRegistry()["sandbox.status"](context.TODO(), nil, ws)
	if !strings.Contains(out, "Mode: block_all") {
		t.Fatalf("status after change = %q", out)
	}
}
