// @vitest-environment happy-dom
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { NotesModule, NOTES_AUTOSAVE_DEBOUNCE_MS } from './NotesModule';

const list = vi.fn();
const create = vi.fn();
const update = vi.fn();
const updateFlags = vi.fn();
const del = vi.fn();

vi.mock('@services/notes.service', () => ({
  notesService: {
    list: (...args: unknown[]) => list(...args),
    create: (...args: unknown[]) => create(...args),
    update: (...args: unknown[]) => update(...args),
    updateFlags: (...args: unknown[]) => updateFlags(...args),
    delete: (...args: unknown[]) => del(...args),
  },
}));

const notes = [
  { id: 'note-1', title: 'Plano', content: 'Conteúdo do plano.', updatedAt: '2026-06-10', pinned: false, archived: false },
  { id: 'note-2', title: 'Ideias', content: 'Lista de ideias.', updatedAt: '2026-06-09', pinned: false, archived: false },
];

describe('NotesModule', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue({ success: true, notes });
  });

  it('lists notes and opens one in the editor', async () => {
    const user = userEvent.setup();
    render(<NotesModule />);

    await user.click(await screen.findByLabelText('Open note Plano'));

    expect((screen.getByLabelText('Note title') as HTMLInputElement).value).toBe('Plano');
    expect((screen.getByLabelText('Note content') as HTMLTextAreaElement).value).toBe('Conteúdo do plano.');
  });

  it('creates a note from the editor', async () => {
    create.mockResolvedValue({ success: true, note: { id: 'note-3', title: 'Nova', content: 'x', updatedAt: 'now' } });
    const user = userEvent.setup();
    render(<NotesModule />);
    await screen.findByLabelText('Open note Plano');

    await user.type(screen.getByLabelText('Note title'), 'Nova');
    await user.type(screen.getByLabelText('Note content'), 'x');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(create).toHaveBeenCalledWith('Nova', 'x', true);
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
  });

  it('creates a note with "Insert into Agent prompt" unchecked (inPrompt=false)', async () => {
    create.mockResolvedValue({ success: true, note: { id: 'note-4', title: 'Privada', content: 'y', updatedAt: 'now', inPrompt: false } });
    const user = userEvent.setup();
    render(<NotesModule />);
    await screen.findByLabelText('Open note Plano');

    await user.click(screen.getByLabelText('New note'));
    await user.type(screen.getByLabelText('Note title'), 'Privada');
    await user.type(screen.getByLabelText('Note content'), 'y');
    await user.click(screen.getByLabelText('Insert into Agent prompt')); // uncheck (defaults checked)
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(create).toHaveBeenCalledWith('Privada', 'y', false);
  });

  it('deletes a note from the list', async () => {
    del.mockResolvedValue({ success: true });
    const user = userEvent.setup();
    render(<NotesModule />);

    await user.click(await screen.findByLabelText('Delete note Ideias'));

    expect(del).toHaveBeenCalledWith('note-2');
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
  });

  it('autosaves an edited note after the 500ms debounce', async () => {
    expect(NOTES_AUTOSAVE_DEBOUNCE_MS).toBe(500);
    update.mockResolvedValue({ success: true, note: { ...notes[0], content: 'Conteúdo do plano. extra' } });
    const user = userEvent.setup();
    render(<NotesModule />);

    await user.click(await screen.findByLabelText('Open note Plano'));
    await user.type(screen.getByLabelText('Note content'), ' extra');

    expect(update).not.toHaveBeenCalled();
    await waitFor(() => expect(update).toHaveBeenCalledWith('note-1', 'Plano', 'Conteúdo do plano. extra'));
  });

  it('pins a note', async () => {
    updateFlags.mockResolvedValue({ success: true, note: { ...notes[0], pinned: true } });
    const user = userEvent.setup();
    render(<NotesModule />);

    await user.click(await screen.findByLabelText('Pin note Plano'));

    expect(updateFlags).toHaveBeenCalledWith('note-1', true, false);
  });

  it('filters notes list by search term (title and content)', async () => {
    const user = userEvent.setup();
    render(<NotesModule />);
    await screen.findByLabelText('Open note Plano');

    await user.type(screen.getByLabelText('Search notes'), 'ideias');

    expect(screen.queryByLabelText('Open note Plano')).toBeNull();
    expect(screen.queryByLabelText('Open note Ideias')).not.toBeNull();
  });

  it('filters by content even when title does not match', async () => {
    const user = userEvent.setup();
    render(<NotesModule />);
    await screen.findByLabelText('Open note Plano');

    // 'plano' matches note-1 content ('Conteúdo do plano.') AND title; 'lista' matches note-2 content.
    await user.type(screen.getByLabelText('Search notes'), 'lista de');

    expect(screen.queryByLabelText('Open note Plano')).toBeNull();
    expect(screen.queryByLabelText('Open note Ideias')).not.toBeNull();
  });

  it('renders archived note with opacity-60 class when showArchived is on', async () => {
    const archivedNote = { id: 'note-3', title: 'Velha', content: 'arquivada', updatedAt: '2026-06-08', pinned: false, archived: true };
    list.mockResolvedValue({ success: true, notes: [...notes, archivedNote] });
    const user = userEvent.setup();
    render(<NotesModule />);
    await screen.findByLabelText('Open note Plano');

    await user.click(screen.getByRole('checkbox', { name: /show archived/i }));

    const archivedArticle = screen.getByLabelText('Open note Velha').closest('article');
    expect(archivedArticle?.className).toContain('opacity-60');
    // Non-archived notes must NOT have opacity-60.
    const normalArticle = screen.getByLabelText('Open note Plano').closest('article');
    expect(normalArticle?.className).not.toContain('opacity-60');
  });
})