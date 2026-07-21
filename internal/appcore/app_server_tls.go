package appcore

import (
	"crypto/tls"
	"time"

	"aw/internal/application"
	"aw/internal/dto"
	"aw/internal/infrastructure/appconfig"
	"aw/internal/infrastructure/webserver"
)

// app_server_tls.go wires the shared server-TLS material into the web/MCP/REST
// server lifecycles and exposes the TLS manager + per-server HTTPS toggles to
// the frontend. The certificate is shared across all three; each server
// independently turns HTTPS on/off via its own toggle. No agent tool reads,
// installs or exports the certificate or key (decision #13).

// noTLSCertMessage is the actionable error returned when a server's HTTPS toggle
// is switched on with no certificate present (decision #3). The frontend pairs
// it with a deep-link to the TLS manager.
const noTLSCertMessage = "No TLS certificate yet — create one in the TLS manager"

// expiringSoonWindow is how close to expiry a certificate is flagged in status.
const expiringSoonWindow = 30 * 24 * time.Hour

// serverTLSConfig returns the shared *tls.Config when enabled is true, or nil
// when TLS is off (the unchanged plaintext path). When enabled but no usable
// certificate exists, it returns an error so the caller fails closed instead of
// silently serving plaintext on the HTTPS toggle.
func (a *App) serverTLSConfig(enabled bool) (*tls.Config, error) {
	if !enabled {
		return nil, nil
	}
	return application.ServerTLSConfigForActiveMode(a.tlsMaterial, appconfig.Store{})
}

// serverTLSCoverageWarning returns a human-readable warning when TLS is on but
// the shared certificate does not cover host, or "" otherwise (decision #4:
// warn, never block). A nil/absent certificate yields no warning here — the
// start path already fails closed on a missing bundle.
func (a *App) serverTLSCoverageWarning(enabled bool, host string) string {
	if !enabled || host == "" {
		return ""
	}
	covers, err := application.ServerTLSCovers(a.tlsMaterial, appconfig.Store{}, host)
	if err != nil || covers {
		return ""
	}
	return "The TLS certificate does not cover " + host + " — regenerate it in the TLS manager to include this address."
}

// serverTLSCoverageWarningForAddrs checks certificate coverage against every
// listen address a server actually bound (Option A can open several
// per-interface listeners) and returns the first warning. The 0.0.0.0 wildcard
// is skipped — it is not a client-facing name. An empty addrs list (server not
// running) falls back to the loopback default clients are shown.
func (a *App) serverTLSCoverageWarningForAddrs(enabled bool, addrs []string) string {
	if len(addrs) == 0 {
		return a.serverTLSCoverageWarning(enabled, "127.0.0.1")
	}
	for _, host := range application.HostsFromListenAddrs(addrs) {
		if host == "0.0.0.0" {
			continue
		}
		if warning := a.serverTLSCoverageWarning(enabled, host); warning != "" {
			return warning
		}
	}
	return ""
}

// tlsIdentityHints collects the bind identities a freshly generated self-signed
// certificate should cover beyond the always-included local set: the detected
// Tailscale IP and any manual web bind address.
func (a *App) tlsIdentityHints() []string {
	var hosts []string
	if ip, err := webserver.DetectTailscaleIP(); err == nil {
		hosts = append(hosts, ip)
	}
	if addr := application.LoadWebServerConfig(appconfig.Store{}).BindAddr; addr != "" {
		hosts = append(hosts, addr)
	}
	return hosts
}

// restartTLSServers bounces only the servers that are running with TLS enabled,
// so a certificate change takes effect without touching autostart, bind, port,
// tokens or the web session key (decision #10).
func (a *App) restartTLSServers() {
	if application.LoadWebServerConfig(appconfig.Store{}).TLSEnabledOrDefault() {
		a.restartWebServerIfRunning()
	}
	if application.McpServerSettings.TLSEnabled(a.vault) {
		a.restartMcpServerIfRunning()
	}
	if application.RestServerSettings.TLSEnabled(a.vault) {
		a.restartRestServerIfRunning()
	}
}

// --- TLS manager (Wails) ----------------------------------------------------

// GetServerTLSStatus returns the sanitized state of the shared certificate.
func (a *App) GetServerTLSStatus() dto.ServerTLSStatus {
	return a.serverTLSStatus("")
}

