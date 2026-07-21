import { describe, expect, it, vi } from 'vitest';
import { userMemoryService } from './user-memory.service';

const GetUserMemoryDoc = vi.fn();
const SetUserMemoryDoc = vi.fn();
const GetUserMemoryDocBackup = vi.fn();

vi.mock('@wails/go/main/App', () => ({
  GetUserMemoryDoc: (...args: unknown[]) => GetUserMemoryDoc(...args),
  SetUserMemoryDoc: (...args: unknown[]) => SetUserMemoryDoc(...args),
  GetUserMemoryDocBackup: (...args: unknown[]) => GetUserMemoryDocBackup(...args),
}));

describe('userMemoryService', () => {
  it('getDoc delegates to GetUserMemoryDoc', async () => {
    GetUserMemoryDoc.mockResolvedValue({
      success: true,
      doc: { content: '- preferred-language: Português', updatedAt: '2026-06-12' },
    });
    const result = await userMemoryService.getDoc();
    expect(result.success).toBe(true);
    expect(result.doc?.content).toContain('preferred-language');
  });

  it('setDoc passes content to SetUserMemoryDoc', async () => {
    SetUserMemoryDoc.mockResolvedValue({ success: true, doc: { content: 'new content' } });
    await userMemoryService.setDoc('new content');
    expect(SetUserMemoryDoc).toHaveBeenCalledWith('new content');
  });

  it('getBackup delegates to GetUserMemoryDocBackup', async () => {
    GetUserMemoryDocBackup.mockResolvedValue({ success: true, doc: { content: 'backup' } });
    const result = await userMemoryService.getBackup();
    expect(result.doc?.content).toBe('backup');
  });
});
