package application

import (
	"fmt"
	"net"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// Agent Firewall use cases: load the ruleset, and validate + persist edits. The
// pure ACL evaluation lives in infrastructure/agentfw; here we own the policy
// lifecycle and the safety invariants enforced before a ruleset is stored.

// LoadFirewallRules returns the persisted rules (deny-all when none).
func LoadFirewallRules(store ports.FirewallSettingsStore) []domain.FirewallRule {
	return store.LoadFirewallConfig().Rules
}

// ListNetworkInterfaces returns the local interfaces the firewall can bind,
// classified by reachability, for the settings picker.
func ListNetworkInterfaces(lister ports.NetworkInterfaceLister) ([]domain.NetworkInterface, error) {
	return lister.ListInterfaces()
}

// ResolveFirewallBindAddrs returns the listen addresses the rules open for a
// service right now (empty = blocked by the firewall). The rebind reconciler
// compares this against a running server's bound set to detect drift.
func ResolveFirewallBindAddrs(resolver ports.FirewallBindResolver, rules []domain.FirewallRule, service string, port int) ([]string, error) {
	return resolver.ResolveBindAddrs(rules, service, port)
}

// HostsFromListenAddrs extracts the host part of each host:port listen
// address, skipping entries that do not parse. Used to check TLS certificate
// coverage against what a server actually bound.
func HostsFromListenAddrs(addrs []string) []string {
	hosts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if host, _, err := net.SplitHostPort(addr); err == nil && host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

// SaveFirewallRules validates then persists the ordered ruleset.
func SaveFirewallRules(store ports.FirewallSettingsStore, rules []domain.FirewallRule) error {
	if err := ValidateFirewallRules(rules); err != nil {
		return err
	}
	return store.SaveFirewallRules(rules)
}

// ValidateFirewallRules checks every rule is well-formed. A PERMIT on a
// non-loopback interface with origin "any" is allowed by explicit user
// decision (2026-07-09): the firewall is UI-only — no agent action can write
// it — so a wide-open rule is the user's informed call. The page copy warns
// about the exposure; validation no longer blocks it.
func ValidateFirewallRules(rules []domain.FirewallRule) error {
	for i, rule := range rules {
		if !domain.ValidFirewallAction(rule.Action) {
			return fmt.Errorf("rule %d: invalid action %q (want PERMIT or DENY)", i+1, rule.Action)
		}
		if !domain.ValidFirewallService(rule.Service) {
			return fmt.Errorf("rule %d: invalid service %q", i+1, rule.Service)
		}
		if !domain.ValidFirewallInterface(rule.Interface) {
			return fmt.Errorf("rule %d: invalid interface %q", i+1, rule.Interface)
		}
		if err := validateOrigin(rule.Origin); err != nil {
			return fmt.Errorf("rule %d: %w", i+1, err)
		}
	}
	return nil
}

// isAnyOrigin reports whether an origin means "every source".
func isAnyOrigin(origin string) bool {
	origin = strings.ToLower(strings.TrimSpace(origin))
	return origin == "" || origin == domain.FirewallOriginAny || origin == "0.0.0.0/0" || origin == "::/0"
}

// validateOrigin accepts "any" forms, a CIDR, or a single IP.
func validateOrigin(origin string) error {
	if isAnyOrigin(origin) {
		return nil
	}
	o := strings.TrimSpace(origin)
	if strings.Contains(o, "/") {
		if _, _, err := net.ParseCIDR(o); err != nil {
			return fmt.Errorf("invalid CIDR origin %q", origin)
		}
		return nil
	}
	if net.ParseIP(o) == nil {
		return fmt.Errorf("invalid IP origin %q", origin)
	}
	return nil
}
