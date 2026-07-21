// @vitest-environment happy-dom
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { McpClientModule } from './McpClientModule';

// Notices now go through the app-wide toast (lib/notify -> sonner); module
// tests assert the notify text instead of inline DOM messages.
const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));


const list = vi.fn();
const add = vi.fn();
const update = vi.fn();
const remove = vi.fn();
const setEnabled = vi.fn();
const test = vi.fn();
const listTools = vi.fn();

vi.mock('@services/mcp-client.service', async (importOriginal) => {
  const original = await importOriginal<typeof import('@services/mcp-client.service')>();
  return {
    ...original,
    mcpConnectionsService: {
      list: (...a: unknown[]) => list(...a),
      add: (...a: unknown[]) => add(...a),
      update: (...a: unknown[]) => update(...a),
      remove: (...a: unknown[]) => remove(...a),
      setEnabled: (...a: unknown[]) => setEnabled(...a),
      test: (...a: unknown[]) => test(...a),
      listTools: (...a: unknown[]) => listTools(...a),
    },
  };
});

const conn = {
  id: 'mcpconn-1',
  name: 'Learn',
  enabled: true,
  transport: 'streamable_http',
  url: 'https://learn.microsoft.com/api/mcp',
  authType: 'none',
  hasSecret: false,
  lastStatus: 'ok',
  lastCheckedAt: '',
  lastError: '',
  toolCount: 2,
  createdAt: 't0',
  updatedAt: 't0',
};

beforeEach(() => {
  vi.clearAllMocks();
  list.mockResolvedValue({ success: true, connections: [conn] });
  add.mockResolvedValue({ success: true, connection: { ...conn, id: 'mcpconn-2', name: 'New' } });
  update.mockResolvedValue({ success: true, connection: conn });
  remove.mockResolvedValue({ success: true, connection: conn });
  setEnabled.mockResolvedValue({ success: true, connection: conn });
  test.mockResolvedValue({ success: true, result: { connectionId: 'mcpconn-1', status: 'ok', toolCount: 2, durationMs: 5, error: '' } });
  listTools.mockResolvedValue({ success: true, connectionId: 'mcpconn-1', tools: [{ name: 'search', description: 'Search docs' }], truncated: false });
});

describe('McpClientModule', () => {
  it('shows the header, untrusted warning and the connection list', async () => {
    render(<McpClientModule />);
    expect(screen.getByText('MCP Client')).toBeTruthy();
    expect(screen.getByText(/untrusted/i)).toBeTruthy();
    expect(await screen.findByText('Learn')).toBeTruthy();
  });

  it('opens the create form and saves a new connection', async () => {
    const user = userEvent.setup();
    render(<McpClientModule />);
    await screen.findByText('Learn');
    await user.click(screen.getByRole('button', { name: 'Add connection' }));
    await user.type(screen.getByPlaceholderText('Microsoft Learn'), 'My Server');
    await user.type(screen.getByPlaceholderText('https://example.com/api/mcp'), 'https://my.test/mcp');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(add).toHaveBeenCalled());
    expect(add).toHaveBeenCalledWith(expect.objectContaining({ name: 'My Server', url: 'https://my.test/mcp' }));
  });

  it('tests a selected connection and shows the status', async () => {
    const user = userEvent.setup();
    render(<McpClientModule />);
    await user.click(await screen.findByText('Learn'));
    await user.click(screen.getByRole('button', { name: 'Test' }));
    await waitFor(() => expect(test).toHaveBeenCalledWith('mcpconn-1'));
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith(expect.stringContaining('Connected — 2 tool')));
  });

  it('lists remote tools as plain text', async () => {
    const user = userEvent.setup();
    render(<McpClientModule />);
    await user.click(await screen.findByText('Learn'));
    await user.click(screen.getByRole('button', { name: 'List tools' }));
    expect(await screen.findByText('search')).toBeTruthy();
    expect(screen.getByText(/Search docs/)).toBeTruthy();
  });

  it('requires confirmation to delete', async () => {
    const user = userEvent.setup();
    render(<McpClientModule />);
    await user.click(await screen.findByText('Learn'));
    await user.click(screen.getByRole('button', { name: 'Delete' }));
    expect(screen.getByText(/Delete Learn\?/)).toBeTruthy();
    expect(remove).not.toHaveBeenCalled();
  });
});
