import { domain } from '@services/models';

export interface PromptDebugSnapshot {
  id: string;
  moduleId?: string;
  timestamp: string;
  provider?: string;
  model?: string;
  turn?: number;
  planMode: boolean;
  systemPrompt?: string;
  userPrompt?: string;
  images?: Array<{ mimeType: string; dataLength: number }>;
  activeTools?: string[];
  raw?: unknown;
}

export type PromptDebugChatMessage = domain.Message & {
  promptDebug?: PromptDebugSnapshot;
};

const PROMPT_DEBUG_CACHE_KEY = 'aw.promptDebugMessages.v1';
const MAX_PROMPT_DEBUG_MESSAGES_PER_CHAT = 20;

function readStoredDebugMessages(): Record<string, PromptDebugChatMessage[]> {
  try {
    const raw = globalThis.sessionStorage?.getItem(PROMPT_DEBUG_CACHE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function writeStoredDebugMessages(value: Record<string, PromptDebugChatMessage[]>): void {
  try {
    globalThis.sessionStorage?.setItem(PROMPT_DEBUG_CACHE_KEY, JSON.stringify(value));
  } catch {
    // Prompt debug is best-effort UI state; storage failures should not break chat.
  }
}

export function promptDebugMessage(snapshot: PromptDebugSnapshot): PromptDebugChatMessage | null {
  const chatId = snapshot.moduleId;
  if (!chatId) return null;
  return Object.assign(new domain.Message({
    id: snapshot.id,
    sessionId: chatId,
    role: 'system',
    content: '',
    createdAt: snapshot.timestamp,
  }), { promptDebug: snapshot });
}

export function addPromptDebugMessage(snapshot: PromptDebugSnapshot): PromptDebugChatMessage | null {
  const chatId = snapshot.moduleId;
  const message = promptDebugMessage(snapshot);
  if (!chatId || !message) return null;
  const stored = readStoredDebugMessages();
  const existing = Array.isArray(stored[chatId]) ? stored[chatId] : [];
  stored[chatId] = [...existing.filter((item) => item.id !== message.id), message]
    .slice(-MAX_PROMPT_DEBUG_MESSAGES_PER_CHAT);
  writeStoredDebugMessages(stored);
  return message;
}

// clearPromptDebugMessages drops a chat's stored snapshots. /new and /clear
// reset the backend transcript this store shadows; without this the snapshots
// would be merged right back into the freshly-emptied chat.
export function clearPromptDebugMessages(chatId: string): void {
  if (!chatId) return;
  const stored = readStoredDebugMessages();
  if (!(chatId in stored)) return;
  delete stored[chatId];
  writeStoredDebugMessages(stored);
}

export function mergePromptDebugMessages(chatId: string, messages: domain.Message[]): domain.Message[] {
  const stored = readStoredDebugMessages()[chatId];
  if (!Array.isArray(stored) || stored.length === 0) return messages;
  const ids = new Set(messages.map((message) => message.id));
  const extras = stored.filter((message) => !ids.has(message.id));
  return [...messages, ...extras].sort((a, b) => String(a.createdAt || '').localeCompare(String(b.createdAt || '')));
}
