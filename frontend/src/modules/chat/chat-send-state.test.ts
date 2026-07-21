import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const streamChatMessage = vi.fn();
vi.mock('@services/chat.service', () => ({
  chatService: { streamChatMessage: (...a: unknown[]) => streamChatMessage(...a) },
}));
vi.mock('@services/events', () => ({ onAwEvent: vi.fn(() => vi.fn()) }));

import {
  __resetChatSendState,
  enqueueChatMessage,
  flushChatQueue,
  getChatQueue,
  claimChat,
  releaseChat,
} from './chat-send-state';

beforeEach(() => {
  __resetChatSendState();
  vi.useFakeTimers();
  streamChatMessage.mockReset();
});
afterEach(() => {
  vi.useRealTimers();
});

describe('chat-send-state queue flush', () => {
  it('retries past a transient "already sending" (the post-stop race) instead of getting stuck', async () => {
    let attempt = 0;
    streamChatMessage.mockImplementation(() => {
      attempt += 1;
      // First attempt loses the race (backend run still clearing), then OK.
      return Promise.resolve(attempt === 1 ? { success: false, error: 'chat is already sending' } : { success: true });
    });

    enqueueChatMessage('queued', [], 'chat-1'); // triggers a flush
    // The first attempt loses the race; the retry timer fires and the second
    // succeeds. Without the retry the message would be stuck forever.
    await vi.advanceTimersByTimeAsync(300);
    await vi.runOnlyPendingTimersAsync();
    expect(attempt).toBe(2);
    expect(getChatQueue().length).toBe(0); // drained, not stuck
  });

  it('a busy chat (claimed) keeps the message queued until released', async () => {
    streamChatMessage.mockResolvedValue({ success: true });
    claimChat('chat-1'); // simulate an in-flight run
    enqueueChatMessage('waiting', [], 'chat-1');
    await vi.runOnlyPendingTimersAsync();
    expect(getChatQueue().length).toBe(1); // not sent while busy
    expect(streamChatMessage).not.toHaveBeenCalled();

    releaseChat('chat-1');
    flushChatQueue('chat-1');
    await vi.runOnlyPendingTimersAsync();
    expect(streamChatMessage).toHaveBeenCalledWith('chat-1', 'waiting', []);
    expect(getChatQueue().length).toBe(0);
  });
});
