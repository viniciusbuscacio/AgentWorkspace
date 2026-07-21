import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ChatModule } from './ChatModule';
import { __resetChatSendState, enqueueChatMessage, markChatSending } from './chat-send-state';

const mocks = vi.hoisted(() => ({
  listMessages: vi.fn(),
  getChatSessionInfo: vi.fn(),
  createChat: vi.fn(),
  streamChatMessage: vi.fn(),
  stopChat: vi.fn(),
  openPipWindow: vi.fn(),
  setPlanMode: vi.fn(),
  newChatSession: vi.fn(),
  compactChat: vi.fn(),
  clearChat: vi.fn(),
  onAwEvent: vi.fn(),
}));

vi.mock('@services/chat.service', () => ({
  chatService: {
    listMessages: mocks.listMessages,
    getChatSessionInfo: mocks.getChatSessionInfo,
    createChat: mocks.createChat,
    streamChatMessage: mocks.streamChatMessage,
    stopChat: mocks.stopChat,
    openPipWindow: mocks.openPipWindow,
    setPlanMode: mocks.setPlanMode,
    newChatSession: mocks.newChatSession,
    compactChat: mocks.compactChat,
    clearChat: mocks.clearChat,
  },
}));

vi.mock('@services/events', () => ({
  onAwEvent: mocks.onAwEvent,
}));

