// @vitest-environment happy-dom
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PermissionsPage } from './PermissionsPage';

// Notices now go through the app-wide toast (lib/notify -> sonner); module
// tests assert the notify text instead of inline DOM messages.
const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));


const load = vi.fn();
const save = vi.fn();
const selectFolder = vi.fn();

vi.mock('@services/permissions.service', async (importOriginal) => {
  const original = await importOriginal<typeof import('@services/permissions.service')>();
  return {
    ...original,
    permissionsService: {
      load: (...args: unknown[]) => load(...args),
      save: (...args: unknown[]) => save(...args),
      selectFolder: (...args: unknown[]) => selectFolder(...args),
    },
  };
});

const baseSettings = {
  mode: 'permit_list',
  allowedFolders: ['~/Projects'],
  builtinDenied: ['~/.ssh', '~/.gnupg', '~/.aws', '/etc/shadow', '/etc/passwd'],
  builtinAllowed: ['/tmp', '/var/tmp'],
  workspaceRoot: '/data/workspace',
  selfDev: false,
  roots: ['/data/workspace', '/tmp'],
};

describe('PermissionsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    load.mockResolvedValue({ success: true, settings: baseSettings });
    save.mockResolvedValue({ success: true, settings: baseSettings });
  });

  // ============================================================
  // SPEC-PINNED: the native save dialog is the security boundary.
  // A redesign that puts the confirmation in the DOM lets the
  // agent approve its own permission change via ui.click.
  // This test MUST pass before any other change ships.
  // ============================================================
  it('[SPEC PIN] save always routes through the native-dialog binding, not a DOM modal', async () => {
    save.mockResolvedValue({ success: false, canceled: true });
    const user = userEvent.setup();
    render(<PermissionsPage />);
    await screen.findByText('Access mode');

    // Make the state dirty so Save is enabled.
    await user.click(await screen.findByRole('button', { name: 'Remove ~/Projects' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));

    // The service (which goes to the Go backend and shows a native dialog) was called.
    expect(save).toHaveBeenCalled();
    // The canceled response surfaces the message without the native-dialog result.
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith(expect.stringContaining('not approved')));
    // No DOM alertdialog — confirmation is outside the webview.
    expect(screen.queryByRole('alertdialog')).toBeNull();
  });

  it('lists the three modes as radios, Balanced selected from the vault', async () => {
    render(<PermissionsPage />);
    await screen.findByText('Access mode');

    const radios = screen.getAllByRole('radio');
    expect(radios).toHaveLength(3);
    expect(screen.getByRole('radio', { name: 'Block all' })).toBeTruthy();
    expect((screen.getByRole('radio', { name: 'Balanced' }) as HTMLInputElement).checked).toBe(true);
    expect(screen.getByRole('radio', { name: 'Permit all' })).toBeTruthy();
    // deny_list is gone from the product.
    expect(screen.queryByText(/Everything except blocked/)).toBeNull();
  });

  it('picking a different mode marks the state dirty and swaps the panel', async () => {
    const user = userEvent.setup();
    render(<PermissionsPage />);
    await screen.findByText('Access mode');

    // Initially not dirty: Save is disabled.
    expect(
      (screen.getByRole('button', { name: 'Save' }) as HTMLButtonElement).disabled,
    ).toBe(true);

    await user.click(screen.getByRole('radio', { name: 'Permit all' }));

    expect(await screen.findByText(/The dangerous mode/)).toBeTruthy();
    expect(
      (screen.getByRole('button', { name: 'Save' }) as HTMLButtonElement).disabled,
    ).toBe(false);
  });

  it('Balanced panel shows built-in rows without a remove affordance', async () => {
    render(<PermissionsPage />);
    await screen.findByText('Access mode');

    // Workspace + built-in allows are marked built-in and not removable.
    expect(screen.getByText('/data/workspace')).toBeTruthy();
    expect(screen.getByText('/tmp')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Remove /tmp' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Remove /data/workspace' })).toBeNull();
    // Custom folder DOES have a remove button (control: the mechanism works).
    expect(screen.getByRole('button', { name: 'Remove ~/Projects' })).toBeTruthy();
  });

  it('hides the folder editor outside Balanced', async () => {
    load.mockResolvedValue({ success: true, settings: { ...baseSettings, mode: 'block_all' } });
    render(<PermissionsPage />);

    expect(await screen.findByText(/most restrictive mode/)).toBeTruthy();
    expect(screen.queryByRole('textbox', { name: 'New path' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Remove ~/Projects' })).toBeNull();
  });

  it('saves added folders through the service', async () => {
    const user = userEvent.setup();
    render(<PermissionsPage />);
    await screen.findByText('Access mode');

    await user.type(screen.getByRole('textbox', { name: 'New path' }), '~/Documents');
    await user.click(screen.getByRole('button', { name: /Add/ }));
    await user.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() =>
      expect(save).toHaveBeenCalledWith('permit_list', ['~/Projects', '~/Documents']),
    );
  });

  it('reports when the native dialog was not approved', async () => {
    save.mockResolvedValue({ success: false, canceled: true });
    const user = userEvent.setup();
    render(<PermissionsPage />);
    await screen.findByText('Access mode');

    await user.click(screen.getByRole('button', { name: 'Remove ~/Projects' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));

    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith(expect.stringContaining('not approved')));
    expect(load).toHaveBeenCalledTimes(1); // no refresh on canceled save
  });

  it('shows the self-dev banner when self-dev is on', async () => {
    load.mockResolvedValue({ success: true, settings: { ...baseSettings, selfDev: true } });
    render(<PermissionsPage />);

    expect(await screen.findByText(/Self-dev mode is on/)).toBeTruthy();
  });
});
