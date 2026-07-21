// Package webserver serves the full Agent Workspace React UI to a remote
// browser over HTTP. It is the shared web host used both by the desktop GUI
// (in-process, toggled in Settings) and by the headless awd daemon. The browser
// talks to the host *App through a generic reflection bridge (POST /bridge) and
// receives events over SSE (GET /events), so the same React app runs unchanged
// in the desktop window and in a remote tab.
//
// A package in the infrastructure layer cannot import package main, so the host
// *App is passed as interface{} and reached by reflection; auth/lifecycle is
// delegated to the Authenticator the composition root implements.
package webserver

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// BindMode selects how the listener is bound and how peers are enforced.
const (
	// BindModeTailscale binds only the tailnet IPv4 and rejects non-tailnet
	// peers. It is the recommended default: traffic rides WireGuard encryption.
	BindModeTailscale = "tailscale"
	// BindModeManual binds a user-chosen address with no tailnet enforcement
	// (optional allowedCIDRs). HTTP is plaintext; the user owns the risk.
	BindModeManual = "manual"
)

// Authenticator is implemented by the composition root (the *App owner). It
// owns the vault-held signing key and the session epoch, so the webserver never
// touches the vault directly.
type Authenticator interface {
	// VerifyOrUnlock authenticates password and returns a session token. When
	// the vault is locked it runs the full unlock lifecycle; when unlocked it
	// verifies the password explicitly (never a bare Unlock, which would accept
	// any password once open).
	VerifyOrUnlock(password string) (token string, err error)
	// VaultUnlocked reports whether the vault is currently unlocked.
	VaultUnlocked() bool
	// ValidSession reports whether token is a live session (signature, epoch
	// and optional expiry all valid).
	ValidSession(token string) bool
	// InvalidateSessions kills every existing session (lock/autolock/regen).
	InvalidateSessions(reason string)
	// RecordActivity resets the auto-lock window on authenticated web activity.
	RecordActivity()
}

// SetupStatus is the pre-auth state the web login/setup screen needs.
type SetupStatus struct {
	VaultExists bool   `json:"vaultExists"`
	Unlocked    bool   `json:"unlocked"`
	BindMode    string `json:"bindMode"`
	BindWarning string `json:"bindWarning,omitempty"`
}

// Setup is the minimal, optional bootstrap surface for when no vault/profile
// exists yet. It is deliberately tiny and rate-limited — the generic bridge is
// never exposed before authentication.
type Setup interface {
	Status() SetupStatus
	// CreateVault creates a new vault with password and returns a session token.
	CreateVault(password string) (token string, err error)
	// RecoverVault resets the password via recovery key and returns a token.
	RecoverVault(recoveryKey, newPassword string) (token string, err error)
}

// Options configures a Server. The host injects everything; the package imports
// no host code.
type Options struct {
	Assets            fs.FS
	Hub               *Hub
	Authenticator     Authenticator
	Setup             Setup
	Port              int
	BindMode          string
	BindAddr          string // explicit address for BindModeManual
	AllowedCIDRs      []string
	SessionTTLMinutes int
	// TLS, when non-nil, serves HTTPS using this config (the shared server-TLS
	// bundle). Nil keeps the listener plaintext (the unchanged default).
	TLS *tls.Config
	// App is the bound host value reflected by the bridge.
	App any
}

// Server is the running web host.
type Server struct {
	opts       Options
	bridge     *bridge
	httpServer *http.Server
	listener   net.Listener
	boundAddr  string
	bindIP     string
	allowed    []*net.IPNet
	loginLimit *rateLimiter
	sseCount   int
	sseMu      sync.Mutex
}

// maxSSEClients caps concurrent event streams (mono-user, ~5 tabs expected).
const maxSSEClients = 16

