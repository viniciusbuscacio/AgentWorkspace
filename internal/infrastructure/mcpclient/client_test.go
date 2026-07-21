package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aw/internal/domain"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type searchArgs struct {
	Query string `json:"query"`
}

// newTestMcpServer starts a real in-process Streamable HTTP MCP server with a
// couple of tools, optionally bearer-protected.
func newTestMcpServer(t *testing.T, token string) string {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-remote", Version: "test"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "search", Description: "Search docs"},
		func(_ context.Context, _ *mcp.CallToolRequest, in searchArgs) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "result for " + in.Query}}}, nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "big", Description: "Returns a lot"},
		func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.Repeat("x", domain.McpMaxOutputBytes+5000)}}}, nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "boom", Description: "Fails"},
		func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "remote failure"}}}, nil, nil
		})

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	var h http.Handler = handler
	if token != "" {
		h = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			handler.ServeHTTP(w, req)
		})
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts.URL
}

func conn(url, authType string) domain.McpConnection {
	return domain.McpConnection{ID: "mcpconn-test", Name: "remote", Enabled: true, Transport: domain.McpTransportStreamableHTTP, URL: url, AuthType: authType}
}

func TestRuntimeTestAndListNoAuth(t *testing.T) {
	url := newTestMcpServer(t, "")
	rt := New("test")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := rt.TestConnection(ctx, conn(url, domain.McpAuthNone), "")
	if err != nil {
		t.Fatalf("test error = %v", err)
	}
	if res.Status != domain.McpStatusOK || res.ToolCount != 3 {
		t.Fatalf("unexpected test result %+v", res)
	}

	tools, err := rt.ListTools(ctx, conn(url, domain.McpAuthNone), "")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	found := false
	for _, tool := range tools.Tools {
		if tool.Name == "search" {
			found = true
			if len(tool.InputSchema) == 0 {
				t.Error("search tool should carry an input schema")
			}
		}
	}
	if !found {
		t.Errorf("search tool missing from %+v", tools.Tools)
	}
}

func TestRuntimeCallToolSuccess(t *testing.T) {
	url := newTestMcpServer(t, "")
	rt := New("test")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := rt.CallTool(ctx, conn(url, domain.McpAuthNone), "", "search", map[string]any{"query": "teams"})
	if err != nil {
		t.Fatalf("call error = %v", err)
	}
	if res.Status != domain.McpCallStatusSuccess || len(res.Content) == 0 {
		t.Fatalf("unexpected call result %+v", res)
	}
	if !strings.Contains(res.Content[0].Text, "result for teams") {
		t.Errorf("unexpected content %q", res.Content[0].Text)
	}
}

func TestRuntimeCallToolTruncates(t *testing.T) {
	url := newTestMcpServer(t, "")
	rt := New("test")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := rt.CallTool(ctx, conn(url, domain.McpAuthNone), "", "big", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated {
		t.Error("oversized output should be truncated")
	}
	total := 0
	for _, b := range res.Content {
		total += len(b.Text)
	}
	if total > domain.McpMaxOutputBytes+64 {
		t.Errorf("output not capped: %d bytes", total)
	}
}

func TestRuntimeRemoteErrorIsResult(t *testing.T) {
	url := newTestMcpServer(t, "")
	rt := New("test")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := rt.CallTool(ctx, conn(url, domain.McpAuthNone), "", "boom", nil)
	if err != nil {
		t.Fatalf("a remote tool error should be a normalized result, not a Go error: %v", err)
	}
	if res.Status != domain.McpCallStatusError {
		t.Errorf("expected error status, got %q", res.Status)
	}
}

func TestRuntimeBearerAuth(t *testing.T) {
	url := newTestMcpServer(t, "right-token")
	rt := New("test")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Correct token works.
	if _, err := rt.TestConnection(ctx, conn(url, domain.McpAuthBearer), "right-token"); err != nil {
		t.Fatalf("correct token should connect: %v", err)
	}
	// Wrong token fails safely (no panic, clear error).
	if _, err := rt.TestConnection(ctx, conn(url, domain.McpAuthBearer), "wrong-token"); err == nil {
		t.Error("wrong token should fail")
	}
}

func TestRuntimeTimeoutCancels(t *testing.T) {
	url := newTestMcpServer(t, "")
	rt := New("test")
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	if _, err := rt.TestConnection(ctx, conn(url, domain.McpAuthNone), ""); err == nil {
		t.Error("an expired context should fail the connection")
	}
}
