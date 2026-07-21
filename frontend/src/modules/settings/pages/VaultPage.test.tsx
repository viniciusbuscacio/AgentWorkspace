// @vitest-environment happy-dom
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { VaultPage } from './VaultPage';

// Notices now go through the app-wide toast (lib/notify -> sonner); module
// tests assert the notify text instead of inline DOM messages.
const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));


// ── Service mocks ─────────────────────────────────────────────────────────────

const vaultStatus = vi.fn();
const vaultLock = vi.fn();
const chooseVaultDir = vi.fn();
const generateRecoveryKey = vi.fn();
const verifyRecoveryKey = vi.fn();
const changePassword = vi.fn();

vi.mock('@services/vault.service', async (importOriginal) => {
  const original = await importOriginal<typeof import('@services/vault.service')>();
  return {
    ...original,
    vaultService: {
      status: (...args: unknown[]) => vaultStatus(...args),
      lock: (...args: unknown[]) => vaultLock(...args),
    },
  };
});

vi.mock('@services/settings.service', async (importOriginal) => {
  const original = await importOriginal<typeof import('@services/settings.service')>();
  return {
    ...original,
    settingsService: {
      chooseVaultDir: (...args: unknown[]) => chooseVaultDir(...args),
      generateRecoveryKey: (...args: unknown[]) => generateRecoveryKey(...args),
      verifyRecoveryKey: (...args: unknown[]) => verifyRecoveryKey(...args),
      changePassword: (...args: unknown[]) => changePassword(...args),
    },
  };
});

// ── Fixtures ──────────────────────────────────────────────────────────────────

const baseStatus = {
  exists: true,
  unlocked: true,
  vaultDir: '/Users/tester/vault',
  hasRecovery: true,
  currentProfile: { id: 'p1', name: 'Main', avatar: '', vaultDir: '/Users/tester/vault', lastUsed: '', createdAt: '', hasVault: true, hasRecovery: true },
};

describe('VaultPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vaultStatus.mockResolvedValue(baseStatus);
    chooseVaultDir.mockResolvedValue(baseStatus);
    generateRecoveryKey.mockResolvedValue({ recoveryKey: 'abc123', error: null });
    verifyRecoveryKey.mockResolvedValue({ valid: true, error: null });
    changePassword.mockResolvedValue({ error: null });
    vaultLock.mockResolvedValue({});
  });

  // ── Spec-pinned tests ──────────────────────────────────────────────────────

  it('[SPEC PIN] danger actions render inside the danger zone section', async () => {
    render(<VaultPage onLocked={vi.fn()} />);
    await screen.findByText('Vault');

    // The danger-zone container must be present.
    const zone = document.querySelector('[data-danger-zone]');
    expect(zone).toBeTruthy();

    // All three dangerous action TITLES are inside that container.
    // Use queryAllByText so duplicate button labels (e.g. button + heading <p>)
    // do not cause the query to throw.
    const { queryAllByText } = within(zone as HTMLElement);
    expect(queryAllByText('Change location').length).toBeGreaterThan(0);
    expect(queryAllByText('Change password').length).toBeGreaterThan(0);
    expect(queryAllByText('Recovery key').length).toBeGreaterThan(0);
  });

  it('[SPEC PIN] Change location routes through the native-dialog binding, never window.confirm', async () => {
    // happy-dom may not define window.confirm — define it so we can assert it
    // was never called (the real confirmation dialog lives in Go, not the DOM).
    const confirmMock = vi.fn();
    const originalConfirm = window.confirm;
    window.confirm = confirmMock;

    try {
      const user = userEvent.setup();
      render(<VaultPage onLocked={vi.fn()} />);
      await screen.findByText('Vault');

      await user.click(screen.getByRole('button', { name: /Change location/ }));

      // The Wails binding (which triggers the native dialog in Go) was called.
      expect(chooseVaultDir).toHaveBeenCalledOnce();
      // The webview window.confirm was NOT used — the confirmation lives in Go.
      expect(confirmMock).not.toHaveBeenCalled();
    } finally {
      window.confirm = originalConfirm;
    }
  });

  // ── Functional smoke tests ─────────────────────────────────────────────────

  it('displays the vault path and profile name from status', async () => {
    render(<VaultPage onLocked={vi.fn()} />);
    await screen.findByText('/Users/tester/vault');
    expect(screen.getByText('Main')).toBeTruthy();
    expect(screen.getByText('Unlocked')).toBeTruthy();
  });

  it('lock vault button calls vaultService.lock and then onLocked', async () => {
    const onLocked = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(<VaultPage onLocked={onLocked} />);
    await screen.findByText('Vault');

    await user.click(screen.getByRole('button', { name: 'Lock vault' }));
    expect(vaultLock).toHaveBeenCalledOnce();
    expect(onLocked).toHaveBeenCalledOnce();
  });

  it('Generate recovery key calls generateRecoveryKey and populates the field', async () => {
    generateRecoveryKey.mockResolvedValue({ recoveryKey: 'my-recovery-key', error: null });
    const user = userEvent.setup();
    render(<VaultPage onLocked={vi.fn()} />);
    await screen.findByText('Vault');

    await user.click(screen.getByRole('button', { name: 'Generate' }));
    expect(generateRecoveryKey).toHaveBeenCalledOnce();
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith(expect.stringContaining('Recovery key generated.')));
  });
});
