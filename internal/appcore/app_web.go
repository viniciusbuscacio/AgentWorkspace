package appcore

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"time"

	"aw/internal/application"
	"aw/internal/dto"
	"aw/internal/infrastructure/appconfig"
	"aw/internal/infrastructure/webserver"
)

// app_web.go wires the shared webserver package into the desktop GUI. Unlike the
// REST/MCP servers it has a lifecycle independent of the vault lock: it starts
// on startup when "web enabled" is set in config.json (readable pre-unlock) and
// is NOT stopped on LockVault — otherwise a remote browser could never log in to
// re-unlock. While the vault is locked, /bridge and /events refuse; /login stays
// live. Sessions die for real on lock/autolock/regenerate via an epoch counter.

// broadcastWebEvent forwards an event to all connected web browsers. Safe with
// a nil hub and before any client connects.
func (a *App) broadcastWebEvent(name string, payload map[string]any) {
	if a == nil || a.webHub == nil {
		return
	}
	a.webHub.Broadcast(webserver.Event{Name: name, Payload: payload})
}

// SetWebAssets injects the embedded React frontend served in web mode. The host
// (GUI shell or awd) owns the //go:embed and passes the sub-FS in, since embed
// is relative to the embedding file's directory and this package has none.
func (a *App) SetWebAssets(f fs.FS) { a.webAssetsFS = f }

// webAssets returns the injected frontend FS for the web server.
func (a *App) webAssets() (fs.FS, error) {
	if a.webAssetsFS == nil {
		return nil, errors.New("web assets not configured")
	}
	return a.webAssetsFS, nil
}

// startWebServerIfEnabled starts the web server on startup when enabled.
func (a *App) startWebServerIfEnabled() {
	if !application.IsWebServerEnabled(appconfig.Store{}) {
		return
	}
	_ = a.startWebServer()
}

func (a *App) startWebServer() error {
	defer a.emitServersState()
	cfg := application.LoadWebServerConfig(appconfig.Store{})
	// Missing assets are not fatal: the server still serves /login and /healthz
	// (the SPA route returns 503). awd built without a staged frontend relies on
	// this; the GUI always injects the embed.
	dist, _ := a.webAssets()
	// Resolve the shared TLS config before locking: when HTTPS is enabled but no
	// usable certificate exists, fail closed rather than serve plaintext.
	tlsCfg, tlsErr := a.serverTLSConfig(cfg.TLSEnabledOrDefault())
	a.webMu.Lock()
	defer a.webMu.Unlock()
	if a.webServer != nil {
		return nil
	}
	if tlsErr != nil {
		a.webErr = tlsErr.Error()
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:        "module.start_failed",
			Severity:     "error",
			Source:       "module",
			ModuleID:     "web",
			ModuleType:   "api_server",
			Message:      "Web server failed to start (TLS enabled but no certificate)",
			Status:       "error",
			ErrorMessage: tlsErr.Error(),
		})
		return tlsErr
	}
	server, err := webserver.NewServer(webserver.Options{
		Assets:            dist,
		Hub:               a.webHub,
		Authenticator:     webAuthenticator{app: a},
		Setup:             webSetup{app: a},
		Port:              cfg.PortOrDefault(),
		BindMode:          cfg.BindModeOrDefault(),
		BindAddr:          cfg.BindAddr,
		AllowedCIDRs:      cfg.AllowedCIDRs,
		SessionTTLMinutes: cfg.SessionTTLOrZero(),
		TLS:               tlsCfg,
		App:               a,
	})
	if err != nil {
		a.webErr = err.Error()
		a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
			Event:        "module.start_failed",
			Severity:     "error",
			Source:       "module",
			ModuleID:     "web",
			ModuleType:   "api_server",
			Message:      "Web server failed to start",
			Status:       "error",
			ErrorMessage: err.Error(),
		})
		return err
	}
	a.webServer = server
	a.webErr = ""
	a.emitLogEvent(a.contextOrBackground(), application.LogEvent{
		Event:      "module.started",
		Severity:   "info",
		Source:     "module",
		ModuleID:   "web",
		ModuleType: "api_server",
		Message:    "Web server started",
		Status:     "ok",
		Attributes: map[string]any{"url": server.URL()},
	})
	return nil
}

// stopWebServer shuts the server down (explicit disable or app shutdown only —
// never on lock).
func (a *App) stopWebServer(ctx context.Context) {
	defer a.emitServersState()
	a.webMu.Lock()
	server := a.webServer
	a.webServer = nil
	a.webMu.Unlock()
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
		ModuleID:   "web",
		ModuleType: "api_server",
		Message:    "Web server stopped",
		Status:     "ok",
	})
}

