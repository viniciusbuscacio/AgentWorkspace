// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { VaultGate, __resetAutoQuickUnlockForTests } from './VaultGate';

const create = vi.fn();
const unlock = vi.fn();
const recover = vi.fn();
const touchIDAvailable = vi.fn();
const touchIDHasPassword = vi.fn();
const touchIDEnroll = vi.fn();
const touchIDUnlock = vi.fn();
const listProfiles = vi.fn();
const selectProfile = vi.fn();
const createProfileAtLocation = vi.fn();
const importVaultProfile = vi.fn();
const clearRecentProfiles = vi.fn();
const selectFolder = vi.fn();

vi.mock('@services/vault.service', () => ({
  vaultService: {
    create: (...args: unknown[]) => create(...args),
    unlock: (...args: unknown[]) => unlock(...args),
    recover: (...args: unknown[]) => recover(...args),
    touchIDAvailable: (...args: unknown[]) => touchIDAvailable(...args),
    touchIDHasPassword: (...args: unknown[]) => touchIDHasPassword(...args),
    touchIDEnroll: (...args: unknown[]) => touchIDEnroll(...args),
    touchIDUnlock: (...args: unknown[]) => touchIDUnlock(...args),
  },
}));

vi.mock('@services/profile.service', () => ({
  profileService: {
    listProfiles: (...args: unknown[]) => listProfiles(...args),
    selectProfile: (...args: unknown[]) => selectProfile(...args),
    createProfileAtLocation: (...args: unknown[]) => createProfileAtLocation(...args),
    importVaultProfile: (...args: unknown[]) => importVaultProfile(...args),
    clearRecentProfiles: (...args: unknown[]) => clearRecentProfiles(...args),
  },
}));

vi.mock('@services/settings.service', () => ({
  settingsService: {
    selectFolder: (...args: unknown[]) => selectFolder(...args),
  },
}));

const recentProfile = {
  id: 'profile-1',
  name: 'AgentWorkspace',
  avatar: 'database_upload',
  vaultDir: '/Users/demo/Desktop/AgentWorkspace',
  lastUsed: '2026-06-09T00:00:00Z',
  createdAt: '2026-06-08T00:00:00Z',
  hasVault: true,
  hasRecovery: true,
};

beforeEach(() => {
  create.mockReset();
  unlock.mockReset();
  recover.mockReset();
  touchIDAvailable.mockReset();
  touchIDHasPassword.mockReset();
  touchIDEnroll.mockReset();
  touchIDUnlock.mockReset();
  listProfiles.mockReset();
  selectProfile.mockReset();
  createProfileAtLocation.mockReset();
  importVaultProfile.mockReset();
  clearRecentProfiles.mockReset();
  selectFolder.mockReset();
  listProfiles.mockResolvedValue([recentProfile]);
  touchIDAvailable.mockResolvedValue(false);
  touchIDHasPassword.mockResolvedValue(false);
  touchIDEnroll.mockResolvedValue({ success: true });
  touchIDUnlock.mockResolvedValue({ success: true });
  document.documentElement.style.removeProperty('--aw-app-zoom');
  __resetAutoQuickUnlockForTests();
});

