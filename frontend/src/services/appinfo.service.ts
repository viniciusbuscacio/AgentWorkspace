import { GetAppVersion } from '@wails/go/main/App';

// Build/version info for Settings › About (and, later, the updater UI).
export const appInfoService = {
  // async so a missing Wails bridge (tests, web mode) surfaces as a rejected
  // promise the caller can catch instead of a synchronous TypeError.
  async getVersion(): Promise<string> {
    return GetAppVersion();
  },
};
