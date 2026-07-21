package domain

// Agent Firewall — a single, ordered access-control list that governs which
// network peers may reach the app's local servers (MCP, REST, Web, and any
// future network module). It is the shared network fence for all listeners.
//
// Model (a real firewall ACL):
//   - Rules are evaluated top-down; the FIRST rule that matches decides.
//   - A rule matches on (service, interface, origin) — see FirewallRule.
//   - Anything that falls through is denied by an implicit, IMMUTABLE final
//     rule: DENY ALL ALL. It is never stored and never editable.
//   - Loopback is NOT implicitly allowed: 127.0.0.1 must be permitted by an
//     explicit rule like any other origin. Default posture is deny-everything.
//
// The rules are non-secret network policy, persisted in config.json so they are
// readable before the vault unlocks (the composition root needs them to decide
// how to bind each listener). The pure evaluator lives in
// internal/infrastructure/agentfw.

const (
	// Rule actions.
	FirewallActionPermit = "PERMIT"
	FirewallActionDeny   = "DENY"

	// Service selectors. FirewallServiceAll ("all") is a wildcard that matches
	// every service; the others name one listener.
	FirewallServiceAll  = "all"
	FirewallServiceMCP  = "mcp"
	FirewallServiceREST = "rest"
	FirewallServiceWeb  = "web"

	// Interface selectors — the local interface a listener binds and a
	// connection arrives on, classified by reachability. FirewallIfaceAll
	// ("all") is a wildcard that binds every interface (0.0.0.0).
	FirewallIfaceAll       = "all"
	FirewallIfaceLoopback  = "loopback"
	FirewallIfaceTailscale = "tailscale"
	FirewallIfaceLAN       = "lan"
	FirewallIfacePublic    = "public"

	// OriginAny matches any source address. "0.0.0.0/0" and "" are equivalent.
	FirewallOriginAny = "any"
)

// FirewallRule is one ACL entry.
//
//	PERMIT  mcp   tailscale  10.10.0.0/16
//	DENY    web   lan        10.40.45.99/32
//
// Origin is a CIDR ("10.0.0.0/16"), a bare IP ("10.40.45.23", treated as a
// single host), or FirewallOriginAny / "0.0.0.0/0" / "" for every source.
type FirewallRule struct {
	Action    string `json:"action"`    // PERMIT | DENY
	Service   string `json:"service"`   // all | mcp | rest | web
	Interface string `json:"interface"` // all | loopback | tailscale | lan | public
	Origin    string `json:"origin"`    // CIDR | IP | any
}

// NetworkInterface is one local IPv4 address the firewall can bind/permit,
// classified by reachability. It backs the settings UI's interface picker.
type NetworkInterface struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Kind string `json:"kind"` // loopback | tailscale | lan | public
}

// FirewallConfig is the persisted Agent Firewall policy (config.json). The
// implicit DENY ALL ALL final rule is not represented here — it is applied by
// the evaluator after the last stored rule.
type FirewallConfig struct {
	Rules []FirewallRule `json:"rules,omitempty"`
}

// FirewallServices lists the concrete (non-wildcard) service ids the firewall
// governs today. New network modules append here.
var FirewallServices = []string{
	FirewallServiceMCP,
	FirewallServiceREST,
	FirewallServiceWeb,
}

// ValidFirewallAction reports whether s is a known rule action.
func ValidFirewallAction(s string) bool {
	return s == FirewallActionPermit || s == FirewallActionDeny
}

// ValidFirewallService reports whether s is a known service selector (including
// the "all" wildcard).
func ValidFirewallService(s string) bool {
	switch s {
	case FirewallServiceAll, FirewallServiceMCP, FirewallServiceREST, FirewallServiceWeb:
		return true
	default:
		return false
	}
}

// ValidFirewallInterface reports whether s is a known interface selector
// (including the "all" wildcard).
func ValidFirewallInterface(s string) bool {
	switch s {
	case FirewallIfaceAll, FirewallIfaceLoopback, FirewallIfaceTailscale, FirewallIfaceLAN, FirewallIfacePublic:
		return true
	default:
		return false
	}
}
