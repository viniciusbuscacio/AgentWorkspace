import { GetLastView, GetLastChatID, SaveLastSession } from '@wails/go/main/App';

export const sessionService = {
  /** Returns the view id that was active when the app was last closed, or "home". */
  getLastView(): Promise<string> {
    try {
      return GetLastView().catch(() => 'home');
    } catch {
      return Promise.resolve('home');
    }
  },

  /** Returns the chat id that was selected when the app was last closed, or "". */
  getLastChatID(): Promise<string> {
    try {
      return GetLastChatID().catch(() => '');
    } catch {
      return Promise.resolve('');
    }
  },

  /** Persists the current view and selected chat id for the next session restore. */
  saveLastSession(view: string, chatID: string): Promise<void> {
    return SaveLastSession(view, chatID).then(() => undefined).catch(() => undefined);
  },
};
