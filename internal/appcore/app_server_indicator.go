package appcore

import "aw/internal/dto"

// The sidebar's ambient exposure indicator: one green dot that exists only
// while at least one NETWORK server (REST, MCP, Web) is actually listening.
// PiP is deliberately excluded — it is local window plumbing, not exposure.
// The frontend mirrors this through GetServerIndicator (initial pull) plus
// the servers:state event pushed after every start/stop, never by polling —
// same pattern as go-passwords' api:state dot.

// GetServerIndicator reports the live listening state of the three network
// servers for the sidebar dot and the Settings › Servers hub.
func (a *App) GetServerIndicator() dto.ServerIndicator {
	// Bare test/harness Apps have no vault; the status getters read settings
	// through it. No vault → nothing meaningful to report.
	if a.vault == nil {
		return dto.ServerIndicator{}
	}
	rest := a.GetRestServerStatus()
	mcp := a.GetMcpServerStatus()
	web := a.GetWebServerStatus()
	return dto.ServerIndicator{
		Rest: dto.ServerIndicatorEntry{Running: rest.Running, Port: rest.Port},
		Mcp:  dto.ServerIndicatorEntry{Running: mcp.Running, Port: mcp.Port},
		Web:  dto.ServerIndicatorEntry{Running: web.Running, Port: web.Port},
	}
}

// emitServersState pushes the aggregate state to the frontend. Called after
// every server start/stop — including failed starts, so the dot never lies.
func (a *App) emitServersState() {
	indicator := a.GetServerIndicator()
	a.emitChatEvent("servers:state", map[string]any{
		"rest": map[string]any{"running": indicator.Rest.Running, "port": indicator.Rest.Port},
		"mcp":  map[string]any{"running": indicator.Mcp.Running, "port": indicator.Mcp.Port},
		"web":  map[string]any{"running": indicator.Web.Running, "port": indicator.Web.Port},
	})
}
