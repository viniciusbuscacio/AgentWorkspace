import {
  VaultStatus,
  UnlockVault,
  LockVault,
  CreateVault,
  ChangePassword,
  RecoverVault,
  GenerateRecoveryKey,
  VerifyRecoveryKey,
  VaultTouchIDAvailable,
  VaultTouchIDHasPassword,
  VaultTouchIDEnroll,
  VaultTouchIDUnlock,
  VaultTouchIDRemove,
  VaultTouchIDRepair,
  CodesignTrustStatus,
  CodesignTrustGrant,
} from '@wails/go/main/App';
import type { dto } from '@wails/go/models';

// Vault service: typed wrappers over the Wails bindings. Types are inherited
// straight from the Go backend (dto.*), so a backend field rename breaks the
// build here instead of failing silently at runtime.

export type VaultStatusInfo = dto.VaultStatusResponse;
export type OperationResult = dto.OperationResult;

export const vaultService = {
  status(): Promise<VaultStatusInfo> {
    return VaultStatus();
  },
  unlock(password: string): Promise<OperationResult> {
    return UnlockVault(password);
  },
  lock(): Promise<OperationResult> {
    return LockVault();
  },
  create(password: string): Promise<OperationResult> {
    return CreateVault(password);
  },
  changePassword(current: string, next: string): Promise<OperationResult> {
    return ChangePassword(current, next);
  },
  recover(recoveryKey: string, newPassword: string): Promise<OperationResult> {
    return RecoverVault(recoveryKey, newPassword);
  },
  generateRecoveryKey(): Promise<OperationResult> {
    return GenerateRecoveryKey();
  },
  verifyRecoveryKey(recoveryKey: string): Promise<OperationResult> {
    return VerifyRecoveryKey(recoveryKey);
  },
  touchIDAvailable(): Promise<boolean> {
    return VaultTouchIDAvailable();
  },
  touchIDHasPassword(): Promise<boolean> {
    return VaultTouchIDHasPassword();
  },
  touchIDEnroll(password: string): Promise<OperationResult> {
    return VaultTouchIDEnroll(password);
  },
  touchIDUnlock(): Promise<OperationResult> {
    return VaultTouchIDUnlock();
  },
  touchIDRemove(): Promise<OperationResult> {
    return VaultTouchIDRemove();
  },
  touchIDRepair(): Promise<OperationResult> {
    return VaultTouchIDRepair();
  },
  codesignTrustStatus(): Promise<dto.CodesignTrustResult> {
    return CodesignTrustStatus();
  },
  codesignTrustGrant(): Promise<OperationResult> {
    return CodesignTrustGrant();
  },
};
