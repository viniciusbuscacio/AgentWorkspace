// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { domain } from '@services/models';
import { ignoreRunDelta, useStreamingMessages } from './chat-streaming.hooks';
import type { AwEventName, AwEventPayloads } from '@services/events';

type Handler = (payload: unknown) => void;
const handlers = new Map<AwEventName, Set<Handler>>();

vi.mock('@services/events', () => ({
  onAwEvent: (name: AwEventName, callback: Handler) => {
    if (!handlers.has(name)) handlers.set(name, new Set());
    handlers.get(name)!.add(callback);
    return () => handlers.get(name)?.delete(callback);
  },
}));

function emit<TName extends AwEventName>(name: TName, payload: AwEventPayloads[TName]) {
  handlers.get(name)?.forEach((handler) => handler(payload));
}

function message(id: string, sessionId: string, role = 'user', content = 'x'): domain.Message {
  return new domain.Message({ id, sessionId, role, content, createdAt: new Date().toISOString() });
}

// Stable reference: an inline [] would re-trigger the hook's baseMessages
// effect on every render (in the app, baseMessages is stable state).
const NO_MESSAGES: domain.Message[] = [];

describe('ignoreRunDelta', () => {
  it('passes chunks without runId (older backend)', () => {
    expect(ignoreRunDelta('run-a', new Set(), undefined)).toBe(false);
  });
  it('drops chunks from stopped runs', () => {
    expect(ignoreRunDelta(undefined, new Set(['run-a']), 'run-a')).toBe(true);
  });
  it('drops stale chunks when another run is active', () => {
    expect(ignoreRunDelta('run-b', new Set(), 'run-a')).toBe(true);
  });
  it('passes chunks from the active run', () => {
    expect(ignoreRunDelta('run-a', new Set(), 'run-a')).toBe(false);
  });
});

describe('useStreamingMessages', () => {
  beforeEach(() => {
    handlers.clear();
  });

  it('appends deltas only for the displayed chat', () => {
    const { result } = renderHook(() => useStreamingMessages('chat-1', NO_MESSAGES));

    act(() => {
      emit('chat:delta', { chatId: 'chat-2', seq: 0, delta: 'wrong chat' });
      emit('chat:delta', { chatId: 'chat-1', seq: 0, delta: 'right chat' });
    });

    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0].content).toBe('right chat');
    expect(result.current.messages[0].sessionId).toBe('chat-1');
  });

  it('drops late deltas after the run is marked stopped', () => {
    const { result } = renderHook(() => useStreamingMessages('chat-1', NO_MESSAGES));

    act(() => {
      emit('chat:start', { chatId: 'chat-1', runId: 'run-a' });
      emit('chat:delta', { chatId: 'chat-1', runId: 'run-a', seq: 0, delta: 'before stop' });
    });
    act(() => {
      result.current.markRunStopped('chat-1');
    });
    act(() => {
      emit('chat:delta', { chatId: 'chat-1', runId: 'run-a', seq: 1, delta: ' after stop' });
    });

    expect(result.current.messages[0].content).toBe('before stop');
  });

  it('drops stale deltas from a previous run once a new run starts', () => {
    const { result } = renderHook(() => useStreamingMessages('chat-1', NO_MESSAGES));

    act(() => {
      emit('chat:start', { chatId: 'chat-1', runId: 'run-b' });
      emit('chat:delta', { chatId: 'chat-1', runId: 'run-a', seq: 7, delta: 'stale' });
      emit('chat:delta', { chatId: 'chat-1', runId: 'run-b', seq: 0, delta: 'fresh' });
    });

    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0].content).toBe('fresh');
  });

  it('never renders messages tagged with another sessionId', () => {
    const base = [message('m1', 'chat-1'), message('leak', 'chat-2')];
    const { result } = renderHook(() => useStreamingMessages('chat-1', base));

    expect(result.current.messages.map((m) => m.id)).toEqual(['m1']);
  });

  it('keeps in-progress streamed tokens when the base reloads mid-run', () => {
    const { result, rerender } = renderHook(
      ({ base }) => useStreamingMessages('chat-1', base),
      { initialProps: { base: NO_MESSAGES } },
    );

    act(() => {
      emit('chat:start', { chatId: 'chat-1', runId: 'run-a' });
      emit('chat:delta', { chatId: 'chat-1', runId: 'run-a', seq: 0, delta: 'Olá' });
    });
    expect(result.current.messages[0].content).toBe('Olá');

    // loadChat resolves mid-run: a fresh base carrying only an empty pending
    // placeholder (no streamed text yet persisted) must NOT wipe "Olá".
    const reloaded = [
      Object.assign(message('pending-assistant-rehydrate-chat-1', 'chat-1', 'assistant', ''), { pending: true }),
    ];
    rerender({ base: reloaded });

    const streamed = result.current.messages.find((m) => m.content === 'Olá');
    expect(streamed).toBeTruthy();
    // The empty placeholder was dropped so the next delta appends to the same
    // bubble instead of splitting the reply.
    act(() => {
      emit('chat:delta', { chatId: 'chat-1', runId: 'run-a', seq: 1, delta: ' mundo' });
    });
    const assistants = result.current.messages.filter((m) => m.role === 'assistant');
    expect(assistants).toHaveLength(1);
    expect(assistants[0].content).toBe('Olá mundo');
  });
});
