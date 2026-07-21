package appcore

import (
	"context"
	"time"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/dto"
	"aw/internal/infrastructure/agentfw"
	"aw/internal/infrastructure/appconfig"
)

// The Agent Firewall settings page reads and writes the shared network ACL that
// governs the MCP and REST servers (and future network modules). Saving new
// rules re-binds those servers so the change takes effect immediately.

// GetAgentFirewall returns the current ruleset, the bindable interfaces, and
// the governed service ids for the settings page.
func (a *App) GetAgentFirewall() dto.AgentFirewallState {
	state := dto.AgentFirewallState{
		Success:  true,
		Rules:    application.LoadFirewallRules(appconfig.Store{}),
		Services: domain.FirewallServices,
	}
	interfaces, err := application.ListNetworkInterfaces(agentfw.Lister{})
	if err != nil {
		state.Success = false
		state.Error = err.Error()
	}
	state.Interfaces = interfaces
	if state.Rules == nil {
		state.Rules = []domain.FirewallRule{}
	}
	return state
}

// SetAgentFirewallRules validates and persists the ordered ruleset, then
// re-applies it to the running servers (running ones bounce onto the new bind
// set; auto-start servers that were blocked now start). Returns the refreshed
// state.
func (a *App) SetAgentFirewallRules(rules []domain.FirewallRule) dto.AgentFirewallState {
	if err := application.SaveFirewallRules(appconfig.Store{}, rules); err != nil {
		state := a.GetAgentFirewall()
		state.Success = false
		state.Error = err.Error()
		return state
	}
	a.reapplyFirewall()
	return a.GetAgentFirewall()
}

// reapplyFirewall bounces the firewall-governed servers so a rule change takes
// effect: running servers restart onto the new bind set, and any server that
// was blocked (no permitting rule) starts again — whether it was wanted by
// auto-start or by a manual Start that the firewall refused.
func (a *App) reapplyFirewall() {
	a.restartRestServerIfRunning()
	a.restartMcpServerIfRunning()
	a.startRestServerIfEnabled()
	a.startMcpServerIfEnabled()
	a.startRestServerIfBlocked()
	a.startMcpServerIfBlocked()
}

// firewallRebindLoop keeps the governed servers' listeners in step with the
// network. Option A resolves concrete bind IPs at start, but interfaces come
// and go (Tailscale up/down, Wi-Fi DHCP changes): every tick the desired bind
// set is re-resolved from the rules and a server whose set drifted is bounced —
// including starting a blocked server whose permitted interface just appeared.
func (a *App) firewallRebindLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.rebindFirewallServers()
		}
	}
}

// rebindFirewallServers reconciles each governed server against the bind set
// its rules resolve to right now. Servers that are neither running nor blocked
// are left alone (stopped means stopped). An interface-enumeration error skips
// the tick — never bounce a healthy server on flaky information.
func (a *App) rebindFirewallServers() {
	rules := application.LoadFirewallRules(appconfig.Store{})

	a.restMu.Lock()
	restServer, restBlocked := a.restServer, a.restFirewallBlocked
	a.restMu.Unlock()
	if restServer != nil || restBlocked {
		desired, err := application.ResolveFirewallBindAddrs(agentfw.Lister{}, rules, domain.FirewallServiceREST, application.RestServerSettings.Port(a.vault))
		switch {
		case err != nil:
			// Enumeration hiccup — try again next tick.
		case restServer != nil && !sameAddrSet(desired, restServer.BoundAddrs()):
			a.restartRestServerIfRunning()
		case restServer == nil && restBlocked && len(desired) > 0:
			_ = a.startRestServer()
		}
	}

	a.mcpMu.Lock()
	mcpServer, mcpBlocked := a.mcpServer, a.mcpFirewallBlocked
	a.mcpMu.Unlock()
	if mcpServer != nil || mcpBlocked {
		desired, err := application.ResolveFirewallBindAddrs(agentfw.Lister{}, rules, domain.FirewallServiceMCP, application.McpServerSettings.Port(a.vault))
		switch {
		case err != nil:
		case mcpServer != nil && !sameAddrSet(desired, mcpServer.BoundAddrs()):
			a.restartMcpServerIfRunning()
		case mcpServer == nil && mcpBlocked && len(desired) > 0:
			_ = a.startMcpServer()
		}
	}
}

// sameAddrSet compares two listen-address lists as sets (order-insensitive).
func sameAddrSet(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	counts := make(map[string]int, len(x))
	for _, addr := range x {
		counts[addr]++
	}
	for _, addr := range y {
		if counts[addr] == 0 {
			return false
		}
		counts[addr]--
	}
	return true
}
