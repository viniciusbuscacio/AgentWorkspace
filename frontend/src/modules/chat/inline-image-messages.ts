import { domain } from '@services/models';
import type { InlineImagePayload } from '@services/events';

// Inline-image messages mirror prompt-debug-messages: client-side system
// messages that the backend pushes out of band (here, a screenshot the agent
// captured of its own UI) and which are merged into the chat view without
// being persisted as real chat turns. The model only ever sees a short text
// acknowledgement; the picture lives purely in the UI, exactly like AW2's
// inline-image side-channel.
//
// Unlike AW2 (and unlike prompt-debug, which is ephemeral), these are kept in
// localStorage so a captured screenshot survives an app restart and stays in
// the chat history — still without ever reaching the model context.

const INLINE_IMAGE_CACHE_KEY = 'aw.inlineImageMessages.v1';
const MAX_INLINE_IMAGES_PER_CHAT = 20;

function store(): Storage | undefined {
  return globalThis.localStorage;
}

function readStored(): Record<string, domain.Message[]> {
  try {
    const raw = store()?.getItem(INLINE_IMAGE_CACHE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function writeStored(value: Record<string, domain.Message[]>): void {
  try {
    store()?.setItem(INLINE_IMAGE_CACHE_KEY, JSON.stringify(value));
  } catch {
    // Likely a quota error: drop the oldest images across all chats and retry
    // once. Inline images are best-effort UI state; never break chat over them.
    try {
      const trimmed: Record<string, domain.Message[]> = {};
      for (const [chatId, msgs] of Object.entries(value)) {
        trimmed[chatId] = Array.isArray(msgs) ? msgs.slice(-3) : [];
      }
      store()?.setItem(INLINE_IMAGE_CACHE_KEY, JSON.stringify(trimmed));
    } catch {
      // Give up persisting; the in-session copy still renders.
    }
  }
}

function mimeFromDataUri(dataUri: string): string {
  return dataUri.match(/^data:([^;,]+)/)?.[1] || 'image/png';
}

function extensionFromMime(mime: string): string {
  return (mime.split('/')[1] || 'png').replace('jpeg', 'jpg');
}

export function inlineImageMessage(payload: InlineImagePayload): domain.Message | null {
  if (!payload?.chatId || !payload?.dataUri) return null;
  const caption = payload.caption?.trim() || 'Screenshot';
  const mime = mimeFromDataUri(payload.dataUri);
  return new domain.Message({
    id: payload.id || `inline-image-${Date.now()}-${Math.random().toString(36).slice(2)}`,
    sessionId: payload.chatId,
    role: 'system',
    content: caption,
    createdAt: new Date().toISOString(),
    attachments: [
      {
        name: `${caption}.${extensionFromMime(mime)}`,
        type: mime,
        dataUri: payload.dataUri,
      },
    ],
  });
}

// addInlineImageMessage persists the image keyed by chat and is idempotent on
// the payload id, so it can be called from both the always-mounted AppShell
// listener (which captures the event even when the user navigated away from the
// chat) and the active ChatModule without producing a duplicate. Returns the
// stored message, or null when it was already present / invalid.
export function addInlineImageMessage(payload: InlineImagePayload): domain.Message | null {
  const message = inlineImageMessage(payload);
  if (!message) return null;
  const stored = readStored();
  const existing = Array.isArray(stored[payload.chatId]) ? stored[payload.chatId] : [];
  if (existing.some((item) => item.id === message.id)) return null;
  stored[payload.chatId] = [...existing, message].slice(-MAX_INLINE_IMAGES_PER_CHAT);
  writeStored(stored);
  return message;
}

// clearInlineImageMessages drops a chat's stored images. /new and /clear
// reset the backend transcript this store shadows; without this the images
// would be merged right back into the freshly-emptied chat.
export function clearInlineImageMessages(chatId: string): void {
  if (!chatId) return;
  const stored = readStored();
  if (!(chatId in stored)) return;
  delete stored[chatId];
  writeStored(stored);
}

export function mergeInlineImageMessages(chatId: string, messages: domain.Message[]): domain.Message[] {
  const stored = readStored()[chatId];
  if (!Array.isArray(stored) || stored.length === 0) return messages;
  const ids = new Set(messages.map((message) => message.id));
  const extras = stored.filter((message) => !ids.has(message.id));
  if (extras.length === 0) return messages;
  return [...messages, ...extras].sort((a, b) =>
    String(a.createdAt || '').localeCompare(String(b.createdAt || '')),
  );
}
