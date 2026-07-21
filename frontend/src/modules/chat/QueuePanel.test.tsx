import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { QueuePanel, type QueuedMessage } from './QueuePanel';

const queue: QueuedMessage[] = [
  { id: 'q1', chatId: 'chat-1', content: 'first queued', attachments: [] },
  { id: 'q2', chatId: 'chat-1', content: 'second queued', attachments: [] },
];

describe('QueuePanel', () => {
  it('starts collapsed: shows the count row but not the messages until clicked', async () => {
    render(<QueuePanel queue={queue} onEdit={vi.fn()} onDelete={vi.fn()} />);
    expect(screen.getByText('2 messages waiting to be sent')).toBeTruthy();
    expect(screen.queryByText('first queued')).toBeNull();
    await userEvent.click(screen.getByText('2 messages waiting to be sent'));
    expect(screen.getByText('first queued')).toBeTruthy();
    // Collapses again.
    await userEvent.click(screen.getByText('2 messages waiting to be sent'));
    expect(screen.queryByText('first queued')).toBeNull();
  });

  it('renders nothing when empty', () => {
    const { container } = render(<QueuePanel queue={[]} onEdit={vi.fn()} onDelete={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });

  it('lists queued messages and deletes one', async () => {
    const onDelete = vi.fn();
    render(<QueuePanel queue={queue} onEdit={vi.fn()} onDelete={onDelete} />);
    // Collapsed by default — expand to see the list.
    expect(screen.getByText('2 messages waiting to be sent')).toBeTruthy();
    await userEvent.click(screen.getByText('2 messages waiting to be sent'));
    expect(screen.getByText('first queued')).toBeTruthy();
    await userEvent.click(screen.getAllByRole('button', { name: 'Delete queued message' })[0]);
    expect(onDelete).toHaveBeenCalledWith('q1');
  });

  it('edits a queued message and saves', async () => {
    const onEdit = vi.fn();
    render(<QueuePanel queue={queue} onEdit={onEdit} onDelete={vi.fn()} />);
    await userEvent.click(screen.getByText('2 messages waiting to be sent'));
    await userEvent.click(screen.getAllByRole('button', { name: 'Edit queued message' })[1]);
    const textarea = screen.getByDisplayValue('second queued');
    await userEvent.clear(textarea);
    await userEvent.type(textarea, 'edited');
    await userEvent.click(screen.getByRole('button', { name: 'Save queued message' }));
    expect(onEdit).toHaveBeenCalledWith('q2', 'edited');
  });
});
