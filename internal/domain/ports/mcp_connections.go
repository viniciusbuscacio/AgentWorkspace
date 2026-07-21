package ports

import (
	"context"

	"aw/internal/domain"
)

// McpConnectionStore persists the MCP Client connection registry in the vault. It
// returns SANITIZED domain.McpConnection values (never the bearer token), and
// tracks HasSecret via the internal secret_key column.
type McpConnectionStore interface {
	VaultUnlockState
	CreateMcpConnection(in domain.McpConnectionInput) (domain.McpConnection, error)
	ListMcpConnections() ([]domain.McpConnection, error)
	GetMcpConnection(id string) (domain.McpConnection, bool, error)
	UpdateMcpConnection(id string, patch domain.McpConnectionPatch) (domain.McpConnection, error)
	DeleteMcpConnection(id string) error
	SetMcpConnectionEnabled(id string, enabled bool) (domain.McpConnection, error)
	// RecordMcpConnectionTest persists the outcome of a connection test
	// (status/error/checkedAt/toolCount) and returns the sanitized connection.
	RecordMcpConnectionTest(id, status, lastError string, toolCount int) (domain.McpConnection, error)
}

// McpCredentialStore stores per-connection bearer tokens in the vault secret
// store. Setting/clearing a secret also flips the connection's HasSecret flag.
type McpCredentialStore interface {
	SetMcpConnectionSecret(connectionID, value string) error
	GetMcpConnectionSecret(connectionID string) (string, bool, error)
	DeleteMcpConnectionSecret(connectionID string) error
}

// McpClientRuntime executes per-operation MCP sessions against a remote server.
// Implementations live in infrastructure/mcpclient and must NOT leak SDK structs
// upward: they return normalized domain DTOs and errors, open/close per call,
// honor the context timeout, and never log tokens/headers.
type McpClientRuntime interface {
	TestConnection(ctx context.Context, conn domain.McpConnection, secret string) (domain.McpConnectionTestResult, error)
	ListTools(ctx context.Context, conn domain.McpConnection, secret string) (domain.McpToolListResult, error)
	CallTool(ctx context.Context, conn domain.McpConnection, secret, tool string, arguments map[string]any) (domain.McpToolCallResult, error)
}
