package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// MCP Client use cases (Block B). This layer validates input, enforces the
// vault-unlocked gate, routes secrets to the credential store (never into the
// connection row), merges partial updates, and applies timeouts. It is UI-free:
// interactive confirmation lives in the tools/action layer. All returned DTOs
// are already sanitized by the store (no token, no secret key).

var errMcpDisabled = errors.New("connection is disabled; enable it before listing or calling tools")

// ListMcpConnectionsUC returns every configured connection (sanitized).
func ListMcpConnectionsUC(store ports.McpConnectionStore) ([]domain.McpConnection, error) {
	if store == nil {
		return nil, errors.New("mcp connection store is required")
	}
	if !store.IsUnlocked() {
		return nil, errVaultLocked
	}
	return store.ListMcpConnections()
}

// GetMcpConnectionUC returns one connection (sanitized).
func GetMcpConnectionUC(store ports.McpConnectionStore, id string) (domain.McpConnection, error) {
	if store == nil {
		return domain.McpConnection{}, errors.New("mcp connection store is required")
	}
	if !store.IsUnlocked() {
		return domain.McpConnection{}, errVaultLocked
	}
	conn, ok, err := store.GetMcpConnection(strings.TrimSpace(id))
	if err != nil {
		return domain.McpConnection{}, err
	}
	if !ok {
		return domain.McpConnection{}, fmt.Errorf("connection %q not found", id)
	}
	return conn, nil
}

// AddMcpConnectionUC validates and creates a connection, routing the optional
// write-only bearer token to the credential store.
func AddMcpConnectionUC(store ports.McpConnectionStore, creds ports.McpCredentialStore, in domain.McpConnectionInput, token string) (domain.McpConnection, error) {
	if store == nil || creds == nil {
		return domain.McpConnection{}, errors.New("mcp connection store is required")
	}
	if !store.IsUnlocked() {
		return domain.McpConnection{}, errVaultLocked
	}
	normalized, err := domain.NormalizeMcpInput(in)
	if err != nil {
		return domain.McpConnection{}, err
	}
	conn, err := store.CreateMcpConnection(normalized)
	if err != nil {
		return domain.McpConnection{}, err
	}
	if strings.TrimSpace(token) != "" {
		if err := creds.SetMcpConnectionSecret(conn.ID, token); err != nil {
			return domain.McpConnection{}, err
		}
		if refreshed, ok, gErr := store.GetMcpConnection(conn.ID); gErr == nil && ok {
			conn = refreshed
		}
	}
	return conn, nil
}

// UpdateMcpConnectionUC applies a partial update and optionally replaces or
// clears the stored token. token and clearToken are mutually exclusive.
func UpdateMcpConnectionUC(store ports.McpConnectionStore, creds ports.McpCredentialStore, id string, patch domain.McpConnectionPatch, token *string, clearToken bool) (domain.McpConnection, error) {
	if store == nil || creds == nil {
		return domain.McpConnection{}, errors.New("mcp connection store is required")
	}
	if !store.IsUnlocked() {
		return domain.McpConnection{}, errVaultLocked
	}
	if token != nil && strings.TrimSpace(*token) != "" && clearToken {
		return domain.McpConnection{}, errors.New("token and clearToken cannot both be set")
	}
	if patch.Name != nil {
		name, err := domain.ValidateMcpName(*patch.Name)
		if err != nil {
			return domain.McpConnection{}, err
		}
		patch.Name = &name
	}
	if patch.URL != nil {
		clean, err := domain.ValidateMcpURL(*patch.URL)
		if err != nil {
			return domain.McpConnection{}, err
		}
		patch.URL = &clean
	}
	if patch.AuthType != nil && !domain.IsValidMcpAuthType(*patch.AuthType) {
		return domain.McpConnection{}, fmt.Errorf("unsupported authType %q (v1 supports none|bearer)", *patch.AuthType)
	}
	conn, err := store.UpdateMcpConnection(strings.TrimSpace(id), patch)
	if err != nil {
		return domain.McpConnection{}, err
	}
	switch {
	case clearToken:
		if err := creds.DeleteMcpConnectionSecret(conn.ID); err != nil {
			return domain.McpConnection{}, err
		}
	case token != nil && strings.TrimSpace(*token) != "":
		if err := creds.SetMcpConnectionSecret(conn.ID, *token); err != nil {
			return domain.McpConnection{}, err
		}
	}
	if refreshed, ok, gErr := store.GetMcpConnection(conn.ID); gErr == nil && ok {
		conn = refreshed
	}
	return conn, nil
}

// RemoveMcpConnectionUC deletes a connection and its stored secret, echoing the
// sanitized deleted connection.
func RemoveMcpConnectionUC(store ports.McpConnectionStore, creds ports.McpCredentialStore, id string) (domain.McpConnection, error) {
	if store == nil || creds == nil {
		return domain.McpConnection{}, errors.New("mcp connection store is required")
	}
	if !store.IsUnlocked() {
		return domain.McpConnection{}, errVaultLocked
	}
	conn, ok, err := store.GetMcpConnection(strings.TrimSpace(id))
	if err != nil {
		return domain.McpConnection{}, err
	}
	if !ok {
		return domain.McpConnection{}, fmt.Errorf("connection %q not found", id)
	}
	// Best-effort secret cleanup first; the row delete is the source of truth.
	_ = creds.DeleteMcpConnectionSecret(conn.ID)
	if err := store.DeleteMcpConnection(conn.ID); err != nil {
		return domain.McpConnection{}, err
	}
	return conn, nil
}

