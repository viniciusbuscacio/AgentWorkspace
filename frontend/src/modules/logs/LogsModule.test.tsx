// @vitest-environment happy-dom
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LogsModule } from './LogsModule';

// Notices now go through the app-wide toast (lib/notify -> sonner); module
// tests assert the notify text instead of inline DOM messages.
const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));


const list = vi.fn();
const dates = vi.fn();
const cleanOld = vi.fn();
const clearAll = vi.fn();

vi.mock('@services/logs.service', () => ({
  logsService: {
    list: (...args: unknown[]) => list(...args),
    dates: (...args: unknown[]) => dates(...args),
    cleanOld: (...args: unknown[]) => cleanOld(...args),
    clearAll: (...args: unknown[]) => clearAll(...args),
  },
}));

describe('LogsModule', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue({ success: true, logs: [] });
    dates.mockResolvedValue({ success: true, dates: [] });
    cleanOld.mockResolvedValue({ success: true, deleted: 2 });
    clearAll.mockResolvedValue({ success: true, deleted: 12 });
  });

  it('requires a second click before cleaning old logs', async () => {
    const user = userEvent.setup();
    render(<LogsModule />);

    await screen.findByText('No logs found.');
    await user.click(screen.getByRole('button', { name: /Clean by retention/ }));

    expect(cleanOld).not.toHaveBeenCalled();
    expect(notifyMock).toHaveBeenCalledWith(expect.stringContaining('Click Confirm retention clean'));

    await user.click(screen.getByRole('button', { name: /Confirm retention clean/ }));

    await waitFor(() => expect(cleanOld).toHaveBeenCalledWith(7));
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith('Deleted 2 old logs.'));
  });

  it('clamps invalid retention days before cleaning', async () => {
    const user = userEvent.setup();
    render(<LogsModule />);

    await screen.findByText('No logs found.');
    const input = screen.getByLabelText('Retention days');
    fireEvent.change(input, { target: { value: '0' } });
    await user.click(screen.getByRole('button', { name: /Clean by retention/ }));
    await user.click(screen.getByRole('button', { name: /Confirm retention clean/ }));

    await waitFor(() => expect(cleanOld).toHaveBeenCalledWith(1));
  });

  it('requires confirmation before clearing every log entry', async () => {
    const user = userEvent.setup();
    render(<LogsModule />);

    await screen.findByText('No logs found.');
    await user.click(screen.getByRole('button', { name: /Clear all logs/ }));

    expect(clearAll).not.toHaveBeenCalled();
    expect(notifyMock).toHaveBeenCalledWith(expect.stringContaining('delete every log entry'));

    await user.click(screen.getByRole('button', { name: /Confirm clear all/ }));

    await waitFor(() => expect(clearAll).toHaveBeenCalledTimes(1));
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith('Deleted 12 logs.'));
  });

  it('renders syslog-like structured rows and detail attributes', async () => {
    list.mockResolvedValue({
      success: true,
      logs: [{
        id: 'log-1',
        timestamp: '2026-06-19T22:10:22Z',
        level: 6,
        levelName: 'Info',
        severity: 'info',
        source: 'chat',
        event: 'chat.run.completed',
        status: 'ok',
        traceId: 'run-1234567890',
        durationMs: 18420,
        message: 'chat run completed',
        attributesJson: '{"model":"gpt-5","tokens.total":42}',
        createdAt: '2026-06-19T22:10:22Z',
      }],
    });
    const user = userEvent.setup();

    render(<LogsModule />);

    expect(await screen.findByText('chat.run.completed')).toBeTruthy();
    expect(screen.getByText('INFO')).toBeTruthy();
    expect(screen.getByText(/model=gpt-5/)).toBeTruthy();

    await user.click(screen.getByText('chat.run.completed'));
    expect(await screen.findByText(/"tokens.total": 42/)).toBeTruthy();
  });

  it('passes v2 filters to the logs service', async () => {
    const user = userEvent.setup();
    render(<LogsModule />);

    await screen.findByText('No logs found.');
    await user.type(screen.getByLabelText('Event prefix'), 'chat.run');
    await user.type(screen.getByLabelText('Trace id'), 'run-1');
    await user.click(screen.getByRole('combobox', { name: 'Log status' }));
    await user.click(await screen.findByRole('option', { name: 'error' }));
    await user.click(screen.getByRole('combobox', { name: 'Safety risk' }));
    await user.click(await screen.findByRole('option', { name: 'high' }));
    await user.click(screen.getByRole('button', { name: /Search/ }));

    await waitFor(() => expect(list).toHaveBeenLastCalledWith('', -1, 'all', '', 'chat.run', 'run-1', 'error', '', '', 'high', 50, 0));
  });
});
