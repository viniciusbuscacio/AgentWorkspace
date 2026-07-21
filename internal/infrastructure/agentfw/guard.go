package agentfw

import (
	"net"
	"net/http"

	"aw/internal/domain"
)

// Guard wraps a handler with the Agent Firewall peer check for one service. It
// is the single enforcement point shared by every local listener, replacing the
// per-server requireLoopbackPeer / peerAllowed copies.
//
// The local address is read from the request context (http.Server populates
// http.LocalAddrContextKey with the accepting listener's address), so one
// http.Server serving several per-interface listeners still knows which
// interface each request arrived on — that is what makes Option A per-interface
// rules enforceable. A request whose local or remote address does not parse as
// an IP is denied outright, before the rules are consulted — otherwise a
// wildcard-interface rule could match a connection whose arrival interface is
// unknown. net/http always populates both, so this only bites exotic callers.
func Guard(rules []domain.FirewallRule, service string, next http.Handler) http.Handler {
	allow := PeerFilter(rules, service)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		local, remote := localIP(r), remoteIP(r.RemoteAddr)
		if net.ParseIP(local) == nil || net.ParseIP(remote) == nil || !allow(local, remote) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// localIP extracts the local IP the connection was accepted on, from the
// context value http.Server sets on every request.
func localIP(r *http.Request) string {
	if addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok && addr != nil {
		return remoteIP(addr.String())
	}
	return ""
}

// remoteIP strips the port from a host:port address, returning the bare IP.
func remoteIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
