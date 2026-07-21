// @vitest-environment happy-dom
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { BrowserModule } from './BrowserModule';

const status = vi.fn();
const start = vi.fn();
const stop = vi.fn();
const tabs = vi.fn();
const screenshot = vi.fn();
const executable = vi.fn();
const pickExecutable = vi.fn();
const setExecutable = vi.fn();
const setAutostart = vi.fn();

vi.mock('@services/browser.service', () => ({
  browserService: {
    status: (...args: unknown[]) => status(...args),
    start: (...args: unknown[]) => start(...args),
    stop: (...args: unknown[]) => stop(...args),
    tabs: (...args: unknown[]) => tabs(...args),
    screenshot: (...args: unknown[]) => screenshot(...args),
    executable: (...args: unknown[]) => executable(...args),
    pickExecutable: (...args: unknown[]) => pickExecutable(...args),
    setExecutable: (...args: unknown[]) => setExecutable(...args),
    setAutostart: (...args: unknown[]) => setAutostart(...args),
  },
}));

const stopped = { id: 'browser-chrome', running: false, port: 9322, profileDir: '/data/browser-profiles/browser-chrome', binary: '/Applications/Chrome' };
// Attached instances report the "(personal profile)" marker; the agent
// profile reports its aw-owned dir.
const running = { ...stopped, running: true, profileDir: '(personal profile)' };
const result = (statusInfo: unknown, extra: Record<string, unknown> = {}) =>
  ({ success: true, status: statusInfo, autostart: false, ...extra });

describe('BrowserModule', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    status.mockResolvedValue(result(stopped));
    executable.mockResolvedValue({ success: true, path: '/Applications/Chrome', defaultPath: '/Applications/Chrome', custom: false });
    tabs.mockResolvedValue({ success: true, tabs: [] });
  });

  it('shows the stopped status with the CDP port and a Connect button', async () => {
    render(<BrowserModule browserId="browser-chrome" title="Google Chrome" />);
    expect(await screen.findByText('Stopped')).toBeTruthy();
    expect(screen.getByText('9322')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Connect/ })).toBeTruthy();
  });

  it('connects the agent browser and lists its tabs', async () => {
    start.mockResolvedValue(result(running));
    status.mockResolvedValueOnce(result(stopped)).mockResolvedValue(result(running));
    tabs.mockResolvedValue({ success: true, tabs: [{ id: 't1', title: 'aw page', url: 'https://example.test' }] });

    const user = userEvent.setup();
    render(<BrowserModule browserId="browser-chrome" title="Google Chrome" />);

    await user.click(await screen.findByRole('button', { name: /Connect/ }));

    expect(start).toHaveBeenCalledWith('browser-chrome');
    await waitFor(() => expect(screen.getByText('aw page')).toBeTruthy());
    expect(screen.getByText('Connected')).toBeTruthy();
  });

  it('disconnects a running browser', async () => {
    status.mockResolvedValue(result(running));
    stop.mockResolvedValue(result(stopped));

    const user = userEvent.setup();
    render(<BrowserModule browserId="browser-chrome" title="Google Chrome" />);

    await user.click(await screen.findByRole('button', { name: /Disconnect/ }));
    expect(stop).toHaveBeenCalledWith('browser-chrome');
  });

  it('saves the auto-connect toggle as a draft applied on Save', async () => {
    setAutostart.mockResolvedValue(result(stopped, { autostart: true }));
    const user = userEvent.setup();
    render(<BrowserModule browserId="browser-chrome" title="Google Chrome" />);

    await user.click(await screen.findByLabelText('Connect automatically when the app starts'));
    await user.click(screen.getByRole('button', { name: /Save/ }));

    expect(setAutostart).toHaveBeenCalledWith('browser-chrome', true);
  });

  it('chooses and saves a custom browser executable path', async () => {
    pickExecutable.mockResolvedValue({ success: true, path: 'C:\\Browsers\\chrome.exe' });
    setExecutable.mockResolvedValue({ success: true, path: 'C:\\Browsers\\chrome.exe', defaultPath: '/Applications/Chrome', custom: true });
    const user = userEvent.setup();
    render(<BrowserModule browserId="browser-chrome" title="Google Chrome" />);

    await user.click(await screen.findByRole('button', { name: /Browse/ }));
    await user.click(screen.getByRole('button', { name: /Save/ }));

    expect(pickExecutable).toHaveBeenCalledWith('browser-chrome');
    expect(setExecutable).toHaveBeenCalledWith('browser-chrome', 'C:\\Browsers\\chrome.exe');
  });
});
