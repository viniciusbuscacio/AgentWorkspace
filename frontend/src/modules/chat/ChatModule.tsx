import { useCallback, useEffect, useRef, useState } from 'react';
import { domain, dto } from '@services/models';
import { chatService } from '@services/chat.service';
import { providerService } from '@services/provider.service';
import { onAwEvent } from '@services/events';
import { useChatAutoScroll } from '@hooks/chat-scroll.hooks';
import { useComposerHeight } from '@hooks/composer.hooks';
import { useStreamingMessages } from '@hooks/chat-streaming.hooks';
import { MessageList } from './MessageList';
import { Composer } from './Composer';
import { QueuePanel } from './QueuePanel';
import {
  claimChat,
  enqueueChatMessage,
  flushChatQueue,
  getChatQueue,
  isChatSending,
  markChatSending,
  releaseChat,
  subscribeChatSendState,
  updateChatQueue,
} from './chat-send-state';
import { SLASH_COMMANDS, isSlashDraft, providerToken, resolveModelArg, type ModelProviderHint } from './slash-commands';
import { useSubagentBlocks } from './subagent-tasks';
import { ToolConfirmDialog } from './ToolConfirmDialog';
import { ChatDebugPanel } from './ChatDebugPanel';
import { addPromptDebugMessage, clearPromptDebugMessages, mergePromptDebugMessages } from './prompt-debug-messages';
import { clearInlineImageMessages, inlineImageMessage, mergeInlineImageMessages } from './inline-image-messages';
import { addModelChangeMessage, addModelListMessage, clearModelChangeMessages, mergeModelChangeMessages } from './model-change-messages';

const providerNotConfiguredMessage = 'No provider is configured yet. Open Settings > LLM Providers to add and activate a provider.';

function isProviderNotConfiguredError(message: string): boolean {
  return /no provider is configured yet/i.test(message);
}

interface ModelDefaults {
  sessionModel: string;
  globalDefault: string;
}

function modelListMarkdown(providers: ModelProviderHint[], defaults: ModelDefaults): string {
  const lines: string[] = ['**Available & enabled providers**', ''];
  // The effective provider: first non-benched in fallback order — the one
  // actually answering while earlier entries are failing.
  const effective = providers.find((p) => !p.benched);
  if (providers.length === 0) {
    lines.push('_No providers are connected._ Add one in Settings › LLM Providers.');
  } else {
    providers.forEach((provider, index) => {
      const model = provider.model ? ` · ${provider.model}` : '';
      const flag = provider.benched
        ? ' — (⚠️ failing, fallback active)'
        : provider.id === effective?.id
          ? ' — **(active)**'
          : '';
      lines.push(`${index + 1}. **${provider.name}**${model} — \`/model ${providerToken(provider.name)}\`${flag}`);
    });
  }
  // Reference lines repeat the matching list entry verbatim (number, name ·
  // model, /model token) so the user can see at a glance which entry a chat
  // maps to. Unmatched labels ("not configured") pass through untouched.
  const refEntry = (label: string): string => {
    const index = providers.findIndex((p) => label.startsWith(p.name));
    if (index < 0) return label;
    return `${index + 1}. ${label} — \`/model ${providerToken(providers[index].name)}\``;
  };
  const usingBenched = providers.find((p) => p.benched && defaults.sessionModel.startsWith(p.name));
  const chatLine = usingBenched && effective
    ? `**This chat is using:** ${effective.name}${effective.model ? ` · ${effective.model}` : ''} **(active)** — ${usingBenched.name} is failing, fallback active`
    : `**This chat is using:** ${refEntry(defaults.sessionModel)}`;
  const defaultBenched = providers.find((p) => p.benched && defaults.globalDefault.startsWith(p.name));
  lines.push(
    '',
    chatLine,
    '',
    `**Agent Workspace default:** ${refEntry(defaults.globalDefault)}${defaultBenched ? ' (⚠️ failing, fallback active)' : ''}`,
    '',
    'Type `/model <provider> [model]` to switch, or `/model default` to use the Agent Workspace default.',
  );
  return lines.join('\n');
}