describe('ChatModule queue', () => {
  // The send/queue state lives at module scope (it survives unmounts by
  // design) — tests must reset it explicitly.
  beforeEach(() => {
    __resetChatSendState();
  });
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('sends queued messages sequentially without starting concurrent backend runs', async () => {
    const user = userEvent.setup();
    const resolvers: Array<(value: unknown) => void> = [];

    mocks.listMessages.mockResolvedValue([]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model' });
    mocks.onAwEvent.mockReturnValue(() => undefined);
    mocks.streamChatMessage.mockImplementation(() => new Promise((resolve) => resolvers.push(resolve)));

    render(
      <ChatModule
        chatId="chat-1"
        onChatsChanged={vi.fn().mockResolvedValue([])}
        onSelectChat={vi.fn()}
      />,
    );

    const input = screen.getByPlaceholderText('Send message...');

    await user.type(input, 'first{enter}');
    await waitFor(() => expect(mocks.streamChatMessage).toHaveBeenCalledTimes(1));

    await user.type(input, 'second{enter}');
    await user.type(input, 'third{enter}');

    expect(mocks.streamChatMessage).toHaveBeenCalledTimes(1);
    expect(screen.getByText('2 messages waiting to be sent')).toBeTruthy();

    resolvers.shift()?.({ success: true });
    await waitFor(() => expect(mocks.streamChatMessage).toHaveBeenCalledTimes(2));
    expect(mocks.streamChatMessage.mock.calls[1][1]).toBe('second');

    resolvers.shift()?.({ success: true });
    await waitFor(() => expect(mocks.streamChatMessage).toHaveBeenCalledTimes(3));
    expect(mocks.streamChatMessage.mock.calls[2][1]).toBe('third');
  });

  it('delivers queued messages to their origin chat, never the displayed one', async () => {
    const user = userEvent.setup();
    const resolvers: Array<(value: unknown) => void> = [];

    mocks.listMessages.mockResolvedValue([]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model' });
    mocks.onAwEvent.mockReturnValue(() => undefined);
    mocks.streamChatMessage.mockImplementation(() => new Promise((resolve) => resolvers.push(resolve)));

    const props = {
      onChatsChanged: vi.fn().mockResolvedValue([]),
      onSelectChat: vi.fn(),
    };
    const { rerender } = render(<ChatModule chatId="chat-1" {...props} />);
    const input = screen.getByPlaceholderText('Send message...');

    // Start a send in chat-1, then queue a second message in chat-1.
    await user.type(input, 'first{enter}');
    await waitFor(() => expect(mocks.streamChatMessage).toHaveBeenCalledTimes(1));
    await user.type(input, 'queued-for-1{enter}');
    expect(screen.getByText('1 message waiting to be sent')).toBeTruthy();

    // Switch to chat-2 while chat-1 is still streaming. Its queue entry must
    // not show here.
    rerender(<ChatModule chatId="chat-2" {...props} />);
    expect(screen.queryByText('1 message waiting to be sent')).toBeNull();

    // chat-1's run completes while chat-2 is displayed: the queued message
    // flushes IMMEDIATELY into chat-1 (chat-inflight spec, Decision 3) —
    // never into the displayed chat.
    resolvers.shift()?.({ success: true });
    await waitFor(() => expect(mocks.streamChatMessage).toHaveBeenCalledTimes(2));
    expect(mocks.streamChatMessage.mock.calls[1][0]).toBe('chat-1');
    expect(mocks.streamChatMessage.mock.calls[1][1]).toBe('queued-for-1');

    // Returning to chat-1: nothing left waiting.
    rerender(<ChatModule chatId="chat-1" {...props} />);
    expect(screen.queryByText('1 message waiting to be sent')).toBeNull();
  });

  it('flushes the next queued message after the user stops the active run', async () => {
    const user = userEvent.setup();

    mocks.listMessages.mockResolvedValue([]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model' });
    mocks.onAwEvent.mockReturnValue(() => undefined);
    mocks.stopChat.mockResolvedValue({ success: true });
    mocks.streamChatMessage
      .mockImplementationOnce(() => new Promise(() => undefined))
      .mockResolvedValue({ success: true });

    render(<ChatModule chatId="chat-1" onChatsChanged={vi.fn().mockResolvedValue([])} onSelectChat={vi.fn()} />);
    const input = screen.getByPlaceholderText('Send message...');

    await user.type(input, 'first{enter}');
    await waitFor(() => expect(mocks.streamChatMessage).toHaveBeenCalledTimes(1));

    await user.type(input, 'queued-after-stop{enter}');
    expect(screen.getByText('1 message waiting to be sent')).toBeTruthy();

    await user.click(screen.getByRole('button', { name: /stop/i }));

    await waitFor(() => expect(mocks.streamChatMessage).toHaveBeenCalledTimes(2));
    expect(mocks.streamChatMessage.mock.calls[1][0]).toBe('chat-1');
    expect(mocks.streamChatMessage.mock.calls[1][1]).toBe('queued-after-stop');
  });
});

// Regression guard for the glass-veil layout bug (polish-wave-3-spec Decision 1).
// The chat column must always have the scroll container expanding first and the
// composer pinned last, regardless of what CSS wrapper is placed around the module.
describe('ChatModule layout', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('scroll container is the first in-flow child and precedes the composer', () => {
    mocks.listMessages.mockResolvedValue([]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test' });
    mocks.onAwEvent.mockReturnValue(() => undefined);

    render(
      <ChatModule
        chatId="chat-1"
        onChatsChanged={vi.fn().mockResolvedValue([])}
        onSelectChat={vi.fn()}
      />,
    );

    const section = document.querySelector('.chat-module') as HTMLElement;
    expect(section).toBeTruthy();

    const scroll = section.querySelector('.chat-scroll');
    const composer = section.querySelector('.chat-composer');
    expect(scroll).toBeTruthy();
    expect(composer).toBeTruthy();

    // Both must be direct children of the chat-module section.
    expect(scroll!.parentElement).toBe(section);
    expect(composer!.parentElement).toBe(section);

    // The expanding list row must come BEFORE the pinned composer row.
    const children = Array.from(section.children);
    const scrollIdx = children.indexOf(scroll as Element);
    const composerIdx = children.indexOf(composer as Element);
    expect(scrollIdx).toBeLessThan(composerIdx);
  });

  it('sending re-pins the chat to the bottom even when scrolled up', async () => {
    const user = userEvent.setup();
    mocks.listMessages.mockResolvedValue([]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test' });
    mocks.onAwEvent.mockReturnValue(() => undefined);
    mocks.streamChatMessage.mockResolvedValue({ success: true });

    render(
      <ChatModule
        chatId="chat-1"
        onChatsChanged={vi.fn().mockResolvedValue([])}
        onSelectChat={vi.fn()}
      />,
    );

    // Fake a tall, scrolled-up container (happy-dom reports 0 for all scroll
    // metrics by default).
    const scroll = document.querySelector('.chat-scroll') as HTMLDivElement;
    let scrollTop = 0;
    Object.defineProperties(scroll, {
      scrollHeight: { configurable: true, get: () => 1000 },
      clientHeight: { configurable: true, get: () => 200 },
      scrollTop: {
        configurable: true,
        get: () => scrollTop,
        set: (value: number) => { scrollTop = value; },
      },
    });
    scroll.dispatchEvent(new Event('scroll'));
    const latestButton = document.querySelector('.scroll-latest') as HTMLElement;
    await waitFor(() => expect(latestButton.className).toContain('is-visible'));

    // Sending is an unambiguous "show me now": the chat re-pins to the bottom.
    await user.type(screen.getByPlaceholderText('Send message...'), 'hello{enter}');
    await waitFor(() => expect(scrollTop).toBe(1000));
    await waitFor(() => expect(latestButton.className).not.toContain('is-visible'));
  });
});

// Regression fences for chat-inflight-state-spec and chat-identity-spec:
// the bugs reported on 2026-06-12 (leave mid-run and return, duplicated
// user bubble, "chat is already sending" in the UI, old chat painting into
// a new one) must stay dead.
describe('ChatModule in-flight state and identity', () => {
  beforeEach(() => {
    __resetChatSendState();
  });
  afterEach(() => {
    vi.clearAllMocks();
  });

  const props = () => ({
    onChatsChanged: vi.fn().mockResolvedValue([]),
    onSelectChat: vi.fn(),
  });

  it('rehydrates a remounted view from the backend: thinking bubble back, sends queue', async () => {
    const user = userEvent.setup();
    mocks.listMessages.mockResolvedValue([]);
    // The backend says a run is in flight for this chat (we left and came back).
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model', streaming: true });
    mocks.onAwEvent.mockReturnValue(() => undefined);

    render(<ChatModule chatId="chat-1" {...props()} />);

    // The thinking bubble is restored from the rehydrate, not from a local send.
    await waitFor(() => {
      expect(document.querySelector('.message-row.assistant.pending')).toBeTruthy();
    });

    // A new message while the run is in flight QUEUES — no backend call, no error.
    const input = screen.getByPlaceholderText('Send message...');
    await user.type(input, 'extra message{enter}');
    expect(screen.getByText('1 message waiting to be sent')).toBeTruthy();
    expect(mocks.streamChatMessage).not.toHaveBeenCalled();
    expect(document.querySelector('.chat-error')).toBeNull();
  });

  it('a backend "already sending" rejection enqueues silently — the string never renders', async () => {
    const user = userEvent.setup();
    mocks.listMessages.mockResolvedValue([]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model' });
    mocks.onAwEvent.mockReturnValue(() => undefined);
    mocks.streamChatMessage.mockResolvedValue({ success: false, error: 'chat is already sending' });

    render(<ChatModule chatId="chat-1" {...props()} />);
    const input = screen.getByPlaceholderText('Send message...');
    await user.type(input, 'race loser{enter}');

    await waitFor(() => expect(screen.getByText('1 message waiting to be sent')).toBeTruthy());
    expect(document.body.textContent).not.toContain('chat is already sending');
    expect(document.querySelector('.chat-error')).toBeNull();
  });

  it('does not queue when local busy state is stale but the backend is idle', async () => {
    const user = userEvent.setup();
    mocks.listMessages.mockResolvedValue([]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model', streaming: false });
    mocks.onAwEvent.mockReturnValue(() => undefined);
    mocks.streamChatMessage.mockResolvedValue({ success: true });

    render(<ChatModule chatId="chat-1" {...props()} />);
    await waitFor(() => expect(mocks.listMessages).toHaveBeenCalledWith('chat-1'));
    markChatSending('chat-1');

    const input = screen.getByPlaceholderText('Send message...');
    await user.type(input, 'stale busy{enter}');

    await waitFor(() => expect(mocks.streamChatMessage).toHaveBeenCalledWith('chat-1', 'stale busy', []));
    expect(screen.queryByText('1 message waiting to be sent')).toBeNull();
    expect(document.querySelector('.chat-error')).toBeNull();
  });

  it('delivers a queued message even when identical text is already persisted (no content-based drop)', async () => {
    // The queue delivers exactly what was enqueued: it must NOT drop a message
    // just because its text matches an already-persisted one. Content-based
    // pruning could not tell a genuine repeat (queuing "continue" twice) from a
    // dedup, so it silently lost the second copy. Flush removes each by id, so
    // double-send is already prevented without guessing by content.
    mocks.listMessages.mockResolvedValue([{
      id: 'u1', sessionId: 'chat-1', role: 'user', content: 'already sent', createdAt: new Date().toISOString(),
    }]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model', streaming: false });
    mocks.onAwEvent.mockReturnValue(() => undefined);
    mocks.streamChatMessage.mockResolvedValue({ success: true });
    markChatSending('chat-1');
    enqueueChatMessage('already sent', [], 'chat-1');

    render(<ChatModule chatId="chat-1" {...props()} />);

    await waitFor(() => expect(mocks.streamChatMessage).toHaveBeenCalledWith('chat-1', 'already sent', []));
    await waitFor(() => expect(screen.queryByText('1 message waiting to be sent')).toBeNull());
  });

  it('keeps Stop enabled while an assistant bubble is live even if local sending state drifted', async () => {
    mocks.listMessages.mockResolvedValue([Object.assign({
      id: 'pending-assistant-rehydrate-chat-1',
      sessionId: 'chat-1',
      role: 'assistant',
      content: '',
      createdAt: new Date().toISOString(),
    }, { pending: true })]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model', streaming: false });
    mocks.onAwEvent.mockReturnValue(() => undefined);

    render(<ChatModule chatId="chat-1" {...props()} />);

    const stop = await screen.findByRole('button', { name: /stop/i }) as HTMLButtonElement;
    await waitFor(() => expect(stop.disabled).toBe(false));
  });

  it('a failed send drops BOTH optimistic bubbles — no duplicated user message', async () => {
    const user = userEvent.setup();
    mocks.listMessages.mockResolvedValue([]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model' });
    mocks.onAwEvent.mockReturnValue(() => undefined);
    mocks.streamChatMessage.mockResolvedValue({ success: false, error: 'provider exploded' });

    render(<ChatModule chatId="chat-1" {...props()} />);
    const input = screen.getByPlaceholderText('Send message...');
    await user.type(input, 'hello there{enter}');

    await waitFor(() => expect(screen.getByText('provider exploded')).toBeTruthy());
    // The optimistic user bubble is gone (and so is the assistant one).
    expect(screen.queryByText('hello there')).toBeNull();
    expect(document.querySelector('.message-row.assistant.pending')).toBeNull();
  });

  it('renders the no-provider error as a deep link to LLM Providers settings', async () => {
    const user = userEvent.setup();
    const onOpenProvidersSettings = vi.fn();
    mocks.listMessages.mockResolvedValue([]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: false, error: 'No provider is configured yet. Open Settings > LLM Providers to add and activate a provider.' });
    mocks.onAwEvent.mockReturnValue(() => undefined);
    mocks.streamChatMessage.mockResolvedValue({ success: false, error: 'No provider is configured yet. Open Settings > LLM Providers to add and activate a provider.' });

    render(<ChatModule chatId="chat-1" {...props()} onOpenProvidersSettings={onOpenProvidersSettings} />);
    await user.type(screen.getByPlaceholderText('Send message...'), 'hello{enter}');

    const link = await screen.findByRole('button', { name: 'No provider is configured yet. Open Settings > LLM Providers to add and activate a provider.' });
    await user.click(link);
    expect(onOpenProvidersSettings).toHaveBeenCalledTimes(1);
  });

  it('a slow load for a previous chat never paints into the displayed one', async () => {
    const chat1Messages = [{
      id: 'm1', sessionId: 'chat-1', role: 'user', content: 'old chat content', createdAt: new Date().toISOString(),
    }];
    let resolveChat1: (value: unknown) => void = () => undefined;
    mocks.listMessages.mockImplementation((chatId: string) => {
      if (chatId === 'chat-1') return new Promise((resolve) => { resolveChat1 = resolve; });
      return Promise.resolve([]);
    });
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model' });
    mocks.onAwEvent.mockReturnValue(() => undefined);

    const shared = props();
    const { rerender } = render(<ChatModule chatId="chat-1" {...shared} />);
    // Switch to chat-2 before chat-1's load resolves, then let it resolve late.
    rerender(<ChatModule chatId="chat-2" {...shared} />);
    resolveChat1(chat1Messages);

    await waitFor(() => expect(mocks.listMessages).toHaveBeenCalledWith('chat-2'));
    expect(screen.queryByText('old chat content')).toBeNull();
  });

  it('deselecting clears the pane immediately — a deleted chat leaves nothing behind', async () => {
    mocks.listMessages.mockResolvedValue([{
      id: 'm1', sessionId: 'chat-1', role: 'user', content: 'soon to be deleted', createdAt: new Date().toISOString(),
    }]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model' });
    mocks.onAwEvent.mockReturnValue(() => undefined);

    const shared = props();
    const { rerender } = render(<ChatModule chatId="chat-1" {...shared} />);
    await waitFor(() => expect(screen.getByText('soon to be deleted')).toBeTruthy());

    rerender(<ChatModule chatId={undefined} {...shared} />);
    expect(screen.queryByText('soon to be deleted')).toBeNull();
  });
});

// Stop must clear the thinking bubble and not show "context canceled" as an
// error — the user stopped on purpose (2026-06-12).
describe('ChatModule stop clears the thinking indicator', () => {
  beforeEach(() => {
    __resetChatSendState();
  });
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('removes the pending bubble on stop and hides the cancel error', async () => {
    const user = userEvent.setup();
    mocks.listMessages.mockResolvedValue([]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model' });
    let errorHandler: ((p: { chatId: string; error: string }) => void) | undefined;
    let refreshHandler: ((p: { chatId?: string }) => void) | undefined;
    mocks.onAwEvent.mockImplementation((name: string, handler: (p: { chatId?: string; error?: string }) => void) => {
      if (name === 'chat:error') errorHandler = handler;
      if (name === 'chat:refresh') refreshHandler = handler;
      return () => undefined;
    });
    mocks.streamChatMessage.mockImplementation(() => new Promise(() => undefined)); // never resolves
    mocks.stopChat.mockResolvedValue({ success: true });

    render(<ChatModule chatId="chat-1" onChatsChanged={vi.fn().mockResolvedValue([])} onSelectChat={vi.fn()} />);
    const input = screen.getByPlaceholderText('Send message...');
    await user.type(input, 'oi{enter}');

    // Thinking bubble present.
    await waitFor(() => expect(document.querySelector('.message-row.assistant.pending')).toBeTruthy());

    // Stop: bubble gone immediately.
    await user.click(screen.getByRole('button', { name: /stop/i }));
    await waitFor(() => expect(document.querySelector('.message-row.assistant.pending')).toBeNull());

    // The backend's "context canceled" error must not surface.
    errorHandler?.({ chatId: 'chat-1', error: 'run failed: context canceled' });
    await waitFor(() => expect(document.querySelector('.chat-error')).toBeNull());

    mocks.listMessages.mockResolvedValue([{
      id: 'u1', sessionId: 'chat-1', role: 'user', content: 'oi', createdAt: new Date().toISOString(),
    }]);
    refreshHandler?.({ chatId: 'chat-1' });
    await waitFor(() => expect(screen.getByText('oi')).toBeTruthy());
  });
});

// An interrupted run must keep what the model already streamed: the content
// bubble survives chat:error and the module reloads so the persisted partial
// (with its interruption marker) can replace it (2026-07-20).
describe('ChatModule preserves streamed content on run error', () => {
  beforeEach(() => {
    __resetChatSendState();
  });
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('keeps the streamed bubble on chat:error and triggers a reload', async () => {
    const user = userEvent.setup();
    mocks.listMessages.mockResolvedValue([]);
    mocks.getChatSessionInfo.mockResolvedValue({ ready: true, provider: 'test', model: 'test-model' });
    // Both ChatModule and the streaming hook register chat:error/chat:delta —
    // collect every handler per event and fan out, or the test only exercises
    // whichever registered last.
    const handlers: Record<string, Array<(p: unknown) => void>> = {};
    mocks.onAwEvent.mockImplementation((name: string, handler: (p: unknown) => void) => {
      (handlers[name] ??= []).push(handler);
      return () => undefined;
    });
    const emit = (name: string, payload: unknown) => (handlers[name] ?? []).forEach((h) => h(payload));
    mocks.streamChatMessage.mockImplementation(() => new Promise(() => undefined)); // never resolves

    render(<ChatModule chatId="chat-1" onChatsChanged={vi.fn().mockResolvedValue([])} onSelectChat={vi.fn()} />);
    const input = screen.getByPlaceholderText('Send message...');
    await user.type(input, 'oi{enter}');
    await waitFor(() => expect(document.querySelector('.message-row.assistant.pending')).toBeTruthy());

    emit('chat:delta', { chatId: 'chat-1', seq: 0, delta: 'partial thinking survived' });
    await waitFor(() => expect(screen.getByText('partial thinking survived')).toBeTruthy());

    // Freeze the reload so the asserted state is the live bubble itself, not
    // the reloaded persisted message.
    const callsBeforeError = mocks.listMessages.mock.calls.length;
    mocks.listMessages.mockImplementation(() => new Promise(() => undefined));
    emit('chat:error', { chatId: 'chat-1', error: '429 Too Many Requests: Provider returned error' });

    // The streamed content stays on screen, the error surfaces, and a reload
    // was requested (it will deliver the persisted partial in the real app).
    await waitFor(() => expect(document.querySelector('.chat-error')).toBeTruthy());
    expect(screen.getByText('partial thinking survived')).toBeTruthy();
    expect(mocks.listMessages.mock.calls.length).toBeGreaterThan(callsBeforeError);
  });
});
