package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"aw/internal/domain"
)

// fakeMcp implements McpConnectionStore + McpCredentialStore over in-memory maps.
type fakeMcp struct {
	unlocked bool
	conns    map[string]domain.McpConnection
	secrets  map[string]string
	seq      int
}

func newFakeMcp() *fakeMcp {
	return &fakeMcp{unlocked: true, conns: map[string]domain.McpConnection{}, secrets: map[string]string{}}
}

func (f *fakeMcp) IsUnlocked() bool { return f.unlocked }

func (f *fakeMcp) CreateMcpConnection(in domain.McpConnectionInput) (domain.McpConnection, error) {
	f.seq++
	id := fmt.Sprintf("mcpconn-%04x", f.seq)
	conn := domain.McpConnection{
		ID: id, Name: in.Name, Enabled: in.Enabled, Transport: in.Transport, URL: in.URL,
		AuthType: in.AuthType, LastStatus: domain.McpStatusUnknown, CreatedAt: "t0", UpdatedAt: "t0",
	}
	f.conns[id] = conn
	return conn, nil
}

func (f *fakeMcp) ListMcpConnections() ([]domain.McpConnection, error) {
	out := make([]domain.McpConnection, 0, len(f.conns))
	for _, c := range f.conns {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeMcp) GetMcpConnection(id string) (domain.McpConnection, bool, error) {
	c, ok := f.conns[id]
	return c, ok, nil
}

func (f *fakeMcp) UpdateMcpConnection(id string, patch domain.McpConnectionPatch) (domain.McpConnection, error) {
	c, ok := f.conns[id]
	if !ok {
		return domain.McpConnection{}, fmt.Errorf("not found")
	}
	if patch.Name != nil {
		c.Name = *patch.Name
	}
	if patch.Enabled != nil {
		c.Enabled = *patch.Enabled
	}
	if patch.URL != nil {
		c.URL = *patch.URL
	}
	if patch.AuthType != nil {
		c.AuthType = *patch.AuthType
	}
	f.conns[id] = c
	return c, nil
}

func (f *fakeMcp) DeleteMcpConnection(id string) error {
	delete(f.conns, id)
	delete(f.secrets, id)
	return nil
}

func (f *fakeMcp) SetMcpConnectionEnabled(id string, enabled bool) (domain.McpConnection, error) {
	c, ok := f.conns[id]
	if !ok {
		return domain.McpConnection{}, fmt.Errorf("not found")
	}
	c.Enabled = enabled
	f.conns[id] = c
	return c, nil
}

func (f *fakeMcp) RecordMcpConnectionTest(id, status, lastError string, toolCount int) (domain.McpConnection, error) {
	c, ok := f.conns[id]
	if !ok {
		return domain.McpConnection{}, fmt.Errorf("not found")
	}
	c.LastStatus, c.LastError, c.ToolCount, c.LastCheckedAt = status, lastError, toolCount, "t1"
	f.conns[id] = c
	return c, nil
}

func (f *fakeMcp) SetMcpConnectionSecret(id, value string) error {
	f.secrets[id] = value
	c := f.conns[id]
	c.HasSecret = true
	f.conns[id] = c
	return nil
}

func (f *fakeMcp) GetMcpConnectionSecret(id string) (string, bool, error) {
	v, ok := f.secrets[id]
	return v, ok, nil
}

func (f *fakeMcp) DeleteMcpConnectionSecret(id string) error {
	delete(f.secrets, id)
	c := f.conns[id]
	c.HasSecret = false
	f.conns[id] = c
	return nil
}

type fakeMcpRuntime struct {
	failTest   bool
	lastTool   string
	lastArgs   map[string]any
	lastSecret string
}

func (r *fakeMcpRuntime) TestConnection(_ context.Context, _ domain.McpConnection, secret string) (domain.McpConnectionTestResult, error) {
	r.lastSecret = secret
	if r.failTest {
		return domain.McpConnectionTestResult{Status: domain.McpStatusError}, errors.New("handshake failed Bearer abc123abc123abc123abc123")
	}
	return domain.McpConnectionTestResult{Status: domain.McpStatusOK, ToolCount: 2, DurationMs: 5}, nil
}

func (r *fakeMcpRuntime) ListTools(_ context.Context, _ domain.McpConnection, _ string) (domain.McpToolListResult, error) {
	return domain.McpToolListResult{Tools: []domain.McpToolSummary{{Name: "search"}, {Name: "create_item"}}}, nil
}

func (r *fakeMcpRuntime) CallTool(_ context.Context, _ domain.McpConnection, secret, tool string, args map[string]any) (domain.McpToolCallResult, error) {
	r.lastTool, r.lastArgs, r.lastSecret = tool, args, secret
	return domain.McpToolCallResult{Status: domain.McpCallStatusSuccess, Content: []domain.McpContentBlock{{Type: "text", Text: "ok"}}}, nil
}

func goodInput() domain.McpConnectionInput {
	return domain.McpConnectionInput{Name: "Learn", Enabled: true, Transport: domain.McpTransportStreamableHTTP, URL: "https://learn.microsoft.com/api/mcp", AuthType: domain.McpAuthNone}
}

func TestAddMcpValidatesAndGeneratesID(t *testing.T) {
	store := newFakeMcp()
	conn, err := AddMcpConnectionUC(store, store, goodInput(), "")
	if err != nil {
		t.Fatalf("add error = %v", err)
	}
	if !strings.HasPrefix(conn.ID, "mcpconn-") {
		t.Errorf("id should be app-generated, got %q", conn.ID)
	}
	if conn.HasSecret {
		t.Error("no-token connection should not have a secret")
	}
}

func TestAddMcpRejectsBadInput(t *testing.T) {
	store := newFakeMcp()
	for _, bad := range []domain.McpConnectionInput{
		{Name: "", URL: "https://x.test", AuthType: "none"},
		{Name: "x", URL: "ftp://x.test", AuthType: "none"},
		{Name: "x", URL: "", AuthType: "none"},
		{Name: "x", URL: "https://x.test", Transport: "stdio", AuthType: "none"},
		{Name: "x", URL: "https://x.test", AuthType: "oauth"},
		{Name: "x", URL: "file:///etc/passwd", AuthType: "none"},
	} {
		if _, err := AddMcpConnectionUC(store, store, bad, ""); err == nil {
			t.Errorf("expected rejection for %+v", bad)
		}
	}
}

func TestAddMcpStoresTokenWriteOnly(t *testing.T) {
	store := newFakeMcp()
	in := goodInput()
	in.AuthType = domain.McpAuthBearer
	conn, err := AddMcpConnectionUC(store, store, in, "super-secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if !conn.HasSecret {
		t.Error("connection with a token should report hasSecret=true")
	}
	// The sanitized DTO never carries the token (no field exists for it).
	if got, ok, _ := store.GetMcpConnectionSecret(conn.ID); !ok || got != "super-secret-token" {
		t.Error("token should be stored in the credential store")
	}
}

func TestUpdateMcpMergesAndClearsToken(t *testing.T) {
	store := newFakeMcp()
	in := goodInput()
	in.AuthType = domain.McpAuthBearer
	conn, _ := AddMcpConnectionUC(store, store, in, "tok")
	// token + clearToken together is rejected.
	tok := "new"
	if _, err := UpdateMcpConnectionUC(store, store, conn.ID, domain.McpConnectionPatch{}, &tok, true); err == nil {
		t.Error("token and clearToken together must be rejected")
	}
	// clearToken removes the secret.
	updated, err := UpdateMcpConnectionUC(store, store, conn.ID, domain.McpConnectionPatch{}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if updated.HasSecret {
		t.Error("clearToken should drop the secret")
	}
	if _, ok, _ := store.GetMcpConnectionSecret(conn.ID); ok {
		t.Error("secret should be deleted from the credential store")
	}
}

func TestRemoveMcpDeletesSecret(t *testing.T) {
	store := newFakeMcp()
	in := goodInput()
	in.AuthType = domain.McpAuthBearer
	conn, _ := AddMcpConnectionUC(store, store, in, "tok")
	if _, err := RemoveMcpConnectionUC(store, store, conn.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := store.GetMcpConnection(conn.ID); ok {
		t.Error("connection should be gone")
	}
	if _, ok, _ := store.GetMcpConnectionSecret(conn.ID); ok {
		t.Error("secret should be deleted with the connection")
	}
}

func TestDisabledConnectionCannotListOrCall(t *testing.T) {
	store := newFakeMcp()
	rt := &fakeMcpRuntime{}
	in := goodInput()
	in.Enabled = false
	conn, _ := AddMcpConnectionUC(store, store, in, "")
	if _, err := ListMcpToolsUC(context.Background(), store, store, rt, conn.ID); !errors.Is(err, errMcpDisabled) {
		t.Errorf("list on disabled = %v, want errMcpDisabled", err)
	}
	if _, err := CallMcpToolUC(context.Background(), store, store, rt, conn.ID, "search", nil); !errors.Is(err, errMcpDisabled) {
		t.Errorf("call on disabled = %v, want errMcpDisabled", err)
	}
}

func TestTestConnectionRecordsOutcome(t *testing.T) {
	store := newFakeMcp()
	conn, _ := AddMcpConnectionUC(store, store, goodInput(), "")
	res, err := TestMcpConnectionUC(context.Background(), store, store, &fakeMcpRuntime{}, conn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.McpStatusOK || res.ToolCount != 2 {
		t.Errorf("unexpected test result %+v", res)
	}
	saved, _, _ := store.GetMcpConnection(conn.ID)
	if saved.LastStatus != domain.McpStatusOK || saved.ToolCount != 2 {
		t.Errorf("test outcome not persisted: %+v", saved)
	}
}

func TestTestConnectionErrorIsSanitized(t *testing.T) {
	store := newFakeMcp()
	conn, _ := AddMcpConnectionUC(store, store, goodInput(), "")
	res, err := TestMcpConnectionUC(context.Background(), store, store, &fakeMcpRuntime{failTest: true}, conn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.McpStatusError {
		t.Errorf("expected error status, got %q", res.Status)
	}
	if strings.Contains(res.Error, "abc123abc123abc123abc123") {
		t.Errorf("error should be scrubbed of token-like values: %q", res.Error)
	}
}

func TestLockedVaultBlocksMcp(t *testing.T) {
	store := newFakeMcp()
	store.unlocked = false
	if _, err := ListMcpConnectionsUC(store); !errors.Is(err, errVaultLocked) {
		t.Errorf("list locked = %v, want errVaultLocked", err)
	}
}

func TestTruncateMcpText(t *testing.T) {
	s := strings.Repeat("a", 100)
	out, truncated := domain.TruncateMcpText(s, 50)
	if !truncated || !strings.HasSuffix(out, "[truncated]") {
		t.Errorf("expected truncation, got %q", out)
	}
	if out2, t2 := domain.TruncateMcpText("short", 50); t2 || out2 != "short" {
		t.Errorf("short text should not truncate, got %q", out2)
	}
}

func TestMcpToolLooksMutating(t *testing.T) {
	for _, ro := range []string{"search", "list_items", "get_doc", "read_file", "fetchData"} {
		if domain.McpToolLooksMutating(ro) {
			t.Errorf("%q should look read-only", ro)
		}
	}
	for _, mut := range []string{"create_item", "delete", "send_email", "", "do_thing"} {
		if !domain.McpToolLooksMutating(mut) {
			t.Errorf("%q should look mutating", mut)
		}
	}
}
