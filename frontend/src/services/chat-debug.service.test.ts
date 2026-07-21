import { describe, expect, it, vi } from 'vitest';
import { chatDebugService } from './chat-debug.service';

const ListChatTurns = vi.fn();

vi.mock('@wails/go/main/App', () => ({
  ListChatTurns: (...args: unknown[]) => ListChatTurns(...args),
}));

describe('chatDebugService', () => {
  it('uses the generated ListChatTurns binding', async () => {
    ListChatTurns.mockResolvedValue([
      { id: 't1', sessionId: 'c1', turnIndex: 0, requestJson: '{}', createdAt: 'now' },
    ]);
    await expect(chatDebugService.listChatTurns('c1')).resolves.toHaveLength(1);
    expect(ListChatTurns).toHaveBeenCalledWith('c1');
  });
});
