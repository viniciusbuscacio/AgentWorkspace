/* eslint-disable @typescript-eslint/no-explicit-any */
// PiP-mode Wails bindings shim. The PiP window is a separate process without
// the unlocked vault, so window.go/window.runtime are backed by the host
// app's token-scoped localhost API (see internal/infrastructure/pip) instead
// of real Wails bindings. Installed by main.tsx before React renders, which
// keeps every chat component byte-identical between the main and PiP windows.

export interface PipConfig {
  api: string;
  token: string;
  chatId: string;
}

export function readPipConfig(): PipConfig | null {
  const config = (window as any).aw_PIP;
  if (!config || typeof config.api !== 'string' || !config.api) return null;
  return config as PipConfig;
}

type Listener = (data: any) => void;

export function installPipBindings(config: PipConfig) {
  // The PiP window is itself a small Wails app, so real bindings exist before
  // this shim replaces window.go — capture the window-control one now.
  const nativePipControl = (window as any).go?.main?.PipControl;

  const bus = new Map<string, Set<Listener>>();

  function emit(name: string, payload: any) {
    bus.get(name)?.forEach((listener) => listener(payload));
  }

  async function api(path: string, init?: RequestInit): Promise<any> {
    const url = new URL(config.api + path);
    url.searchParams.set('token', config.token);
    const response = await fetch(url, {
      headers: { 'Content-Type': 'application/json' },
      ...init,
    });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(body?.error || `PiP API error ${response.status}`);
    }
    return body;
  }

  function connectEvents() {
    const url = new URL(`${config.api}/api/events`);
    url.searchParams.set('token', config.token);
    url.searchParams.set('chatId', config.chatId);
    const events = new EventSource(url);
    events.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data || '{}');
        if (data?.name) emit(data.name, data.payload ?? {});
      } catch {
        // Ignore malformed frames; the heartbeat comments never reach here.
      }
    };
    events.onerror = () => {
      emit('chat:error', { chatId: config.chatId, error: 'PiP connection lost. Reopen PiP from aw.' });
    };
  }

  async function snapshot() {
    return api(`/api/chat?chatId=${encodeURIComponent(config.chatId)}`);
  }

  // Resolves like the real StreamChatMessage: when this turn finishes.
  function streamChatMessage(chatId: string): Promise<any> {
    return new Promise((resolve) => {
      const cleanup: Array<() => void> = [];
      const settle = (value: any) => {
        cleanup.forEach((off) => off());
        resolve(value);
      };
      cleanup.push(on('chat:done', (payload: any) => {
        if (payload.chatId === chatId) settle({ success: true, sessionId: chatId });
      }));
      cleanup.push(on('chat:error', (payload: any) => {
        if (payload.chatId === chatId) settle({ success: false, sessionId: chatId, error: payload.error || 'Message failed' });
      }));
    });
  }

  function on(name: string, listener: Listener): () => void {
    if (!bus.has(name)) bus.set(name, new Set());
    bus.get(name)!.add(listener);
    return () => bus.get(name)?.delete(listener);
  }

  const notAvailable = { success: false, error: 'Not available in the PiP window' };

  const app: Record<string, (...args: any[]) => Promise<any>> = {
    ListChats: async () => {
      const data = await snapshot();
      return data.chat ? [data.chat] : [];
    },
    ListMessages: async () => (await snapshot()).messages || [],
    RecentMessages: async (_chatId: string, limit: number) => {
      const messages = (await snapshot()).messages || [];
      return messages.slice(-Math.max(0, limit));
    },
    GetChatSessionInfo: (chatId: string) => api(`/api/session-info?chatId=${encodeURIComponent(chatId)}`),
    StreamChatMessage: async (chatId: string, text: string, attachments: any[]) => {
      const result = streamChatMessage(chatId);
      await api('/api/send', { method: 'POST', body: JSON.stringify({ chatId, text, attachments }) });
      return result;
    },
    StopChat: async (chatId: string) => {
      await api('/api/stop', { method: 'POST', body: JSON.stringify({ chatId }) });
      return { success: true };
    },
    TranscribeAudio: (fileName: string, mimeType: string, dataUri: string) =>
      api('/api/transcribe', { method: 'POST', body: JSON.stringify({ fileName, mimeType, dataUri }) }),
    StartVoiceCapture: () => api('/api/voice/start', { method: 'POST', body: '{}' }),
    StopVoiceCapture: () => api('/api/voice/stop', { method: 'POST', body: '{}' }),
    CancelVoiceCapture: () => api('/api/voice/cancel', { method: 'POST', body: '{}' }),
    GetPlanMode: (chatId: string) => api(`/api/plan?chatId=${encodeURIComponent(chatId)}`),
    SetPlanMode: (chatId: string, enabled: boolean) =>
      api('/api/plan', { method: 'POST', body: JSON.stringify({ chatId, enabled }) }),
    SetChatModel: (chatId: string, provider: string, model: string) =>
      api('/api/set-chat-model', { method: 'POST', body: JSON.stringify({ chatId, provider, model }) }),
    ResolveToolConfirmation: (id: string, approved: boolean) =>
      api('/api/confirm', { method: 'POST', body: JSON.stringify({ id, approved }) }),
    NewChatSession: (chatId: string) =>
      api('/api/chat-op', { method: 'POST', body: JSON.stringify({ action: 'new', chatId }) }),
    CompactChat: (chatId: string) =>
      api('/api/chat-op', { method: 'POST', body: JSON.stringify({ action: 'compact', chatId }) }),
    ClearChat: (chatId: string) =>
      api('/api/chat-op', { method: 'POST', body: JSON.stringify({ action: 'clear', chatId }) }),
    ListChatTurns: async () => [],
    CreateChat: async () => { throw new Error('Not available in the PiP window'); },
    OpenPipWindow: async () => notAvailable,
  };

  (window as any).go = { main: { App: app } };
  (window as any).runtime = {
    EventsOn: on,
    EventsOnMultiple: (name: string, listener: Listener) => on(name, listener),
    EventsOnce: (name: string, listener: Listener) => {
      const off = on(name, (data) => { off(); listener(data); });
      return off;
    },
    EventsOff: (name: string) => bus.delete(name),
    EventsEmit: emit,
    LogInfo: console.log,
    LogError: console.error,
    LogDebug: console.debug,
    LogWarning: console.warn,
    Environment: () => Promise.resolve({ buildType: 'production', platform: 'darwin', arch: 'arm64' }),
  };

  // The vault gates everything: once it locks, this window must not keep
  // showing decrypted chat content — close it (all lock paths broadcast this).
  on('vault:auto-locked', () => {
    try {
      void nativePipControl?.Close?.();
    } catch {
      // Binding unavailable — the API is dead post-lock either way.
    }
  });

  if (typeof EventSource !== 'undefined') connectEvents();
}
