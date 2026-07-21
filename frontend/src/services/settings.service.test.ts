import { describe, expect, it, vi } from 'vitest';
import { settingsService } from './settings.service';

const GetAppZoomPercent = vi.fn();
const SetAppZoomPercent = vi.fn();
const RecordActivity = vi.fn();

vi.mock('@wails/go/main/App', () => ({
  GetAppZoomPercent: (...args: unknown[]) => GetAppZoomPercent(...args),
  SetAppZoomPercent: (...args: unknown[]) => SetAppZoomPercent(...args),
  RecordActivity: (...args: unknown[]) => RecordActivity(...args),
  GetAutoLockMinutes: vi.fn(),
  SetAutoLockMinutes: vi.fn(),
  ChooseVaultDir: vi.fn(),
  SelectFolder: vi.fn(),
  GenerateRecoveryKey: vi.fn(),
  VerifyRecoveryKey: vi.fn(),
  ChangePassword: vi.fn(),
  SetSecret: vi.fn(),
  GetSecret: vi.fn(),
  HasSecret: vi.fn(),
  ListSecrets: vi.fn(),
  DeleteSecret: vi.fn(),
}));

describe('settingsService', () => {
  it('wraps zoom and activity bindings', async () => {
    GetAppZoomPercent.mockResolvedValue(110);
    SetAppZoomPercent.mockResolvedValue({ success: true });
    await expect(settingsService.getAppZoomPercent()).resolves.toBe(110);
    await settingsService.setAppZoomPercent(120);
    await settingsService.recordActivity();
    expect(SetAppZoomPercent).toHaveBeenCalledWith(120);
    expect(RecordActivity).toHaveBeenCalled();
  });
});
