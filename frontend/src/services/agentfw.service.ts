import { GetAgentFirewall, SetAgentFirewallRules } from '@wails/go/main/App';
import type { domain, dto } from '@wails/go/models';

export type NetworkInterface = domain.NetworkInterface;
export type AgentFirewallState = dto.AgentFirewallState;

// FirewallRuleInput is the plain shape the UI builds. The generated
// domain.FirewallRule class is not structurally assignable from an object
// literal (it carries methods), so we cast at the service boundary — the only
// place allowed to touch the Wails bindings.
export interface FirewallRuleInput {
  action: string;
  service: string;
  interface: string;
  origin: string;
}

// agentfwService is the only place the Agent Firewall UI talks to Go. The
// backend validates and persists the ordered rule list, then re-binds the
// governed servers, so setRules returns the refreshed state.
export const agentfwService = {
  getState(): Promise<AgentFirewallState> {
    return GetAgentFirewall();
  },
  setRules(rules: FirewallRuleInput[]): Promise<AgentFirewallState> {
    return SetAgentFirewallRules(rules as unknown as domain.FirewallRule[]);
  },
};