interface ChatModuleProps {
  chatId: string | undefined;
  onChatsChanged: () => Promise<domain.Chat[]>;
  onSelectChat: (chatId: string | undefined) => void;
  onOpenProvidersSettings?: () => void;
  /** Rendering inside the PiP window: hides the pop-out button. */
  isPip?: boolean;
}

export function ChatModule({ chatId, onChatsChanged, onSelectChat, onOpenProvidersSettings, isPip }: ChatModuleProps) {
  const [baseMessages, setBaseMessages] = useState<domain.Message[]>([]);
  const [sessionInfo, setSessionInfo] = useState<dto.ChatSessionInfo | null>(null);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [debugOpen, setDebugOpen] = useState(false);
  const [modelProviders, setModelProviders] = useState<ModelProviderHint[]>([]);
  // The Agent Workspace global default (active provider in Settings), shown by
  // /model list so the user can tell it apart from this chat's override.
  const [globalDefault, setGlobalDefault] = useState('');
  // In-flight + queue truth lives OUTSIDE the component (chat-send-state):
  // this view unmounts on navigation and must not forget running turns.
  // `queue` and `sending` are render mirrors of that store.
  const [queue, setQueue] = useState(getChatQueue());
  // chatIdRef mirrors the displayed chat (AW2's moduleIdRef): async sends
  // compare against it before touching the visible message list.
  const chatIdRef = useRef(chatId);
  useEffect(() => {
    chatIdRef.current = chatId;
  }, [chatId]);
  useEffect(() => subscribeChatSendState(() => {
    setQueue(getChatQueue());
    const displayed = chatIdRef.current;
    setSending(displayed ? isChatSending(displayed) : false);
  }), []);
  const { messages, setMessages, markRunStopped } = useStreamingMessages(chatId, baseMessages);
  const spawnBlocks = useSubagentBlocks(chatId);
  const { scrollRef, isAtBottom, scrollToBottom } = useChatAutoScroll([messages, sending]);
  const { height: composerHeight, beginResize } = useComposerHeight();
  const hasLiveAssistant = messages.some((message) => {
    const candidate = message as domain.Message & { pending?: boolean; streaming?: boolean };
    return candidate.role === 'assistant' && (candidate.pending || candidate.streaming);
  });
  const runVisible = sending || hasLiveAssistant;

  const loadChat = useCallback(async () => {
    if (!chatId) return;
    const target = chatId;
    const [loadedMessages, info] = await Promise.all([
      chatService.listMessages(target),
      chatService.getChatSessionInfo(target),
    ]);
    // Staleness guard: a slow response for a chat that is no longer
    // displayed is DISCARDED — it must never paint into another chat.
    if (chatIdRef.current !== target) return;
    if (info?.streaming) {
      // Rehydrate from the backend truth: a run is in flight for this chat
      // (we navigated away and back, or another window started it). Mark the
      // chat busy and restore the thinking bubble; deltas and the done
      // reload take it from here.
      markChatSending(target);
      const hasLiveAssistant = loadedMessages.some((message) => {
        const candidate = message as domain.Message & { pending?: boolean; streaming?: boolean };
        return candidate.role === 'assistant' && (candidate.pending || candidate.streaming);
      });
      if (!hasLiveAssistant) {
        // Seed the live bubble with the text the run already streamed (kept
        // in backend memory): switching modules or opening the PiP mid-run
        // no longer restarts the reply from a blank thinking bubble.
        const partial = info.partialReply ?? '';
        loadedMessages.push(Object.assign(new domain.Message({
          id: `pending-assistant-rehydrate-${target}`,
          sessionId: target,
          role: 'assistant',
          content: partial,
          createdAt: new Date().toISOString(),
        }), partial ? { streaming: true } : { pending: true }));
      }
    } else {
      // The queue only holds messages not yet flushed (flushChatQueue removes
      // each by id before sending), so there is nothing to reconcile against the
      // persisted chat here — release the claim and drain the next one. A prior
      // content-based prune dropped queued messages whose text merely repeated an
      // earlier delivered message (e.g. queuing "continue" twice lost the second).
      releaseChat(target);
      flushChatQueue(target);
    }
    setBaseMessages(mergeModelChangeMessages(target, mergeInlineImageMessages(target, mergePromptDebugMessages(target, loadedMessages))));
    setSessionInfo(info);
  }, [chatId]);

  useEffect(() => {
    if (!chatId) {
      // Deselected (deleted chat, no selection): clear immediately — the
      // previous chat's content never outlives its selection.
      setBaseMessages([]);
      setSessionInfo(null);
      return;
    }
    void loadChat();
  }, [chatId, loadChat]);

  // Connected & enabled providers feed the /model auto-complete, and the active
  // provider gives the global default shown by /model list. Best-effort: the PiP
  // window does not expose provider status, so /model there falls back to typing
  // the provider by hand.
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const status = await providerService.getProviderStatus();
        if (cancelled) return;
        const benched = await providerService.getBenchedProviders().catch(() => [] as string[]);
        if (cancelled) return;
        const fallbackOrder = await providerService.getProviderFallbackOrder().catch(() => [] as string[]);
        if (cancelled) return;
        const all = status.providers || [];
        // Fallback order: the active provider first, then the persisted
        // priority — the order requests actually try providers in.
        const rank = (id: string) => {
          if (id === status.active) return -1;
          const at = fallbackOrder.indexOf(id);
          return at === -1 ? fallbackOrder.length : at;
        };
        const hints = all
          .filter((provider) => provider.connected && provider.enabled)
          .sort((a, b) => rank(a.id) - rank(b.id))
          .map((provider) => ({
            id: provider.id,
            name: provider.name,
            model: provider.model || provider.defaultModel || '',
            connected: provider.connected,
            models: provider.models || [],
            benched: benched.includes(provider.id),
          }));
        setModelProviders(hints);
        const active = all.find((provider) => provider.id === status.active);
        setGlobalDefault(active ? `${active.name}${active.model ? ` · ${active.model}` : ''}` : '');
      } catch {
        // Provider status unavailable; /model still works by typing.
      }
    })();
    return () => { cancelled = true; };
  }, [chatId]);

  // Remove still-bouncing EMPTY assistant bubbles from the displayed list —
  // used on stop and on a failed/canceled run. A bubble that already streamed
  // content stays on screen: the backend persists the partial (with an
  // interruption marker) and the reload replaces the live bubble with it, so
  // dropping it here would flash the text away before the reload lands.
  const dropPendingBubbles = useCallback(() => {
    setMessages((current) => current.filter((message) => {
      const candidate = message as domain.Message & { pending?: boolean; streaming?: boolean };
      return !(
        candidate.role === 'assistant'
        && (candidate.pending || candidate.streaming)
        && (message.content?.length ?? 0) === 0
      );
    }));
  }, [setMessages]);

  // Display-scoped reactions: only events whose chatId matches the displayed
  // chat refresh the list or surface errors. chat:start also reloads so a
  // run started elsewhere (queue flush, PiP, aw chat.send) shows its user
  // message as soon as the run begins, not only on done.
  useEffect(() => {
    const offStart = onAwEvent('chat:start', (payload) => {
      if (payload.chatId === chatId) void loadChat();
    });
    const offDone = onAwEvent('chat:done', (payload) => {
      if (payload.chatId === chatId) void loadChat();
    });
    const offRefresh = onAwEvent('chat:refresh', (payload) => {
      if (!payload.chatId || payload.chatId === chatId) void loadChat();
    });
    const offError = onAwEvent('chat:error', (payload) => {
      if (payload.chatId !== chatId) return;
      // Either way the run may have persisted an interrupted partial reply —
      // reload so it replaces the live bubble (kept by dropPendingBubbles when
      // it has content).
      dropPendingBubbles();
      void loadChat();
      // A user-initiated stop cancels the backend run, which surfaces as
      // "context canceled" (or "Stopped") — that is not an error to show, and
      // the thinking bubble must stop bouncing. Stay quiet.
      if (/context canceled|canceled|cancelled|^stopped$/i.test(payload.error || '')) {
        return;
      }
      setError(payload.error);
    });
    const offCompacted = onAwEvent('chat:compacted', (payload) => {
      if (payload.chatId === chatId) void loadChat();
    });
    const offPromptDebug = onAwEvent('prompt:debug', (payload) => {
      const message = addPromptDebugMessage(payload.snapshot);
      if (!message || payload.snapshot.moduleId !== chatId) return;
      setMessages((current) => current.some((item) => item.id === message.id) ? current : [...current, message]);
    });
    const offInlineImage = onAwEvent('chat:inline-image', (payload) => {
      // Persistence is handled once at the AppShell level (always mounted, even
      // when the user navigated away); here we only render it live when this is
      // the active chat. Dedup by the payload's stable id.
      const message = inlineImageMessage(payload);
      if (!message || payload.chatId !== chatId) return;
      setMessages((current) => current.some((item) => item.id === message.id) ? current : [...current, message]);
    });
    return () => {
      offStart();
      offDone();
      offRefresh();
      offError();
      offCompacted();
      offPromptDebug();
      offInlineImage();
    };
  }, [chatId, loadChat, dropPendingBubbles, setMessages]);

  // Switching chats shows that chat's own in-flight state, not the previous
  // chat's spinner. (Run bookkeeping itself lives in chat-send-state.)
  useEffect(() => {
    setSending(Boolean(chatId && isChatSending(chatId)));
    setError('');
  }, [chatId]);

  const performSend = useCallback(async (text: string, attachments: domain.Attachment[], explicitTarget?: string) => {
    let targetChatId = explicitTarget ?? chatIdRef.current;
    let claimToken = 0;
    if (targetChatId) {
      // Claim atomically (check + set in one step): a busy chat means the
      // message queues — NEVER an error in the user's face.
      claimToken = claimChat(targetChatId);
      if (!claimToken) {
        const info = await chatService.getChatSessionInfo(targetChatId).catch(() => null);
        if (info && !info.streaming) {
          releaseChat(targetChatId);
          claimToken = claimChat(targetChatId);
        }
      }
      if (!claimToken) {
        enqueueChatMessage(text, attachments, targetChatId);
        return;
      }
    } else {
      // No chat selected: create one. Nothing can be in flight for it yet.
      const chat = await chatService.createChat();
      onSelectChat(chat.id);
      await onChatsChanged();
      targetChatId = chat.id;
      chatIdRef.current = chat.id;
      claimToken = claimChat(targetChatId);
    }

    const isDisplayed = () => chatIdRef.current === targetChatId;
    if (isDisplayed()) {
      setError('');
      setSending(true);
    }

    // On failure BOTH optimistic bubbles go: an orphaned user bubble next to
    // the persisted one was the duplicated-message bug.
    let pendingUserId = '';
    let pendingAssistantId = '';
    const dropPending = () => {
      if (pendingUserId || pendingAssistantId) {
        setMessages((current) => current.filter(
          (message) => message.id !== pendingUserId && message.id !== pendingAssistantId,
        ));
      }
    };
    try {
      // Optimistic bubbles only join the visible list while their chat is the
      // displayed one; either way they are tagged with the target sessionId,
      // and the hook filters by sessionId as a second fence.
      if (isDisplayed()) {
        const now = Date.now();
        const optimisticUser = new domain.Message({
          id: `pending-user-${now}`,
          sessionId: targetChatId,
          role: 'user',
          content: text,
          createdAt: new Date().toISOString(),
          attachments,
        });
        pendingUserId = optimisticUser.id;
        const pendingAssistant = Object.assign(new domain.Message({
          id: `pending-assistant-${now}`,
          sessionId: targetChatId,
          role: 'assistant',
          content: '',
          createdAt: new Date().toISOString(),
        }), { pending: true });
        pendingAssistantId = pendingAssistant.id;
        setMessages((current) => [...current, optimisticUser, pendingAssistant]);
      }

      const result = await chatService.streamChatMessage(targetChatId, text, attachments);
      if (!result.success) {
        dropPending();
        // The backend is the last fence against concurrent runs. If it says
        // a run is in flight (race we lost), the message QUEUES — the error
        // string is an internal signal, never UI. The claim is NOT released:
        // the chat really is busy, and the run's own done/error event is
        // what frees it and flushes the queue.
        if (/already sending/i.test(result.error || '')) {
          claimToken = 0;
          enqueueChatMessage(text, attachments, targetChatId);
          return;
        }
        if (isDisplayed()) setError(result.error || 'Message failed');
        return;
      }
      if (isDisplayed()) await loadChat();
      await onChatsChanged();
    } catch (err) {
      dropPending();
      if (isDisplayed()) setError(err instanceof Error ? err.message : 'Message failed');
    } finally {
      // Token-scoped: if the done event already released this claim and a
      // queued message took a NEW claim, this finally must not free it.
      if (claimToken) releaseChat(targetChatId, claimToken);
      if (isDisplayed()) setSending(isChatSending(targetChatId));
      flushChatQueue(targetChatId);
    }
  }, [loadChat, onChatsChanged, onSelectChat, setMessages]);

  const runAction = useCallback(async (action: () => Promise<dto.ChatOperationResult | dto.OperationResult>) => {
    const result = await action();
    if (!result.success) {
      setError(result.error || 'Action failed');
      return;
    }
    await Promise.all([loadChat(), onChatsChanged()]);
  }, [loadChat, onChatsChanged]);

  const applyPlanMode = useCallback(async (enabled: boolean) => {
    if (!chatId) return;
    const result = await chatService.setPlanMode(chatId, enabled);
    if (result.success) {
      setSessionInfo((current) => current ? new dto.ChatSessionInfo({ ...current, planMode: enabled }) : current);
    }
  }, [chatId]);

  const applyModelChange = useCallback(async (args: string) => {
    if (!chatId) return;
    if (!args.trim() || args.trim().toLowerCase() === 'list') {
      const activeName = sessionInfo?.provider
        ? (modelProviders.find((p) => p.id.toLowerCase() === sessionInfo.provider.toLowerCase())?.name ?? sessionInfo.provider)
        : '';
      const sessionModel = activeName
        ? `${activeName}${sessionInfo?.model ? ` · ${sessionInfo.model}` : ''}`
        : 'the Agent Workspace default';
      // Fetch the bench state NOW — the mount-time hints predate any failure
      // that happened in this very chat.
      const benchedNow = await providerService.getBenchedProviders().catch(() => [] as string[]);
      const hintsNow = modelProviders.map((p) => ({ ...p, benched: benchedNow.includes(p.id) }));
      const listMessage = addModelListMessage(chatId, modelListMarkdown(hintsNow, {
        sessionModel,
        globalDefault: globalDefault || 'not configured',
      }));
      if (listMessage) setMessages((current) => [...current, listMessage]);
      await loadChat();
      return;
    }
    const trimmed = args.trim();
    let provider: string;
    let model: string;
    if (/^(default|clear|reset|global)$/i.test(trimmed)) {
      provider = '';
      model = '';
    } else {
      // The auto-complete emits a hyphen-slugged provider ("Maritaca-IA"); resolve
      // it (or a hand-typed spaced name) to the real provider id. Fall back to the
      // raw first token so the backend can still try to match it.
      const resolved = resolveModelArg(trimmed, modelProviders);
      if (resolved) {
        provider = resolved.providerId;
        model = resolved.model;
      } else {
        const parts = trimmed.split(/\s+/);
        provider = parts[0].replace(/-/g, ' ');
        model = parts.slice(1).join(' ');
      }
    }
    const result = await chatService.setChatModel(chatId, provider, model);
    if (!result.success) {
      setError(result.error || 'Could not change the model for this chat');
      return;
    }
    const message = addModelChangeMessage(chatId, result);
    if (message) setMessages((current) => [...current, message]);
    await loadChat();
  }, [chatId, sessionInfo, setMessages, loadChat, modelProviders, globalDefault]);

  const handleSlash = useCallback(async (raw: string) => {
    const trimmed = raw.trim();
    const [name, ...rest] = trimmed.split(/\s+/);
    const args = rest.join(' ');
    // /new and /clear reset the backend transcript; the client-side system
    // bubbles (model changes, inline images, prompt debug) shadow that
    // transcript in web storage and must be dropped too, or the reload merges
    // them straight back into the emptied chat.
    const clearSideChannelMessages = (id: string) => {
      clearModelChangeMessages(id);
      clearInlineImageMessages(id);
      clearPromptDebugMessages(id);
    };
    switch (name) {
      case '/new':
        if (chatId) {
          clearSideChannelMessages(chatId);
          await runAction(() => chatService.newChatSession(chatId));
        }
        break;
      case '/compact':
        if (chatId) await runAction(() => chatService.compactChat(chatId));
        break;
      case '/model':
        await applyModelChange(args);
        break;
      case '/plan':
        await applyPlanMode(!(sessionInfo?.planMode ?? false));
        break;
      case '/session-info':
        setNotice(sessionInfo?.ready ? `${sessionInfo.provider} · ${sessionInfo.model}` : (sessionInfo?.error || 'No active provider'));
        break;
      case '/clear':
        if (chatId) {
          clearSideChannelMessages(chatId);
          await runAction(() => chatService.clearChat(chatId));
        }
        break;
      case '/debug':
        setDebugOpen(true);
        break;
      case '/help':
        setNotice(`Commands: ${SLASH_COMMANDS.map((c) => c.name).join(', ')}`);
        break;
      default:
        setNotice(`Unknown command: ${name}. Type /help for available commands.`);
    }
  }, [chatId, sessionInfo, runAction, applyPlanMode, applyModelChange]);

  const transcribeAudio = useCallback(async (fileName: string, mimeType: string, dataUri: string) => {
    setError('');
    const result = await chatService.transcribeAudio(fileName, mimeType, dataUri);
    if (!result.success) throw new Error(result.error || 'Could not transcribe audio');
    return result.text || '';
  }, []);

  // Native (Go/ffmpeg) recording, used when the WebView has no getUserMedia.
  const startNativeVoice = useCallback(async () => {
    setError('');
    const result = await chatService.startVoiceCapture();
    if (!result.success) throw new Error(result.error || 'Could not start recording');
  }, []);

  const stopNativeVoice = useCallback(async () => {
    const result = await chatService.stopVoiceCapture();
    if (!result.success) throw new Error(result.error || 'Could not transcribe recording');
    return result.text || '';
  }, []);

  const cancelNativeVoice = useCallback(() => {
    void chatService.cancelVoiceCapture();
  }, []);

  function submit(text: string, attachments: domain.Attachment[]) {
    setNotice('');
    // Sending is an unambiguous "show me now": re-pin the chat to the bottom
    // even if the user had scrolled up (reading history never blocks sending).
    scrollToBottom({ force: true });
    if (isSlashDraft(text) && attachments.length === 0) {
      void handleSlash(text);
      return;
    }
    const target = chatId;
    if (target && (isChatSending(target) || getChatQueue().some((message) => message.chatId === target))) {
      void (async () => {
        const liveAssistantVisible = messages.some((message) => {
          const candidate = message as domain.Message & { pending?: boolean; streaming?: boolean };
          return candidate.role === 'assistant' && (candidate.pending || candidate.streaming);
        });
        if (isChatSending(target) && !liveAssistantVisible) {
          const info = await chatService.getChatSessionInfo(target).catch(() => null);
          if (info && !info.streaming) {
            releaseChat(target);
          }
        }
        if (!isChatSending(target) && !getChatQueue().some((message) => message.chatId === target)) {
          await performSend(text, attachments, target);
          return;
        }
        enqueueChatMessage(text, attachments, target);
      })();
      return;
    }
    void performSend(text, attachments, target);
  }

  async function openExternalChatWindow() {
    if (!chatId) return;
    setError('');
    setNotice('');
    const result = await chatService.openPipWindow(chatId);
    if (!result.success) {
      setError(result.error || 'Could not open external chat window');
    }
  }

  async function stop() {
    if (!chatId) return;
    // Register the stop locally first so late deltas from the canceled run
    // are dropped (AW2's stoppedRunIds), then cancel the backend run. The
    // release is authoritative (no token): the user said stop.
    markRunStopped(chatId);
    releaseChat(chatId);
    // Stop the bouncing dots immediately — do not wait for the cancel event.
    dropPendingBubbles();
    try {
      const result = await chatService.stopChat(chatId);
      if (result && result.success === false) {
        setError(result.error || 'Could not stop the current run.');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not stop the current run.');
    } finally {
      // Always settle the UI and drain the queue, even if the stop failed —
      // otherwise a rejected stopChat leaves the spinner logic and the queue
      // stuck. Stop = cancel the current run AND send the next queued message.
      setSending(false);
      flushChatQueue(chatId);
    }
  }

  const isEmpty = messages.length === 0;

  return (
    <section
      className="chat-module"
      style={{ ['--composer-textarea-height' as string]: `${composerHeight}px` }}
    >
      {/* Discreet overflow menu (clean AW2-style screen: no header button bar). */}

      <div ref={scrollRef} className={`chat-scroll ${isEmpty ? 'is-empty' : ''}`}>
        <MessageList messages={messages} spawnBlocks={spawnBlocks} />
      </div>

      <button
        className={`scroll-latest ${isAtBottom ? '' : 'is-visible'}`}
        type="button"
        aria-label="Scroll to latest message"
        title="Scroll to latest message"
        onClick={() => scrollToBottom({ force: true })}
      >
        <span className="material-symbols-outlined" aria-hidden="true">keyboard_arrow_down</span>
      </button>

      {error && (
        <div className="chat-error">
          {isProviderNotConfiguredError(error) && onOpenProvidersSettings ? (
            <button type="button" className="chat-error-link" onClick={onOpenProvidersSettings}>
              {providerNotConfiguredMessage}
            </button>
          ) : error}
        </div>
      )}
      {notice && (
        <div className="chat-notice">
          <span>{notice}</span>
          <button type="button" aria-label="Dismiss" onClick={() => setNotice('')}>
            <span className="material-symbols-outlined">close</span>
          </button>
        </div>
      )}
      <QueuePanel
        queue={queue.filter((message) => message.chatId === chatId)}
        onEdit={(id, content) => updateChatQueue((current) => content
          ? current.map((m) => (m.id === id ? { ...m, content } : m))
          : current.filter((m) => m.id !== id))}
        onDelete={(id) => updateChatQueue((current) => current.filter((m) => m.id !== id))}
      />
      <Composer
        sending={runVisible}
        onSubmit={submit}
        onStop={stop}
        onTranscribeAudio={transcribeAudio}
        onStartNativeVoice={startNativeVoice}
        onStopNativeVoice={stopNativeVoice}
        onCancelNativeVoice={cancelNativeVoice}
        onVoiceError={setError}
        isPip={isPip}
        draftKey={chatId ?? 'new-chat'}
        onOpenPip={!isPip && chatId ? () => void openExternalChatWindow() : undefined}
        height={composerHeight}
        onResizeStart={beginResize}
        modelProviders={modelProviders}
      />
      <ToolConfirmDialog />
      <ChatDebugPanel chatId={chatId} open={debugOpen} onClose={() => setDebugOpen(false)} />
    </section>
  );
}
