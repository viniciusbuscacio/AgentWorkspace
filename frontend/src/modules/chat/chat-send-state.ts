import { onAwEvent } from '@services/events';
import { chatService } from '@services/chat.service';
import type { QueuedMessage } from './QueuePanel';

// In-flight and queued-send state for EVERY chat, outside the React tree.
// ChatModule unmounts when the user navigates away; this state must survive
// that. The backend (chatRuns) is the real source of truth — this store
// mirrors it through the chat events and the session-info rehydrate, and a
// remounted view reads it instead of assuming "nothing is happening".

const sendingByChat: Record<string, boolean> = {};
// Claims are tokenized so an outdated finally{} cannot release a NEWER run's
// claim (a chat:done event may release + flush the next queued message while
// the previous send's promise is still settling).
const claimTokens: Record<string, number> = {};
let claimSeq = 0;
let queue: QueuedMessage[] = [];
// Per-chat retry counter for the post-stop race: the backend run the user
// just stopped is still clearing chatRuns, so the next send briefly hits
// "already sending". We retry for ~2s; the entry frees within ~100ms.
const flushRetries: Record<string, number> = {};
const MAX_FLUSH_RETRIES = 12;
const listeners = new Set<() => void>();

function notify() {
  listeners.forEach((listener) => listener());
}

export function subscribeChatSendState(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function isChatSending(chatId: string): boolean {
  return Boolean(sendingByChat[chatId]);
}

/** Atomically claim a chat for one run. Returns 0 when already claimed. */
export function claimChat(chatId: string): number {
  if (sendingByChat[chatId]) return 0;
  sendingByChat[chatId] = true;
  claimTokens[chatId] = ++claimSeq;
  notify();
  return claimTokens[chatId];
}

/** A run started elsewhere (PiP window, aw chat.send) marks its chat busy. */
export function markChatSending(chatId: string) {
  if (sendingByChat[chatId]) return;
  sendingByChat[chatId] = true;
  claimTokens[chatId] = ++claimSeq;
  notify();
}

/**
 * Release a claim. With a token, only the matching claim is released (a
 * stale finally never frees a newer run); without one the release is
 * authoritative (the backend said the run ended, or the user hit stop).
 */
export function releaseChat(chatId: string, token?: number) {
  if (!sendingByChat[chatId]) return;
  if (token && claimTokens[chatId] !== token) return;
  delete sendingByChat[chatId];
  delete claimTokens[chatId];
  notify();
}

export function getChatQueue(): QueuedMessage[] {
  return queue;
}

export function updateChatQueue(updater: (current: QueuedMessage[]) => QueuedMessage[]) {
  queue = updater(queue);
  notify();
}

export function enqueueChatMessage(text: string, attachments: QueuedMessage['attachments'], chatId: string) {
  updateChatQueue((current) => [...current, {
    id: `queued-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    chatId,
    content: text,
    attachments,
  }]);
  // An idle chat flushes immediately; a busy one flushes on its done event.
  flushChatQueue(chatId);
}

/**
 * Dispatch the oldest queued message for the chat — strictly one at a time,
 * and even when no chat view is mounted (the completion events drive the
 * next flush). The displayed view paints the run through the chat events;
 * errors surface through chat:error like any other run.
 */
export function flushChatQueue(chatId: string) {
  const next = queue.find((message) => message.chatId === chatId);
  if (!next) {
    delete flushRetries[chatId];
    return;
  }
  const token = claimChat(chatId);
  if (!token) return;
  updateChatQueue((current) => current.filter((message) => message.id !== next.id));
  void chatService.streamChatMessage(chatId, next.content, next.attachments)
    .then((result) => {
      releaseChat(chatId, token);
      if (result && result.success === false) {
        // Never lose a queued message: park it back at the head.
        updateChatQueue((current) => [next, ...current]);
        if (/already sending/i.test(result.error || '')) {
          // Race: a run (usually the one the user just stopped) is still
          // clearing on the backend. Retry briefly — do NOT mark sending,
          // which would block our own retry. If it keeps failing, a foreign
          // run is genuinely active: wait for its completion event instead.
          const attempts = (flushRetries[chatId] ?? 0) + 1;
          flushRetries[chatId] = attempts;
          if (attempts <= MAX_FLUSH_RETRIES) {
            setTimeout(() => flushChatQueue(chatId), 150);
          } else {
            delete flushRetries[chatId];
            markChatSending(chatId);
          }
        }
        return;
      }
      delete flushRetries[chatId];
      flushChatQueue(chatId);
    })
    .catch(() => {
      releaseChat(chatId, token);
      updateChatQueue((current) => [next, ...current]);
    });
}

/**
 * Global bookkeeping, installed ONCE by AppShell for the app's lifetime:
 * runs started anywhere mark their chat busy; completion releases the chat
 * and flushes its queue. This is what keeps the queue draining while the
 * user is on another view.
 */
export function initChatSendState(): () => void {
  const offStart = onAwEvent('chat:start', ({ chatId }) => markChatSending(chatId));
  const onRunEnded = ({ chatId }: { chatId: string }) => {
    releaseChat(chatId);
    flushChatQueue(chatId);
  };
  const offDone = onAwEvent('chat:done', onRunEnded);
  const offError = onAwEvent('chat:error', onRunEnded);
  return () => {
    offStart();
    offDone();
    offError();
  };
}

/** Test-only: reset the module-scope state between cases. */
export function __resetChatSendState() {
  for (const key of Object.keys(sendingByChat)) delete sendingByChat[key];
  for (const key of Object.keys(claimTokens)) delete claimTokens[key];
  for (const key of Object.keys(flushRetries)) delete flushRetries[key];
  queue = [];
  listeners.clear();
}
