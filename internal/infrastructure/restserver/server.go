// Package restserver exposes the running Agent Workspace app to external HTTP clients
// that do not speak MCP, mirroring the mcpserver package: the same aw action
// registry, the same vault-held bearer token, the same strictly-local rules
// (loopback bind, non-loopback peers refused even with a valid token), and
// the same lifecycle (runs while the vault is unlocked, down on lock).
package restserver

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/infrastructure/agentfw"
)

// DefaultAddr is the fixed local endpoint external clients are told to use.
const DefaultAddr = "127.0.0.1:9301"

// Backend is the port the composition root implements to route aw actions
// into the app. It is shape-identical to mcpserver.Backend so one adapter
// serves both servers.
type Backend interface {
	// CallAw runs one aw action with a JSON-object argument string and
	// returns the action's JSON output.
	CallAw(ctx context.Context, action string, argsJSON string) (string, error)
	// AwDescription documents the aw dispatcher for the enabled action groups.
	AwDescription() string
}

// ChatReply is the result of one full chat turn driven over REST.
type ChatReply struct {
	ChatID             string
	Reply              string
	AssistantMessageID string
}

// ChatBackend drives a full model/chat turn (the model decides tools, runs
// subagents, etc.) and returns the final assistant reply. It is optional: when
// nil, the chat route is not registered and the production app is unaffected.
type ChatBackend interface {
	RunChat(ctx context.Context, chatID string, text string, stream bool) (ChatReply, error)
}

// Config carries the server settings resolved by the composition root.
type Config struct {
	// Addr is the legacy single listen address; it must be loopback. Used only
	// when BindAddrs is empty (kept for tests and callers that predate the
	// Agent Firewall). Defaults to DefaultAddr.
	Addr string
	// Service, when non-empty, switches the server into Agent Firewall mode: it
	// binds only the interfaces the Rules permit for this service (Option A) and
	// gates every peer through Rules. It is the firewall service id
	// (domain.FirewallServiceREST).
	Service string
	// Rules is the Agent Firewall ruleset applied in firewall mode.
	Rules []domain.FirewallRule
	// FirewallPort is the port bound in firewall mode (the permitted interfaces
	// are resolved from Rules; the port comes from settings).
	FirewallPort int
	// BindAddrs overrides the resolved firewall bind addresses (tests only).
	BindAddrs []string
	// Token is the bearer token clients must present. Required.
	Token string
	// Version is reported on the index endpoint.
	Version string
	// ChatBackend, when non-nil, enables POST /api/chat for a full chat turn.
	// Optional and nil-guarded so the desktop app (which does not set it) is
	// unaffected.
	ChatBackend ChatBackend
	// TLS, when non-nil, serves HTTPS using this config (the shared server-TLS
	// bundle). Nil keeps the listener plaintext (the unchanged default).
	TLS *tls.Config
}

// Server is the token-scoped localhost REST endpoint.
type Server struct {
	addr        string
	boundAddrs  []string
	secure      bool
	version     string
	tokenHash   [sha256.Size]byte
	backend     Backend
	chatBackend ChatBackend
	server      *http.Server
}

// NewServer starts the REST server and returns it. The server runs until
// Shutdown is called.
func NewServer(backend Backend, cfg Config) (*Server, error) {
	if backend == nil {
		return nil, errors.New("rest server backend is required")
	}
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, errors.New("rest server bearer token is required")
	}
	version := cfg.Version
	if version == "" {
		version = "dev"
	}
	s := &Server{
		secure:      cfg.TLS != nil,
		version:     version,
		tokenHash:   sha256.Sum256([]byte(token)),
		backend:     backend,
		chatBackend: cfg.ChatBackend,
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api", s.requireBearer(http.HandlerFunc(s.handleIndex)))
	mux.Handle("GET /api/actions", s.requireBearer(http.HandlerFunc(s.handleActions)))
	mux.Handle("POST /api/aw", s.requireBearer(http.HandlerFunc(s.handleAw)))
	if s.chatBackend != nil {
		mux.Handle("POST /api/chat", s.requireBearer(http.HandlerFunc(s.handleChat)))
	}
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.handleResourceMetadata)

	listeners, displayAddr, handler, err := s.bind(cfg, mux)
	if err != nil {
		return nil, err
	}
	s.addr = displayAddr
	s.server = &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	for _, ln := range listeners {
		go func(l net.Listener) { _ = s.server.Serve(l) }(ln)
	}
	return s, nil
}

