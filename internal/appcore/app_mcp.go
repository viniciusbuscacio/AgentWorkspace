package appcore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
	"aw/internal/infrastructure/agentfw"
	"aw/internal/infrastructure/appconfig"
	"aw/internal/infrastructure/mcpserver"
)

// awBackend adapts the App (interface/composition root) to the
// mcpserver.Backend and restserver.Backend ports (they are shape-identical),
// routing external tool calls into the same aw action registry the in-app
// agent uses — confirmation flow included, so destructive actions still
// require human approval in the aw window.
type awBackend struct {
	app *App
}

func (b awBackend) CallAw(ctx context.Context, action string, argsJSON string) (string, error) {
	dispatcher := b.app.awDispatch
	if dispatcher == nil {
		return "", errors.New("aw dispatcher is not available")
	}
	return dispatcher.Call(ctx, action, argsJSON)
}

func (b awBackend) AwDescription() string {
	dispatcher := b.app.awDispatch
	if dispatcher == nil {
		return "Control dispatcher for Agent Workspace."
	}
	return dispatcher.Description()
}

// startMcpServerIfEnabled starts the MCP server after an unlock when the
// auto-start toggle is on. Failures land in mcpErr for the settings page —
// and are retried briefly in the background (see retryRestServerStart; the
// typical failure is the previous instance holding the port on restart).
func (a *App) startMcpServerIfEnabled() {
	if !application.McpServerSettings.Autostart(a.vault) {
		return
	}
	if err := a.startMcpServer(); err != nil {
		go a.retryMcpServerStart()
	}
}

// retryMcpServerStart mirrors retryRestServerStart for the MCP server.
func (a *App) retryMcpServerStart() {
	a.mcpMu.Lock()
	if a.mcpRetrying {
		a.mcpMu.Unlock()
		return
	}
	a.mcpRetrying = true
	a.mcpMu.Unlock()
	defer func() {
		a.mcpMu.Lock()
		a.mcpRetrying = false
		a.mcpMu.Unlock()
	}()
	for attempt := 0; attempt < 12; attempt++ {
		time.Sleep(5 * time.Second)
		if !application.McpServerSettings.Autostart(a.vault) {
			return
		}
		if err := a.startMcpServer(); err == nil {
			return
		}
	}
}

func (a *App) startMcpServer() error {
	defer a.emitServersState()
	token, tokenErr := application.McpServerSettings.EnsureToken(a.vault)
	port := application.McpServerSettings.Port(a.vault)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	started := time.Now()
	a.mcpMu.Lock()
	defer a.mcpMu.Unlock()
	// Every start attempt re-derives the blocked-by-firewall want; only the
	// ErrNoPermittedInterface branch below re-arms it.
	a.mcpFirewallBlocked = false
	if tokenErr != nil {
		a.mcpErr = tokenErr.Error()
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:        "module.start_failed",
			Severity:     "error",
			Source:       "module",
			ModuleID:     "mcp",
			ModuleType:   "api_server",
			Message:      "MCP server failed to start",
			Status:       "error",
			ErrorMessage: tokenErr.Error(),
		})
		return tokenErr
	}
	if a.mcpServer != nil {
		return nil
	}
	tlsCfg, tlsErr := a.serverTLSConfig(application.McpServerSettings.TLSEnabled(a.vault))
	if tlsErr != nil {
		a.mcpErr = tlsErr.Error()
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:        "module.start_failed",
			Severity:     "error",
			Source:       "module",
			ModuleID:     "mcp",
			ModuleType:   "api_server",
			Message:      "MCP server failed to start (TLS enabled but no certificate)",
			Status:       "error",
			ErrorMessage: tlsErr.Error(),
		})
		return tlsErr
	}
	server, err := mcpserver.NewServer(awBackend{app: a}, mcpserver.Config{
		Service:      domain.FirewallServiceMCP,
		Rules:        application.LoadFirewallRules(appconfig.Store{}),
		FirewallPort: port,
		Token:        token,
		TLS:          tlsCfg,
	})
	if errors.Is(err, agentfw.ErrNoPermittedInterface) {
		a.mcpErr = firewallBlockedMessage("MCP")
		a.mcpFirewallBlocked = true
		return nil
	}
	if err != nil {
		a.mcpErr = err.Error()
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:        "module.start_failed",
			Severity:     "error",
			Source:       "module",
			ModuleID:     "mcp",
			ModuleType:   "api_server",
			Message:      "MCP server failed to start",
			Status:       "error",
			DurationMs:   time.Since(started).Milliseconds(),
			ErrorMessage: err.Error(),
			Attributes: map[string]any{
				"addr": addr,
			},
		})
		return err
	}
	a.mcpServer = server
	a.mcpErr = ""
	a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
		Event:      "module.started",
		Severity:   "info",
		Source:     "module",
		ModuleID:   "mcp",
		ModuleType: "api_server",
		Message:    "MCP server started",
		Status:     "ok",
		DurationMs: time.Since(started).Milliseconds(),
		Attributes: map[string]any{
			"addr": addr,
			"url":  server.URL(),
		},
	})
	return nil
}

