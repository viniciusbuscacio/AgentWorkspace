package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aw/internal/domain"
)

func TestAwActionsListsExpectedActions(t *testing.T) {
	ws := &workspace{root: t.TempDir(), selfManage: true, allowShell: true}
	result, err := ws.awDispatch(nil, awArgs{Action: "aw.actions"})
	if err != nil {
		t.Fatalf("aw.actions error = %v", err)
	}
	var actions []string
	if err := json.Unmarshal([]byte(result.Result), &actions); err != nil {
		t.Fatalf("unmarshal actions: %v\n%s", err, result.Result)
	}
	have := map[string]bool{}
	for _, action := range actions {
		have[action] = true
	}
	for _, want := range []string{
		"aw.actions",
		"system.selfcode",
		"system.state",
		"system.spawn",
		"document.read_safe",
		"fs.read",
		"fs.write",
		"fs.list",
		"shell.exec",
		"git.status",
		"git.exec",
		"github.status",
		"github.exec",
	} {
		if !have[want] {
			t.Fatalf("aw.actions missing %q in %v", want, actions)
		}
	}
}

func TestAwActionsRegisterVisualReadSafeOnlyWhenExtractorExists(t *testing.T) {
	ws := &workspace{root: t.TempDir(), selfManage: true}
	result, err := ws.awDispatch(nil, awArgs{Action: "aw.actions"})
	if err != nil {
		t.Fatalf("aw.actions error = %v", err)
	}
	if strings.Contains(result.Result, `"visual.read_safe"`) {
		t.Fatalf("visual.read_safe should be hidden without extractor: %s", result.Result)
	}

	ws.visualExtractFn = func(context.Context, []byte, string) (string, error) { return "", nil }
	result, err = ws.awDispatch(nil, awArgs{Action: "aw.actions"})
	if err != nil {
		t.Fatalf("aw.actions with extractor error = %v", err)
	}
	if !strings.Contains(result.Result, `"visual.read_safe"`) {
		t.Fatalf("visual.read_safe should be registered with extractor: %s", result.Result)
	}
}

func TestAwActionsHideShellWhenAllowShellFalse(t *testing.T) {
	ws := &workspace{root: t.TempDir(), selfManage: true}
	result, err := ws.awDispatch(nil, awArgs{Action: "aw.actions"})
	if err != nil {
		t.Fatalf("aw.actions error = %v", err)
	}
	var actions []string
	if err := json.Unmarshal([]byte(result.Result), &actions); err != nil {
		t.Fatalf("unmarshal actions: %v\n%s", err, result.Result)
	}
	for _, action := range actions {
		if action == "shell.exec" || action == "git.exec" || action == "git.status" {
			t.Fatalf("shell/git action %q should be hidden when allowShell is false: %v", action, actions)
		}
	}
}

// The "hands" are not a self-dev privilege: with allowShell alone the main
// agent gets shell, files, git and office (the Permissions sandbox fences
// each call — permit_all IS permit all), while system.* stays self-dev-only.
func TestAwActionsHandsRegisteredWithoutSelfManage(t *testing.T) {
	ws := &workspace{root: t.TempDir(), allowShell: true}
	result, err := ws.awDispatch(nil, awArgs{Action: "aw.actions"})
	if err != nil {
		t.Fatalf("aw.actions error = %v", err)
	}
	var actions []string
	if err := json.Unmarshal([]byte(result.Result), &actions); err != nil {
		t.Fatalf("unmarshal actions: %v\n%s", err, result.Result)
	}
	have := map[string]bool{}
	for _, action := range actions {
		have[action] = true
	}
	for _, want := range []string{
		"shell.exec", "git.status", "git.exec",
		"fs.read", "fs.write", "fs.edit", "fs.list",
		"document.read_safe", "office.read", "office.replace", "office.create",
	} {
		if !have[want] {
			t.Fatalf("%q should register with allowShell alone: %v", want, actions)
		}
	}
	for _, hidden := range []string{"system.selfcode", "system.state"} {
		if have[hidden] {
			t.Fatalf("%q must stay self-dev-only: %v", hidden, actions)
		}
	}
}

func TestAwSystemSelfcodeReadsSELFCODE(t *testing.T) {
	ws := &workspace{root: t.TempDir(), selfManage: true}
	if err := os.MkdirAll(filepath.Join(ws.root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws.root, "docs", "SELFCODE.md"), []byte("self map"), 0o644); err != nil {
		t.Fatalf("seed SELFCODE.md: %v", err)
	}
	result, err := ws.awDispatch(nil, awArgs{Action: "system.selfcode"})
	if err != nil {
		t.Fatalf("system.selfcode error = %v", err)
	}
	if result.Result != "self map" {
		t.Fatalf("system.selfcode = %q, want self map", result.Result)
	}
}

