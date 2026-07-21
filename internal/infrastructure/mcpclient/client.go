// Package mcpclient is the infrastructure adapter that lets Agent Workspace act
// as an MCP CLIENT against external MCP servers (Block B of the MCP spec). It is
// the ONLY place the MCP Go SDK is used on the client side; it normalizes remote
// tools/results into pure domain DTOs and never leaks SDK structs upward.
//
// v1 supports Streamable HTTP + optional bearer auth. Every operation opens a
// session, does its work under the caller's context deadline, and closes — no
// background supervisor. Tokens and headers are never logged.
package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"aw/internal/domain"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Runtime implements ports.McpClientRuntime using the MCP Go SDK.
type Runtime struct {
	version string
}

// New builds the client runtime. version identifies this client to remote
// servers in the MCP handshake.
func New(version string) *Runtime {
	if version == "" {
		version = "dev"
	}
	return &Runtime{version: version}
}

// bearerRoundTripper injects the Authorization header on every request. It
// clones the request so it never mutates a shared one, and never logs the token.
type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (b bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	base := b.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}

// connect opens a per-operation MCP session. Bearer auth is injected only when
// the connection is configured for it AND a secret is present.
func (r *Runtime) connect(ctx context.Context, conn domain.McpConnection, secret string) (*mcp.ClientSession, error) {
	if !domain.IsValidMcpTransport(conn.Transport) {
		return nil, fmt.Errorf("unsupported transport %q", conn.Transport)
	}
	httpClient := http.DefaultClient
	if conn.AuthType == domain.McpAuthBearer && secret != "" {
		httpClient = &http.Client{Transport: bearerRoundTripper{token: secret, base: http.DefaultTransport}}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "agent-workspace", Version: r.version}, nil)
	transport := &mcp.StreamableClientTransport{Endpoint: conn.URL, HTTPClient: httpClient}
	return client.Connect(ctx, transport, nil)
}

// TestConnection opens a session, lists tools, and reports the outcome.
func (r *Runtime) TestConnection(ctx context.Context, conn domain.McpConnection, secret string) (domain.McpConnectionTestResult, error) {
	start := time.Now()
	session, err := r.connect(ctx, conn, secret)
	if err != nil {
		return domain.McpConnectionTestResult{Status: domain.McpStatusError, DurationMs: time.Since(start).Milliseconds()}, err
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	dur := time.Since(start).Milliseconds()
	if err != nil {
		return domain.McpConnectionTestResult{Status: domain.McpStatusError, DurationMs: dur}, err
	}
	return domain.McpConnectionTestResult{Status: domain.McpStatusOK, ToolCount: len(tools.Tools), DurationMs: dur}, nil
}

// ListTools fetches and normalizes the remote tool catalog, capped by count and
// serialized size (spec B5).
func (r *Runtime) ListTools(ctx context.Context, conn domain.McpConnection, secret string) (domain.McpToolListResult, error) {
	session, err := r.connect(ctx, conn, secret)
	if err != nil {
		return domain.McpToolListResult{}, err
	}
	defer session.Close()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		return domain.McpToolListResult{}, err
	}
	out := domain.McpToolListResult{ConnectionID: conn.ID, Tools: make([]domain.McpToolSummary, 0, len(listed.Tools))}
	bytes := 0
	for _, t := range listed.Tools {
		if t == nil {
			continue
		}
		if len(out.Tools) >= domain.McpMaxToolCount {
			out.Truncated = true
			break
		}
		summary := domain.McpToolSummary{Name: t.Name, Description: t.Description}
		if t.InputSchema != nil {
			if raw, mErr := json.Marshal(t.InputSchema); mErr == nil {
				summary.InputSchema = raw
			}
		}
		bytes += len(summary.Name) + len(summary.Description) + len(summary.InputSchema)
		if bytes > domain.McpMaxToolListBytes {
			out.Truncated = true
			break
		}
		out.Tools = append(out.Tools, summary)
	}
	return out, nil
}

// CallTool calls a remote tool and normalizes its content, truncating the total
// text to the output cap (spec B5). A remote-reported tool error is returned as
// a normalized result with status=error, not a Go error.
func (r *Runtime) CallTool(ctx context.Context, conn domain.McpConnection, secret, tool string, arguments map[string]any) (domain.McpToolCallResult, error) {
	session, err := r.connect(ctx, conn, secret)
	if err != nil {
		return domain.McpToolCallResult{}, err
	}
	defer session.Close()
	var args any
	if arguments != nil {
		args = arguments
	}
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return domain.McpToolCallResult{}, err
	}
	out := domain.McpToolCallResult{
		ConnectionID: conn.ID,
		Tool:         tool,
		Status:       domain.McpCallStatusSuccess,
		Content:      make([]domain.McpContentBlock, 0, len(res.Content)),
	}
	if res.IsError {
		out.Status = domain.McpCallStatusError
	}
	budget := domain.McpMaxOutputBytes
	for _, c := range res.Content {
		block := normalizeContent(c)
		if block.Type == "text" {
			truncated, didTrunc := domain.TruncateMcpText(block.Text, budget)
			block.Text = truncated
			budget -= len(block.Text)
			if didTrunc {
				out.Truncated = true
				out.Content = append(out.Content, block)
				break
			}
			if budget <= 0 {
				out.Truncated = true
			}
		}
		out.Content = append(out.Content, block)
		if budget <= 0 {
			break
		}
	}
	return out, nil
}

// normalizeContent maps one SDK content block to the domain block. Only text is
// surfaced; other modalities become a safe type marker (v1 does not return
// binary blobs to the agent).
func normalizeContent(c mcp.Content) domain.McpContentBlock {
	switch v := c.(type) {
	case *mcp.TextContent:
		return domain.McpContentBlock{Type: "text", Text: v.Text}
	case *mcp.ImageContent:
		return domain.McpContentBlock{Type: "image", Text: "[image content omitted]"}
	case *mcp.AudioContent:
		return domain.McpContentBlock{Type: "audio", Text: "[audio content omitted]"}
	default:
		return domain.McpContentBlock{Type: "unsupported", Text: "[non-text content omitted]"}
	}
}
