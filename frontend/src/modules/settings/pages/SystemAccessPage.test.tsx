// @vitest-environment happy-dom
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SystemAccessPage } from './SystemAccessPage';

const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));

const getMacosPermissions = vi.fn();
const openSystemSettingsPane = vi.fn();
const probeMacosPermission = vi.fn();

vi.mock('@services/settings.service', async (importOriginal) => {
  const original = await importOriginal<typeof import('@services/settings.service')>();
  return {
    ...original,
    settingsService: {
      getMacosPermissions: (...args: unknown[]) => getMacosPermissions(...args),
      openSystemSettingsPane: (...args: unknown[]) => openSystemSettingsPane(...args),
      probeMacosPermission: (...args: unknown[]) => probeMacosPermission(...args),
    },
  };
});

const MIC = {
  id: 'microphone',
  name: 'Microphone',
  why: 'Voice capture (ffmpeg :default)',
  settingsPane: 'Privacy & Security › Microphone',
  deepLink: 'x-apple.systempreferences:com.apple.preference.security?Privacy_Microphone',
  hasProbe: true,
};

describe('SystemAccessPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows the empty-platform note when the inventory is empty (non-macOS)', async () => {
    getMacosPermissions.mockResolvedValue({ items: [] });
    render(<SystemAccessPage />);
    expect(await screen.findByText(/Nothing to manage on this platform/)).toBeTruthy();
    expect(screen.queryByText('macOS permissions')).toBeNull();
  });

  it('shows the macOS card when items are present', async () => {
    getMacosPermissions.mockResolvedValue({ items: [MIC] });
    render(<SystemAccessPage />);
    expect(await screen.findByText('macOS permissions')).toBeTruthy();
    expect(screen.getByText('Microphone')).toBeTruthy();
  });

  it('calls openSystemSettingsPane with the deep link when Open is clicked', async () => {
    openSystemSettingsPane.mockResolvedValue({ success: true });
    getMacosPermissions.mockResolvedValue({ items: [MIC] });
    const user = userEvent.setup();
    render(<SystemAccessPage />);
    await user.click(await screen.findByText('Open System Settings'));
    expect(openSystemSettingsPane).toHaveBeenCalledWith(MIC.deepLink);
  });

  it('shows Test button only for rows with hasProbe=true', async () => {
    getMacosPermissions.mockResolvedValue({
      items: [
        {
          id: 'app_management',
          name: 'App Management',
          why: 'Updating/replacing app bundles',
          settingsPane: 'Privacy & Security › App Management',
          deepLink: 'x-apple.systempreferences:com.apple.preference.security?Privacy_AppManagement',
          hasProbe: false,
        },
      ],
    });
    render(<SystemAccessPage />);
    await screen.findByText('App Management');
    expect(screen.queryByRole('button', { name: 'Test' })).toBeNull();
    expect(screen.getByText('enable if app updates fail')).toBeTruthy();
  });

  it('marks the row inconclusive and toasts the detail when the probe cannot decide', async () => {
    getMacosPermissions.mockResolvedValue({ items: [MIC] });
    probeMacosPermission.mockResolvedValue({ id: 'microphone', status: 'unknown', detail: 'audio capture did not respond within 3s' });
    const user = userEvent.setup();
    render(<SystemAccessPage />);
    await screen.findByText('Microphone');

    await user.click(screen.getByRole('button', { name: 'Test' }));

    expect(await screen.findByText("couldn't determine")).toBeTruthy();
    expect(notifyMock).toHaveBeenCalledWith('audio capture did not respond within 3s');
  });

  it('shows unknown status before probing and probes on Test', async () => {
    getMacosPermissions.mockResolvedValue({ items: [MIC] });
    probeMacosPermission.mockResolvedValue({ id: 'microphone', status: 'granted' });
    const user = userEvent.setup();
    render(<SystemAccessPage />);
    await screen.findByText('Microphone');
    expect(screen.getByText('unknown — test it')).toBeTruthy();

    await user.click(screen.getByRole('button', { name: 'Test' }));
    expect(probeMacosPermission).toHaveBeenCalledWith('microphone');
    expect(await screen.findByText('granted')).toBeTruthy();
  });
});