// stopMcpServer shuts the MCP server down (vault lock, settings action, app
// exit).
func (a *App) stopMcpServer(ctx context.Context) {
	defer a.emitServersState()
	a.mcpMu.Lock()
	server := a.mcpServer
	a.mcpServer = nil
	// A stop withdraws the want-to-run; a follow-up start (bounce) re-arms the
	// blocked flag if the firewall still refuses it.
	a.mcpFirewallBlocked = false
	a.mcpMu.Unlock()
	if server == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	server.Shutdown(ctx)
	a.emitLogEvent(ctx, application.LogEvent{
		Event:      "module.stopped",
		Severity:   "info",
		Source:     "module",
		ModuleID:   "mcp",
		ModuleType: "api_server",
		Message:    "MCP server stopped",
		Status:     "ok",
	})
}

// restartMcpServerIfRunning applies new settings (port, token) to a live
// server by bouncing it.
func (a *App) restartMcpServerIfRunning() {
	a.mcpMu.Lock()
	running := a.mcpServer != nil
	a.mcpMu.Unlock()
	if running {
		a.stopMcpServer(a.ctx)
		_ = a.startMcpServer()
	}
}

// startMcpServerIfBlocked retries a start the firewall previously refused —
// called after a rule change or when a permitted interface appears, so a
// manually started server comes back without another Start click.
func (a *App) startMcpServerIfBlocked() {
	a.mcpMu.Lock()
	blocked := a.mcpFirewallBlocked
	a.mcpMu.Unlock()
	if blocked {
		_ = a.startMcpServer()
	}
}

// shutdown closes the background servers when the Wails app exits.
func (a *App) Shutdown(ctx context.Context) {
	a.emitLogEvent(ctx, application.LogEvent{
		Event:    "app.lifecycle.stopping",
		Severity: "info",
		Source:   "app",
		Message:  "app stopping",
		Status:   "ok",
	})
	a.closePipServer(ctx)
	a.stopMcpServer(ctx)
	a.stopRestServer(ctx)
	a.stopWebServer(ctx)
	// Final synchronous write-back (spec Q8): Lock snapshots the working copy
	// over the master so closing the app — even to swap machines — always
	// leaves the master fresh. No-op when already locked or in legacy mode.
	_ = application.LockVault(a.vault)
	a.emitLogEvent(ctx, application.LogEvent{
		Event:    "app.lifecycle.stopped",
		Severity: "info",
		Source:   "app",
		Message:  "app stopped",
		Status:   "ok",
	})
}

// GetMcpServerStatus reports the MCP server settings and runtime state for
// the settings page.
func (a *App) GetMcpServerStatus() dto.APIServerStatus {
	a.mcpMu.Lock()
	server := a.mcpServer
	errMsg := a.mcpErr
	a.mcpMu.Unlock()
	status := dto.APIServerStatus{
		Success:   true,
		Error:     errMsg,
		Autostart: application.McpServerSettings.Autostart(a.vault),
		Running:   server != nil,
		Port:      application.McpServerSettings.Port(a.vault),
	}
	var boundAddrs []string
	if server != nil {
		status.URL = server.URL()
		boundAddrs = server.BoundAddrs()
	}
	status.TLSEnabled = application.McpServerSettings.TLSEnabled(a.vault)
	status.CoverageWarning = a.serverTLSCoverageWarningForAddrs(status.TLSEnabled, boundAddrs)
	return status
}

// StartMcpServer starts the server immediately (settings page action).
func (a *App) StartMcpServer() dto.APIServerStatus {
	_ = a.startMcpServer()
	return a.GetMcpServerStatus()
}

// StopMcpServer stops the server immediately (settings page action). With
// auto-start on it will come back on the next vault unlock.
func (a *App) StopMcpServer() dto.APIServerStatus {
	a.stopMcpServer(a.ctx)
	return a.GetMcpServerStatus()
}

// SetMcpServerAutostart persists the auto-start toggle without touching the
// running server.
func (a *App) SetMcpServerAutostart(enabled bool) dto.APIServerStatus {
	if err := application.McpServerSettings.SetAutostart(a.vault, enabled); err != nil {
		status := a.GetMcpServerStatus()
		status.Success = false
		status.Error = err.Error()
		return status
	}
	return a.GetMcpServerStatus()
}

// SetMcpServerPort persists the listen port and restarts a running server on
// the new port.
func (a *App) SetMcpServerPort(port int) dto.APIServerStatus {
	if err := application.McpServerSettings.SetPort(a.vault, port); err != nil {
		status := a.GetMcpServerStatus()
		status.Success = false
		status.Error = err.Error()
		return status
	}
	a.restartMcpServerIfRunning()
	return a.GetMcpServerStatus()
}

// GetMcpServerToken returns the bearer token, generating it on first use so
// the settings page can always show a copyable value.
func (a *App) GetMcpServerToken() SecretResult {
	token, err := application.McpServerSettings.EnsureToken(a.vault)
	if err != nil {
		return SecretResult{Success: false, Error: err.Error()}
	}
	return SecretResult{Success: true, Value: token, Exists: true}
}

// RegenerateMcpServerToken rotates the bearer token and restarts the server
// when it is running, invalidating every connected MCP client.
func (a *App) RegenerateMcpServerToken() SecretResult {
	token, err := application.McpServerSettings.RegenerateToken(a.vault)
	if err != nil {
		return SecretResult{Success: false, Error: err.Error()}
	}
	a.restartMcpServerIfRunning()
	return SecretResult{Success: true, Value: token, Exists: true}
}
