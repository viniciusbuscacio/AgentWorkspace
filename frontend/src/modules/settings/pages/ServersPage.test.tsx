// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ServersPage } from './ServersPage';

const restStatus = vi.fn();
const restStart = vi.fn();
const restStop = vi.fn();
const mcpStatus = vi.fn();
const webStatus = vi.fn();
const openModuleView = vi.fn();

vi.mock('@services/rest.service', () => ({
  restService: {
    getStatus: (...a: unknown[]) => restStatus(...a),
    start: (...a: unknown[]) => restStart(...a),
    stop: (...a: unknown[]) => restStop(...a),
  },
}));
vi.mock('@services/mcp.service', () => ({
  mcpService: { getStatus: (...a: unknown[]) => mcpStatus(...a) },
}));
vi.mock('@services/webaccess.service', () => ({
  webaccessService: { getStatus: (...a: unknown[]) => webStatus(...a) },
}));
vi.mock('@services/events', () => ({
  onAwEvent: () => () => undefined,
}));
vi.mock('@/lib/open-view', () => ({
  openModuleView: (...a: unknown[]) => openModuleView(...a),
}));

beforeEach(() => {
  vi.clearAllMocks();
  restStatus.mockResolvedValue({ running: true, port: 9301, url: 'http://127.0.0.1:9301/api/aw' });
  mcpStatus.mockResolvedValue({ running: false, port: 9300 });
  webStatus.mockResolvedValue({ running: false, port: 9443 });
});

describe('ServersPage', () => {
  it('shows one card per server with live status', async () => {
    render(<ServersPage />);
    await waitFor(() => expect(screen.getByText(/Running — http:\/\/127\.0\.0\.1:9301/)).toBeTruthy());
    expect(screen.getByText('REST API')).toBeTruthy();
    expect(screen.getByText('MCP Server')).toBeTruthy();
    expect(screen.getByText('Web Access')).toBeTruthy();
    expect(screen.getAllByText('Stopped')).toHaveLength(2);
    // Running server offers Stop; stopped ones offer Start.
    expect(screen.getAllByRole('button', { name: /stop/i })).toHaveLength(1);
    expect(screen.getAllByRole('button', { name: /start/i })).toHaveLength(2);
  });

  it('stops a running server and deep-links to the full settings view', async () => {
    restStop.mockResolvedValue({ running: false, port: 9301 });
    const user = userEvent.setup();
    render(<ServersPage />);
    await waitFor(() => expect(screen.getByText(/Running —/)).toBeTruthy());

    await user.click(screen.getByRole('button', { name: /stop/i }));
    expect(restStop).toHaveBeenCalledTimes(1);

    await user.click(screen.getAllByRole('button', { name: /open settings/i })[0]);
    expect(openModuleView).toHaveBeenCalledWith('rest-server');
  });
});
