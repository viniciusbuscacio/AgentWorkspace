import { ReportUIState, ResolveUICommand } from '@wails/go/main/App';

/** UI-managed state mirrored to the backend for the aw app.state action. */
export interface UiStateReport {
  theme?: string;
  fontFamily?: string;
  fontSize?: number;
  view?: string;
  chatId?: string;
}

export const uiService = {
  /**
   * Best-effort: keeps the backend informed of theme/font/view changes. Never
   * throws so it stays safe in dev/browser/test environments without a Wails
   * backend.
   */
  async reportUiState(state: UiStateReport): Promise<void> {
    try {
      await ReportUIState(state as Record<string, unknown>);
    } catch {
      // No backend available (vitest/jsdom or plain-browser dev) — ignore.
    }
  },
  /** Delivers the result (or error) of a ui:command round-trip to the backend. */
  async resolveUiCommand(id: string, resultJSON: string, errMsg: string): Promise<void> {
    try {
      await ResolveUICommand(id, resultJSON, errMsg);
    } catch {
      // No backend available — ignore.
    }
  },
};
