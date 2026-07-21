// @vitest-environment happy-dom
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { UserMemoryPage } from './UserMemoryPage';

// Notices now go through the app-wide toast (lib/notify -> sonner); module
// tests assert the notify text instead of inline DOM messages.
const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));


const getDoc = vi.fn();
const setDoc = vi.fn();
const getBackup = vi.fn();

vi.mock('@services/events', () => ({
  onAwEvent: vi.fn(() => vi.fn()),
}));

vi.mock('@services/user-memory.service', async (importOriginal) => {
  const original = await importOriginal<typeof import('@services/user-memory.service')>();
  return {
    ...original,
    userMemoryService: {
      getDoc: (...args: unknown[]) => getDoc(...args),
      setDoc: (...args: unknown[]) => setDoc(...args),
      getBackup: (...args: unknown[]) => getBackup(...args),
    },
  };
});

const sampleDoc = {
  content: '- preferred-language: Português\n- editor: Neovim',
  updatedAt: '2026-06-12T10:00:00Z',
  backup: '',
  lastCondensedAt: '',
};

describe('UserMemoryPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getDoc.mockResolvedValue({ success: true, doc: sampleDoc });
  });

  it('renders the document content in the textarea', async () => {
    render(<UserMemoryPage />);

    const textarea = await screen.findByRole('textbox');
    expect(textarea).toBeTruthy();
    expect((textarea as HTMLTextAreaElement).value).toContain('preferred-language');
    expect((textarea as HTMLTextAreaElement).value).toContain('Neovim');
  });

  it('save button is disabled when doc has not changed', async () => {
    render(<UserMemoryPage />);
    await screen.findByRole('textbox');

    const saveBtn = screen.getByRole('button', { name: 'Save' }) as HTMLButtonElement;
    expect(saveBtn.disabled).toBe(true);
  });

  it('saves edited content and refreshes', async () => {
    setDoc.mockResolvedValue({
      success: true,
      doc: { ...sampleDoc, content: '- editor: VSCode' },
    });
    const user = userEvent.setup();
    render(<UserMemoryPage />);

    const textarea = await screen.findByRole('textbox');
    await user.clear(textarea);
    await user.type(textarea, '- editor: VSCode');

    const saveBtn = screen.getByRole('button', { name: 'Save' }) as HTMLButtonElement;
    expect(saveBtn.disabled).toBe(false);
    await user.click(saveBtn);

    expect(setDoc).toHaveBeenCalledWith('- editor: VSCode');
    await waitFor(() => expect(getDoc).toHaveBeenCalledTimes(1));
  });

  it('cancel reverts the textarea to the loaded content', async () => {
    const user = userEvent.setup();
    render(<UserMemoryPage />);

    const textarea = await screen.findByRole('textbox') as HTMLTextAreaElement;
    await user.clear(textarea);
    await user.type(textarea, 'changed');

    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(textarea.value).toContain('preferred-language');
  });

  it('shows undo button when backup is available', async () => {
    getDoc.mockResolvedValue({
      success: true,
      doc: { ...sampleDoc, backup: 'old content' },
    });
    render(<UserMemoryPage />);

    expect(await screen.findByText('Undo condensation')).toBeTruthy();
  });

  it('loads backup into draft on undo click', async () => {
    getDoc.mockResolvedValue({
      success: true,
      doc: { ...sampleDoc, backup: 'backup content here' },
    });
    getBackup.mockResolvedValue({
      success: true,
      doc: { content: 'backup content here' },
    });
    const user = userEvent.setup();
    render(<UserMemoryPage />);
    await screen.findByRole('textbox');

    const undoBtn = await screen.findByText('Undo condensation');
    await user.click(undoBtn);

    const textarea = screen.getByRole('textbox') as HTMLTextAreaElement;
    await waitFor(() => expect(textarea.value).toContain('backup content here'));
  });

  it('shows error message when save fails', async () => {
    setDoc.mockResolvedValue({ success: false, error: 'vault locked' });
    const user = userEvent.setup();
    render(<UserMemoryPage />);

    const textarea = await screen.findByRole('textbox');
    await user.clear(textarea);
    await user.type(textarea, 'anything');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(notifyMock).toHaveBeenCalledWith('vault locked'));
  });

  it('shows empty state placeholder when doc is empty', async () => {
    getDoc.mockResolvedValue({ success: true, doc: { content: '', updatedAt: '' } });
    render(<UserMemoryPage />);

    const textarea = await screen.findByRole('textbox') as HTMLTextAreaElement;
    expect(textarea.placeholder).toContain('Nothing saved yet');
  });
});
