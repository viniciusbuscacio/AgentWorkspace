package appcore

import (
	"context"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
	"aw/internal/infrastructure/mcpclient"
)

// MCP Client panel bindings (Apps → MCP Client module). The UI receives
// SANITIZED DTOs only — never the bearer token or secret key. These share the
// same application use cases as the mcp.* aw actions, so the agent and the UI
// cannot drift. Token is accepted only as write-only input.

func (a *App) mcpRuntime() *mcpclient.Runtime { return mcpclient.New("agent-workspace") }

func (a *App) ListMcpConnections() dto.McpConnectionsResult {
	conns, err := application.ListMcpConnectionsUC(a.vault)
	if err != nil {
		return dto.McpConnectionsResult{Success: false, Error: err.Error(), Connections: []domain.McpConnection{}}
	}
	return dto.McpConnectionsResult{Success: true, Connections: conns}
}

func (a *App) GetMcpConnection(id string) dto.McpConnectionResult {
	conn, err := application.GetMcpConnectionUC(a.vault, id)
	if err != nil {
		return dto.McpConnectionResult{Success: false, Error: err.Error()}
	}
	return dto.McpConnectionResult{Success: true, Connection: &conn}
}

func (a *App) AddMcpConnection(name, url, transport, authType, token string, enabled bool) dto.McpConnectionResult {
	conn, err := application.AddMcpConnectionUC(a.vault, a.vault, domain.McpConnectionInput{
		Name: name, Enabled: enabled, Transport: transport, URL: url, AuthType: authType,
	}, token)
	if err != nil {
		return dto.McpConnectionResult{Success: false, Error: err.Error()}
	}
	return dto.McpConnectionResult{Success: true, Connection: &conn}
}

// UpdateMcpConnection takes the full edited values from the form. token is
// write-only: empty means "keep current secret"; clearToken removes it.
func (a *App) UpdateMcpConnection(id, name, url, authType string, enabled bool, token string, clearToken bool) dto.McpConnectionResult {
	patch := domain.McpConnectionPatch{Name: &name, URL: &url, AuthType: &authType, Enabled: &enabled}
	var tokenPtr *string
	if token != "" {
		tokenPtr = &token
	}
	conn, err := application.UpdateMcpConnectionUC(a.vault, a.vault, id, patch, tokenPtr, clearToken)
	if err != nil {
		return dto.McpConnectionResult{Success: false, Error: err.Error()}
	}
	return dto.McpConnectionResult{Success: true, Connection: &conn}
}

func (a *App) RemoveMcpConnection(id string) dto.McpConnectionResult {
	conn, err := application.RemoveMcpConnectionUC(a.vault, a.vault, id)
	if err != nil {
		return dto.McpConnectionResult{Success: false, Error: err.Error()}
	}
	return dto.McpConnectionResult{Success: true, Connection: &conn}
}

func (a *App) SetMcpConnectionEnabled(id string, enabled bool) dto.McpConnectionResult {
	conn, err := application.SetMcpConnectionEnabledUC(a.vault, id, enabled)
	if err != nil {
		return dto.McpConnectionResult{Success: false, Error: err.Error()}
	}
	return dto.McpConnectionResult{Success: true, Connection: &conn}
}

func (a *App) TestMcpConnection(id string) dto.McpConnectionTestResult {
	res, err := application.TestMcpConnectionUC(context.Background(), a.vault, a.vault, a.mcpRuntime(), id)
	if err != nil {
		return dto.McpConnectionTestResult{Success: false, Error: err.Error()}
	}
	return dto.McpConnectionTestResult{Success: true, Result: &res}
}

func (a *App) ListMcpTools(id string) dto.McpToolsResult {
	res, err := application.ListMcpToolsUC(context.Background(), a.vault, a.vault, a.mcpRuntime(), id)
	if err != nil {
		return dto.McpToolsResult{Success: false, Error: err.Error(), Tools: []dto.McpToolView{}}
	}
	tools := make([]dto.McpToolView, 0, len(res.Tools))
	for _, t := range res.Tools {
		tools = append(tools, dto.McpToolView{Name: t.Name, Description: t.Description})
	}
	return dto.McpToolsResult{Success: true, ConnectionID: res.ConnectionID, Tools: tools, Truncated: res.Truncated}
}
