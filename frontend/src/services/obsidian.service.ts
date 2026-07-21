import {
  ChooseObsidianAlwaysReadFile,
  ChooseObsidianVaultDir,
  GetObsidianSettings,
  SetObsidianSettings,
  TestObsidianAccess,
} from '@wails/go/main/App';
import type { dto } from '@wails/go/models';

export type ObsidianSettings = dto.ObsidianSettings;

// obsidianService wraps the Obsidian module bindings: the module setup
// (vault folder, Write toggle, always-read notes) and the native pickers.
export const obsidianService = {
  getSettings(): Promise<dto.ObsidianSettings> {
    return GetObsidianSettings();
  },
  setSettings(settings: dto.ObsidianSettings): Promise<dto.OperationResult> {
    return SetObsidianSettings(settings);
  },
  chooseVaultDir(): Promise<{ canceled: boolean; path?: string }> {
    return ChooseObsidianVaultDir();
  },
  chooseAlwaysReadFile(vaultDir: string): Promise<dto.ObsidianFileDialogResult> {
    return ChooseObsidianAlwaysReadFile(vaultDir);
  },
  testAccess(): Promise<dto.ObsidianTestResult> {
    return TestObsidianAccess();
  },
};
