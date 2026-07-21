import { describe, expect, it, vi } from 'vitest';
import { providerService } from './provider.service';

const GetProviderStatus = vi.fn();
const SaveProviderConfig = vi.fn();
const StartProviderBrowserAuth = vi.fn();

vi.mock('@wails/go/main/App', () => ({
  GetProviderStatus: (...args: unknown[]) => GetProviderStatus(...args),
  SaveProviderConfig: (...args: unknown[]) => SaveProviderConfig(...args),
  StartProviderBrowserAuth: (...args: unknown[]) => StartProviderBrowserAuth(...args),
  SwitchProvider: vi.fn(),
  DeleteProviderCredential: vi.fn(),
  CreateCustomProvider: vi.fn(),
  RenameCustomProvider: vi.fn(),
  DeleteCustomProvider: vi.fn(),
  DiscardCustomProvider: vi.fn(),
}));

describe('providerService', () => {
  it('uses generated provider bindings', async () => {
    GetProviderStatus.mockResolvedValue({ active: 'openrouter', providers: [] });
    SaveProviderConfig.mockResolvedValue({ success: true, saved: true });
    await expect(providerService.getProviderStatus()).resolves.toMatchObject({ active: 'openrouter' });
    await providerService.saveProviderConfig({ provider: 'openrouter', model: 'model', setActive: true } as never);
    expect(SaveProviderConfig).toHaveBeenCalledWith({ provider: 'openrouter', model: 'model', setActive: true });
  });
});