describe('VaultGate', () => {
  it('shows AW2-style start choices and recent vaults', async () => {
    render(<VaultGate status={{ exists: true, unlocked: false } as never} onUnlocked={vi.fn()} />);

    expect(screen.getByText('Create new vault')).toBeTruthy();
    expect(screen.getByText('Open existing vault')).toBeTruthy();
    await waitFor(() => expect(screen.getByText('Recent vaults')).toBeTruthy());
    expect(screen.getByText('AgentWorkspace')).toBeTruthy();
    expect(screen.getByText('/Users/demo/Desktop/AgentWorkspace')).toBeTruthy();
  });

  it('keeps the locked vault screen centered under app zoom', () => {
    document.documentElement.style.setProperty('--aw-app-zoom', '2');
    render(<VaultGate status={{ exists: true, unlocked: false } as never} onUnlocked={vi.fn()} />);

    const root = screen.getByTestId('auth-card-root');
    expect(root.style.width).toBe('50vw');
    expect(root.style.minHeight).toBe('50vh');
    expect(root.className).toContain('items-center');
    expect(root.className).toContain('justify-center');
  });

  it('clears the recent vaults list without touching disk vaults', async () => {
    clearRecentProfiles.mockResolvedValue({ success: true });
    const user = userEvent.setup();

    render(<VaultGate status={{ exists: true, unlocked: false } as never} onUnlocked={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Recent vaults')).toBeTruthy());

    listProfiles.mockResolvedValue([]);
    await user.click(screen.getByRole('button', { name: /Clear/ }));

    expect(clearRecentProfiles).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.queryByText('Recent vaults')).toBeNull());
  });

  it('selects a recent profile before unlocking', async () => {
    selectProfile.mockResolvedValue({ success: true, profile: recentProfile });
    unlock.mockResolvedValue({ success: true });
    const onUnlocked = vi.fn();
    const user = userEvent.setup();

    render(<VaultGate status={{ exists: true, unlocked: false } as never} onUnlocked={onUnlocked} />);
    await user.click(await screen.findByText('AgentWorkspace'));
    expect(selectProfile).toHaveBeenCalledWith('profile-1');

    await user.type(screen.getByLabelText('Password'), 'hunter2');
    await user.click(screen.getByRole('button', { name: 'Unlock with password' }));
    expect(unlock).toHaveBeenCalledWith('hunter2');
    expect(onUnlocked).toHaveBeenCalled();
  });

  it('imports an existing vault folder and switches to unlock', async () => {
    selectFolder.mockResolvedValue({ canceled: false, path: '/Users/demo/ExistingVault' });
    importVaultProfile.mockResolvedValue({
      success: true,
      profile: { ...recentProfile, id: 'profile-2', name: 'ExistingVault', vaultDir: '/Users/demo/ExistingVault' },
    });
    const user = userEvent.setup();

    render(<VaultGate status={{ exists: false, unlocked: false } as never} onUnlocked={vi.fn()} />);
    await user.click(screen.getByText('Open existing vault'));

    expect(importVaultProfile).toHaveBeenCalledWith('/Users/demo/ExistingVault');
    expect(await screen.findByText('Unlock ExistingVault')).toBeTruthy();
  });

  it('creates a profile at the chosen location before creating the vault', async () => {
    selectFolder.mockResolvedValue({ canceled: false, path: '/Users/demo/Desktop' });
    createProfileAtLocation.mockResolvedValue({ success: true, profile: recentProfile });
    create.mockResolvedValue({ success: true, recoveryKey: 'recovery-key' });
    const user = userEvent.setup();

    render(<VaultGate status={{ exists: false, unlocked: false } as never} onUnlocked={vi.fn()} />);
    await user.click(screen.getByText('Create new vault'));
    await user.click(screen.getByText('Choose location...'));
    await user.clear(screen.getByLabelText('Folder name'));
    await user.type(screen.getByLabelText('Folder name'), 'awVault');
    await user.type(screen.getByLabelText('Password'), 'secret');
    await user.type(screen.getByLabelText('Confirm password'), 'secret');
    await user.click(screen.getByRole('button', { name: 'Create vault' }));

    expect(createProfileAtLocation).toHaveBeenCalledWith('/Users/demo/Desktop', 'awVault');
    expect(create).toHaveBeenCalledWith('secret');
    expect(await screen.findByText('Save your recovery key')).toBeTruthy();
    expect(screen.getByText('recovery-key')).toBeTruthy();
  });

  it('does NOT enroll Touch ID on create unless the switch is on', async () => {
    touchIDAvailable.mockResolvedValue(true);
    touchIDHasPassword.mockResolvedValue(false);
    selectFolder.mockResolvedValue({ canceled: false, path: '/Users/demo/Desktop' });
    createProfileAtLocation.mockResolvedValue({ success: true, profile: recentProfile });
    create.mockResolvedValue({ success: true, recoveryKey: 'recovery-key' });
    const user = userEvent.setup();

    render(<VaultGate status={{ exists: false, unlocked: false } as never} onUnlocked={vi.fn()} />);
    await user.click(screen.getByText('Create new vault'));
    await user.click(screen.getByText('Choose location...'));
    await user.type(screen.getByLabelText('Password'), 'secret');
    await user.type(screen.getByLabelText('Confirm password'), 'secret');
    await user.click(screen.getByRole('button', { name: 'Create vault' }));

    await screen.findByText('Save your recovery key');
    expect(touchIDEnroll).not.toHaveBeenCalled();
  });

  it('enrolls Touch ID on create only after opting in', async () => {
    touchIDAvailable.mockResolvedValue(true);
    touchIDHasPassword.mockResolvedValue(false);
    selectFolder.mockResolvedValue({ canceled: false, path: '/Users/demo/Desktop' });
    createProfileAtLocation.mockResolvedValue({ success: true, profile: recentProfile });
    create.mockResolvedValue({ success: true, recoveryKey: 'recovery-key' });
    const user = userEvent.setup();

    render(<VaultGate status={{ exists: false, unlocked: false } as never} onUnlocked={vi.fn()} />);
    await user.click(screen.getByText('Create new vault'));
    await user.click(screen.getByText('Choose location...'));
    await user.type(screen.getByLabelText('Password'), 'secret');
    await user.type(screen.getByLabelText('Confirm password'), 'secret');
    await user.click(await screen.findByRole('switch'));
    await user.click(screen.getByRole('button', { name: 'Create vault' }));

    await screen.findByText('Save your recovery key');
    expect(touchIDEnroll).toHaveBeenCalledWith('secret');
  });

  it('blocks create when passwords do not match', async () => {
    selectFolder.mockResolvedValue({ canceled: false, path: '/Users/demo/Desktop' });
    const user = userEvent.setup();

    render(<VaultGate status={{ exists: false, unlocked: false } as never} onUnlocked={vi.fn()} />);
    await user.click(screen.getByText('Create new vault'));
    await user.click(screen.getByText('Choose location...'));
    await user.type(screen.getByLabelText('Password'), 'secret1');
    await user.type(screen.getByLabelText('Confirm password'), 'secret2');
    await user.click(screen.getByRole('button', { name: 'Create vault' }));

    expect(screen.getByText('Passwords do not match')).toBeTruthy();
    expect(create).not.toHaveBeenCalled();
  });

  it('auto quick-unlocks on launch when a valid credential is stored', async () => {
    touchIDAvailable.mockResolvedValue(true);
    touchIDHasPassword.mockResolvedValue(true);
    touchIDUnlock.mockResolvedValue({ success: true });
    const onUnlocked = vi.fn().mockResolvedValue(undefined);

    render(<VaultGate status={{ exists: true, unlocked: false, currentProfile: recentProfile } as never} onUnlocked={onUnlocked} />);

    await waitFor(() => expect(touchIDUnlock).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(onUnlocked).toHaveBeenCalled());
  });

  it('does NOT auto-unlock again after a lock (one attempt per app run)', async () => {
    touchIDAvailable.mockResolvedValue(true);
    touchIDHasPassword.mockResolvedValue(true);
    touchIDUnlock.mockResolvedValue({ success: true });
    const onUnlocked = vi.fn().mockResolvedValue(undefined);

    const first = render(<VaultGate status={{ exists: true, unlocked: false, currentProfile: recentProfile } as never} onUnlocked={onUnlocked} />);
    await waitFor(() => expect(touchIDUnlock).toHaveBeenCalledTimes(1));
    first.unmount();

    // The gate re-mounts after "Lock vault": it must stay on the start screen
    // and never silently reopen the vault.
    render(<VaultGate status={{ exists: true, unlocked: false, currentProfile: recentProfile } as never} onUnlocked={onUnlocked} />);
    await screen.findByText('Recent vaults');
    expect(touchIDUnlock).toHaveBeenCalledTimes(1);
  });

  it('falls back to the password form when the stored credential expired', async () => {
    touchIDAvailable.mockResolvedValue(true);
    touchIDHasPassword.mockResolvedValue(true);
    touchIDUnlock.mockResolvedValue({ success: false, error: 'quick unlock expired after a week without a password unlock — enter your password once to re-arm it' });
    const onUnlocked = vi.fn();

    render(<VaultGate status={{ exists: true, unlocked: false, currentProfile: recentProfile } as never} onUnlocked={onUnlocked} />);

    await screen.findByText(/quick unlock expired/i);
    expect(onUnlocked).not.toHaveBeenCalled();
    expect(screen.getByLabelText('Password')).toBeTruthy();
  });
});