// bind opens the listener(s) and picks the peer guard for the active mode.
// Firewall mode (cfg.BindAddrs non-empty) binds every permitted interface and
// gates through the Agent Firewall; legacy mode binds one loopback address and
// keeps the strict loopback-only guard.
func (s *Server) bind(cfg Config, mux http.Handler) ([]net.Listener, string, http.Handler, error) {
	wrap := func(ln net.Listener) net.Listener {
		if cfg.TLS != nil {
			return tls.NewListener(ln, cfg.TLS)
		}
		return ln
	}
	if cfg.Service != "" || len(cfg.BindAddrs) > 0 {
		addrs := cfg.BindAddrs
		if len(addrs) == 0 {
			resolved, err := agentfw.BindAddrs(cfg.Rules, cfg.Service, cfg.FirewallPort)
			if err != nil {
				return nil, "", nil, fmt.Errorf("rest server resolve firewall bind: %w", err)
			}
			addrs = resolved
		}
		if len(addrs) == 0 {
			return nil, "", nil, agentfw.ErrNoPermittedInterface
		}
		var listeners []net.Listener
		for _, addr := range addrs {
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				for _, prev := range listeners {
					_ = prev.Close()
				}
				return nil, "", nil, fmt.Errorf("rest server listen on %s: %w", addr, err)
			}
			listeners = append(listeners, wrap(ln))
		}
		s.boundAddrs = addrs
		return listeners, primaryDisplayAddr(addrs), agentfw.Guard(cfg.Rules, cfg.Service, mux), nil
	}

	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = DefaultAddr
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, "", nil, fmt.Errorf("invalid rest server address %q: %w", addr, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return nil, "", nil, fmt.Errorf("rest server must bind a loopback address, got %q", addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, "", nil, fmt.Errorf("rest server listen on %s: %w", addr, err)
	}
	s.boundAddrs = []string{ln.Addr().String()}
	return []net.Listener{wrap(ln)}, ln.Addr().String(), requireLoopbackPeer(mux), nil
}

// primaryDisplayAddr picks the address shown to the user: a loopback bind when
// present (most reachable from the host), a wildcard rewritten to 127.0.0.1,
// else the first address.
func primaryDisplayAddr(addrs []string) string {
	for _, a := range addrs {
		if host, _, err := net.SplitHostPort(a); err == nil {
			if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
				return a
			}
		}
	}
	if len(addrs) > 0 {
		if host, port, err := net.SplitHostPort(addrs[0]); err == nil && host == "0.0.0.0" {
			return net.JoinHostPort("127.0.0.1", port)
		}
		return addrs[0]
	}
	return ""
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() string { return s.addr }

// BoundAddrs returns every listen address the server bound (host:port), as
// resolved at start. The firewall rebind watcher compares it against the
// currently desired bind set to detect network-interface changes.
func (s *Server) BoundAddrs() []string { return append([]string(nil), s.boundAddrs...) }

// URL returns the primary endpoint external clients call. The scheme reflects
// whether TLS is enabled.
func (s *Server) URL() string { return s.scheme() + "://" + s.addr + "/api/aw" }

func (s *Server) metadataURL() string {
	return s.scheme() + "://" + s.addr + "/.well-known/oauth-protected-resource"
}

// scheme returns "https" when serving over TLS, "http" otherwise.
func (s *Server) scheme() string {
	if s.secure {
		return "https"
	}
	return "http"
}

// Shutdown gracefully stops the server. Safe on a nil receiver.
func (s *Server) Shutdown(ctx context.Context) {
	if s == nil || s.server == nil {
		return
	}
	_ = s.server.Shutdown(ctx)
}

type awRequest struct {
	Action string          `json:"action"`
	Args   json.RawMessage `json:"args,omitempty"`
}

