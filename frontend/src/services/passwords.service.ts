import { DeletePassword, ListPasswords, SavePassword } from '@wails/go/main/App';
import type { dto } from '@wails/go/models';

export interface PasswordEntry {
  id: string;
  name: string;
  username: string;
  password: string;
  url: string;
  notes: string;
  updatedAt: number;
}

export type PasswordsResult = dto.PasswordsResult & { passwords?: PasswordEntry[] };
export type PasswordResult = dto.PasswordResult & { password?: PasswordEntry };

export const passwordsService = {
  list(): Promise<PasswordsResult> {
    return ListPasswords() as Promise<PasswordsResult>;
  },
  save(entry: Partial<PasswordEntry>): Promise<PasswordResult> {
    return SavePassword(entry as never) as Promise<PasswordResult>;
  },
  delete(id: string): Promise<dto.OperationResult> {
    return DeletePassword(id);
  },
};
