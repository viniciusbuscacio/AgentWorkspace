package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"aw/internal/domain"
)

func mcpWorkspace(added bool) *workspace {
	ws := &workspace{
		mcpConnections: &McpConnectionFuncs{
			List: func(context.Context) (any, error) {
				return []domain.McpConnection{{ID: "mcpconn-1", Name: "Learn"}}, nil
			},
			ListTools: func(_ context.Context, _ string) (domain.McpToolListResult, error) {
				return domain.McpToolListResult{ConnectionID: "mcpconn-1", Tools: []domain.McpToolSummary{{Name: "search", Description: "Search"}}}, nil
			},
			CallTool: func(_ context.Context, _, tool string, _ map[string]any) (domain.McpToolCallResult, error) {
				return domain.McpToolCallResult{ConnectionID: "mcpconn-1", Tool: tool, Status: domain.McpCallStatusSuccess, Content: []domain.McpContentBlock{{Type: "text", Text: "hello"}}}, nil
			},
		},
	}
	if added {
		ws.addedModulesFn = func() []string { return []string{"mcp-client"} }
	}
	return ws
}

func TestMcpActionsFencedByModule(t *testing.T) {
	// Module not added → no mcp.* actions.
	off := mcpWorkspace(false)
	for _, action := range []string{"mcp.connections.list", "mcp.tools.list", "mcp.tools.call"} {
		if _, ok := off.awRegistry()[action]; ok {
			t.Errorf("%q must not register before the module is added", action)
		}
	}
	// Module added → mcp.* actions present.
	on := mcpWorkspace(true)
	for _, action := range []string{
		"mcp.connections.list", "mcp.connections.get", "mcp.connections.add", "mcp.connections.update",
		"mcp.connections.remove", "mcp.connections.set_enabled", "mcp.connections.test",
		"mcp.tools.list", "mcp.tools.call",
	} {
		if _, ok := on.awRegistry()[action]; !ok {
			t.Errorf("%q must register after the module is added", action)
		}
	}
}

func TestMcpToolListAttachesExternalSafety(t *testing.T) {
	ws := mcpWorkspace(true)
	out, err := ws.awRegistry()["mcp.tools.list"](context.Background(), map[string]any{"connectionId": "mcpconn-1"}, ws)
	if err != nil {
		t.Fatalf("list tools error = %v", err)
	}
	if !strings.Contains(out, "external_safety") || !strings.Contains(out, "untrusted") {
		t.Errorf("remote tool list must carry external_safety/untrusted metadata: %s", out)
	}
	if !strings.Contains(out, "search") {
		t.Errorf("tool list missing the tool: %s", out)
	}
}

func TestMcpToolCallReadToolNoConfirm(t *testing.T) {
	ws := mcpWorkspace(true) // no confirmer, autoApprove false
	out, err := ws.awRegistry()["mcp.tools.call"](context.Background(), map[string]any{"connectionId": "mcpconn-1", "tool": "search"}, ws)
	if err != nil {
		t.Fatalf("read-only tool call should not require confirmation: %v", err)
	}
	if !strings.Contains(out, "external_safety") {
		t.Errorf("tool call output must carry external_safety: %s", out)
	}
}

func TestMcpToolCallMutatingRequiresConfirm(t *testing.T) {
	ws := mcpWorkspace(true) // no confirmer + autoApprove false ⇒ strict confirm denies
	_, err := ws.awRegistry()["mcp.tools.call"](context.Background(), map[string]any{"connectionId": "mcpconn-1", "tool": "delete_item"}, ws)
	if !errors.Is(err, ErrConfirmationDenied) {
		t.Errorf("mutating remote tool must be gated, got %v", err)
	}
}

func TestMcpActionUnavailableWhenCallbackNil(t *testing.T) {
	ws := &workspace{
		mcpConnections: &McpConnectionFuncs{}, // no callbacks wired
		addedModulesFn: func() []string { return []string{"mcp-client"} },
	}
	_, err := ws.awRegistry()["mcp.connections.list"](context.Background(), nil, ws)
	if err == nil || !strings.Contains(err.Error(), "mcp connections") {
		t.Errorf("expected unavailable error, got %v", err)
	}
}
