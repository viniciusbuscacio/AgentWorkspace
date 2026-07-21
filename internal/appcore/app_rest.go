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
	"aw/internal/infrastructure/restserver"
)

// firewallBlockedMessage is the status shown when a server is enabled but no
// Agent Firewall rule permits it — the deny-all default. It is guidance, not an
// error.
func firewallBlockedMessage(service string) string {
	return "Blocked by Agent Firewall: no PERMIT rule for " + service +
		". Add one in Settings → Agent Firewall."
}

// The REST server mirrors the MCP server (app_mcp.go) for clients that do not
// speak MCP: same awBackend adapter, same vault-held settings shape, same
// lifecycle (up on unlock when auto-start is on, down on lock and shutdown).

// startRestServerIfEnabled starts the REST server after an unlock when the
// auto-start toggle is on. Failures land in restErr for the settings page —
// and are retried briefly in the background: the typical failure is the
// previous app instance still holding the port during a restart handoff,
// which used to leave the server down until a manual toggle.
func (a *App) startRestServerIfEnabled() {
	if !application.RestServerSettings.Autostart(a.vault) {
		return
	}
	if err := a.startRestServer(); err != nil {
		go a.retryRestServerStart()
	}
}

// retryRestServerStart retries a failed auto-start for up to a minute. The
// restRetrying flag keeps concurrent triggers (unlock, firewall reapply) from
// stacking loops; a successful start, a disabled toggle, or a firewall block
// (which returns nil and re-arms its own path) ends the loop.
func (a *App) retryRestServerStart() {
	a.restMu.Lock()
	if a.restRetrying {
		a.restMu.Unlock()
		return
	}
	a.restRetrying = true
	a.restMu.Unlock()
	defer func() {
		a.restMu.Lock()
		a.restRetrying = false
		a.restMu.Unlock()
	}()
	for attempt := 0; attempt < 12; attempt++ {
		time.Sleep(5 * time.Second)
		if !application.RestServerSettings.Autostart(a.vault) {
			return
		}
		if err := a.startRestServer(); err == nil {
			return
		}
	}
}

func (a *App) startRestServer() error {
	defer a.emitServersState()
	token, tokenErr := application.RestServerSettings.EnsureToken(a.vault)
	port := application.RestServerSettings.Port(a.vault)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	started := time.Now()
	a.restMu.Lock()
	defer a.restMu.Unlock()
	// Every start attempt re-derives the blocked-by-firewall want; only the
	// ErrNoPermittedInterface branch below re-arms it.
	a.restFirewallBlocked = false
	if tokenErr != nil {
		a.restErr = tokenErr.Error()
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:        "module.start_failed",
			Severity:     "error",
			Source:       "module",
			ModuleID:     "rest",
			ModuleType:   "api_server",
			Message:      "REST server failed to start",
			Status:       "error",
			ErrorMessage: tokenErr.Error(),
		})
		return tokenErr
	}
	if a.restServer != nil {
		return nil
	}
	tlsCfg, tlsErr := a.serverTLSConfig(application.RestServerSettings.TLSEnabled(a.vault))
	if tlsErr != nil {
		a.restErr = tlsErr.Error()
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:        "module.start_failed",
			Severity:     "error",
			Source:       "module",
			ModuleID:     "rest",
			ModuleType:   "api_server",
			Message:      "REST server failed to start (TLS enabled but no certificate)",
			Status:       "error",
			ErrorMessage: tlsErr.Error(),
		})
		return tlsErr
	}
	server, err := restserver.NewServer(awBackend{app: a}, restserver.Config{
		Service:      domain.FirewallServiceREST,
		Rules:        application.LoadFirewallRules(appconfig.Store{}),
		FirewallPort: port,
		Token:        token,
		TLS:          tlsCfg,
	})
	if errors.Is(err, agentfw.ErrNoPermittedInterface) {
		a.restErr = firewallBlockedMessage("REST")
		a.restFirewallBlocked = true
		return nil
	}
	if err != nil {
		a.restErr = err.Error()
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:        "module.start_failed",
			Severity:     "error",
			Source:       "module",
			ModuleID:     "rest",
			ModuleType:   "api_server",
			Message:      "REST server failed to start",
			Status:       "error",
			DurationMs:   time.Since(started).Milliseconds(),
			ErrorMessage: err.Error(),
			Attributes: map[string]any{
				"addr": addr,
			},
		})
		return err
	}
	a.restServer = server
	a.restErr = ""
	a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
		Event:      "module.started",
		Severity:   "info",
		Source:     "module",
		ModuleID:   "rest",
		ModuleType: "api_server",
		Message:    "REST server started",
		Status:     "ok",
		DurationMs: time.Since(started).Milliseconds(),
		Attributes: map[string]any{
			"addr": addr,
			"url":  server.URL(),
		},
	})
	return nil
}

