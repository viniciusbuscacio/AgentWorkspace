package vault

import (
	"testing"

	"aw/internal/domain"
)

func newUnlockedVault(t *testing.T) *Vault {
	t.Helper()
	v := New(t.TempDir())
	if _, err := v.Create("senha1234"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { _ = v.Lock() })
	return v
}

func TestMcpConnectionCrudRoundTrip(t *testing.T) {
	v := newUnlockedVault(t)

	conn, err := v.CreateMcpConnection(domain.McpConnectionInput{
		Name: "Learn", Enabled: true, Transport: domain.McpTransportStreamableHTTP,
		URL: "https://learn.microsoft.com/api/mcp", AuthType: domain.McpAuthBearer,
	})
	if err != nil {
		t.Fatalf("create error = %v", err)
	}
	if conn.ID == "" || conn.HasSecret {
		t.Fatalf("unexpected new connection %+v", conn)
	}

	// Secret set flips HasSecret without exposing the token in the DTO.
	if err := v.SetMcpConnectionSecret(conn.ID, "tok-123"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := v.GetMcpConnection(conn.ID)
	if err != nil || !ok {
		t.Fatalf("get error=%v ok=%v", err, ok)
	}
	if !got.HasSecret {
		t.Error("connection should report hasSecret after SetMcpConnectionSecret")
	}
	if value, ok, _ := v.GetMcpConnectionSecret(conn.ID); !ok || value != "tok-123" {
		t.Error("secret round-trip failed")
	}

	// Update merges partial fields.
	disabled := false
	if _, err := v.UpdateMcpConnection(conn.ID, domain.McpConnectionPatch{Enabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	got, _, _ = v.GetMcpConnection(conn.ID)
	if got.Enabled {
		t.Error("update should have disabled the connection")
	}
	if got.Name != "Learn" {
		t.Error("update should keep unset fields")
	}

	// Test outcome persists.
	if _, err := v.RecordMcpConnectionTest(conn.ID, domain.McpStatusOK, "", 3); err != nil {
		t.Fatal(err)
	}
	got, _, _ = v.GetMcpConnection(conn.ID)
	if got.LastStatus != domain.McpStatusOK || got.ToolCount != 3 {
		t.Errorf("test outcome not persisted: %+v", got)
	}

	// Delete removes the connection AND its secret.
	if err := v.DeleteMcpConnection(conn.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := v.GetMcpConnection(conn.ID); ok {
		t.Error("connection should be gone")
	}
	if _, ok, _ := v.GetMcpConnectionSecret(conn.ID); ok {
		t.Error("secret should be deleted with the connection")
	}
}

func TestMcpConnectionPersistsAcrossLock(t *testing.T) {
	v := newUnlockedVault(t)
	conn, err := v.CreateMcpConnection(domain.McpConnectionInput{
		Name: "Persist", Enabled: true, Transport: domain.McpTransportStreamableHTTP,
		URL: "https://example.test/mcp", AuthType: domain.McpAuthNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	// Locked vault rejects access.
	if _, err := v.ListMcpConnections(); err == nil {
		t.Error("locked vault should reject ListMcpConnections")
	}
	if err := v.Unlock("senha1234"); err != nil {
		t.Fatal(err)
	}
	list, err := v.ListMcpConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != conn.ID {
		t.Fatalf("connection did not survive lock cycle: %+v", list)
	}
}

func TestMcpClearSecretDropsHasSecret(t *testing.T) {
	v := newUnlockedVault(t)
	conn, _ := v.CreateMcpConnection(domain.McpConnectionInput{
		Name: "S", Enabled: true, Transport: domain.McpTransportStreamableHTTP,
		URL: "https://example.test/mcp", AuthType: domain.McpAuthBearer,
	})
	_ = v.SetMcpConnectionSecret(conn.ID, "x")
	if err := v.DeleteMcpConnectionSecret(conn.ID); err != nil {
		t.Fatal(err)
	}
	got, _, _ := v.GetMcpConnection(conn.ID)
	if got.HasSecret {
		t.Error("clearing the secret should drop hasSecret")
	}
}
