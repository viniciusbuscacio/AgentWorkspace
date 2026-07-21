// @vitest-environment happy-dom
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentFirewallPage } from './AgentFirewallPage';

// Notices now go through the app-wide toast (lib/notify -> sonner); module
// tests assert the notify text instead of inline DOM messages.
const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));


const getState = vi.fn();
const setRules = vi.fn();

vi.mock('@services/agentfw.service', () => ({
  agentfwService: {
    getState: (...a: unknown[]) => getState(...a),
    setRules: (...a: unknown[]) => setRules(...a),
  },
}));

const LOOPBACK_RULE = { action: 'PERMIT', service: 'rest', interface: 'loopback', origin: '127.0.0.1/32' };
const STATE = {
  success: true,
  rules: [LOOPBACK_RULE],
  interfaces: [{ name: 'lo0', ip: '127.0.0.1', kind: 'loopback' }],
  services: ['mcp', 'rest', 'web'],
};

describe('AgentFirewallPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getState.mockResolvedValue(STATE);
    setRules.mockResolvedValue(STATE);
  });

  it('loads rules as plain text rows plus the immutable final rule', async () => {
    render(<AgentFirewallPage />);
    // Display mode: text, not form controls.
    expect(await screen.findByText('127.0.0.1/32')).toBeTruthy();
    expect(screen.getByText('PERMIT')).toBeTruthy();
    expect(screen.queryByDisplayValue('127.0.0.1/32')).toBeNull();
    expect(screen.getByText(/implicit · always last/)).toBeTruthy();
  });

  it('edit switches a row to the form and check closes it', async () => {
    render(<AgentFirewallPage />);
    await screen.findByText('127.0.0.1/32');
    await userEvent.click(screen.getByRole('button', { name: 'Edit rule' }));
    expect(screen.getByDisplayValue('127.0.0.1/32')).toBeTruthy();
    await userEvent.click(screen.getByRole('button', { name: 'Done editing' }));
    expect(screen.queryByDisplayValue('127.0.0.1/32')).toBeNull();
  });

  it('saves edited rules and confirms', async () => {
    setRules.mockResolvedValue({
      ...STATE,
      rules: [{ ...LOOPBACK_RULE, origin: '127.0.0.1' }],
    });
    render(<AgentFirewallPage />);
    await screen.findByText('127.0.0.1/32');
    await userEvent.click(screen.getByRole('button', { name: 'Edit rule' }));
    const origin = screen.getByDisplayValue('127.0.0.1/32');
    await userEvent.clear(origin);
    await userEvent.type(origin, '127.0.0.1');
    await userEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(setRules).toHaveBeenCalledTimes(1));
    expect(setRules.mock.calls[0][0]).toEqual([{ ...LOOPBACK_RULE, origin: '127.0.0.1' }]);
    await waitFor(() => expect(notifyMock).toHaveBeenCalledWith('Firewall rules saved and applied.'));
  });

  it('keeps the in-progress edits when the backend rejects the save', async () => {
    // A failed save echoes the OLD persisted rules; applying them would wipe
    // what the user typed. The page must keep the local edits and show the error.
    setRules.mockResolvedValue({
      ...STATE,
      success: false,
      error: 'rule 1: invalid IP origin "not-an-ip"',
    });
    render(<AgentFirewallPage />);
    await screen.findByText('127.0.0.1/32');
    await userEvent.click(screen.getByRole('button', { name: 'Edit rule' }));
    const origin = screen.getByDisplayValue('127.0.0.1/32');
    await userEvent.clear(origin);
    await userEvent.type(origin, 'not-an-ip');
    await userEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByText(/invalid IP origin/)).toBeTruthy();
    expect(screen.getByDisplayValue('not-an-ip')).toBeTruthy();
    expect(screen.queryByDisplayValue('127.0.0.1/32')).toBeNull();
  });

  it('cancel restores the last saved rules and leaves edit mode', async () => {
    render(<AgentFirewallPage />);
    await screen.findByText('127.0.0.1/32');
    await userEvent.click(screen.getByRole('button', { name: 'Edit rule' }));
    const origin = screen.getByDisplayValue('127.0.0.1/32');
    await userEvent.clear(origin);
    await userEvent.type(origin, '10.0.0.0/8');
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(await screen.findByText('127.0.0.1/32')).toBeTruthy();
    expect(screen.queryByDisplayValue('127.0.0.1/32')).toBeNull();
    expect(setRules).not.toHaveBeenCalled();
  });
});
