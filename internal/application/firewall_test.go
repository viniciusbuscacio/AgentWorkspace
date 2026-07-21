package application

import (
	"testing"

	"aw/internal/domain"
)

func rule(action, service, iface, origin string) domain.FirewallRule {
	return domain.FirewallRule{Action: action, Service: service, Interface: iface, Origin: origin}
}

func TestValidateFirewallRulesAcceptsWellFormed(t *testing.T) {
	rules := []domain.FirewallRule{
		rule("PERMIT", "rest", "loopback", "127.0.0.1/32"),
		rule("PERMIT", "mcp", "tailscale", "10.10.0.0/16"),
		rule("PERMIT", "web", "lan", "10.40.45.23"),
		rule("DENY", "all", "lan", "any"),         // DENY may use "any"
		rule("PERMIT", "rest", "loopback", "any"), // loopback PERMIT may use "any"
	}
	if err := ValidateFirewallRules(rules); err != nil {
		t.Fatalf("well-formed rules rejected: %v", err)
	}
}

// A wide-open PERMIT is the user's informed call (the firewall is UI-only;
// the agent cannot write it) — validation must accept it.
func TestValidateFirewallRulesAllowsAnyOriginPermitEverywhere(t *testing.T) {
	for _, iface := range []string{"loopback", "tailscale", "lan", "public", "all"} {
		for _, origin := range []string{"any", "", "0.0.0.0/0"} {
			if err := ValidateFirewallRules([]domain.FirewallRule{rule("PERMIT", "rest", iface, origin)}); err != nil {
				t.Fatalf("PERMIT %s %q should be allowed by user decision, got %v", iface, origin, err)
			}
		}
	}
}

func TestValidateFirewallRulesRejectsMalformed(t *testing.T) {
	cases := []domain.FirewallRule{
		rule("ALLOW", "rest", "loopback", "127.0.0.1"),    // bad action
		rule("PERMIT", "ftp", "loopback", "127.0.0.1"),    // bad service
		rule("PERMIT", "rest", "wifi", "127.0.0.1"),       // bad interface
		rule("PERMIT", "rest", "loopback", "10.0.0.0/33"), // bad CIDR
		rule("PERMIT", "rest", "loopback", "not-an-ip"),   // bad IP
	}
	for _, c := range cases {
		if err := ValidateFirewallRules([]domain.FirewallRule{c}); err == nil {
			t.Fatalf("malformed rule accepted: %+v", c)
		}
	}
}