func TestAwUnknownActionReturnsClearError(t *testing.T) {
	ws := &workspace{root: t.TempDir(), selfManage: true}
	_, err := ws.awDispatch(nil, awArgs{Action: "nope.missing"})
	if err == nil {
		t.Fatalf("unknown action error = nil")
	}
	if !strings.Contains(err.Error(), "unknown action") || !strings.Contains(err.Error(), "aw.actions") {
		t.Fatalf("unknown action error = %q, want clear aw.actions hint", err.Error())
	}
}

func TestAwFsWriteAutoApproveWritesWithoutPrompt(t *testing.T) {
	ws := &workspace{root: t.TempDir(), selfManage: true, autoApprove: true}
	result, err := ws.awDispatch(nil, awArgs{
		Action: "fs.write",
		Args:   `{"path":"sub/out.txt","content":"aw data"}`,
	})
	if err != nil {
		t.Fatalf("fs.write error = %v", err)
	}
	if result.Action != "fs.write" {
		t.Fatalf("Action = %q, want fs.write", result.Action)
	}
	data, err := os.ReadFile(filepath.Join(ws.root, "sub", "out.txt"))
	if err != nil || string(data) != "aw data" {
		t.Fatalf("written file = %q, err = %v", string(data), err)
	}
}

func TestAwSystemStateUsesCallback(t *testing.T) {
	ws := &workspace{
		root:       t.TempDir(),
		selfManage: true,
		stateFn: func(_ context.Context) (any, error) {
			return map[string]any{"ready": true, "name": "aw"}, nil
		},
	}
	result, err := ws.awDispatch(nil, awArgs{Action: "system.state"})
	if err != nil {
		t.Fatalf("system.state error = %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(result.Result), &state); err != nil {
		t.Fatalf("unmarshal state: %v\n%s", err, result.Result)
	}
	if state["ready"] != true || state["name"] != "aw" {
		t.Fatalf("state = %#v, want callback values", state)
	}
}

func TestWebReadObservationActionsUseCallbacksAndChatScope(t *testing.T) {
	var saved WebReadSaveInput
	ws := &workspace{
		root: t.TempDir(),
		webRead: &WebReadFuncs{
			Save: func(_ context.Context, input WebReadSaveInput) (any, error) {
				saved = input
				return map[string]any{"id": "webobs-1"}, nil
			},
			Latest: func(_ context.Context, chatID, browser, siteKey, rawURL string) (any, bool, error) {
				if chatID != "chat-1" || browser != "edge" || siteKey != "https://mail.google.com" || rawURL != "" {
					t.Fatalf("latest args = %q %q %q %q", chatID, browser, siteKey, rawURL)
				}
				return map[string]any{"id": "webobs-1"}, true, nil
			},
		},
	}
	ctx := domain.WithChatSessionScope(context.Background(), "chat-1")
	ctx = domain.WithExternalTaintScope(ctx, "run-1")
	reg := ws.awRegistry()
	if _, err := reg["web_read.observation.save"](ctx, map[string]any{
		"browser": "edge",
		"siteKey": "https://mail.google.com",
		"payload": map[string]any{"items": []any{map[string]any{"id": "obs-1"}}},
	}, ws); err != nil {
		t.Fatalf("web_read.observation.save error = %v", err)
	}
	if saved.ChatID != "chat-1" || saved.RunID != "run-1" || saved.Browser != "edge" || saved.SiteKey != "https://mail.google.com" {
		t.Fatalf("saved = %+v", saved)
	}
	result, err := reg["web_read.observation.latest"](ctx, map[string]any{
		"browser": "edge",
		"siteKey": "https://mail.google.com",
	}, ws)
	if err != nil {
		t.Fatalf("web_read.observation.latest error = %v", err)
	}
	if !strings.Contains(result, `"found": true`) || !strings.Contains(result, `"webobs-1"`) {
		t.Fatalf("latest result = %s", result)
	}
}

func TestAllowedActionsIntersectsRegistry(t *testing.T) {
	added := []string{"browser-chrome"}
	ws, _ := browserWorkspace(t, &added)
	ws.allowedActions = map[string]bool{"browser.status": true}
	reg := ws.awRegistry()
	if _, ok := reg["browser.status"]; !ok {
		t.Fatal("allowed browser.status missing")
	}
	if _, ok := reg["browser.snapshot"]; ok {
		t.Fatal("browser.snapshot should be removed by host allowlist")
	}
	if _, ok := reg["aw.actions"]; !ok {
		t.Fatal("aw.actions should always remain available")
	}
}
