// @vitest-environment happy-dom
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SecurityPage } from './SecurityPage';

// Notices now go through the app-wide toast (lib/notify -> sonner); module
// tests assert the notify text instead of inline DOM messages.
const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));


const getAutoLockMinutes = vi.fn();
const setAutoLockMinutes = vi.fn();
const listSecrets = vi.fn();
const getMacosPermissions = vi.fn();
const openSystemSettingsPane = vi.fn();
const probeMacosPermission = vi.fn();
const touchIDAvailable = vi.fn();
const touchIDHasPassword = vi.fn();
const touchIDRemove = vi.fn();
const touchIDRepair = vi.fn();
const codesignTrustStatus = vi.fn();
const codesignTrustGrant = vi.fn();

vi.mock('@services/settings.service', async (importOriginal) => {
  const original = await importOriginal<typeof import('@services/settings.service')>();
  return {
    ...original,
    settingsService: {
      getAutoLockMinutes: (...args: unknown[]) => getAutoLockMinutes(...args),
      setAutoLockMinutes: (...args: unknown[]) => setAutoLockMinutes(...args),
      listSecrets: (...args: unknown[]) => listSecrets(...args),
      getMacosPermissions: (...args: unknown[]) => getMacosPermissions(...args),
      openSystemSettingsPane: (...args: unknown[]) => openSystemSettingsPane(...args),
      probeMacosPermission: (...args: unknown[]) => probeMacosPermission(...args),
    },
  };
});

vi.mock('@services/vault.service', () => ({
  vaultService: {
    touchIDAvailable: (...args: unknown[]) => touchIDAvailable(...args),
    touchIDHasPassword: (...args: unknown[]) => touchIDHasPassword(...args),
    touchIDRemove: (...args: unknown[]) => touchIDRemove(...args),
    touchIDRepair: (...args: unknown[]) => touchIDRepair(...args),
    codesignTrustStatus: (...args: unknown[]) => codesignTrustStatus(...args),
    codesignTrustGrant: (...args: unknown[]) => codesignTrustGrant(...args),
  },
}));


describe('SecurityPage — auto-lock', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listSecrets.mockResolvedValue([]);
    getMacosPermissions.mockResolvedValue({ items: [] });
    touchIDAvailable.mockResolvedValue(false);
    touchIDHasPassword.mockResolvedValue(false);
    touchIDRemove.mockResolvedValue({ success: true });
    codesignTrustStatus.mockResolvedValue({ supported: true, hasCertificate: true, trusted: true, certificateName: 'aw-Local Code Signing' });
    codesignTrustGrant.mockResolvedValue({ success: true });
    touchIDRepair.mockResolvedValue({ success: true });
  });

  it('shows Never selected on fresh state (0 from backend)', async () => {
    getAutoLockMinutes.mockResolvedValue(0);
    render(<SecurityPage />);
    // After load the combobox should display "Never"
    const trigger = await screen.findByRole('combobox');
    expect(trigger.textContent).toContain('Never');
  });

  it('maps Never to 0 when saved', async () => {
    getAutoLockMinutes.mockResolvedValue(15);
    setAutoLockMinutes.mockResolvedValue({});
    const user = userEvent.setup();
    render(<SecurityPage />);
    // Wait for load (shows 15 minutes initially)
    await screen.findByRole('combobox');

    // Open the select and pick Never
    await user.click(screen.getByRole('combobox'));
    await user.click(await screen.findByRole('option', { name: 'Never' }));

    // Save
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(setAutoLockMinutes).toHaveBeenCalledWith(0);
  });

  it('shows the Never hint only when Never is selected', async () => {
    getAutoLockMinutes.mockResolvedValue(0);
    render(<SecurityPage />);
    await screen.findByRole('combobox');
    expect(screen.getByText(/stays unlocked until you lock it manually/)).toBeTruthy();
  });
});

describe('SecurityPage — Touch ID and codesign trust', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getAutoLockMinutes.mockResolvedValue(0);
    listSecrets.mockResolvedValue([]);
    touchIDAvailable.mockResolvedValue(false);
    touchIDHasPassword.mockResolvedValue(false);
    touchIDRemove.mockResolvedValue({ success: true });
    codesignTrustStatus.mockResolvedValue({ supported: true, hasCertificate: true, trusted: true, certificateName: 'aw-Local Code Signing' });
    codesignTrustGrant.mockResolvedValue({ success: true });
    touchIDRepair.mockResolvedValue({ success: true });
  });

  it('offers the trust fix when the signing certificate is untrusted', async () => {
    getAutoLockMinutes.mockResolvedValue(0);
    codesignTrustStatus.mockResolvedValue({ supported: true, hasCertificate: true, trusted: false, certificateName: 'aw-Local Code Signing' });
    const user = userEvent.setup();
    render(<SecurityPage />);

    const button = await screen.findByRole('button', { name: 'Trust certificate' });
    await user.click(button);
    expect(codesignTrustGrant).toHaveBeenCalled();
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith(expect.stringContaining('Certificate trusted')));
  });

  it('repairs the Keychain entry from the Touch ID card', async () => {
    getAutoLockMinutes.mockResolvedValue(0);
    touchIDAvailable.mockResolvedValue(true);
    touchIDHasPassword.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<SecurityPage />);

    await user.click(await screen.findByRole('button', { name: 'Repair Keychain permission' }));
    expect(touchIDRepair).toHaveBeenCalled();
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith(expect.stringContaining('Keychain entry repaired')));
  });

  it('hides the trust fix when the certificate is already trusted', async () => {
    getAutoLockMinutes.mockResolvedValue(0);
    render(<SecurityPage />);
    await screen.findByRole('combobox');
    expect(screen.queryByRole('button', { name: 'Trust certificate' })).toBeNull();
  });
});
