import { GetUserMemoryDoc, GetUserMemoryDocBackup, SetUserMemoryDoc } from '@wails/go/main/App';
import type { domain, dto } from '@wails/go/models';

export type UserMemoryDoc = domain.UserMemoryDoc;
export type UserMemoryDocResult = dto.UserMemoryDocResult;

/** Prompt cap in runes: document content is truncated to this length before
 *  injecting into the agent context. Shown in the Settings UI as guidance. */
export const USER_MEMORY_PROMPT_CAP = 8000;

/** Condensation threshold: when the document exceeds this many characters the
 *  agent will condense it on the next unlock (at most once per day). */
export const USER_MEMORY_CONDENSE_THRESHOLD = 6000;

export const userMemoryService = {
  getDoc(): Promise<UserMemoryDocResult> {
    return GetUserMemoryDoc();
  },
  setDoc(content: string): Promise<UserMemoryDocResult> {
    return SetUserMemoryDoc(content);
  },
  getBackup(): Promise<UserMemoryDocResult> {
    return GetUserMemoryDocBackup();
  },
};
