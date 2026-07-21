// Package agentfw is the Agent Firewall mechanism: the pure ACL evaluator plus
// the network helpers shared by every local listener (MCP, REST, Web, future).
// It replaces the divergent per-server peer checks (webserver's peerAllowed and
// the copy-pasted requireLoopbackPeer in restserver/mcpserver) with one audited
// implementation.
//
// The evaluator itself is pure (deterministic, no I/O) so it is exhaustively
// unit-tested here; interface enumeration and listener binding are separate
// helpers that touch net.* but never reach the network.
package agentfw

import (
	"net"
	"strings"

	"aw/internal/domain"
)

// tailscaleCGNAT is the Tailscale/CGNAT range (RFC 6598). It is not RFC1918, so
// it is classified before the private test in ClassifyIP.
var tailscaleCGNAT = mustCIDR("100.64.0.0/10")

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic("agentfw: bad constant CIDR " + s)
	}
	return n
}

// Decide evaluates the ruleset for one inbound connection and reports whether it
// is permitted. Rules are walked top-down; the first rule matching (service,
// iface, srcIP) wins. If none match, the connection is DENIED — the implicit,
// immutable "DENY ALL ALL" final rule. Loopback is not special-cased: it is
// allowed only when an explicit rule permits it.
//
//   - service: the concrete listener id (domain.FirewallServiceMCP, ...).
//   - iface:   the reachability class of the LOCAL address the connection
//     arrived on (ClassifyIP of r's local addr).
//   - srcIP:   the remote peer IP (no port).
func Decide(rules []domain.FirewallRule, service, iface, srcIP string) bool {
	for _, rule := range rules {
		if ruleMatches(rule, service, iface, srcIP) {
			return rule.Action == domain.FirewallActionPermit
		}
	}
	return false // implicit DENY ALL ALL
}

// ruleMatches reports whether a rule applies to (service, iface, srcIP). A rule
// with an invalid action never matches, so a malformed entry can neither permit
// nor block by accident — it is simply skipped.
func ruleMatches(rule domain.FirewallRule, service, iface, srcIP string) bool {
	if !domain.ValidFirewallAction(rule.Action) {
		return false
	}
	if !selectorMatches(rule.Service, service, domain.FirewallServiceAll) {
		return false
	}
	if !selectorMatches(rule.Interface, iface, domain.FirewallIfaceAll) {
		return false
	}
	return MatchOrigin(rule.Origin, srcIP)
}

// selectorMatches reports whether a rule selector matches a concrete value. The
// wildcard token matches anything; otherwise it is a case-insensitive equality.
func selectorMatches(selector, value, wildcard string) bool {
	selector = strings.ToLower(strings.TrimSpace(selector))
	if selector == wildcard {
		return true
	}
	return selector == strings.ToLower(strings.TrimSpace(value))
}

// MatchOrigin reports whether srcIP falls within a rule origin. Origin is a CIDR
// ("10.0.0.0/16"), a bare host IP ("10.40.45.23"), or an "any" form
// (domain.FirewallOriginAny, "", "0.0.0.0/0", "::/0"). An unparseable origin
// matches nothing.
func MatchOrigin(origin, srcIP string) bool {
	origin = strings.ToLower(strings.TrimSpace(origin))
	if origin == "" || origin == domain.FirewallOriginAny || origin == "0.0.0.0/0" || origin == "::/0" {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(srcIP))
	if ip == nil {
		return false
	}
	if strings.Contains(origin, "/") {
		_, network, err := net.ParseCIDR(origin)
		if err != nil {
			return false
		}
		return network.Contains(ip)
	}
	host := net.ParseIP(origin)
	return host != nil && host.Equal(ip)
}

// BindKinds returns the set of interface kinds a service must listen on under
// Option A ("bind only where a rule permits"): every distinct interface named
// by a PERMIT rule for the service. DENY rules never open a socket. The wildcard
// interface (domain.FirewallIfaceAll) is returned as-is, meaning "bind all
// interfaces" — the caller resolves it to 0.0.0.0. Order is stable
// (first-seen). An empty result means the service stays fully closed.
func BindKinds(rules []domain.FirewallRule, service string) []string {
	var out []string
	seen := map[string]bool{}
	for _, rule := range rules {
		if rule.Action != domain.FirewallActionPermit {
			continue
		}
		if !selectorMatches(rule.Service, service, domain.FirewallServiceAll) {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(rule.Interface))
		if !domain.ValidFirewallInterface(kind) || seen[kind] {
			continue
		}
		seen[kind] = true
		out = append(out, kind)
	}
	return out
}

// ClassifyIP buckets a local address by reachability, matching the firewall's
// interface selectors. The order matters: the Tailscale CGNAT range is checked
// before the private test because it is not RFC1918.
func ClassifyIP(ip net.IP) string {
	switch {
	case ip == nil:
		return ""
	case ip.IsLoopback():
		return domain.FirewallIfaceLoopback
	case tailscaleCGNAT.Contains(ip):
		return domain.FirewallIfaceTailscale
	case ip.IsPrivate() || ip.IsLinkLocalUnicast():
		return domain.FirewallIfaceLAN
	default:
		return domain.FirewallIfacePublic
	}
}

// ClassifyHost is ClassifyIP for a host string (no port). Unparseable hosts
// classify as "".
func ClassifyHost(host string) string {
	return ClassifyIP(net.ParseIP(strings.TrimSpace(host)))
}
