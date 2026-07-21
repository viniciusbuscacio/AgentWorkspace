package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aw/internal/domain"
)

// McpConnectionFuncs are the injected MCP Client operations (Block B). The
// composition root wires them to application use cases over the vault store,
// credential store and the MCP client runtime — this package never touches the
// SDK or the vault directly. Registered only while the mcp-client module is
// added (structural fencing, like Notes/Tasks).
type McpConnectionFuncs struct {
	List       func(ctx context.Context) (any, error)
	Get        func(ctx context.Context, id string) (any, error)
	Add        func(ctx context.Context, in domain.McpConnectionInput, token string) (any, error)
	Update     func(ctx context.Context, id string, patch domain.McpConnectionPatch, token *string, clearToken bool) (any, error)
	Remove     func(ctx context.Context, id string) (any, error)
	SetEnabled func(ctx context.Context, id string, enabled bool) (any, error)
	Test       func(ctx context.Context, id string) (any, error)
	ListTools  func(ctx context.Context, connectionID string) (domain.McpToolListResult, error)
	CallTool   func(ctx context.Context, connectionID, tool string, arguments map[string]any) (domain.McpToolCallResult, error)
}

// registerMcpConnectionActions wires the mcp.* action group. The model-facing
// surface stays the single aw tool; remote MCP tools are NOT exposed as separate
// model tools — they are reached through these actions.
func registerMcpConnectionActions(reg map[string]AwActionHandler) {
	reg["mcp.connections.list"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		if w.mcpConnections == nil || w.mcpConnections.List == nil {
			return "", errUnavailable("mcp connections")
		}
		result, err := w.mcpConnections.List(ctx)
		if err != nil {
			return "", err
		}
		return awJSON(map[string]any{"connections": result})
	}

	reg["mcp.connections.get"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.mcpConnections == nil || w.mcpConnections.Get == nil {
			return "", errUnavailable("mcp connections")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.mcpConnections.Get(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(map[string]any{"connection": result})
	}

	reg["mcp.connections.add"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.mcpConnections == nil || w.mcpConnections.Add == nil {
			return "", errUnavailable("mcp connections")
		}
		name, err := awRequiredStringArg(args, "name")
		if err != nil {
			return "", err
		}
		url, err := awRequiredStringArg(args, "url")
		if err != nil {
			return "", err
		}
		enabled, err := awBoolArg(args, "enabled", true)
		if err != nil {
			return "", err
		}
		transport, _, err := awStringArg(args, "transport")
		if err != nil {
			return "", err
		}
		authType, _, err := awStringArg(args, "authType")
		if err != nil {
			return "", err
		}
		token, _, err := awStringArg(args, "token")
		if err != nil {
			return "", err
		}
		result, err := w.mcpConnections.Add(ctx, domain.McpConnectionInput{
			Name: name, Enabled: enabled, Transport: transport, URL: url, AuthType: authType,
		}, token)
		if err != nil {
			return "", err
		}
		return awJSON(map[string]any{"connection": result})
	}

	reg["mcp.connections.update"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.mcpConnections == nil || w.mcpConnections.Update == nil {
			return "", errUnavailable("mcp connections")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		name, hasName, err := awStringArg(args, "name")
		if err != nil {
			return "", err
		}
		url, hasURL, err := awStringArg(args, "url")
		if err != nil {
			return "", err
		}
		authType, hasAuth, err := awStringArg(args, "authType")
		if err != nil {
			return "", err
		}
		enabled, hasEnabled, err := awOptionalBoolArg(args, "enabled")
		if err != nil {
			return "", err
		}
		token, hasToken, err := awStringArg(args, "token")
		if err != nil {
			return "", err
		}
		clearToken, err := awBoolArg(args, "clearToken", false)
		if err != nil {
			return "", err
		}
		patch := domain.McpConnectionPatch{
			Name:     optionalArg(name, hasName),
			URL:      optionalArg(url, hasURL),
			AuthType: optionalArg(authType, hasAuth),
			Enabled:  optionalArg(enabled, hasEnabled),
		}
		result, err := w.mcpConnections.Update(ctx, id, patch, optionalArg(token, hasToken), clearToken)
		if err != nil {
			return "", err
		}
		return awJSON(map[string]any{"connection": result})
	}

	reg["mcp.connections.remove"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.mcpConnections == nil || w.mcpConnections.Remove == nil {
			return "", errUnavailable("mcp connections")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.mcpConnections.Remove(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(map[string]any{"removed": result})
	}

	reg["mcp.connections.set_enabled"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.mcpConnections == nil || w.mcpConnections.SetEnabled == nil {
			return "", errUnavailable("mcp connections")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		enabled, present, err := awOptionalBoolArg(args, "enabled")
		if err != nil {
			return "", err
		}
		if !present {
			return "", fmt.Errorf("enabled is required")
		}
		result, err := w.mcpConnections.SetEnabled(ctx, id, enabled)
		if err != nil {
			return "", err
		}
		return awJSON(map[string]any{"connection": result})
	}

	reg["mcp.connections.test"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.mcpConnections == nil || w.mcpConnections.Test == nil {
			return "", errUnavailable("mcp connections")
		}
		id, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		result, err := w.mcpConnections.Test(ctx, id)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	reg["mcp.tools.list"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.mcpConnections == nil || w.mcpConnections.ListTools == nil {
			return "", errUnavailable("mcp connections")
		}
		connectionID, err := awRequiredStringArg(args, "connectionId")
		if err != nil {
			return "", err
		}
		result, err := w.mcpConnections.ListTools(ctx, connectionID)
		if err != nil {
			return "", err
		}
		// Remote tool descriptions/schemas are untrusted external data.
		return w.mcpResultWithSafety(ctx, result, mcpToolListText(result),
			"tool_description", "mcp.tools.list:"+connectionID)
	}

	reg["mcp.tools.call"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		if w.mcpConnections == nil || w.mcpConnections.CallTool == nil {
			return "", errUnavailable("mcp connections")
		}
		connectionID, err := awRequiredStringArg(args, "connectionId")
		if err != nil {
			return "", err
		}
		tool, err := awRequiredStringArg(args, "tool")
		if err != nil {
			return "", err
		}
		arguments, err := awMapArg(args, "arguments")
		if err != nil {
			return "", err
		}
		// A remote tool that does not clearly read is treated as mutating: gate it
		// behind the strict external-side-effect confirmation (auto-approved only in
		// headless/no-confirmer contexts so external clients never hang).
		if domain.McpToolLooksMutating(tool) {
			approved, cErr := w.requireConfirmationStrict(ctx, ConfirmRequest{
				Tool:    "mcp.tools.call",
				Summary: fmt.Sprintf("Call remote MCP tool %q on connection %q", tool, connectionID),
				Args:    map[string]any{"connectionId": connectionID, "tool": tool},
			})
			if cErr != nil {
				return "", cErr
			}
			if !approved {
				return "", ErrConfirmationDenied
			}
		}
		result, err := w.mcpConnections.CallTool(ctx, connectionID, tool, arguments)
		if err != nil {
			return "", err
		}
		// Remote tool output is untrusted external content.
		return w.mcpResultWithSafety(ctx, result, mcpCallText(result),
			"tool_output", "mcp.tools.call:"+connectionID+"/"+tool)
	}
}

// mcpResultWithSafety serializes a remote MCP result and attaches external_safety
// metadata derived by running the textual payload through the external-content
// pipeline, so the agent always sees the untrusted-data notice.
func (w *workspace) mcpResultWithSafety(ctx context.Context, result any, text, sourceType, origin string) (string, error) {
	processed := w.processExternalContent(ctx, text, externalProcessOptions{
		SourceType: sourceType,
		Origin:     origin,
		Mode:       domain.ExternalContentModePreserveVerbatim,
		MaxChars:   domain.McpMaxOutputBytes,
	})
	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return "", err
	}
	m["external_safety"] = processed.ExternalSafety
	return awJSON(m)
}

func mcpToolListText(result domain.McpToolListResult) string {
	var b strings.Builder
	for _, t := range result.Tools {
		b.WriteString(t.Name)
		b.WriteString("\n")
		b.WriteString(t.Description)
		b.WriteString("\n")
		b.Write(t.InputSchema)
		b.WriteString("\n")
	}
	return b.String()
}

func mcpCallText(result domain.McpToolCallResult) string {
	var b strings.Builder
	for _, c := range result.Content {
		b.WriteString(c.Text)
		b.WriteString("\n")
	}
	return b.String()
}

// awMapArg parses an object argument (e.g. tool call arguments). Absent ⇒ nil.
func awMapArg(args map[string]any, name string) (map[string]any, error) {
	value, ok := args[name]
	if !ok || value == nil {
		return nil, nil
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON object", name)
	}
	return m, nil
}