func (a *App) restartWebServerIfRunning() {
	a.webMu.Lock()
	running := a.webServer != nil
	a.webMu.Unlock()
	if running {
		a.stopWebServer(a.ctx)
		_ = a.startWebServer()
	}
}

// --- session tokens ---------------------------------------------------------

func (a *App) currentWebEpoch() int64 {
	a.webMu.Lock()
	defer a.webMu.Unlock()
	return a.webSessionEpoch
}

// InvalidateWebSessions bumps the epoch so every existing token stops verifying.
// Called on lock, autolock and signing-key regeneration.
func (a *App) InvalidateWebSessions(string) {
	a.webMu.Lock()
	a.webSessionEpoch++
	a.webMu.Unlock()
}

func (a *App) webSigningKey() ([]byte, error) {
	key, err := application.WebServerSettings.EnsureToken(a.vault)
	if err != nil {
		return nil, err
	}
	return []byte(key), nil
}

func (a *App) mintWebSession() (string, error) {
	key, err := a.webSigningKey()
	if err != nil {
		return "", err
	}
	cfg := application.LoadWebServerConfig(appconfig.Store{})
	now := time.Now()
	var exp int64
	if ttl := cfg.SessionTTLOrZero(); ttl > 0 {
		exp = now.Add(time.Duration(ttl) * time.Minute).Unix()
	}
	return webserver.SignSession(key, a.currentWebEpoch(), now.Unix(), exp), nil
}

func (a *App) validWebSession(token string) bool {
	key, err := a.webSigningKey()
	if err != nil {
		return false
	}
	epoch, ok := webserver.VerifySession(key, token)
	// Signature/expiry valid AND minted under the current epoch (lock, autolock
	// and key regeneration all bump the epoch, killing older tokens).
	return ok && epoch == a.currentWebEpoch()
}

// --- webserver.Authenticator ------------------------------------------------

type webAuthenticator struct{ app *App }

func (w webAuthenticator) VerifyOrUnlock(password string) (string, error) {
	if application.IsVaultUnlocked(w.app.vault) {
		if err := application.VerifyVaultPassword(w.app.vault, password); err != nil {
			return "", err
		}
	} else {
		res := w.app.UnlockVault(password)
		if !res.Success {
			if res.Error != "" {
				return "", errors.New(res.Error)
			}
			return "", errors.New("unlock failed")
		}
	}
	return w.app.mintWebSession()
}

func (w webAuthenticator) VaultUnlocked() bool              { return application.IsVaultUnlocked(w.app.vault) }
func (w webAuthenticator) ValidSession(token string) bool   { return w.app.validWebSession(token) }
func (w webAuthenticator) InvalidateSessions(reason string) { w.app.InvalidateWebSessions(reason) }
func (w webAuthenticator) RecordActivity()                  { w.app.recordActivity() }

// --- webserver.Setup --------------------------------------------------------

type webSetup struct{ app *App }

func (w webSetup) Status() webserver.SetupStatus {
	cfg := application.LoadWebServerConfig(appconfig.Store{})
	return webserver.SetupStatus{
		VaultExists: application.VaultExists(w.app.vault),
		Unlocked:    application.IsVaultUnlocked(w.app.vault),
		BindMode:    cfg.BindModeOrDefault(),
	}
}

func (w webSetup) CreateVault(password string) (string, error) {
	res := w.app.CreateVault(password)
	if !res.Success {
		return "", errors.New(nonEmpty(res.Error, "vault creation failed"))
	}
	return w.app.mintWebSession()
}

func (w webSetup) RecoverVault(recoveryKey, newPassword string) (string, error) {
	res := w.app.RecoverVault(recoveryKey, newPassword)
	if !res.Success {
		return "", errors.New(nonEmpty(res.Error, "vault recovery failed"))
	}
	return w.app.mintWebSession()
}

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// --- bound settings methods -------------------------------------------------

