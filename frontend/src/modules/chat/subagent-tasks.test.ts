import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { SubagentPayload, ChatDonePayload } from '@services/events';

const listeners: Record<string, ((payload: unknown) => void) | undefined> = {};

vi.mock('@services/events', () => ({
  onAwEvent: vi.fn((name: string, cb: (payload: unknown) => void) => {
    listeners[name] = cb;
    return () => { listeners[name] = undefined; };
  }),
}));

const { initSubagentTasks, getSubagentBlocks, clearSubagentBlocks } = await import('./subagent-tasks');

function emitSubagent(payload: SubagentPayload) {
  listeners['chat:subagent']?.(payload);
}
function emitDone(payload: ChatDonePayload) {
  listeners['chat:done']?.(payload);
}

describe('subagent-tasks store', () => {
  beforeEach(() => {
    clearSubagentBlocks('chat-1');
    initSubagentTasks();
  });

  it('start creates a running block keyed by runId', () => {
    emitSubagent({ chatId: 'chat-1', runId: 'r1', phase: 'start', tasks: [{ id: 'a', task: 'Task A' }, { id: 'b', task: 'Task B' }] });
    const [block] = getSubagentBlocks('chat-1');
    expect(block.runId).toBe('r1');
    expect(block.messageId).toBeUndefined();
    expect(block.tasks.map((t) => [t.id, t.status])).toEqual([['a', 'running'], ['b', 'running']]);
  });

  it('task-done patches only the matching task, keeping the label', () => {
    emitSubagent({ chatId: 'chat-1', runId: 'r1', phase: 'start', tasks: [{ id: 'a', task: 'Task A' }, { id: 'b', task: 'Task B' }] });
    emitSubagent({ chatId: 'chat-1', runId: 'r1', phase: 'task-done', taskId: 'a', result: { id: 'a', status: 'success', output: 'done A', elapsedMs: 12 } });
    const [a, b] = getSubagentBlocks('chat-1')[0].tasks;
    expect(a).toMatchObject({ id: 'a', task: 'Task A', status: 'success', output: 'done A' });
    expect(b).toMatchObject({ id: 'b', status: 'running' });
  });

  it('chat:done anchors the finished block to its message', () => {
    emitSubagent({ chatId: 'chat-1', runId: 'r1', phase: 'start', tasks: [{ id: 'a', task: 'A' }] });
    emitSubagent({ chatId: 'chat-1', runId: 'r1', phase: 'end', results: [{ id: 'a', status: 'success', output: 'ok', elapsedMs: 5 }] });
    expect(getSubagentBlocks('chat-1')[0].messageId).toBeUndefined();
    emitDone({ chatId: 'chat-1', runId: 'r1', messageId: 'msg-99' });
    expect(getSubagentBlocks('chat-1')[0].messageId).toBe('msg-99');
  });

  it('a chat:done for an unrelated run does not anchor the block', () => {
    emitSubagent({ chatId: 'chat-1', runId: 'r1', phase: 'start', tasks: [{ id: 'a', task: 'A' }] });
    emitDone({ chatId: 'chat-1', runId: 'r2', messageId: 'other' });
    expect(getSubagentBlocks('chat-1')[0].messageId).toBeUndefined();
  });

  it('keeps blocks separated per chat', () => {
    emitSubagent({ chatId: 'chat-1', runId: 'r1', phase: 'start', tasks: [{ id: 'a', task: 'A' }] });
    expect(getSubagentBlocks('chat-2')).toEqual([]);
  });
});
