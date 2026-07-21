import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { HomeModule } from './HomeModule';
import type { AppModule } from './home-modules';

// --- server service mocks (the server modules' card toggles) ---
const mockMcpGetStatus = vi.fn();
const mockMcpStart = vi.fn();
const mockMcpStop = vi.fn();

vi.mock('@services/mcp.service', () => ({
  mcpService: {
    getStatus: (...args: unknown[]) => mockMcpGetStatus(...args),
    start: (...args: unknown[]) => mockMcpStart(...args),
    stop: (...args: unknown[]) => mockMcpStop(...args),
  },
}));
vi.mock('@services/rest.service', () => ({
  restService: { getStatus: vi.fn().mockResolvedValue({ running: false }), start: vi.fn(), stop: vi.fn() },
}));
vi.mock('@services/webaccess.service', () => ({
  webaccessService: { getStatus: vi.fn().mockResolvedValue({ running: false }), start: vi.fn(), stop: vi.fn() },
}));

// --- browser service mock ---
const mockBrowserStatus = vi.fn();
const mockBrowserStart = vi.fn();
const mockBrowserStop = vi.fn();

vi.mock('@services/browser.service', () => ({
  browserService: {
    status: (...args: unknown[]) => mockBrowserStatus(...args),
    start: (...args: unknown[]) => mockBrowserStart(...args),
    stop: (...args: unknown[]) => mockBrowserStop(...args),
    tabs: vi.fn(),
    screenshot: vi.fn(),
  },
}));

const catalog: AppModule[] = [
  {
    type: 'chat',
    name: 'Chat',
    icon: 'chat',
    description: 'Chat with the agent. Supports markdown, code blocks and streaming.',
    added: true,
    core: true,
  },
  {
    type: 'notes',
    name: 'Notes',
    icon: 'note',
    description: 'Personal notes the agent can read and write.',
    added: true,
  },
];

const browserChromeModule: AppModule = {
  type: 'browser-chrome',
  name: 'Google Chrome',
  icon: 'open_in_browser',
  description: 'A real browser the agent drives, with a dedicated aw profile.',
};

const stoppedBrowserStatus = {
  success: true,
  status: { running: false, port: 9322, profileDir: '/profiles/chrome', binary: '/bin/chrome' },
};
const runningBrowserStatus = {
  success: true,
  status: { running: true, port: 9322, profileDir: '/profiles/chrome', binary: '/bin/chrome' },
};

describe('HomeModule', () => {
  it('shows the Open Apps and Modules screen with the catalog cards', () => {
    render(<HomeModule modules={catalog} onOpenModule={vi.fn()} />);
    expect(screen.getByText('Open Apps and Modules')).toBeTruthy();
    expect(screen.getByText('Chat')).toBeTruthy();
    expect(screen.getByText(/Chat with the agent/)).toBeTruthy();
    expect(screen.getByText('Notes')).toBeTruthy();
  });

  it('cards do not show an added badge (removed per spec item 2)', () => {
    render(<HomeModule modules={catalog} onOpenModule={vi.fn()} />);
    // The check_circle badge was removed: no card shows a workspace badge.
    expect(screen.queryByLabelText('Notes is in your workspace')).toBeNull();
    expect(screen.queryByLabelText('Chat is in your workspace')).toBeNull();
  });

  it('opens the module when its card is clicked', async () => {
    const onOpenModule = vi.fn();
    render(<HomeModule modules={catalog} onOpenModule={onOpenModule} />);
    await userEvent.click(screen.getByText('Chat'));
    expect(onOpenModule).toHaveBeenCalledWith('chat');
  });

  it('opens the module via keyboard (Enter)', async () => {
    const onOpenModule = vi.fn();
    render(<HomeModule modules={catalog} onOpenModule={onOpenModule} />);
    screen.getByRole('button', { name: 'Chat' }).focus();
    await userEvent.keyboard('{Enter}');
    expect(onOpenModule).toHaveBeenCalledWith('chat');
  });
});

