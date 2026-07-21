// Package mcpserver exposes the running Agent Workspace app to external MCP clients over
// streamable HTTP, mirroring how Chrome exposes itself over CDP. It serves a
// single multiplexed tool — aw — backed by the same action registry the
// in-app agent uses, behind a vault-held bearer token. The server only binds
// loopback addresses and refuses non-loopback peers even with a valid token;
// it runs while the vault is unlocked and is shut down on lock.
package mcpserver

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

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DefaultAddr is the fixed local endpoint external agents are told to use.
const DefaultAddr = "127.0.0.1:9300"

// Backend is the port the composition root implements to route aw actions
// into the app (pip.Backend pattern). It keeps this package decoupled from
// the tools registry and the concrete App type.
type Backend interface {
	// CallAw runs one aw action with a JSON-object argument string and
	// returns the action's JSON output.
	CallAw(ctx context.Context, action string, argsJSON string) (string, error)
	// AwDescription documents the aw tool for the enabled action groups.
	AwDescription() string
}

// Config carries the server settings resolved by the composition root.
type Config struct {
	// Addr is the legacy single listen address; it must be loopback. Used only
	// when BindAddrs is empty. Defaults to DefaultAddr.
	Addr string
	// Service, when non-empty, switches the server into Agent Firewall mode: it
	// binds only the interfaces the Rules permit for this service (Option A) and
	// gates every peer through Rules. It is the firewall service id
	// (domain.FirewallServiceMCP).
	Service string
	// Rules is the Agent Firewall ruleset applied in firewall mode.
	Rules []domain.FirewallRule
	// FirewallPort is the port bound in firewall mode.
	FirewallPort int
	// BindAddrs overrides the resolved firewall bind addresses (tests only).
	BindAddrs []string
	// Token is the bearer token MCP clients must present. Required.
	Token string
	// Version is reported in the MCP handshake.
	Version string
	// TLS, when non-nil, serves HTTPS using this config (the shared server-TLS
	// bundle). Nil keeps the listener plaintext (the unchanged default).
	TLS *tls.Config
}

// Server is the token-scoped localhost MCP endpoint.
type Server struct {
	addr       string
	boundAddrs []string
	secure     bool
	tokenHash  [sha256.Size]byte
	server     *http.Server
}

type awToolArgs struct {
	Action string `json:"action" jsonschema:"The aw action to run. Use aw.actions to list every available action."`
	Args   string `json:"args,omitempty" jsonschema:"The action arguments as a JSON object string, for example {\"theme\":\"ocean\"}."`
}

// NewServer starts the MCP server and returns it. The server runs until
// Shutdown is called.
func NewServer(backend Backend, cfg Config) (*Server, error) {
	if backend == nil {
		return nil, errors.New("mcp server backend is required")
	}
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, errors.New("mcp server bearer token is required")
	}
	version := cfg.Version
	if version == "" {
		version = "dev"
	}
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "aw",
		Title:   "Agent Workspace",
		Version: version,
	}, nil)
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "aw",
		Description: backend.AwDescription(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in awToolArgs) (*mcp.CallToolResult, any, error) {
		out, err := backend.CallAw(ctx, in.Action, in.Args)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: out}}}, nil, nil
	})

	s := &Server{
		secure:    cfg.TLS != nil,
		tokenHash: sha256.Sum256([]byte(token)),
	}
	streamable := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return mcpServer },
		nil, // defaults keep the SDK's DNS-rebinding (Host header) protection on
	)
	mux := http.NewServeMux()
	mux.Handle("/mcp", s.requireBearer(streamable))
	mux.HandleFunc("/.well-known/oauth-protected-resource", s.handleResourceMetadata)

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
// gates through the Agent Firewall; legacy mode binds one loopback address with
// the strict loopback-only guard.
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
				return nil, "", nil, fmt.Errorf("mcp server resolve firewall bind: %w", err)
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
				return nil, "", nil, fmt.Errorf("mcp server listen on %s: %w", addr, err)
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
		return nil, "", nil, fmt.Errorf("invalid mcp server address %q: %w", addr, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return nil, "", nil, fmt.Errorf("mcp server must bind a loopback address, got %q", addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, "", nil, fmt.Errorf("mcp server listen on %s: %w", addr, err)
	}
	s.boundAddrs = []string{ln.Addr().String()}
	return []net.Listener{wrap(ln)}, ln.Addr().String(), requireLoopbackPeer(mux), nil
}

// primaryDisplayAddr picks the address shown to the user: a loopback bind when
// present, a wildcard rewritten to 127.0.0.1, else the first address.
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

// URL returns the MCP endpoint external clients connect to. The scheme reflects
// whether TLS is enabled.
func (s *Server) URL() string { return s.scheme() + "://" + s.addr + "/mcp" }

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

// requireBearer enforces the vault-held token. 401 responses carry the RFC
// 9728 resource_metadata pointer so spec-following clients can discover the
// auth requirements.
func (s *Server) requireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		candidate, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || !s.tokenMatches(strings.TrimSpace(candidate)) {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer resource_metadata=%q", s.metadataURL()))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
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
// Phase 2 ships bearer-only auth; authorization_servers arrives with OAuth in
// phase 4.
func (s *Server) handleResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"resource":                 s.URL(),
		"resource_name":            "Agent Workspace MCP",
		"bearer_methods_supported": []string{"header"},
	})
}

// requireLoopbackPeer refuses non-loopback peers even when they hold a valid
// token: the v1 design is strictly local, with no CIDR allowlist.
func requireLoopbackPeer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemote(r.RemoteAddr) {
			http.Error(w, "forbidden", http.StatusForbidden)
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
