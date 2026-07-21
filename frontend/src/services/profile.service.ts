import {
  ClearRecentProfiles,
  CreateProfileAtLocation,
  ImportVaultProfile,
  ListProfiles,
  SelectProfile,
} from '@wails/go/main/App';
import type { domain, dto } from '@wails/go/models';

export type ProfileInfo = domain.ProfileInfo;
export type OperationResult = dto.OperationResult;

export const profileService = {
  listProfiles(): Promise<ProfileInfo[]> {
    return ListProfiles();
  },
  selectProfile(id: string): Promise<OperationResult> {
    return SelectProfile(id);
  },
  createProfileAtLocation(parentDir: string, folderName: string): Promise<OperationResult> {
    return CreateProfileAtLocation(parentDir, folderName);
  },
  importVaultProfile(vaultDir: string): Promise<OperationResult> {
    return ImportVaultProfile(vaultDir);
  },
  clearRecentProfiles(): Promise<OperationResult> {
    return ClearRecentProfiles();
  },
};
