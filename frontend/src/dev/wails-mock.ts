/* eslint-disable @typescript-eslint/no-explicit-any */
// DEV-ONLY Wails runtime mock. Lets the React frontend boot in a plain browser
// (npm run dev) so the UI can be inspected/debugged via Edge/CDP without the Go
// backend. NEVER loaded inside the real Wails app (guarded by `window.go`).

type Listener = (data: any) => void;
const bus = new Map<string, Set<Listener>>();

function emit(name: string, data: any) {
  bus.get(name)?.forEach((cb) => cb(data));
}

const runtime: Record<string, any> = {
  EventsOn(name: string, cb: Listener) {
    if (!bus.has(name)) bus.set(name, new Set());
    bus.get(name)!.add(cb);
    return () => bus.get(name)?.delete(cb);
  },
  EventsOnMultiple(name: string, cb: Listener) {
    return runtime.EventsOn(name, cb);
  },
  EventsOnce(name: string, cb: Listener) {
    const off = runtime.EventsOn(name, (d: any) => { off(); cb(d); });
    return off;
  },
  EventsOff(name: string) { bus.delete(name); },
  EventsEmit(name: string, data: any) { emit(name, data); },
  LogInfo: console.log,
  LogError: console.error,
  LogDebug: console.debug,
  LogWarning: console.warn,
  Environment: () => Promise.resolve({ buildType: 'dev', platform: 'darwin', arch: 'arm64' }),
};

// ---- in-memory store ----
let chatSeq = 2;
const chats: any[] = [
  { id: 'chat-1', title: 'Welcome chat', archived: false, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() },
  { id: 'chat-2', title: 'Second chat', archived: false, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() },
];
const messages: Record<string, any[]> = {
  'chat-1': [
    { id: 'm1', sessionId: 'chat-1', role: 'user', content: 'Olá, isso é um mock dev.', createdAt: new Date().toISOString(), attachments: [] },
    { id: 'm2', sessionId: 'chat-1', role: 'assistant', content: 'Oi! Estou rodando **fora** do Wails, só pra debug da UI. 🎛️', createdAt: new Date().toISOString(), attachments: [] },
  ],
  'chat-2': [],
};

const ok = (extra: any = {}) => Promise.resolve({ success: true, ...extra });

const impl: Record<string, (...a: any[]) => any> = {
  VaultStatus: () => Promise.resolve({ unlocked: true, exists: true, currentProfile: { id: 'p1', name: 'Dev Vault', hasVault: true } }),
  ListProfiles: () => Promise.resolve([{ id: 'p1', name: 'Dev Vault', hasVault: true }]),
  ClearRecentProfiles: () => ok(),
  GetAppZoomPercent: () => Promise.resolve(100),
  SetAppZoomPercent: () => ok(),
  GetAutoLockMinutes: () => Promise.resolve(15),
  GetActiveTheme: () => Promise.resolve('midnight'),
  SaveActiveTheme: () => ok(),
  GetCustomThemes: () => Promise.resolve({}),
  SaveCustomTheme: () => ok(),
  DeleteCustomTheme: () => ok(),
  SuggestThemeFromPalette: () => Promise.resolve({ success: false, error: 'mock: AI unavailable' }),
  RecordActivity: () => Promise.resolve(),
  ListChats: () => Promise.resolve(chats.map((c) => ({ ...c }))),
  CreateChat: () => {
    chatSeq += 1;
    const chat = { id: `chat-${chatSeq}`, title: '', archived: false, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() };
    chats.unshift(chat);
    messages[chat.id] = [];
    return Promise.resolve({ ...chat });
  },
  CreateChatWithTitle: (title: string) => impl.CreateChat().then((c: any) => { c.title = title; const f = chats.find((x) => x.id === c.id); if (f) f.title = title; return c; }),
  ListMessages: (id: string) => Promise.resolve((messages[id] || []).map((m) => ({ ...m }))),
  RecentMessages: (id: string, limit: number) => Promise.resolve((messages[id] || []).slice(-limit)),
  GetAppVersion: () => Promise.resolve('dev'),
  GetChatSessionInfo: () => Promise.resolve({ ready: true, provider: 'mock', model: 'mock-model-1', planMode: false }),
  // REST "listening" so the sidebar exposure dot is visible in browser dev.
  GetServerIndicator: () => Promise.resolve({
    rest: { running: true, port: 9301 },
    mcp: { running: false, port: 0 },
    web: { running: false, port: 0 },
  }),
  GetPlanMode: () => ok({ enabled: false }),
  SetPlanMode: (_id: string, enabled: boolean) => ok({ enabled }),
  SetChatModel: (_id: string, provider: string, model: string) =>
    ok({ provider, providerName: provider || 'mock', model: model || 'mock-model-1', cleared: !provider }),
  StopChat: () => ok(),
  OpenPipWindow: () => ok(),
  ResolveToolConfirmation: () => ok(),
  ListChatTurns: () => Promise.resolve([]),
  RenameChat: (id: string, title: string) => { const c = chats.find((x) => x.id === id); if (c) c.title = title; return ok(); },
  ClearChat: (id: string) => { messages[id] = []; return ok(); },
  NewChatSession: () => ok(),
  CompactChat: () => ok(),
  SetChatArchived: (id: string, archived: boolean) => { const c = chats.find((x) => x.id === id); if (c) c.archived = archived; return ok(); },
  DeleteChat: (id: string) => { const i = chats.findIndex((x) => x.id === id); if (i >= 0) chats.splice(i, 1); delete messages[id]; return ok(); },
  StreamChatMessage: (id: string, text: string, attachments: any[]) => {
    const userMsg = { id: `u-${Date.now()}`, sessionId: id, role: 'user', content: text, createdAt: new Date().toISOString(), attachments: attachments || [] };
    (messages[id] ||= []).push(userMsg);
    emit('chat:start', { chatId: id });
    const reply = `Recebi: "${text}". Esta é uma resposta simulada do mock dev, em streaming, pra você ver os bubbles e o auto-scroll funcionando.`;
    const words = reply.split(' ');
    let seq = 0;
    let acc = '';
    const tick = () => {
      if (seq < words.length) {
        const delta = (seq === 0 ? '' : ' ') + words[seq];
        acc += delta;
        emit('chat:delta', { chatId: id, seq, delta });
        seq += 1;
        setTimeout(tick, 60);
      } else {
        const assistantMsg = { id: `a-${Date.now()}`, sessionId: id, role: 'assistant', content: acc, createdAt: new Date().toISOString(), attachments: [] };
        messages[id].push(assistantMsg);
        emit('chat:done', { chatId: id, messageId: assistantMsg.id });
      }
    };
    setTimeout(tick, 120);
    return ok();
  },
};

const appProxy = new Proxy(impl, {
  get(target, prop: string) {
    if (prop in target) return target[prop];
    // Unknown binding → harmless success.
    return (..._args: any[]) => ok();
  },
});

export function installWailsMock() {
  (window as any).runtime = new Proxy(runtime, {
    get(target, prop: string) {
      if (prop in target) return (target as any)[prop];
      return () => undefined;
    },
  });
  (window as any).go = { main: { App: appProxy } };
  console.info('[dev] Wails runtime mock installed — UI debug mode (no Go backend).');
}