// ---------------------------------------------------------------------------
// Browser card toggle tests
// ---------------------------------------------------------------------------
describe('browser card toggle', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockBrowserStatus.mockResolvedValue(stoppedBrowserStatus);
    mockBrowserStart.mockResolvedValue(runningBrowserStatus);
    mockBrowserStop.mockResolvedValue(stoppedBrowserStatus);
  });

  it('shows a toggle switch on browser module cards', async () => {
    render(<HomeModule modules={[browserChromeModule]} onOpenModule={vi.fn()} />);
    expect(await screen.findByRole('switch', { name: 'Toggle browser-chrome' })).toBeTruthy();
  });

  it('toggle reflects the stopped state', async () => {
    render(<HomeModule modules={[browserChromeModule]} onOpenModule={vi.fn()} />);
    const toggle = await screen.findByRole('switch', { name: 'Toggle browser-chrome' });
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('false'));
  });

  it('toggle reflects the running state', async () => {
    mockBrowserStatus.mockResolvedValue(runningBrowserStatus);
    render(<HomeModule modules={[browserChromeModule]} onOpenModule={vi.fn()} />);
    const toggle = await screen.findByRole('switch', { name: 'Toggle browser-chrome' });
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('true'));
  });

  it('clicking toggle ON attaches via browserService.start', async () => {
    const user = userEvent.setup();
    render(<HomeModule modules={[browserChromeModule]} onOpenModule={vi.fn()} />);
    const toggle = await screen.findByRole('switch', { name: 'Toggle browser-chrome' });
    // Wait for the initial status poll to complete so the toggle becomes enabled.
    await waitFor(() => expect(toggle.hasAttribute('disabled')).toBe(false));
    await user.click(toggle);
    expect(mockBrowserStart).toHaveBeenCalledWith('browser-chrome');
  });

  it('clicking toggle OFF calls browserService.stop', async () => {
    mockBrowserStatus.mockResolvedValue(runningBrowserStatus);
    mockBrowserStop.mockResolvedValue(stoppedBrowserStatus);
    const user = userEvent.setup();
    render(<HomeModule modules={[browserChromeModule]} onOpenModule={vi.fn()} />);
    const toggle = await screen.findByRole('switch', { name: 'Toggle browser-chrome' });
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('true'));
    await user.click(toggle);
    expect(mockBrowserStop).toHaveBeenCalledWith('browser-chrome');
  });

  it('toggle is disabled while busy', async () => {
    let resolveStart!: (v: unknown) => void;
    mockBrowserStart.mockReturnValue(new Promise((res) => { resolveStart = res; }));
    const user = userEvent.setup();
    render(<HomeModule modules={[browserChromeModule]} onOpenModule={vi.fn()} />);
    const toggle = await screen.findByRole('switch', { name: 'Toggle browser-chrome' });
    await waitFor(() => expect(toggle.hasAttribute('disabled')).toBe(false));
    await user.click(toggle);
    expect(toggle.hasAttribute('disabled')).toBe(true);
    resolveStart(runningBrowserStatus);
  });

  it('toggle click does not trigger onOpenModule', async () => {
    const onOpenModule = vi.fn();
    const user = userEvent.setup();
    render(<HomeModule modules={[browserChromeModule]} onOpenModule={onOpenModule} />);
    const toggle = await screen.findByRole('switch', { name: 'Toggle browser-chrome' });
    await waitFor(() => expect(toggle.hasAttribute('disabled')).toBe(false));
    await user.click(toggle);
    expect(onOpenModule).not.toHaveBeenCalled();
  });

  it('non-browser cards do not show a toggle', () => {
    render(<HomeModule modules={catalog} onOpenModule={vi.fn()} />);
    expect(screen.queryByRole('switch')).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Server module cards: ordinary catalog cards with a runtime toggle
// ---------------------------------------------------------------------------
describe('server module cards', () => {
  const mcpModule: AppModule = {
    type: 'mcp-server',
    name: 'MCP Server',
    icon: 'mcp',
    description: 'Expose this app to external agents over MCP.',
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockBrowserStatus.mockResolvedValue(stoppedBrowserStatus);
    mockMcpGetStatus.mockResolvedValue({ running: false });
    mockMcpStart.mockResolvedValue({ running: true });
    mockMcpStop.mockResolvedValue({ running: false });
  });

  it('renders the server card in the single grid with a runtime toggle', async () => {
    render(<HomeModule modules={[...catalog, mcpModule]} onOpenModule={vi.fn()} />);
    expect(await screen.findByText('MCP Server')).toBeTruthy();
    const toggle = await screen.findByRole('switch', { name: 'Toggle mcp-server' });
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('false'));
  });

  it('toggle ON calls the service start without opening the module', async () => {
    const onOpenModule = vi.fn();
    const user = userEvent.setup();
    render(<HomeModule modules={[...catalog, mcpModule]} onOpenModule={onOpenModule} />);
    const toggle = await screen.findByRole('switch', { name: 'Toggle mcp-server' });
    await waitFor(() => expect(toggle.hasAttribute('disabled')).toBe(false));
    await user.click(toggle);
    expect(mockMcpStart).toHaveBeenCalledTimes(1);
    expect(onOpenModule).not.toHaveBeenCalled();
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('true'));
  });

  it('clicking the card body opens the module like any other card', async () => {
    const onOpenModule = vi.fn();
    const user = userEvent.setup();
    render(<HomeModule modules={[...catalog, mcpModule]} onOpenModule={onOpenModule} />);
    await user.click(await screen.findByText('MCP Server'));
    expect(onOpenModule).toHaveBeenCalledWith('mcp-server');
  });
});