// SetMcpConnectionEnabledUC toggles a connection on/off.
func SetMcpConnectionEnabledUC(store ports.McpConnectionStore, id string, enabled bool) (domain.McpConnection, error) {
	if store == nil {
		return domain.McpConnection{}, errors.New("mcp connection store is required")
	}
	if !store.IsUnlocked() {
		return domain.McpConnection{}, errVaultLocked
	}
	return store.SetMcpConnectionEnabled(strings.TrimSpace(id), enabled)
}

// TestMcpConnectionUC opens a per-operation session, runs the handshake +
// ListTools, persists the outcome, and returns it. A disabled connection can
// still be tested (a manual user verification before enabling).
func TestMcpConnectionUC(ctx context.Context, store ports.McpConnectionStore, creds ports.McpCredentialStore, runtime ports.McpClientRuntime, id string) (domain.McpConnectionTestResult, error) {
	conn, secret, err := resolveMcpConnection(store, creds, id)
	if err != nil {
		return domain.McpConnectionTestResult{}, err
	}
	opCtx, cancel := mcpOpContext(ctx)
	defer cancel()
	result, runErr := runtime.TestConnection(opCtx, conn, secret)
	result.ConnectionID = conn.ID
	errText := ""
	if runErr != nil {
		result.Status = domain.McpStatusError
		result.Error = sanitizeMcpError(runErr)
		errText = result.Error
	} else if result.Status == "" {
		result.Status = domain.McpStatusOK
	}
	// Persisting the outcome is best-effort: the test itself already ran.
	_, _ = store.RecordMcpConnectionTest(conn.ID, result.Status, errText, result.ToolCount)
	return result, nil
}

// ListMcpToolsUC lists a connection's remote tools (disabled connections are
// rejected). Truncation/size caps are enforced by the runtime adapter.
func ListMcpToolsUC(ctx context.Context, store ports.McpConnectionStore, creds ports.McpCredentialStore, runtime ports.McpClientRuntime, id string) (domain.McpToolListResult, error) {
	conn, secret, err := resolveMcpConnection(store, creds, id)
	if err != nil {
		return domain.McpToolListResult{}, err
	}
	if !conn.Enabled {
		return domain.McpToolListResult{}, errMcpDisabled
	}
	opCtx, cancel := mcpOpContext(ctx)
	defer cancel()
	result, runErr := runtime.ListTools(opCtx, conn, secret)
	if runErr != nil {
		return domain.McpToolListResult{}, fmt.Errorf("%s", sanitizeMcpError(runErr))
	}
	result.ConnectionID = conn.ID
	return result, nil
}

// CallMcpToolUC calls a remote tool (disabled connections are rejected). The
// result content is normalized + truncated by the runtime adapter.
func CallMcpToolUC(ctx context.Context, store ports.McpConnectionStore, creds ports.McpCredentialStore, runtime ports.McpClientRuntime, id, tool string, arguments map[string]any) (domain.McpToolCallResult, error) {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return domain.McpToolCallResult{}, errors.New("tool name is required")
	}
	conn, secret, err := resolveMcpConnection(store, creds, id)
	if err != nil {
		return domain.McpToolCallResult{}, err
	}
	if !conn.Enabled {
		return domain.McpToolCallResult{}, errMcpDisabled
	}
	opCtx, cancel := mcpOpContext(ctx)
	defer cancel()
	result, runErr := runtime.CallTool(opCtx, conn, secret, tool, arguments)
	result.ConnectionID = conn.ID
	result.Tool = tool
	if runErr != nil {
		result.Status = domain.McpCallStatusError
		if result.Content == nil {
			result.Content = []domain.McpContentBlock{{Type: "text", Text: sanitizeMcpError(runErr)}}
		}
		return result, nil // a remote error is a normalized result, not a Go error
	}
	if result.Status == "" {
		result.Status = domain.McpCallStatusSuccess
	}
	return result, nil
}

// resolveMcpConnection loads a connection and its secret, enforcing the
// vault-unlocked gate.
func resolveMcpConnection(store ports.McpConnectionStore, creds ports.McpCredentialStore, id string) (domain.McpConnection, string, error) {
	if store == nil || creds == nil {
		return domain.McpConnection{}, "", errors.New("mcp connection store is required")
	}
	if !store.IsUnlocked() {
		return domain.McpConnection{}, "", errVaultLocked
	}
	conn, ok, err := store.GetMcpConnection(strings.TrimSpace(id))
	if err != nil {
		return domain.McpConnection{}, "", err
	}
	if !ok {
		return domain.McpConnection{}, "", fmt.Errorf("connection %q not found", id)
	}
	secret := ""
	if conn.AuthType == domain.McpAuthBearer {
		if value, ok, gErr := creds.GetMcpConnectionSecret(conn.ID); gErr == nil && ok {
			secret = value
		}
	}
	return conn, secret, nil
}

func mcpOpContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, time.Duration(domain.McpDefaultTimeoutMs)*time.Millisecond)
}

// sanitizeMcpError returns a short, secret-free error string. It strips any
// occurrence of a bearer token shape just in case an SDK error echoed a header.
func sanitizeMcpError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 400 {
		msg = msg[:400] + "…"
	}
	return ScrubChatSecrets(msg)
}
