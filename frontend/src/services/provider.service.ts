import {
  CreateCustomProvider,
  DeleteCustomProvider,
  DeleteCustomProviderConfirmed,
  DeleteProviderCredential,
  DeleteProviderCredentialConfirmed,
  DiscardCustomProvider,
  GetProviderBalance,
  GetProviderFallbackOrder,
  GetProviderStatus,
  GetBenchedProviders,
  ListProviderModels,
  RenameCustomProvider,
  SaveProviderConfig,
  SetProviderEnabled,
  SetProviderFallbackOrder,
  StartProviderBrowserAuth,
  SwitchProvider,
  TestProviderConnection,
} from '@wails/go/main/App';
import type { domain, dto } from '@wails/go/models';

export type ProviderStatus = domain.ProviderStatus;
export type ProviderInfo = domain.ProviderInfo;
export type ProviderSaveConfigInput = domain.ProviderSaveConfigInput;
export type ProviderOperationResult = domain.ProviderOperationResult;
export type ProviderTestResult = domain.ProviderTestResult;
export type ProviderBalanceResult = domain.ProviderBalanceResult;
export type ProviderModelsResult = domain.ProviderModelsResult;

export const providerService = {
  getProviderStatus(): Promise<ProviderStatus> {
    return GetProviderStatus();
  },
  saveProviderConfig(input: ProviderSaveConfigInput): Promise<ProviderOperationResult> {
    return SaveProviderConfig(input);
  },
  startProviderBrowserAuth(provider: string, model: string): Promise<ProviderOperationResult> {
    return StartProviderBrowserAuth(provider, model);
  },
  switchProvider(provider: string, model: string): Promise<ProviderOperationResult> {
    return SwitchProvider(provider, model);
  },
  setProviderEnabled(provider: string, enabled: boolean): Promise<ProviderOperationResult> {
    return SetProviderEnabled(provider, enabled);
  },
  deleteProviderCredential(provider: string): Promise<ProviderOperationResult> {
    return DeleteProviderCredential(provider);
  },
  // No-dialog variant for web mode: the remote browser confirms with a React
  // modal first, since DeleteProviderCredential opens a native dialog the web
  // bridge denies. Desktop keeps the native dialog (UI automation can't
  // self-approve it).
  deleteProviderCredentialConfirmed(provider: string): Promise<ProviderOperationResult> {
    return DeleteProviderCredentialConfirmed(provider);
  },
  createCustomProvider(name: string): Promise<ProviderOperationResult> {
    return CreateCustomProvider(name);
  },
  renameCustomProvider(id: string, name: string): Promise<ProviderOperationResult> {
    return RenameCustomProvider(id, name);
  },
  deleteCustomProvider(id: string): Promise<ProviderOperationResult> {
    return DeleteCustomProvider(id);
  },
  // No-dialog variant for web mode (see deleteProviderCredentialConfirmed).
  deleteCustomProviderConfirmed(id: string): Promise<ProviderOperationResult> {
    return DeleteCustomProviderConfirmed(id);
  },
  discardCustomProvider(id: string): Promise<ProviderOperationResult> {
    return DiscardCustomProvider(id);
  },
  testProviderConnection(provider: string): Promise<ProviderTestResult> {
    return TestProviderConnection(provider);
  },
  getProviderBalance(provider: string): Promise<ProviderBalanceResult> {
    return GetProviderBalance(provider);
  },
  // Live model list from the provider's own models endpoint (falls back to the
  // static catalog server-side when unconfigured or the fetch fails).
  getBenchedProviders(): Promise<string[]> {
    return GetBenchedProviders();
  },
  listProviderModels(provider: string): Promise<ProviderModelsResult> {
    return ListProviderModels(provider);
  },
  // The persisted failover priority (config.json providerFallbackOrder) — the
  // order the backend actually tries providers in after the active one.
  getProviderFallbackOrder(): Promise<string[]> {
    return GetProviderFallbackOrder();
  },
  setProviderFallbackOrder(ids: string[]): Promise<dto.OperationResult> {
    return SetProviderFallbackOrder(ids);
  },
};
