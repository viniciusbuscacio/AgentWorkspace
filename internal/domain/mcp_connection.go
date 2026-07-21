package domain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// MCP Client domain (Block B of the MCP spec). Pure types and validation
// for connecting Agent Workspace to external MCP servers as a client. NO SDK,
// vault, HTTP, process or Wails imports here — the MCP Go SDK stays in the
// infrastructure adapter. v1 is Streamable-HTTP + bearer only.

// MCP connection transports. v1 supports only Streamable HTTP; stdio/SSE are
// explicit non-goals (added later with their own validated fields).
const (
	McpTransportStreamableHTTP = "streamable_http"
)

// MCP connection auth types (v1).
const (
	McpAuthNone   = "none"
	McpAuthBearer = "bearer"
)

// MCP connection status tokens.
const (
	McpStatusUnknown  = "unknown"
	McpStatusDisabled = "disabled"
	McpStatusOK       = "ok"
	McpStatusError    = "error"
)

// Tool-call status tokens.
const (
	McpCallStatusSuccess = "success"
	McpCallStatusError   = "error"
)

// McpConnection is the SANITIZED representation of one configured external MCP
// server. It never carries the bearer token or the internal secret key — only
// HasSecret. This is what list/get/add/update/test/set_enabled return.
type McpConnection struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Transport     string `json:"transport"`
	URL           string `json:"url"`
	AuthType      string `json:"authType"`
	HasSecret     bool   `json:"hasSecret"`
	LastStatus    string `json:"lastStatus"`
	LastCheckedAt string `json:"lastCheckedAt"`
	LastError     string `json:"lastError"`
	ToolCount     int    `json:"toolCount"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// McpConnectionInput is the validated metadata for creating a connection. The
// bearer token is NOT here — it is routed separately to the credential store so
// secrets never sit in the connection row or flow through the connection store.
type McpConnectionInput struct {
	Name      string
	Enabled   bool
	Transport string
	URL       string
	AuthType  string
}

// McpConnectionPatch is a partial update; nil fields keep the current value.
type McpConnectionPatch struct {
	Name     *string
	Enabled  *bool
	URL      *string
	AuthType *string
}

// McpToolSummary is one normalized remote tool. Description and schema come from
// the remote server and are UNTRUSTED external data, not instructions.
type McpToolSummary struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// McpToolListResult is the normalized output of listing a connection's tools.
type McpToolListResult struct {
	ConnectionID string           `json:"connectionId"`
	Tools        []McpToolSummary `json:"tools"`
	Truncated    bool             `json:"truncated"`
}

// McpContentBlock is one normalized content block of a tool result. v1 only
// surfaces text; non-text blocks are summarized as a type marker.
type McpContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// McpToolCallResult is the normalized output of a remote tool call (before the
// tools layer attaches external_safety).
type McpToolCallResult struct {
	ConnectionID string            `json:"connectionId"`
	Tool         string            `json:"tool"`
	Status       string            `json:"status"`
	Content      []McpContentBlock `json:"content"`
	Truncated    bool              `json:"truncated"`
}

// McpConnectionTestResult is the outcome of a connection handshake + ListTools.
type McpConnectionTestResult struct {
	ConnectionID string `json:"connectionId"`
	Status       string `json:"status"`
	ToolCount    int    `json:"toolCount"`
	DurationMs   int64  `json:"durationMs"`
	Error        string `json:"error,omitempty"`
}

// v1 caps (spec B5). Enforced in the application/adapter so a remote server can
// never flood the agent context or hang it.
const (
	McpDefaultTimeoutMs = 30000
	McpMaxOutputBytes   = 32 * 1024 // remote tool output returned to the agent
	McpMaxToolCount     = 200       // remote tool list cap (count)
	McpMaxToolListBytes = 64 * 1024 // remote tool list cap (serialized bytes)
	McpMaxNameLen       = 80
	McpMaxURLLen        = 2048
)

// IsValidMcpTransport reports whether transport is supported in v1.
func IsValidMcpTransport(transport string) bool {
	return transport == McpTransportStreamableHTTP
}

// IsValidMcpAuthType reports whether authType is supported in v1.
func IsValidMcpAuthType(authType string) bool {
	return authType == McpAuthNone || authType == McpAuthBearer
}

// ValidateMcpName checks a user-facing connection name.
func ValidateMcpName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("connection name is required")
	}
	if len(name) > McpMaxNameLen {
		return "", fmt.Errorf("connection name is too long (max %d)", McpMaxNameLen)
	}
	return name, nil
}

// ValidateMcpURL enforces an http(s) absolute URL. It rejects empty, relative,
// file, shell, or command-like values so a connection can never become a path
// to local execution.
func ValidateMcpURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("connection url is required")
	}
	if len(raw) > McpMaxURLLen {
		return "", fmt.Errorf("connection url is too long")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("connection url is not a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("connection url must be http:// or https://")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("connection url must include a host")
	}
	return raw, nil
}

// NormalizeMcpInput validates and normalizes a create input.
func NormalizeMcpInput(in McpConnectionInput) (McpConnectionInput, error) {
	name, err := ValidateMcpName(in.Name)
	if err != nil {
		return McpConnectionInput{}, err
	}
	if in.Transport == "" {
		in.Transport = McpTransportStreamableHTTP
	}
	if !IsValidMcpTransport(in.Transport) {
		return McpConnectionInput{}, fmt.Errorf("unsupported transport %q (v1 supports %q only)", in.Transport, McpTransportStreamableHTTP)
	}
	if in.AuthType == "" {
		in.AuthType = McpAuthNone
	}
	if !IsValidMcpAuthType(in.AuthType) {
		return McpConnectionInput{}, fmt.Errorf("unsupported authType %q (v1 supports none|bearer)", in.AuthType)
	}
	cleanURL, err := ValidateMcpURL(in.URL)
	if err != nil {
		return McpConnectionInput{}, err
	}
	return McpConnectionInput{Name: name, Enabled: in.Enabled, Transport: in.Transport, URL: cleanURL, AuthType: in.AuthType}, nil
}

// TruncateMcpText truncates s to maxBytes, returning the (possibly) shortened
// string and whether it was truncated. Bytes (not runes) so the cap is a hard
// wire-size bound; a clean rune boundary is preserved.
func TruncateMcpText(s string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s, false
	}
	cut := maxBytes
	// back up to a valid UTF-8 boundary
	for cut > 0 && (s[cut]&0xC0) == 0x80 {
		cut--
	}
	return s[:cut] + "…[truncated]", true
}

// McpConnectionSecretKey is the deterministic internal vault key for a
// connection's bearer token. It is metadata only and never returned to UI/model.
func McpConnectionSecretKey(connectionID string) string {
	return "_mcp_connection_" + connectionID + "_token"
}

// mcpReadOnlyToolHints are name fragments that strongly suggest a remote tool
// only reads. Everything else is treated as potentially mutating, so the
// tool/action layer defaults to confirmation for unknown/mutating remote calls.
var mcpReadOnlyToolHints = []string{"search", "list", "get", "read", "fetch", "lookup", "find", "query", "describe", "show", "view"}

// McpToolLooksMutating is a conservative heuristic: a tool whose name does not
// clearly read is treated as mutating (default-to-confirmation). Remote tool
// names are untrusted, so this only informs the confirmation gate, never trust.
func McpToolLooksMutating(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return true
	}
	for _, hint := range mcpReadOnlyToolHints {
		if strings.Contains(lower, hint) {
			return false
		}
	}
	return true
}
