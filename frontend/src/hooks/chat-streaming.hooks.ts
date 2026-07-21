import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { domain } from '@services/models';
import { onAwEvent, type ChatDeltaPayload } from '@services/events';

type StreamingMessage = domain.Message & {
  pending?: boolean;
  streaming?: boolean;
};

export function appendStreamingDelta(messages: domain.Message[], payload: ChatDeltaPayload): domain.Message[] {
  const activeAssistant = messages.find((message) => {
    const candidate = message as StreamingMessage;
    return candidate.sessionId === payload.chatId && candidate.role === 'assistant' && (candidate.pending || candidate.streaming);
  }) as StreamingMessage | undefined;
  if (activeAssistant) {
    return messages.map((message) => {
      if (message.id !== activeAssistant.id) return message;
      return Object.assign(new domain.Message({ ...message, content: `${message.content}${payload.delta}` }), {
        pending: false,
        streaming: true,
      });
    });
  }

  const tempId = `streaming-${payload.chatId}`;
  const existing = messages.find((message) => message.id === tempId);
  if (existing) {
    return messages.map((message) =>
      message.id === tempId ? new domain.Message({ ...message, content: `${message.content}${payload.delta}` }) : message,
    );
  }
  return [
    ...messages,
    new domain.Message({
      id: tempId,
      sessionId: payload.chatId,
      role: 'assistant',
      content: payload.delta,
      createdAt: new Date().toISOString(),
    }),
  ];
}

// Reconcile a freshly loaded persisted base with the in-progress stream. A
// loadChat that resolves mid-run (chat:start triggers it, and chat:refresh /
// auto-compaction can too) hands us a base WITHOUT the tokens already streamed
// into the live bubble. Blindly resetting to base drops those opening tokens
// and the next delta starts a fresh bubble, so they visibly vanish. When a run
// is still active for the chat, splice the accumulated streaming bubble back in
// (replacing the empty placeholder that shares its id) and drop any other empty
// pending assistant placeholder so the next delta cannot split the reply.
export function mergeStreamingIntoBase(
  base: domain.Message[],
  current: domain.Message[],
  chatId: string,
  runActive: boolean,
): domain.Message[] {
  if (!runActive) return base;
  const live = current.find((message) => {
    const candidate = message as StreamingMessage;
    return candidate.sessionId === chatId
      && candidate.role === 'assistant'
      && (candidate.content?.length ?? 0) > 0
      && (message.id === `streaming-${chatId}` || candidate.streaming || candidate.pending);
  });
  if (!live) return base;
  let replaced = false;
  const merged = base
    .map((message) => {
      if (message.id === live.id) { replaced = true; return live; }
      return message;
    })
    .filter((message) => {
      if (message.id === live.id) return true;
      const candidate = message as StreamingMessage;
      const emptyPending = candidate.role === 'assistant'
        && candidate.sessionId === chatId
        && Boolean(candidate.pending)
        && (message.content?.length ?? 0) === 0;
      return !emptyPending;
    });
  return replaced ? merged : [...merged, live];
}

// Port of AW2's shouldIgnoreRun: a chunk is dropped when its run was stopped
// locally, or when another run is already active for the chat (stale chunk).
// Chunks without a runId (older backend) pass through.
export function ignoreRunDelta(activeRunId: string | undefined, stoppedRuns: Set<string>, runId?: string): boolean {
  if (!runId) return false;
  if (stoppedRuns.has(runId)) return true;
  return Boolean(activeRunId && runId !== activeRunId);
}

export function useStreamingMessages(chatId: string | undefined, baseMessages: domain.Message[]) {
  const [messages, setMessages] = useState<domain.Message[]>(baseMessages);
  // Run registry covers ALL chats (not just the displayed one), so a late
  // delta from a run stopped in another chat is still recognized after the
  // user switches back (AW2's activeRunIdRef + stoppedRunIdsRef).
  const activeRunsRef = useRef<Record<string, string>>({});
  const stoppedRunsRef = useRef<Set<string>>(new Set());
  const prevChatIdRef = useRef(chatId);

  useEffect(() => {
    const chatChanged = prevChatIdRef.current !== chatId;
    prevChatIdRef.current = chatId;
    setMessages((current) => {
      if (!chatId) return baseMessages;
      if (chatChanged) {
        // A real chat switch resets to the target's base, but carry over any
        // optimistic bubbles already tagged with the chat we're switching INTO
        // (a brand-new chat's first send sets chatId, then adds them) so they
        // don't flash away before the persisted reload. A normal switch has
        // none for the target and resets cleanly.
        const baseIds = new Set(baseMessages.map((message) => message.id));
        const carried = current.filter((message) => message.sessionId === chatId && !baseIds.has(message.id));
        return carried.length ? [...baseMessages, ...carried] : baseMessages;
      }
      // Same chat: a base reload mid-run must preserve the streaming bubble.
      return mergeStreamingIntoBase(baseMessages, current, chatId, Boolean(activeRunsRef.current[chatId]));
    });
  }, [baseMessages, chatId]);

  useEffect(() => {
    const offStart = onAwEvent('chat:start', (payload) => {
      if (payload.runId) activeRunsRef.current[payload.chatId] = payload.runId;
    });
    const clearRun = (payload: { chatId: string; runId?: string }) => {
      if (!payload.runId || activeRunsRef.current[payload.chatId] === payload.runId) {
        delete activeRunsRef.current[payload.chatId];
      }
    };
    const offDone = onAwEvent('chat:done', clearRun);
    const offError = onAwEvent('chat:error', clearRun);
    return () => {
      offStart();
      offDone();
      offError();
    };
  }, []);

  useEffect(() => {
    const off = onAwEvent('chat:delta', (payload) => {
      if (!chatId || payload.chatId !== chatId) return;
      if (ignoreRunDelta(activeRunsRef.current[payload.chatId], stoppedRunsRef.current, payload.runId)) return;
      setMessages((current) => appendStreamingDelta(current, payload));
    });
    return off;
  }, [chatId]);

  // markRunStopped registers the chat's active run as stopped so its late
  // deltas are dropped (the backend keeps streaming for a moment after the
  // context is canceled).
  const markRunStopped = useCallback((targetChatId: string) => {
    const active = activeRunsRef.current[targetChatId];
    if (active) stoppedRunsRef.current.add(active);
    delete activeRunsRef.current[targetChatId];
  }, []);

  // Defense in depth: never render a message that belongs to another chat,
  // whatever path appended it (optimistic sends racing a chat switch). With
  // no chat selected nothing renders — a deleted chat's messages must not
  // linger on screen.
  const visibleMessages = useMemo(() => {
    if (!chatId) return [];
    return messages.filter((message) => !message.sessionId || message.sessionId === chatId);
  }, [messages, chatId]);

  return { messages: visibleMessages, setMessages, markRunStopped };
}
