// @vitest-environment happy-dom
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PasswordsModule } from './PasswordsModule';

// Notices now go through the app-wide toast (lib/notify -> sonner); module
// tests assert the notify text instead of inline DOM messages.
const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));


const list = vi.fn();
const save = vi.fn();
const del = vi.fn();

vi.mock('@services/passwords.service', () => ({
  passwordsService: {
    list: (...args: unknown[]) => list(...args),
    save: (...args: unknown[]) => save(...args),
    delete: (...args: unknown[]) => del(...args),
  },
}));

const webModeControl = vi.hoisted(() => ({ enabled: false }));
vi.mock('@/web/web-bindings', () => ({
  isWebMode: () => webModeControl.enabled,
}));

const entries = [
  { id: 'pw-1', name: 'Email', username: 'user@example.test', password: 'secret-1', url: '', notes: '' },
  { id: 'pw-2', name: 'Bank', username: '', password: 'secret-2', url: 'https://bank.test', notes: '' },
];

const writeText = vi.fn();
const readText = vi.fn();

describe('PasswordsModule', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    webModeControl.enabled = false;
    list.mockResolvedValue({ success: true, passwords: entries });
    writeText.mockResolvedValue(undefined);
    readText.mockResolvedValue('');
    Object.defineProperty(globalThis.navigator, 'clipboard', {
      value: { writeText, readText },
      configurable: true,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('starts with the editor closed; picking a credential opens it', async () => {
    const user = userEvent.setup();
    render(<PasswordsModule />);
    expect(await screen.findByText('Email')).toBeTruthy();
    // Right side stays empty until the user picks something.
    expect(screen.queryByLabelText('Password name')).toBeNull();

    await user.click(screen.getByText('Email'));
    expect((screen.getByLabelText('Password name') as HTMLInputElement).value).toBe('Email');
    // Password stays hidden by default.
    expect((screen.getByLabelText('Password') as HTMLInputElement).type).toBe('password');
  });

  it('Add Password opens the empty form and X closes the panel', async () => {
    const user = userEvent.setup();
    render(<PasswordsModule />);
    await screen.findByText('Email');

    await user.click(screen.getByRole('button', { name: /Add Password/ }));
    expect(screen.getByText('New credential')).toBeTruthy();
    expect((screen.getByLabelText('Password name') as HTMLInputElement).value).toBe('');

    await user.click(screen.getByRole('button', { name: 'Close panel' }));
    expect(screen.queryByLabelText('Password name')).toBeNull();
  });

  it('saves edits and clears the unsaved indicator', async () => {
    save.mockResolvedValue({ success: true, password: { ...entries[0], name: 'Email pessoal' } });
    const user = userEvent.setup();
    render(<PasswordsModule />);
    await screen.findByText('Email');
    await user.click(screen.getByText('Email'));

    await user.clear(screen.getByLabelText('Password name'));
    await user.type(screen.getByLabelText('Password name'), 'Email pessoal');
    expect(screen.getByText('Unsaved changes')).toBeTruthy();

    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(save).toHaveBeenCalledWith(expect.objectContaining({ id: 'pw-1', name: 'Email pessoal' }));
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith('Saved.'));
    expect(screen.queryByText('Unsaved changes')).toBeNull();
  });

  it('switching credentials mid-edit asks before discarding', async () => {
    const user = userEvent.setup();
    render(<PasswordsModule />);
    await screen.findByText('Email');
    await user.click(screen.getByText('Email'));

    await user.type(screen.getByLabelText('Username'), 'x');
    await user.click(screen.getByText('Bank'));

    expect(screen.getByText('Discard unsaved changes?')).toBeTruthy();
    await user.click(screen.getByRole('button', { name: 'Discard' }));
    await waitFor(() => expect((screen.getByLabelText('Password name') as HTMLInputElement).value).toBe('Bank'));
  });

  it('clean selection switches without any prompt', async () => {
    const user = userEvent.setup();
    render(<PasswordsModule />);
    await screen.findByText('Email');

    await user.click(screen.getByText('Bank'));
    expect(screen.queryByText('Discard unsaved changes?')).toBeNull();
    expect((screen.getByLabelText('Password name') as HTMLInputElement).value).toBe('Bank');
  });

  it('delete requires confirmation and only then deletes', async () => {
    del.mockResolvedValue({ success: true });
    const user = userEvent.setup();
    render(<PasswordsModule />);
    await screen.findByText('Email');
    await user.click(screen.getByText('Email'));

    await user.click(screen.getByRole('button', { name: /Delete/ }));
    expect(del).not.toHaveBeenCalled();
    expect(screen.getByText('Delete credential?')).toBeTruthy();

    await user.click(screen.getByRole('button', { name: 'Delete' }));
    await waitFor(() => expect(del).toHaveBeenCalledWith('pw-1'));
  });

  it('copy reports success and auto-clears the clipboard when unchanged', async () => {
    vi.useFakeTimers();
    render(<PasswordsModule />);
    await vi.waitFor(() => { if (!screen.queryByText('Email')) throw new Error('not loaded'); });
    fireEvent.click(screen.getByText('Email'));

    readText.mockResolvedValue('secret-1');
    fireEvent.click(screen.getByRole('button', { name: /Copy/ }));
    await vi.waitFor(() => { if (writeText.mock.calls.length === 0) throw new Error('no copy yet'); });
    expect(writeText).toHaveBeenCalledWith('secret-1');
    await vi.waitFor(() => { expect(notifyMock).toHaveBeenCalledWith(expect.stringContaining('Copied. The clipboard clears')); });

    await vi.advanceTimersByTimeAsync(26_000);
    expect(writeText).toHaveBeenLastCalledWith('');
  });

  it('copy leaves a changed clipboard alone', async () => {
    vi.useFakeTimers();
    render(<PasswordsModule />);
    await vi.waitFor(() => { if (!screen.queryByText('Email')) throw new Error('not loaded'); });
    fireEvent.click(screen.getByText('Email'));

    readText.mockResolvedValue('something else the user copied');
    fireEvent.click(screen.getByRole('button', { name: /Copy/ }));
    await vi.waitFor(() => { if (writeText.mock.calls.length === 0) throw new Error('no copy yet'); });

    await vi.advanceTimersByTimeAsync(26_000);
    expect(writeText).toHaveBeenCalledTimes(1); // only the copy, no clear
  });

  it('copy failure reports an error instead of a false success', async () => {
    writeText.mockRejectedValue(new Error('denied'));
    // fireEvent, not userEvent: userEvent.setup() replaces navigator.clipboard
    // with its own stub, which would silently bypass the mock above.
    render(<PasswordsModule />);
    await screen.findByText('Email');
    fireEvent.click(screen.getByText('Email'));

    await waitFor(() => expect((screen.getByRole('button', { name: /Copy/ }) as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(screen.getByRole('button', { name: /Copy/ }));
    await waitFor(() => expect(writeText).toHaveBeenCalled());
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith('Could not copy to the clipboard.'));
    expect(notifyMock).not.toHaveBeenCalledWith(expect.stringMatching(/Copied\./));
  });

  it('web mode shows the desktop-only notice and never calls the service', async () => {
    webModeControl.enabled = true;
    render(<PasswordsModule />);
    expect(screen.getByText(/only available on the desktop app/)).toBeTruthy();
    expect(list).not.toHaveBeenCalled();
  });
});
