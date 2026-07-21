import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ChatDebugPanel } from './ChatDebugPanel';

const listChatTurns = vi.fn();

vi.mock('@services/chat-debug.service', () => ({
  chatDebugService: {
    listChatTurns: (...args: unknown[]) => listChatTurns(...args),
  },
}));

beforeEach(() => {
  listChatTurns.mockReset();
});

describe('ChatDebugPanel', () => {
  it('renders nothing when closed', () => {
    const { container } = render(<ChatDebugPanel chatId="c1" open={false} onClose={vi.fn()} />);
    expect(container.firstChild).toBeNull();
    expect(listChatTurns).not.toHaveBeenCalled();
  });

  it('lists turns and expands the raw request', async () => {
    listChatTurns.mockResolvedValue([
      {
        id: 't1',
        sessionId: 'c1',
        turnIndex: 0,
        requestJson: '{"model":"gpt","messages":[{"role":"user","content":"hi"}]}',
        responseText: 'hello there',
        toolCallsJson: '[]',
        model: 'gpt',
        promptTokens: 12,
        completionTokens: 5,
        finishReason: 'stop',
        createdAt: 'now',
      },
    ]);

    render(<ChatDebugPanel chatId="c1" open onClose={vi.fn()} />);

    await waitFor(() => expect(listChatTurns).toHaveBeenCalledWith('c1'));
    expect(await screen.findByText(/hello there/)).toBeTruthy();
    // Raw request is pretty-printed JSON, present in the DOM.
    expect(screen.getByText(/"model": "gpt"/)).toBeTruthy();
  });

  it('shows an empty state when there are no turns', async () => {
    listChatTurns.mockResolvedValue([]);
    render(<ChatDebugPanel chatId="c1" open onClose={vi.fn()} />);
    expect(await screen.findByText(/No turns recorded/)).toBeTruthy();
  });

  it('calls onClose from the Close button', async () => {
    listChatTurns.mockResolvedValue([]);
    const onClose = vi.fn();
    render(<ChatDebugPanel chatId="c1" open onClose={onClose} />);
    await screen.findByText(/No turns recorded/);
    await userEvent.click(screen.getByText('Close'));
    expect(onClose).toHaveBeenCalled();
  });
});