// NewServer binds the listener and starts serving. It returns an error when
// BindModeTailscale is requested but no tailnet IP exists, or the port is busy.
func NewServer(opts Options) (*Server, error) {
	if opts.App == nil {
		return nil, fmt.Errorf("webserver: App is required")
	}
	if opts.Authenticator == nil {
		return nil, fmt.Errorf("webserver: Authenticator is required")
	}
	if opts.Hub == nil {
		opts.Hub = NewHub()
	}
	if opts.Port <= 0 || opts.Port > 65535 {
		opts.Port = DefaultPort
	}

	bindIP, err := resolveBindIP(opts)
	if err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(bindIP, fmt.Sprintf("%d", opts.Port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("webserver: listen %s: %w", addr, err)
	}
	boundAddr := listener.Addr().String()
	// Wrap the listener for HTTPS when a TLS config is supplied. Done at the
	// listener (not ServeTLS) so the same /healthz, /login, bridge, SSE and asset
	// routes ride one TLS endpoint — no plaintext exception for healthcheck.
	if opts.TLS != nil {
		listener = tls.NewListener(listener, opts.TLS)
	}

	s := &Server{
		opts:       opts,
		bridge:     newBridge(opts.App),
		listener:   listener,
		boundAddr:  boundAddr,
		bindIP:     bindIP,
		allowed:    parseCIDRs(opts.AllowedCIDRs),
		loginLimit: newRateLimiter(10, time.Minute),
	}

	mux := http.NewServeMux()
	mux.Handle("/healthz", s.peerGuard(http.HandlerFunc(s.handleHealthz)))
	mux.Handle("/login", s.peerGuard(http.HandlerFunc(s.handleLogin)))
	mux.Handle("/setup/", s.peerGuard(http.HandlerFunc(s.handleSetup)))
	mux.Handle("/bridge", s.peerGuard(http.HandlerFunc(s.handleBridge)))
	mux.Handle("/events", s.peerGuard(http.HandlerFunc(s.handleEvents)))
	mux.Handle("/", s.peerGuard(assetHandler(opts.Assets)))

	s.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      0, // 0: SSE streams must not be write-deadlined.
		IdleTimeout:       120 * time.Second,
	}
	go func() { _ = s.httpServer.Serve(listener) }()
	return s, nil
}

// resolveBindIP determines the listen IP from the bind mode.
func resolveBindIP(opts Options) (string, error) {
	switch opts.BindMode {
	case BindModeManual:
		addr := strings.TrimSpace(opts.BindAddr)
		if addr == "" {
			return "", fmt.Errorf("webserver: manual bind mode requires a bind address")
		}
		return addr, nil
	default: // BindModeTailscale and anything unset
		return DetectTailscaleIP()
	}
}

// URL is the address a browser should open. The scheme reflects whether TLS is
// enabled.
func (s *Server) URL() string {
	return s.scheme() + "://" + s.boundAddr
}

// scheme returns "https" when serving over TLS, "http" otherwise.
func (s *Server) scheme() string {
	if s.opts.TLS != nil {
		return "https"
	}
	return "http"
}

// BindIP returns the resolved bind IP (for the settings page).
func (s *Server) BindIP() string { return s.bindIP }

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) {
	if s == nil || s.httpServer == nil {
		return
	}
	_ = s.httpServer.Shutdown(ctx)
}

// peerGuard enforces the bind-mode peer policy and sets baseline headers. In
// Tailscale mode it rejects any peer outside 100.64.0.0/10; in manual mode it
// applies allowedCIDRs when configured.
func (s *Server) peerGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !s.peerAllowed(host) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		// No wildcard CORS in web mode; the app is served same-origin.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) peerAllowed(host string) bool {
	if host == "" {
		return false
	}
	// Loopback is always allowed (the host machine itself, dev, tray probes).
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	switch s.opts.BindMode {
	case BindModeManual:
		if len(s.allowed) == 0 {
			return true // manual mode with no CIDR policy: bind addr is the gate
		}
		return hostInAnyCIDR(host, s.allowed)
	default:
		return isTailscalePeer(host)
	}
}

// --- handlers ---------------------------------------------------------------

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"running": true,
		"locked":  !s.opts.Authenticator.VaultUnlocked(),
	})
}

