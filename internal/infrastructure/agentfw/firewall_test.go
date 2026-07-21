package agentfw

import (
	"net"
	"testing"

	"aw/internal/domain"
)

func permit(service, iface, origin string) domain.FirewallRule {
	return domain.FirewallRule{Action: domain.FirewallActionPermit, Service: service, Interface: iface, Origin: origin}
}

func deny(service, iface, origin string) domain.FirewallRule {
	return domain.FirewallRule{Action: domain.FirewallActionDeny, Service: service, Interface: iface, Origin: origin}
}

func TestDecideDefaultDeny(t *testing.T) {
	// No rules => implicit DENY ALL ALL, even for loopback.
	if Decide(nil, domain.FirewallServiceREST, domain.FirewallIfaceLoopback, "127.0.0.1") {
		t.Fatal("empty ruleset must deny (loopback is not implicitly allowed)")
	}
}

func TestDecidePermitAndScope(t *testing.T) {
	rules := []domain.FirewallRule{
		permit(domain.FirewallServiceMCP, domain.FirewallIfaceTailscale, "10.10.0.0/16"),
	}
	if !Decide(rules, domain.FirewallServiceMCP, domain.FirewallIfaceTailscale, "10.10.5.9") {
		t.Fatal("in-range tailscale peer to mcp must be permitted")
	}
	// Wrong service, wrong interface, out-of-range origin each fall through to deny.
	if Decide(rules, domain.FirewallServiceREST, domain.FirewallIfaceTailscale, "10.10.5.9") {
		t.Fatal("rule is scoped to mcp; rest must fall through to deny")
	}
	if Decide(rules, domain.FirewallServiceMCP, domain.FirewallIfaceLAN, "10.10.5.9") {
		t.Fatal("rule is scoped to tailscale; lan must fall through to deny")
	}
	if Decide(rules, domain.FirewallServiceMCP, domain.FirewallIfaceTailscale, "10.11.0.1") {
		t.Fatal("out-of-CIDR origin must fall through to deny")
	}
}

func TestDecideFirstMatchWinsDenyExceptionAbovePermit(t *testing.T) {
	// Real firewall behavior: a DENY exception above a broad PERMIT carves a hole.
	rules := []domain.FirewallRule{
		deny(domain.FirewallServiceWeb, domain.FirewallIfaceLAN, "10.40.45.99/32"),
		permit(domain.FirewallServiceWeb, domain.FirewallIfaceLAN, "10.40.0.0/16"),
	}
	if Decide(rules, domain.FirewallServiceWeb, domain.FirewallIfaceLAN, "10.40.45.99") {
		t.Fatal("the DENY exception must win over the broader PERMIT below it")
	}
	if !Decide(rules, domain.FirewallServiceWeb, domain.FirewallIfaceLAN, "10.40.1.2") {
		t.Fatal("a LAN peer not covered by the DENY must be permitted by the broad rule")
	}
}

func TestDecideOrderMattersPermitAboveDeny(t *testing.T) {
	// Flipping the order flips the outcome for the carved host — order matters.
	rules := []domain.FirewallRule{
		permit(domain.FirewallServiceWeb, domain.FirewallIfaceLAN, "10.40.0.0/16"),
		deny(domain.FirewallServiceWeb, domain.FirewallIfaceLAN, "10.40.45.99/32"),
	}
	if !Decide(rules, domain.FirewallServiceWeb, domain.FirewallIfaceLAN, "10.40.45.99") {
		t.Fatal("with PERMIT first, the broad rule matches before the DENY is reached")
	}
}

func TestDecideWildcards(t *testing.T) {
	rules := []domain.FirewallRule{
		permit(domain.FirewallServiceAll, domain.FirewallIfaceLoopback, domain.FirewallOriginAny),
	}
	for _, svc := range domain.FirewallServices {
		if !Decide(rules, svc, domain.FirewallIfaceLoopback, "127.0.0.1") {
			t.Fatalf("wildcard service rule must permit %s on loopback", svc)
		}
	}
	if Decide(rules, domain.FirewallServiceMCP, domain.FirewallIfaceLAN, "10.0.0.1") {
		t.Fatal("loopback-scoped rule must not permit a LAN connection")
	}
}

