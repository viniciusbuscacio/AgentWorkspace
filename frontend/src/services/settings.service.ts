import {
  ChooseVaultDir,
  DeleteSecret,
  GenerateRecoveryKey,
  GetAppZoomPercent,
  GetAutoLockMinutes,
  GetMacosPermissions,
  GetPromptDebugMode,
  GetSecret,
  GetSubagentMode,
  HasSecret,
  ListSecrets,
  OpenSystemSettingsPane,
  ProbeMacosPermission,
  RecordActivity,
  SelectFolder,
  SetAppZoomPercent,
  SetAutoLockMinutes,
  SetPromptDebugMode,
  SetSecret,
  SetSubagentMode,
  VerifyRecoveryKey,
  ChangePassword,
} from '@wails/go/main/App';
import type { dto } from '@wails/go/models';

export type OperationResult = dto.OperationResult;
export type FolderDialogResult = dto.FolderDialogResult;
export type SecretResult = dto.SecretResult;

export const settingsService = {
  getAppZoomPercent(): Promise<number> {
    return GetAppZoomPercent();
  },
  setAppZoomPercent(percent: number): Promise<OperationResult> {
    return SetAppZoomPercent(percent);
  },
  recordActivity(): Promise<void> {
    return RecordActivity();
  },
  getAutoLockMinutes(): Promise<number> {
    return GetAutoLockMinutes();
  },
  setAutoLockMinutes(minutes: number): Promise<OperationResult> {
    return SetAutoLockMinutes(minutes);
  },
  getPromptDebugMode(): Promise<dto.PromptDebugModeResponse> {
    return GetPromptDebugMode();
  },
  setPromptDebugMode(enabled: boolean): Promise<OperationResult> {
    return SetPromptDebugMode(enabled);
  },
  getSubagentMode(): Promise<dto.SubagentModeResponse> {
    return GetSubagentMode();
  },
  setSubagentMode(mode: string): Promise<OperationResult> {
    return SetSubagentMode(mode);
  },
  chooseVaultDir(): Promise<dto.VaultStatusResponse> {
    return ChooseVaultDir();
  },
  selectFolder(): Promise<FolderDialogResult> {
    return SelectFolder();
  },
  generateRecoveryKey(): Promise<OperationResult> {
    return GenerateRecoveryKey();
  },
  verifyRecoveryKey(recoveryKey: string): Promise<OperationResult> {
    return VerifyRecoveryKey(recoveryKey);
  },
  changePassword(currentPassword: string, newPassword: string): Promise<OperationResult> {
    return ChangePassword(currentPassword, newPassword);
  },
  setSecret(name: string, value: string): Promise<OperationResult> {
    return SetSecret(name, value);
  },
  getSecret(name: string): Promise<SecretResult> {
    return GetSecret(name);
  },
  hasSecret(name: string): Promise<SecretResult> {
    return HasSecret(name);
  },
  listSecrets(): Promise<string[]> {
    return ListSecrets();
  },
  deleteSecret(name: string): Promise<OperationResult> {
    return DeleteSecret(name);
  },
  getMacosPermissions(): Promise<dto.MacosPermissionsResult> {
    return GetMacosPermissions();
  },
  openSystemSettingsPane(deepLink: string): Promise<OperationResult> {
    return OpenSystemSettingsPane(deepLink);
  },
  probeMacosPermission(id: string): Promise<dto.MacosProbeResult> {
    return ProbeMacosPermission(id);
  },
};
