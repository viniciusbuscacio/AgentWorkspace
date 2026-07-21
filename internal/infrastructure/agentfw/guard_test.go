package agentfw

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"aw/internal/domain"
)

// okHandler marks that the guard let the request through.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func guardedRequest(t *testing.T, rules []domain.FirewallRule, localAddr net.Addr, remoteAddr string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if localAddr != nil {
		req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, localAddr))
	}
	rec := httptest.NewRecorder()
	Guard(rules, domain.FirewallServiceREST, okHandler).ServeHTTP(rec, req)
	return rec.Code
}

func TestGuardPermitsMatchingPeer(t *testing.T) {
	rules := []domain.FirewallRule{permit(domain.FirewallServiceREST, domain.FirewallIfaceLoopback, "127.0.0.1/32")}
	local := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9301}
	if code := guardedRequest(t, rules, local, "127.0.0.1:54321"); code != http.StatusOK {
		t.Fatalf("permitted loopback peer got %d, want 200", code)
	}
}

func TestGuardDeniesNonMatchingPeer(t *testing.T) {
	rules := []domain.FirewallRule{permit(domain.FirewallServiceREST, domain.FirewallIfaceLoopback, "127.0.0.1/32")}
	local := &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 9301}
	if code := guardedRequest(t, rules, local, "10.0.0.9:54321"); code != http.StatusForbidden {
		t.Fatalf("LAN peer against loopback-only rules got %d, want 403", code)
	}
}

func TestGuardDeniesUnresolvableLocalAddr(t *testing.T) {
	// Even a rule set that would match every interface must not pass a request
	// whose arrival interface is unknown (no LocalAddrContextKey).
	rules := []domain.FirewallRule{permit(domain.FirewallServiceREST, domain.FirewallIfaceAll, domain.FirewallOriginAny)}
	if code := guardedRequest(t, rules, nil, "127.0.0.1:54321"); code != http.StatusForbidden {
		t.Fatalf("missing local addr got %d, want 403", code)
	}
}

func TestGuardDeniesUnresolvableRemoteAddr(t *testing.T) {
	rules := []domain.FirewallRule{permit(domain.FirewallServiceREST, domain.FirewallIfaceAll, domain.FirewallOriginAny)}
	local := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9301}
	if code := guardedRequest(t, rules, local, "not-an-address"); code != http.StatusForbidden {
		t.Fatalf("unparseable remote addr got %d, want 403", code)
	}
}
