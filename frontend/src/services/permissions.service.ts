import { GetSandboxSettings, SaveSandboxSettings, SelectFolder } from '@wails/go/main/App';
import type { dto } from '@wails/go/models';

export type SandboxSettings = dto.SandboxSettings;
export type SandboxSettingsResult = dto.SandboxSettingsResult;

export const SANDBOX_MODES = ['block_all', 'permit_list', 'permit_all'] as const;
export type SandboxMode = (typeof SANDBOX_MODES)[number];

export const permissionsService = {
  load(): Promise<SandboxSettingsResult> {
    return GetSandboxSettings();
  },
  // Save triggers a NATIVE confirmation dialog in the backend before
  // persisting; a canceled dialog comes back with canceled=true.
  save(mode: string, allowedFolders: string[]): Promise<SandboxSettingsResult> {
    return SaveSandboxSettings(mode, allowedFolders);
  },
  async selectFolder(): Promise<string | null> {
    const result = await SelectFolder();
    if (!result || result.canceled || !result.path) return null;
    return result.path;
  },
};
