package appconfig

import "aw/internal/domain"

// Agent Firewall rules live in config.json (not the vault) because the
// composition root must read them before the vault is unlocked to decide how to
// bind each network server. They are non-secret network policy. Store
// implements ports.FirewallSettingsStore; the composition root reaches these
// through the application layer, never directly.

// LoadFirewallConfig returns the persisted firewall policy. An absent block
// yields an empty ruleset, which the evaluator treats as deny-all.
func (Store) LoadFirewallConfig() domain.FirewallConfig {
	if fw := Load().Firewall; fw != nil {
		return *fw
	}
	return domain.FirewallConfig{}
}

// SaveFirewallRules persists the ordered ruleset wholesale (the list is the
// unit of edit). A nil slice is stored as an empty, non-nil rule list so the
// merge in Save replaces rather than preserves the previous value.
func (Store) SaveFirewallRules(rules []domain.FirewallRule) error {
	if rules == nil {
		rules = []domain.FirewallRule{}
	}
	return Save(Config{Firewall: &domain.FirewallConfig{Rules: rules}})
}
