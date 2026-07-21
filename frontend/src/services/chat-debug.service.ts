import { ListChatTurns } from '@wails/go/main/App';
import type { domain } from '@wails/go/models';

export type LLMTurn = domain.LLMTurn;

export const chatDebugService = {
  listChatTurns(chatId: string): Promise<LLMTurn[]> {
    return ListChatTurns(chatId);
  },
};