// handleAw runs one aw action. args may be a JSON object (preferred for REST
// clients) or a pre-encoded JSON string, matching the aw tool contract.
func (s *Server) handleAw(w http.ResponseWriter, r *http.Request) {
	var req awRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return
	}
	argsJSON, err := normalizeArgs(req.Args)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.backend.CallAw(r.Context(), req.Action, argsJSON)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"action": req.Action,
		"result": rawOrString(out),
	})
}

type chatRequest struct {
	ChatID string `json:"chatId"`
	Text   string `json:"text"`
	Stream bool   `json:"stream,omitempty"`
}

// handleChat drives a full model/chat turn: the model decides tools, runs
// subagents, reads the browser, and returns the final assistant reply. Only
// non-streaming is implemented today; stream=true is accepted but ignored.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if s.chatBackend == nil {
		writeJSONError(w, http.StatusNotFound, "chat backend not enabled")
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return
	}
	if strings.TrimSpace(req.ChatID) == "" {
		writeJSONError(w, http.StatusBadRequest, "chatId is required")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeJSONError(w, http.StatusBadRequest, "text is required")
		return
	}
	// Streaming is not yet wired; always run the non-streaming path.
	reply, err := s.chatBackend.RunChat(r.Context(), req.ChatID, req.Text, false)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"chatId":             reply.ChatID,
		"reply":              reply.Reply,
		"assistantMessageId": reply.AssistantMessageID,
	})
}

// handleActions lists every available action (the aw.actions catalog).
func (s *Server) handleActions(w http.ResponseWriter, r *http.Request) {
	out, err := s.backend.CallAw(r.Context(), "aw.actions", "")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"actions": rawOrString(out)})
}

// handleIndex documents the API for a client poking at the base URL.
func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	endpoints := map[string]string{
		"GET /api/actions": "list every available aw action",
		"POST /api/aw":     `run one action: {"action":"app.theme.set","args":{"theme":"ocean"}}`,
	}
	if s.chatBackend != nil {
		endpoints["POST /api/chat"] = `drive a full chat turn (model decides tools): {"chatId":"t1","text":"liste minhas abas abertas"}`
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":        "Agent Workspace REST API",
		"version":     s.version,
		"description": s.backend.AwDescription(),
		"endpoints":   endpoints,
	})
}

// normalizeArgs accepts an object (encoded for the dispatcher), a JSON string
// (passed through) or nothing.
func normalizeArgs(raw json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}
	if strings.HasPrefix(trimmed, `"`) {
		var inner string
		if err := json.Unmarshal(raw, &inner); err != nil {
			return "", fmt.Errorf("invalid args string: %w", err)
		}
		return inner, nil
	}
	if !strings.HasPrefix(trimmed, "{") {
		return "", errors.New("args must be a JSON object or a JSON-encoded object string")
	}
	return trimmed, nil
}

// rawOrString embeds action output as JSON when it is valid JSON (always the
// case for registry handlers) and falls back to a plain string otherwise.
func rawOrString(out string) any {
	if json.Valid([]byte(out)) {
		return json.RawMessage(out)
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

// requireBearer enforces the vault-held token. 401 responses carry the RFC
// 9728 resource_metadata pointer, mirroring the MCP server.
func (s *Server) requireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		candidate, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || !s.tokenMatches(strings.TrimSpace(candidate)) {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer resource_metadata=%q", s.metadataURL()))
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// tokenMatches compares hashes so the comparison is constant-time and does
// not leak the candidate length.
func (s *Server) tokenMatches(candidate string) bool {
	sum := sha256.Sum256([]byte(candidate))
	return subtle.ConstantTimeCompare(s.tokenHash[:], sum[:]) == 1
}

// handleResourceMetadata serves the RFC 9728 protected-resource metadata.
func (s *Server) handleResourceMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 s.scheme() + "://" + s.addr + "/api",
		"resource_name":            "Agent Workspace REST API",
		"bearer_methods_supported": []string{"header"},
	})
}

// requireLoopbackPeer refuses non-loopback peers even when they hold a valid
// token: the v1 design is strictly local, with no CIDR allowlist.
func requireLoopbackPeer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemote(r.RemoteAddr) {
			writeJSONError(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackRemote(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