func TestDecideSkipsInvalidAction(t *testing.T) {
	rules := []domain.FirewallRule{
		{Action: "ALLOWMAYBE", Service: domain.FirewallServiceREST, Interface: domain.FirewallIfaceAll, Origin: domain.FirewallOriginAny},
		permit(domain.FirewallServiceREST, domain.FirewallIfaceLoopback, "127.0.0.1"),
	}
	// The malformed first rule must neither permit nor block; the real rule decides.
	if !Decide(rules, domain.FirewallServiceREST, domain.FirewallIfaceLoopback, "127.0.0.1") {
		t.Fatal("a malformed rule must be skipped, letting a later valid rule apply")
	}
}

func TestMatchOrigin(t *testing.T) {
	cases := []struct {
		origin, ip string
		want       bool
	}{
		{domain.FirewallOriginAny, "8.8.8.8", true},
		{"", "8.8.8.8", true},
		{"0.0.0.0/0", "203.0.113.1", true},
		{"10.0.0.0/8", "10.9.9.9", true},
		{"10.0.0.0/8", "11.0.0.1", false},
		{"192.168.1.50", "192.168.1.50", true},
		{"192.168.1.50", "192.168.1.51", false},
		{"10.40.45.23/32", "10.40.45.23", true},
		{"10.40.45.23/32", "10.40.45.24", false},
		{"not-a-cidr", "10.0.0.1", false},
		{"10.0.0.0/8", "not-an-ip", false},
	}
	for _, c := range cases {
		if got := MatchOrigin(c.origin, c.ip); got != c.want {
			t.Errorf("MatchOrigin(%q, %q) = %v, want %v", c.origin, c.ip, got, c.want)
		}
	}
}

func TestBindKinds(t *testing.T) {
	rules := []domain.FirewallRule{
		permit(domain.FirewallServiceMCP, domain.FirewallIfaceTailscale, "10.10.0.0/16"),
		permit(domain.FirewallServiceMCP, domain.FirewallIfaceLoopback, "127.0.0.1"),
		permit(domain.FirewallServiceMCP, domain.FirewallIfaceTailscale, "10.20.0.0/16"), // dup kind
		deny(domain.FirewallServiceMCP, domain.FirewallIfaceLAN, "10.0.0.0/8"),           // DENY opens nothing
		permit(domain.FirewallServiceWeb, domain.FirewallIfaceLAN, "10.40.0.0/16"),       // other service
		permit(domain.FirewallServiceAll, domain.FirewallIfacePublic, domain.FirewallOriginAny),
	}
	got := BindKinds(rules, domain.FirewallServiceMCP)
	want := []string{domain.FirewallIfaceTailscale, domain.FirewallIfaceLoopback, domain.FirewallIfacePublic}
	if len(got) != len(want) {
		t.Fatalf("BindKinds = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("BindKinds order = %v, want %v", got, want)
		}
	}
	// REST has no service-specific rule, but the "all/public" wildcard reaches it.
	if k := BindKinds(rules, domain.FirewallServiceREST); len(k) != 1 || k[0] != domain.FirewallIfacePublic {
		t.Fatalf("wildcard rule must open public for rest, got %v", k)
	}
	// With no matching PERMIT at all, a service binds nothing (stays fully closed).
	closed := []domain.FirewallRule{permit(domain.FirewallServiceWeb, domain.FirewallIfaceLAN, "10.40.0.0/16")}
	if k := BindKinds(closed, domain.FirewallServiceREST); len(k) != 0 {
		t.Fatalf("service with no matching permit must bind nothing, got %v", k)
	}
}

func TestClassifyIP(t *testing.T) {
	cases := []struct {
		ip   string
		want string
	}{
		{"127.0.0.1", domain.FirewallIfaceLoopback},
		{"::1", domain.FirewallIfaceLoopback},
		{"100.101.102.103", domain.FirewallIfaceTailscale},
		{"192.168.1.10", domain.FirewallIfaceLAN},
		{"10.0.0.5", domain.FirewallIfaceLAN},
		{"8.8.8.8", domain.FirewallIfacePublic},
	}
	for _, c := range cases {
		if got := ClassifyIP(net.ParseIP(c.ip)); got != c.want {
			t.Errorf("ClassifyIP(%s) = %q, want %q", c.ip, got, c.want)
		}
	}
	if ClassifyIP(nil) != "" {
		t.Error("nil IP must classify as empty")
	}
	if ClassifyHost("nonsense") != "" {
		t.Error("unparseable host must classify as empty")
	}
}
