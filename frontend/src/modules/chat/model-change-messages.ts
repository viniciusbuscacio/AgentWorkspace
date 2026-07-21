import { domain } from '@services/models';
import type { ChatModelResult } from '@services/chat.service';

// Model-change messages are client-side system bubbles announcing that this
// chat's provider/model override changed (via the /model command). They mirror
// inline-image-messages: rendered like a normal message in a system-colored
// bubble, persisted in localStorage so they survive an app restart, and NEVER
// sent to the model — a per-chat override is UI/session state, not a chat turn.

const MODEL_CHANGE_CACHE_KEY = 'aw.modelChangeMessages.v1';
const MAX_MODEL_CHANGES_PER_CHAT = 20;

function store(): Storage | undefined {
  return globalThis.localStorage;
}

function readStored(): Record<string, domain.Message[]> {
  try {
    const raw = store()?.getItem(MODEL_CHANGE_CACHE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function writeStored(value: Record<string, domain.Message[]>): void {
  try {
    store()?.setItem(MODEL_CHANGE_CACHE_KEY, JSON.stringify(value));
  } catch {
    try {
      const trimmed: Record<string, domain.Message[]> = {};
      for (const [chatId, msgs] of Object.entries(value)) {
        trimmed[chatId] = Array.isArray(msgs) ? msgs.slice(-3) : [];
      }
      store()?.setItem(MODEL_CHANGE_CACHE_KEY, JSON.stringify(trimmed));
    } catch {
      // Best-effort UI state; never break the chat over it.
    }
  }
}

// modelChangeText renders the announcement shown in the system bubble.
export function modelChangeText(result: ChatModelResult): string {
  const provider = result.providerName || result.provider || 'default provider';
  const model = result.model ? ` · ${result.model}` : '';
  if (result.cleared) {
    return `Model reverted to the global default: ${provider}${model}`;
  }
  return `Model changed for this chat: ${provider}${model}`;
}

function makeId(chatId: string, seed: string): string {
  // Deterministic-ish id without Date.now collisions across rapid changes: the
  // seed (provider+model+cleared) plus a short random suffix keeps re-issuing
  // the same change idempotent within a render while still allowing history.
  return `model-change-${chatId}-${seed}-${Math.random().toString(36).slice(2, 8)}`;
}

// persistSystemMessage builds a system-role message, persists it keyed by chat
// (so it survives reopen/restart like the inline-image side-channel), and
// returns it for immediate rendering. Never sent to the model.
function persistSystemMessage(chatId: string, content: string, seed: string): domain.Message | null {
  if (!chatId || !content) return null;
  const message = new domain.Message({
    id: makeId(chatId, seed),
    sessionId: chatId,
    role: 'system',
    content,
    createdAt: new Date().toISOString(),
  });
  const stored = readStored();
  const existing = Array.isArray(stored[chatId]) ? stored[chatId] : [];
  stored[chatId] = [...existing, message].slice(-MAX_MODEL_CHANGES_PER_CHAT);
  writeStored(stored);
  return message;
}

// addModelChangeMessage announces a per-chat model change in a system bubble.
export function addModelChangeMessage(chatId: string, result: ChatModelResult): domain.Message | null {
  if (!chatId || !result?.success) return null;
  const seed = `${result.cleared ? 'clear' : result.provider || 'p'}-${result.model || 'm'}`;
  return persistSystemMessage(chatId, modelChangeText(result), seed);
}

// addModelListMessage renders the /model list output in the same system bubble.
export function addModelListMessage(chatId: string, content: string): domain.Message | null {
  return persistSystemMessage(chatId, content, 'list');
}

// clearModelChangeMessages drops a chat's stored bubbles. /new and /clear
// reset the backend transcript this store shadows; without this the bubbles
// would be merged right back into the freshly-emptied chat.
export function clearModelChangeMessages(chatId: string): void {
  if (!chatId) return;
  const stored = readStored();
  if (!(chatId in stored)) return;
  delete stored[chatId];
  writeStored(stored);
}

export function mergeModelChangeMessages(chatId: string, messages: domain.Message[]): domain.Message[] {
  const stored = readStored()[chatId];
  if (!Array.isArray(stored) || stored.length === 0) return messages;
  const ids = new Set(messages.map((message) => message.id));
  const extras = stored.filter((message) => !ids.has(message.id));
  if (extras.length === 0) return messages;
  return [...messages, ...extras].sort((a, b) =>
    String(a.createdAt || '').localeCompare(String(b.createdAt || '')),
  );
}