// serverTLSStatus assembles the sanitized status, optionally carrying an action
// error. It never includes the private key, PEM material or filesystem path.
func (a *App) serverTLSStatus(errMsg string) dto.ServerTLSStatus {
	status := dto.ServerTLSStatus{
		Success: errMsg == "",
		Error:   errMsg,
		Mode:    application.LoadServerTLSMode(appconfig.Store{}),
	}
	info, ok, err := application.ServerTLSInfo(a.tlsMaterial, appconfig.Store{})
	if err != nil {
		status.Success = false
		if status.Error == "" {
			status.Error = err.Error()
		}
		return status
	}
	status.HasCertificate = ok
	status.Ready = ok
	if ok {
		status.SelfSigned = info.SelfSigned
		status.Subject = info.Subject
		status.Issuer = info.Issuer
		status.NotBefore = info.NotBefore.Format(time.RFC3339)
		status.NotAfter = info.NotAfter.Format(time.RFC3339)
		status.FingerprintSHA256 = info.FingerprintSHA256
		status.DNSNames = info.DNSNames
		status.IPAddresses = info.IPAddresses
		status.ExpiringSoon = time.Until(info.NotAfter) <= expiringSoonWindow
	}
	return status
}

// CreateSelfSignedCertificate generates the app-managed self-signed certificate
// (decision #2) and switches the active mode to self_signed, then restarts the
// servers running with TLS.
func (a *App) CreateSelfSignedCertificate() dto.ServerTLSStatus {
	if _, err := application.CreateSelfSignedServerTLS(a.tlsMaterial, appconfig.Store{}, a.tlsIdentityHints()); err != nil {
		return a.serverTLSStatus(err.Error())
	}
	a.restartTLSServers()
	return a.GetServerTLSStatus()
}

// RegenerateSelfSignedCertificate replaces the self-signed key+cert with a fresh
// one covering the current identity union. Same operation as create; the UI
// distinguishes them so it can warn that the browser may re-prompt.
func (a *App) RegenerateSelfSignedCertificate() dto.ServerTLSStatus {
	return a.CreateSelfSignedCertificate()
}

// InstallCustomCertificate validates and installs a user-supplied PEM chain +
// key (decision #2/#9) and switches the active mode to custom. On any validation
// failure the current certificate and servers are left intact.
func (a *App) InstallCustomCertificate(certPEM string, keyPEM string) dto.ServerTLSStatus {
	if _, err := application.InstallCustomServerTLS(a.tlsMaterial, appconfig.Store{}, certPEM, keyPEM); err != nil {
		return a.serverTLSStatus(err.Error())
	}
	a.restartTLSServers()
	return a.GetServerTLSStatus()
}

// --- per-server HTTPS toggles (Wails) ---------------------------------------

// SetWebServerTLSEnabled persists the web HTTPS toggle and bounces a running
// server. Enabling with no certificate is refused with an actionable error
// (decision #3); the toggle stays off.
func (a *App) SetWebServerTLSEnabled(enabled bool) dto.WebServerStatus {
	if enabled && !application.HasServerTLSCertificate(a.tlsMaterial, appconfig.Store{}) {
		return a.webStatusErr(noTLSCertMessage)
	}
	if err := application.SetWebServerTLSEnabled(appconfig.Store{}, enabled); err != nil {
		return a.webStatusErr(err.Error())
	}
	a.restartWebServerIfRunning()
	return a.GetWebServerStatus()
}

// SetMcpServerTLSEnabled persists the MCP HTTPS toggle and bounces a running
// server. Enabling with no certificate is refused (decision #3).
func (a *App) SetMcpServerTLSEnabled(enabled bool) dto.APIServerStatus {
	if enabled && !application.HasServerTLSCertificate(a.tlsMaterial, appconfig.Store{}) {
		status := a.GetMcpServerStatus()
		status.Success = false
		status.Error = noTLSCertMessage
		return status
	}
	if err := application.McpServerSettings.SetTLSEnabled(a.vault, enabled); err != nil {
		status := a.GetMcpServerStatus()
		status.Success = false
		status.Error = err.Error()
		return status
	}
	a.restartMcpServerIfRunning()
	return a.GetMcpServerStatus()
}

// SetRestServerTLSEnabled persists the REST HTTPS toggle and bounces a running
// server. Enabling with no certificate is refused (decision #3).
func (a *App) SetRestServerTLSEnabled(enabled bool) dto.APIServerStatus {
	if enabled && !application.HasServerTLSCertificate(a.tlsMaterial, appconfig.Store{}) {
		status := a.GetRestServerStatus()
		status.Success = false
		status.Error = noTLSCertMessage
		return status
	}
	if err := application.RestServerSettings.SetTLSEnabled(a.vault, enabled); err != nil {
		status := a.GetRestServerStatus()
		status.Success = false
		status.Error = err.Error()
		return status
	}
	a.restartRestServerIfRunning()
	return a.GetRestServerStatus()
}
