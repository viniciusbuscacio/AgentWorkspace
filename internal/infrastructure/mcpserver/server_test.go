package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeBackend struct {
	lastAction string
	lastArgs   string
}

func (b *fakeBackend) CallAw(_ context.Context, action string, argsJSON string) (string, error) {
	b.lastAction, b.lastArgs = action, argsJSON
	if action == "boom" {
		return "", fmt.Errorf("action failed")
	}
	return `{"ok":true}`, nil
}

func (b *fakeBackend) AwDescription() string { return "test aw dispatcher" }

func startTestServer(t *testing.T) (*Server, *fakeBackend) {
	t.Helper()
	backend := &fakeBackend{}
	server, err := NewServer(backend, Config{Addr: "127.0.0.1:0", Token: "secret-token", Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})
	return server, backend
}

func TestNewServerValidatesConfig(t *testing.T) {
	if _, err := NewServer(nil, Config{Token: "x"}); err == nil {
		t.Fatal("expected an error without a backend")
	}
	if _, err := NewServer(&fakeBackend{}, Config{Addr: "127.0.0.1:0"}); err == nil {
		t.Fatal("expected an error without a token")
	}
	if _, err := NewServer(&fakeBackend{}, Config{Addr: "0.0.0.0:0", Token: "x"}); err == nil {
		t.Fatal("expected an error for a non-loopback bind address")
	}
}

func TestMcpEndpointRequiresBearerToken(t *testing.T) {
	server, _ := startTestServer(t)

	resp, err := http.Post(server.URL(), "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}
	authenticate := resp.Header.Get("WWW-Authenticate")
	if !strings.Contains(authenticate, "resource_metadata=") {
		t.Fatalf("expected RFC 9728 resource_metadata hint, got %q", authenticate)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL(), strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer wrong-token")
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 with a wrong token, got %d", resp2.StatusCode)
	}
}

func TestResourceMetadataIsServedWithoutToken(t *testing.T) {
	server, _ := startTestServer(t)

	resp, err := http.Get(server.metadataURL())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for protected-resource metadata, got %d", resp.StatusCode)
	}
	var payload struct {
		Resource string `json:"resource"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Resource != server.URL() {
		t.Fatalf("expected resource %q, got %q", server.URL(), payload.Resource)
	}
}

// bearerTransport injects the Authorization header on every request, the way
// a real MCP client configured with our token does.
type bearerTransport struct {
	token string
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}

func TestMcpClientCanCallAwTool(t *testing.T) {
	server, backend := startTestServer(t)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:   server.URL(),
		HTTPClient: &http.Client{Transport: bearerTransport{token: "secret-token"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "aw" {
		t.Fatalf("expected exactly the aw tool, got %+v", tools.Tools)
	}
	if tools.Tools[0].Description != "test aw dispatcher" {
		t.Fatalf("unexpected tool description %q", tools.Tools[0].Description)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "aw",
		Arguments: map[string]any{"action": "app.state", "args": ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("expected success, got tool error: %+v", result.Content)
	}
	if backend.lastAction != "app.state" {
		t.Fatalf("expected backend to receive app.state, got %q", backend.lastAction)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || text.Text != `{"ok":true}` {
		t.Fatalf("unexpected tool result content: %+v", result.Content)
	}

	failed, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "aw",
		Arguments: map[string]any{"action": "boom"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !failed.IsError {
		t.Fatal("expected a tool error for a failing action")
	}
}

func TestIsLoopbackRemote(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:51234":   true,
		"[::1]:51234":       true,
		"192.168.1.5:51234": false,
		"10.0.0.2:80":       false,
		"not-an-address":    false,
	}
	for addr, want := range cases {
		if got := isLoopbackRemote(addr); got != want {
			t.Errorf("isLoopbackRemote(%q) = %v, want %v", addr, got, want)
		}
	}
}
