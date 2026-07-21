// @vitest-environment happy-dom
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { TasksModule } from './TasksModule';

const list = vi.fn();
const add = vi.fn();
const update = vi.fn();
const del = vi.fn();
const addAttachment = vi.fn();
const getAttachmentData = vi.fn();
const deleteAttachment = vi.fn();

vi.mock('@services/tasks.service', () => ({
  BACKLOG_STATUSES: ['open', 'in-progress', 'needs-validation', 'completed'],
  tasksService: {
    list: (...args: unknown[]) => list(...args),
    add: (...args: unknown[]) => add(...args),
    update: (...args: unknown[]) => update(...args),
    delete: (...args: unknown[]) => del(...args),
    addAttachment: (...args: unknown[]) => addAttachment(...args),
    getAttachmentData: (...args: unknown[]) => getAttachmentData(...args),
    deleteAttachment: (...args: unknown[]) => deleteAttachment(...args),
  },
}));

const items = [
  { id: 'task-1', title: 'Revisar spec', body: '', status: 'open', position: 1, attachments: [] },
  { id: 'task-2', title: 'Fase 4', body: '', status: 'completed', position: 2, attachments: [] },
];

describe('TasksModule', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue({ success: true, items });
  });

  it('lists items with their status', async () => {
    render(<TasksModule />);
    expect(await screen.findByText('Revisar spec')).toBeTruthy();
    expect(screen.getByText('Completed')).toBeTruthy();
    expect(screen.getByLabelText('Mark Fase 4 as open')).toBeTruthy();
  });

  it('adds an item', async () => {
    add.mockResolvedValue({ success: true, item: { id: 'task-3', title: 'Nova', body: '', status: 'open', position: 3, attachments: [] } });
    const user = userEvent.setup();
    render(<TasksModule />);
    await screen.findByText('Revisar spec');

    await user.type(screen.getByLabelText('New task'), 'Nova');
    await user.click(screen.getByRole('button', { name: 'Add' }));

    expect(add).toHaveBeenCalledWith('Nova');
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
  });

  it('toggles an item status', async () => {
    update.mockResolvedValue({ success: true });
    const user = userEvent.setup();
    render(<TasksModule />);

    await user.click(await screen.findByLabelText('Mark Revisar spec as completed'));

    expect(update).toHaveBeenCalledWith('task-1', '', '', 'completed');
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
  });

  it('edits detail title body and status', async () => {
    update.mockResolvedValue({ success: true });
    const user = userEvent.setup();
    render(<TasksModule />);

    await user.click(await screen.findByLabelText('Open task Revisar spec'));
    await user.clear(screen.getByLabelText('Tasks item title'));
    await user.type(screen.getByLabelText('Tasks item title'), 'Revisar UI');
    await user.type(screen.getByLabelText('Tasks item body'), 'Critérios');
    // Status is a ZoomSafeSelect: open the combobox, pick the option.
    await user.click(screen.getByRole('combobox', { name: 'Tasks item status' }));
    await user.click(screen.getByRole('option', { name: 'In progress' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(update).toHaveBeenCalledWith('task-1', 'Revisar UI', 'Critérios', 'in-progress');
  });

  it('keeps unsaved edits when a refresh replaces the list mid-edit', async () => {
    update.mockResolvedValue({ success: true });
    const user = userEvent.setup();
    render(<TasksModule />);

    await user.click(await screen.findByLabelText('Open task Revisar spec'));
    await user.type(screen.getByLabelText('Tasks item body'), 'texto ainda não salvo');

    // A list-side status toggle triggers update + refresh with fresh item
    // objects; the open editor must NOT be re-synced (that wiped edits).
    list.mockResolvedValue({
      success: true,
      items: items.map((item) => ({ ...item })),
    });
    await user.click(screen.getByLabelText('Mark Revisar spec as completed'));
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));

    expect((screen.getByLabelText('Tasks item body') as HTMLTextAreaElement).value).toBe('texto ainda não salvo');

    // And Save persists the list-toggled status, not the stale editor one.
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(update).toHaveBeenLastCalledWith('task-1', 'Revisar spec', 'texto ainda não salvo', 'completed');
  });

  it('creates a spec chat from the detail view', async () => {
    update.mockResolvedValue({ success: true });
    const createSpecChat = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(<TasksModule createSpecChat={createSpecChat} />);

    await user.click(await screen.findByLabelText('Open task Revisar spec'));
    await user.click(screen.getByRole('button', { name: 'Create spec in chat' }));

    expect(update).toHaveBeenCalledWith('task-1', 'Revisar spec', '', 'open');
    await waitFor(() => expect(createSpecChat).toHaveBeenCalledWith('Revisar spec', ''));
  });

  it('disables the spec chat button without a handler', async () => {
    const user = userEvent.setup();
    render(<TasksModule />);

    await user.click(await screen.findByLabelText('Open task Revisar spec'));

    expect((screen.getByRole('button', { name: 'Create spec in chat' }) as HTMLButtonElement).disabled).toBe(true);
  });

  it('deletes an item', async () => {
    del.mockResolvedValue({ success: true });
    const user = userEvent.setup();
    render(<TasksModule />);

    await user.click(await screen.findByLabelText('Delete Fase 4'));

    expect(del).toHaveBeenCalledWith('task-2');
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
  });
});
