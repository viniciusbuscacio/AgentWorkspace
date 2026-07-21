import { describe, expect, it } from 'vitest';
import { domain } from '@services/models';
import { appendStreamingDelta } from './chat-streaming.hooks';

describe('appendStreamingDelta', () => {
  it('creates and appends to the streaming assistant message', () => {
    const first = appendStreamingDelta([], { chatId: 'chat-1', seq: 0, delta: 'Oi' });
    expect(first[0]?.content).toBe('Oi');
    const second = appendStreamingDelta(first, { chatId: 'chat-1', seq: 1, delta: ' mundo' });
    expect(second).toHaveLength(1);
    expect(second[0]?.content).toBe('Oi mundo');
  });

  it('updates the pending assistant bubble created before the backend responds', () => {
    const pending = Object.assign(new domain.Message({
      id: 'pending-assistant-1',
      sessionId: 'chat-1',
      role: 'assistant',
      content: '',
      createdAt: new Date().toISOString(),
    }), { pending: true });

    const first = appendStreamingDelta([pending], { chatId: 'chat-1', seq: 0, delta: 'Oi' });
    expect(first).toHaveLength(1);
    expect(first[0]?.id).toBe('pending-assistant-1');
    expect(first[0]?.content).toBe('Oi');
    expect((first[0] as domain.Message & { pending?: boolean }).pending).toBe(false);
  });
});
