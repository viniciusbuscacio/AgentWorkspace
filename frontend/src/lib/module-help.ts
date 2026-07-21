import { chatService } from '@services/chat.service';
import { userMemoryService } from '@services/user-memory.service';
import { notify } from '@/lib/notify';

// Module help ("?" button): opens a fresh chat in the PiP window with the
// help question already sent. The question is always English (predictable for
// the model); the answer language comes from the user's memory doc — the
// `preferred-language:` line Settings › Memory already carries — falling back
// to English when absent. The agent answers from the bundled workspace-guide
// skill docs.

export function preferredLanguageFrom(content: string): string {
  const match = /preferred[-_ ]?language\s*[:=]\s*(.+)/i.exec(content);
  const value = match?.[1]?.trim().replace(/[.;,]+$/, '');
  return value || 'English';
}

export async function openModuleHelp(moduleName: string): Promise<void> {
  let language = 'English';
  try {
    const result = await userMemoryService.getDoc();
    if (result.success && result.doc?.content) {
      language = preferredLanguageFrom(result.doc.content);
    }
  } catch {
    // Memory unavailable — English is the documented default.
  }
  try {
    const title = `Help — ${moduleName}`;
    // One help chat per module: the title is the marker. An active chat with
    // it is reopened as-is (no re-asking); archiving (or renaming) it makes
    // the next click start fresh.
    const chats = await chatService.listChats();
    const existing = chats.find((chat) => !chat.archived && chat.title === title);
    if (existing) {
      const reopened = await chatService.openPipWindow(existing.id);
      if (!reopened.success) notify(reopened.error || 'Could not open the help window.');
      return;
    }
    const chat = await chatService.createChatWithTitle(title);
    const question = `What does the ${moduleName} module do? Explain in the user's preferred language: ${language}`;
    // Fire the question before opening the window so the PiP attaches to a
    // chat that is already streaming; don't await — the promise resolves only
    // when the whole reply finishes.
    void chatService.streamChatMessage(chat.id, question, []);
    const opened = await chatService.openPipWindow(chat.id);
    if (!opened.success) notify(opened.error || 'Could not open the help window.');
  } catch (err) {
    notify(err instanceof Error ? err.message : 'Could not open module help.');
  }
}
