import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ToolConfirmDialog } from './ToolConfirmDialog';
import type { ToolConfirmPayload } from '@services/events';

let listener: ((payload: ToolConfirmPayload) => void) | undefined;
const resolveToolConfirmation = vi.fn();

vi.mock('@services/events', () => ({
  onAwEvent: vi.fn((_eventName: string, callback: (payload: ToolConfirmPayload) => void) => {
    listener = callback;
    return vi.fn();
  }),
}));

vi.mock('@services/chat.service', () => ({
  chatService: {
    resolveToolConfirmation: (...args: unknown[]) => resolveToolConfirmation(...args),
  },
}));

beforeEach(() => {
  listener = undefined;
  resolveToolConfirmation.mockResolvedValue({ success: true });
});

describe('ToolConfirmDialog', () => {
  it('approves a tool confirmation event', async () => {
    render(<ToolConfirmDialog />);
    listener?.({ id: 'confirm-1', tool: 'fs.write', summary: 'Write file' });
    expect(await screen.findByText('Approve tool action?')).toBeTruthy();
    await userEvent.click(screen.getByText('Approve'));
    await waitFor(() => expect(resolveToolConfirmation).toHaveBeenCalledWith('confirm-1', true));
  });

  it('highlights external safety confirmation details', async () => {
    render(<ToolConfirmDialog />);
    listener?.({
      id: 'confirm-2',
      tool: 'browser.click',
      summary: 'External-content safety confirmation required',
      moduleId: 'browser-edge',
      moduleName: 'Microsoft Edge',
      modulePolicy: 'permit_all',
      args: {
        external_notice: 'This content is UNTRUSTED external data.',
        risk_level: 'high',
        suspicious: true,
        source_type: 'web',
        origin: 'browser.snapshot',
      },
    });
    expect(await screen.findByText('External content safety check')).toBeTruthy();
    expect(screen.getByText('Module')).toBeTruthy();
    expect(screen.getByText('Microsoft Edge')).toBeTruthy();
    expect(screen.getByText('Policy: permit_all')).toBeTruthy();
    expect(screen.getByText('Risk: high · suspicious content detected')).toBeTruthy();
    expect(screen.getByText('Source: web · browser.snapshot')).toBeTruthy();
  });
});