// stopRestServer shuts the REST server down (vault lock, settings action, app
// exit).
func (a *App) stopRestServer(ctx context.Context) {
	defer a.emitServersState()
	a.restMu.Lock()
	server := a.restServer
	a.restServer = nil
	// A stop withdraws the want-to-run; a follow-up start (bounce) re-arms the
	// blocked flag if the firewall still refuses it.
	a.restFirewallBlocked = false
	a.restMu.Unlock()
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
		ModuleID:   "rest",
		ModuleType: "api_server",
		Message:    "REST server stopped",
		Status:     "ok",
	})
}

// restartRestServerIfRunning applies new settings (port, token) to a live
// server by bouncing it.
func (a *App) restartRestServerIfRunning() {
	a.restMu.Lock()
	running := a.restServer != nil
	a.restMu.Unlock()
	if running {
		a.stopRestServer(a.ctx)
		_ = a.startRestServer()
	}
}

// startRestServerIfBlocked retries a start the firewall previously refused —
// called after a rule change or when a permitted interface appears, so a
// manually started server comes back without another Start click.
func (a *App) startRestServerIfBlocked() {
	a.restMu.Lock()
	blocked := a.restFirewallBlocked
	a.restMu.Unlock()
	if blocked {
		_ = a.startRestServer()
	}
}

// GetRestServerStatus reports the REST server settings and runtime state for
// the settings page.
func (a *App) GetRestServerStatus() dto.APIServerStatus {
	a.restMu.Lock()
	server := a.restServer
	errMsg := a.restErr
	a.restMu.Unlock()
	status := dto.APIServerStatus{
		Success:   true,
		Error:     errMsg,
		Autostart: application.RestServerSettings.Autostart(a.vault),
		Running:   server != nil,
		Port:      application.RestServerSettings.Port(a.vault),
	}
	var boundAddrs []string
	if server != nil {
		status.URL = server.URL()
		boundAddrs = server.BoundAddrs()
	}
	status.TLSEnabled = application.RestServerSettings.TLSEnabled(a.vault)
	status.CoverageWarning = a.serverTLSCoverageWarningForAddrs(status.TLSEnabled, boundAddrs)
	return status
}

// StartRestServer starts the server immediately (settings page action).
func (a *App) StartRestServer() dto.APIServerStatus {
	_ = a.startRestServer()
	return a.GetRestServerStatus()
}

// StopRestServer stops the server immediately (settings page action). With
// auto-start on it will come back on the next vault unlock.
func (a *App) StopRestServer() dto.APIServerStatus {
	a.stopRestServer(a.ctx)
	return a.GetRestServerStatus()
}

// SetRestServerAutostart persists the auto-start toggle without touching the
// running server.
func (a *App) SetRestServerAutostart(enabled bool) dto.APIServerStatus {
	if err := application.RestServerSettings.SetAutostart(a.vault, enabled); err != nil {
		status := a.GetRestServerStatus()
		status.Success = false
		status.Error = err.Error()
		return status
	}
	return a.GetRestServerStatus()
}

// SetRestServerPort persists the listen port and restarts a running server on
// the new port.
func (a *App) SetRestServerPort(port int) dto.APIServerStatus {
	if err := application.RestServerSettings.SetPort(a.vault, port); err != nil {
		status := a.GetRestServerStatus()
		status.Success = false
		status.Error = err.Error()
		return status
	}
	a.restartRestServerIfRunning()
	return a.GetRestServerStatus()
}

// GetRestServerToken returns the bearer token, generating it on first use so
// the settings page can always show a copyable value.
func (a *App) GetRestServerToken() SecretResult {
	token, err := application.RestServerSettings.EnsureToken(a.vault)
	if err != nil {
		return SecretResult{Success: false, Error: err.Error()}
	}
	return SecretResult{Success: true, Value: token, Exists: true}
}

// RegenerateRestServerToken rotates the bearer token and restarts the server
// when it is running, invalidating every connected REST client.
func (a *App) RegenerateRestServerToken() SecretResult {
	token, err := application.RestServerSettings.RegenerateToken(a.vault)
	if err != nil {
		return SecretResult{Success: false, Error: err.Error()}
	}
	a.restartRestServerIfRunning()
	return SecretResult{Success: true, Value: token, Exists: true}
}