// GetWebServerStatus reports settings and runtime state for the settings page.
func (a *App) GetWebServerStatus() dto.WebServerStatus {
	cfg := application.LoadWebServerConfig(appconfig.Store{})
	a.webMu.Lock()
	server := a.webServer
	errMsg := a.webErr
	a.webMu.Unlock()
	status := dto.WebServerStatus{
		Success:           true,
		Error:             errMsg,
		Enabled:           application.IsWebServerEnabled(appconfig.Store{}),
		Running:           server != nil,
		Port:              cfg.PortOrDefault(),
		BindMode:          cfg.BindModeOrDefault(),
		BindAddr:          cfg.BindAddr,
		AllowedCIDRs:      cfg.AllowedCIDRs,
		SessionTTLMinutes: cfg.SessionTTLOrZero(),
	}
	if ip, err := webserver.DetectTailscaleIP(); err == nil {
		status.TailscaleIP = ip
		status.TailscaleDetected = true
	}
	if server != nil {
		status.URL = server.URL()
	}
	status.TLSEnabled = cfg.TLSEnabledOrDefault()
	coverageHost := cfg.BindAddr
	if cfg.BindModeOrDefault() == webserver.BindModeTailscale {
		coverageHost = status.TailscaleIP
	}
	status.CoverageWarning = a.serverTLSCoverageWarning(status.TLSEnabled, coverageHost)
	return status
}

// ListWebServerInterfaces lists local IPv4 bind candidates for the manual-mode
// interface picker in Settings, classified by reachability (loopback / private /
// tailscale / public) so the UI can warn proportionally. Read-only local
// enumeration; no external I/O.
func (a *App) ListWebServerInterfaces() dto.WebBindInterfacesResult {
	candidates, err := webserver.ListBindCandidates()
	if err != nil {
		return dto.WebBindInterfacesResult{Success: false, Error: err.Error()}
	}
	out := make([]dto.WebBindCandidate, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, dto.WebBindCandidate{Iface: c.Iface, IP: c.IP, Kind: c.Kind})
	}
	return dto.WebBindInterfacesResult{Success: true, Candidates: out}
}

// StartWebServer starts the server immediately (settings action).
func (a *App) StartWebServer() dto.WebServerStatus {
	if err := a.startWebServer(); err != nil {
		status := a.GetWebServerStatus()
		status.Success = false
		status.Error = err.Error()
		return status
	}
	return a.GetWebServerStatus()
}

// StopWebServer stops the server immediately (settings action).
func (a *App) StopWebServer() dto.WebServerStatus {
	a.stopWebServer(a.ctx)
	return a.GetWebServerStatus()
}

// webStatusErr builds an error status without touching the running server.
func (a *App) webStatusErr(msg string) dto.WebServerStatus {
	status := a.GetWebServerStatus()
	status.Success = false
	status.Error = msg
	return status
}

// SetWebServerEnabled persists the auto-start preference (start the web server
// on app launch and keep it up across vault lock/unlock). It does NOT start or
// stop the running server now — that is the Start now / Stop now action
// (StartWebServer / StopWebServer). This mirrors the MCP/REST auto-start switch,
// which only saves the preference: the switch is edited as a draft in Settings
// and persisted with Save, while Start/Stop control the live server.
func (a *App) SetWebServerEnabled(enabled bool) dto.WebServerStatus {
	if err := application.SetWebServerEnabled(appconfig.Store{}, enabled); err != nil {
		return a.webStatusErr(err.Error())
	}
	return a.GetWebServerStatus()
}

// SetWebServerPort persists the port and bounces a running server.
func (a *App) SetWebServerPort(port int) dto.WebServerStatus {
	if err := application.SetWebServerPort(appconfig.Store{}, port); err != nil {
		return a.webStatusErr(err.Error())
	}
	a.restartWebServerIfRunning()
	return a.GetWebServerStatus()
}

// SetWebServerBindMode persists the bind mode/addr/CIDRs and bounces a running
// server.
func (a *App) SetWebServerBindMode(mode, bindAddr string, allowedCIDRs []string) dto.WebServerStatus {
	if err := application.SetWebServerBind(appconfig.Store{}, strings.TrimSpace(mode), strings.TrimSpace(bindAddr), allowedCIDRs); err != nil {
		return a.webStatusErr(err.Error())
	}
	a.restartWebServerIfRunning()
	return a.GetWebServerStatus()
}

// SetWebServerSessionTTL persists the session lifetime in minutes (0 = no time
// expiry). Existing sessions keep their already-minted expiry.
func (a *App) SetWebServerSessionTTL(minutes int) dto.WebServerStatus {
	if err := application.SetWebServerSessionTTL(appconfig.Store{}, minutes); err != nil {
		return a.webStatusErr(err.Error())
	}
	return a.GetWebServerStatus()
}

// RegenerateWebServerSessionKey rotates the signing key and invalidates every
// live session.
func (a *App) RegenerateWebServerSessionKey() dto.WebServerStatus {
	if _, err := application.WebServerSettings.RegenerateToken(a.vault); err != nil {
		status := a.GetWebServerStatus()
		status.Success = false
		status.Error = err.Error()
		return status
	}
	a.InvalidateWebSessions("regenerate")
	return a.GetWebServerStatus()
}