type loginRequest struct {
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token,omitempty"`
	Error string `json:"error,omitempty"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.loginLimit.allow(clientKey(r)) {
		writeJSON(w, http.StatusTooManyRequests, loginResponse{Error: "too many attempts, slow down"})
		return
	}
	var req loginRequest
	if err := decodeLimited(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: err.Error()})
		return
	}
	token, err := s.opts.Authenticator.VerifyOrUnlock(req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "invalid password"})
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{Token: token})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if s.opts.Setup == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "setup not available"})
		return
	}
	switch strings.TrimPrefix(r.URL.Path, "/setup/") {
	case "status":
		writeJSON(w, http.StatusOK, s.opts.Setup.Status())
	case "create":
		s.handleSetupCreate(w, r)
	case "recover":
		s.handleSetupRecover(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "unknown setup endpoint"})
	}
}

func (s *Server) handleSetupCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.loginLimit.allow(clientKey(r)) {
		writeJSON(w, http.StatusTooManyRequests, loginResponse{Error: "too many attempts, slow down"})
		return
	}
	var req loginRequest
	if err := decodeLimited(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: err.Error()})
		return
	}
	token, err := s.opts.Setup.CreateVault(req.Password)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{Token: token})
}

type recoverRequest struct {
	RecoveryKey string `json:"recoveryKey"`
	NewPassword string `json:"newPassword"`
}

func (s *Server) handleSetupRecover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.loginLimit.allow(clientKey(r)) {
		writeJSON(w, http.StatusTooManyRequests, loginResponse{Error: "too many attempts, slow down"})
		return
	}
	var req recoverRequest
	if err := decodeLimited(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: err.Error()})
		return
	}
	token, err := s.opts.Setup.RecoverVault(req.RecoveryKey, req.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{Token: token})
}

func (s *Server) handleBridge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if reason := s.gateSession(bearerToken(r)); reason != "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": reason})
		return
	}
	var req bridgeRequest
	if err := decodeLimited(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	// Authenticated activity keeps the vault from auto-locking under remote use.
	s.opts.Authenticator.RecordActivity()

	res := s.safeDispatch(req)
	if res.badReq != "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": res.badReq})
		return
	}
	if res.serverErr != "" {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": res.serverErr})
		return
	}
	if res.appErr != "" {
		// Mirror Wails: a returned error rejects the frontend promise.
		writeJSON(w, http.StatusOK, map[string]any{"error": res.appErr})
		return
	}
	writeJSON(w, http.StatusOK, res.body)
}

// safeDispatch wraps the reflection call in a recover so a panicking method
// becomes a 500 instead of crashing the process.
func (s *Server) safeDispatch(req bridgeRequest) (res bridgeOutcome) {
	defer func() {
		if rec := recover(); rec != nil {
			res = bridgeOutcome{serverErr: fmt.Sprintf("internal error: %v", rec)}
		}
	}()
	d := s.bridge.dispatch(req)
	return bridgeOutcome{body: d.body, appErr: d.appErr, badReq: d.badReq}
}

type bridgeOutcome struct {
	body      any
	appErr    string
	badReq    string
	serverErr string
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if reason := s.gateSession(r.URL.Query().Get("token")); reason != "" {
		http.Error(w, reason, http.StatusUnauthorized)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	if !s.acquireSSE() {
		http.Error(w, "too many event streams", http.StatusServiceUnavailable)
		return
	}
	defer s.releaseSSE()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	events, unsubscribe := s.opts.Hub.subscribe()
	defer unsubscribe()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case event := <-events:
			bytes, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
			flusher.Flush()
		}
	}
}

// gateSession returns "" when the request may proceed, or a reason
// ("locked"/"invalid_session") otherwise.
func (s *Server) gateSession(token string) string {
	if !s.opts.Authenticator.VaultUnlocked() {
		return "locked"
	}
	if token == "" || !s.opts.Authenticator.ValidSession(token) {
		return "invalid_session"
	}
	return ""
}

func (s *Server) acquireSSE() bool {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()
	if s.sseCount >= maxSSEClients {
		return false
	}
	s.sseCount++
	return true
}

func (s *Server) releaseSSE() {
	s.sseMu.Lock()
	s.sseCount--
	s.sseMu.Unlock()
}
