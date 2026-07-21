import { useEffect, useReducer } from 'react';
import { onAwEvent, type SubagentPayload, type ChatDonePayload } from '@services/events';

// Live subagent (spawn) state per chat, driven by the backend `chat:subagent`
// lifecycle events — NOT by parsing the assistant text. Kept outside the React
// tree (like chat-send-state) so a run started while the user is on another view
// still updates, and the card is intact when they return.
//
// Each spawn is a block keyed by the run's id. While running it renders at the
// bottom of the timeline (the live turn); on chat:done it is anchored to that
// run's assistant message so it stays IN PLACE as newer messages arrive instead
// of floating at the end.

export type SubagentTaskStatus = 'running' | 'success' | 'error' | 'timeout';

export interface SubagentTask {
  id: string;
  task: string;
  status: SubagentTaskStatus;
  output?: string;
  elapsedMs?: number;
}

export interface SpawnBlock {
  runId: string;
  /** Assistant message this spawn belongs to; set on chat:done. Undefined while running. */
  messageId?: string;
  tasks: SubagentTask[];
}

const MAX_BLOCKS_PER_CHAT = 20;
const EMPTY: SpawnBlock[] = [];
const blocksByChat: Record<string, SpawnBlock[]> = {};
const listeners = new Set<() => void>();

function notify() {
  listeners.forEach((listener) => listener());
}

export function subscribeSubagentTasks(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function getSubagentBlocks(chatId: string | undefined): SpawnBlock[] {
  return (chatId && blocksByChat[chatId]) || EMPTY;
}

export function clearSubagentBlocks(chatId: string) {
  if (blocksByChat[chatId]) {
    delete blocksByChat[chatId];
    notify();
  }
}

function blockKey(event: SubagentPayload): string {
  return event.runId || 'live';
}

function updateBlock(chatId: string, runId: string, update: (block: SpawnBlock) => SpawnBlock) {
  const blocks = blocksByChat[chatId] || [];
  const index = blocks.findIndex((block) => block.runId === runId);
  if (index === -1) return;
  const next = blocks.slice();
  next[index] = update(next[index]);
  blocksByChat[chatId] = next;
  notify();
}

function applySubagentEvent(event: SubagentPayload) {
  const chatId = event.chatId;
  if (!chatId) return;
  const runId = blockKey(event);

  if (event.phase === 'start') {
    const existing = (blocksByChat[chatId] || []).filter((block) => block.runId !== runId);
    const block: SpawnBlock = {
      runId,
      tasks: (event.tasks || []).map((task) => ({ id: task.id, task: task.task, status: 'running' as const })),
    };
    blocksByChat[chatId] = [...existing, block].slice(-MAX_BLOCKS_PER_CHAT);
    notify();
    return;
  }

  if (event.phase === 'task-done') {
    updateBlock(chatId, runId, (block) => ({
      ...block,
      tasks: block.tasks.map((task) => {
        if (event.result && task.id === event.result.id) {
          return { ...task, status: event.result.status, output: event.result.output, elapsedMs: event.result.elapsedMs };
        }
        if (!event.result && event.taskId && task.id === event.taskId) {
          return { ...task, status: 'error' as const };
        }
        return task;
      }),
    }));
    return;
  }

  if (event.phase === 'end') {
    updateBlock(chatId, runId, (block) => {
      const labelById = new Map(block.tasks.map((task) => [task.id, task.task]));
      return {
        ...block,
        tasks: (event.results || []).map((result) => ({
          id: result.id,
          task: labelById.get(result.id) || result.id,
          status: result.status,
          output: result.output,
          elapsedMs: result.elapsedMs,
        })),
      };
    });
  }
}

// On run completion, anchor that run's spawn block to its assistant message so
// the card renders in place instead of trailing the newest messages.
function applyChatDone(event: ChatDonePayload) {
  if (!event.chatId || !event.runId || !event.messageId) return;
  updateBlock(event.chatId, event.runId, (block) => (block.messageId ? block : { ...block, messageId: event.messageId }));
}

/**
 * Global listeners installed once (by AppShell) for the app's lifetime: spawn
 * lifecycle events update the per-chat blocks; chat:done anchors the finished
 * block to its message.
 */
export function initSubagentTasks(): () => void {
  const offSubagent = onAwEvent('chat:subagent', applySubagentEvent);
  const offDone = onAwEvent('chat:done', applyChatDone);
  return () => {
    offSubagent();
    offDone();
  };
}

/** Subscribe a component to the live subagent blocks for a chat. */
export function useSubagentBlocks(chatId: string | undefined): SpawnBlock[] {
  const [, forceRender] = useReducer((n: number) => n + 1, 0);
  useEffect(() => subscribeSubagentTasks(forceRender), []);
  return getSubagentBlocks(chatId);
}
