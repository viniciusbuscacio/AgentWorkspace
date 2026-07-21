import { beforeEach, describe, expect, it } from 'vitest';
import {
  addModelChangeMessage,
  addModelListMessage,
  clearModelChangeMessages,
  mergeModelChangeMessages,
  modelChangeText,
} from './model-change-messages';
import type { ChatModelResult } from '@services/chat.service';

function result(overrides: Partial<ChatModelResult>): ChatModelResult {
  return { success: true, ...overrides } as ChatModelResult;
}

describe('model-change-messages', () => {
  beforeEach(() => {
    globalThis.localStorage?.clear();
  });

  it('phrases a per-chat change and a revert differently', () => {
    expect(modelChangeText(result({ providerName: 'OpenAI', model: 'gpt-5.5' }))).toContain(
      'Model changed for this chat: OpenAI · gpt-5.5',
    );
    expect(modelChangeText(result({ providerName: 'Anthropic', model: 'claude', cleared: true }))).toContain(
      'reverted to the global default: Anthropic · claude',
    );
  });

  it('builds a system-role message and persists it for the chat', () => {
    const message = addModelChangeMessage('chat-1', result({ providerName: 'OpenAI', model: 'gpt-5.5', provider: 'openai' }));
    expect(message).not.toBeNull();
    expect(message?.role).toBe('system');
    expect(message?.sessionId).toBe('chat-1');

    const merged = mergeModelChangeMessages('chat-1', []);
    expect(merged).toHaveLength(1);
    expect(merged[0].id).toBe(message?.id);
  });

  it('renders the /model list output as a persisted system bubble', () => {
    const message = addModelListMessage('chat-1', '**Models for this chat**\n\n- `openai` · gpt-5.5');
    expect(message?.role).toBe('system');
    expect(message?.content).toContain('Models for this chat');
    // Persisted like a change bubble so it survives reopen.
    expect(mergeModelChangeMessages('chat-1', [])).toHaveLength(1);
  });

  it('clear drops only the given chat, so /new and /clear start truly empty', () => {
    addModelChangeMessage('chat-1', result({ providerName: 'OpenAI', model: 'gpt-5.5', provider: 'openai' }));
    addModelChangeMessage('chat-2', result({ providerName: 'OpenRouter', model: 'r1', provider: 'openrouter' }));

    clearModelChangeMessages('chat-1');

    // The emptied chat gets nothing merged back; the other chat keeps its bubble.
    expect(mergeModelChangeMessages('chat-1', [])).toEqual([]);
    expect(mergeModelChangeMessages('chat-2', []).length).toBe(1);
  });

  it('does not persist a failed change', () => {
    expect(addModelChangeMessage('chat-1', result({ success: false, error: 'nope' }))).toBeNull();
    expect(mergeModelChangeMessages('chat-1', [])).toHaveLength(0);
  });

  it('merge is idempotent by id and keeps chats isolated', () => {
    const message = addModelChangeMessage('chat-1', result({ providerName: 'OpenAI', model: 'gpt-5.5' }));
    const existing = message ? [message] : [];
    // Already present -> no duplicate.
    expect(mergeModelChangeMessages('chat-1', existing)).toHaveLength(1);
    // A different chat sees nothing.
    expect(mergeModelChangeMessages('chat-2', [])).toHaveLength(0);
  });
});
