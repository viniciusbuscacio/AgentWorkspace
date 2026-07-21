import { describe, expect, it, vi } from 'vitest';
import { permissionsService, SANDBOX_MODES } from './permissions.service';

const GetSandboxSettings = vi.fn();
const SaveSandboxSettings = vi.fn();
const SelectFolder = vi.fn();

vi.mock('@wails/go/main/App', () => ({
  GetSandboxSettings: (...args: unknown[]) => GetSandboxSettings(...args),
  SaveSandboxSettings: (...args: unknown[]) => SaveSandboxSettings(...args),
  SelectFolder: (...args: unknown[]) => SelectFolder(...args),
}));

describe('permissionsService', () => {
  it('wraps load and save bindings', async () => {
    GetSandboxSettings.mockResolvedValue({ success: true, settings: { mode: 'permit_list' } });
    SaveSandboxSettings.mockResolvedValue({ success: true });

    await expect(permissionsService.load()).resolves.toEqual({ success: true, settings: { mode: 'permit_list' } });
    await permissionsService.save('permit_list', ['~/Projects']);
    expect(SaveSandboxSettings).toHaveBeenCalledWith('permit_list', ['~/Projects']);
  });

  it('maps the folder dialog result', async () => {
    SelectFolder.mockResolvedValue({ canceled: false, path: '/Users/me/Projects' });
    await expect(permissionsService.selectFolder()).resolves.toBe('/Users/me/Projects');

    SelectFolder.mockResolvedValue({ canceled: true });
    await expect(permissionsService.selectFolder()).resolves.toBeNull();
  });

  it('exposes the three modes (deny_list was removed as confusing)', () => {
    expect([...SANDBOX_MODES]).toEqual(['block_all', 'permit_list', 'permit_all']);
  });
});
