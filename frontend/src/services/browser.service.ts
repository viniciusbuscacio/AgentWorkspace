import {
  BrowserScreenshot,
  BrowserStart,
  BrowserStatus,
  BrowserStop,
  BrowserTabs,
  GetBrowserExecutable,
  PickBrowserExecutable,
  SetBrowserAutostart,
  SetBrowserExecutable,
} from '@wails/go/main/App';
import type { domain, dto } from '@wails/go/models';

export type BrowserStatusInfo = domain.BrowserStatus;
export type BrowserTab = domain.BrowserTab;
export type BrowserStatusResult = dto.BrowserStatusResult;
export type BrowserExecutableResult = dto.BrowserExecutableResult;
export type BrowserTabsResult = dto.BrowserTabsResult;
export type BrowserScreenshotResult = dto.BrowserScreenshotResult;

export const browserService = {
  // start launches the agent's isolated browser (or attaches to one already
  // running). The user's own browser is never touched.
  start(id: string): Promise<BrowserStatusResult> {
    return BrowserStart(id);
  },
  stop(id: string): Promise<BrowserStatusResult> {
    return BrowserStop(id);
  },
  status(id: string): Promise<BrowserStatusResult> {
    return BrowserStatus(id);
  },
  executable(id: string): Promise<BrowserExecutableResult> {
    return GetBrowserExecutable(id);
  },
  pickExecutable(id: string): Promise<BrowserExecutableResult> {
    return PickBrowserExecutable(id);
  },
  setExecutable(id: string, path: string): Promise<BrowserExecutableResult> {
    return SetBrowserExecutable(id, path);
  },
  // setAutostart records whether this Agent Browser launches automatically as
  // soon as the vault unlocks. It does not start or stop a running browser.
  setAutostart(id: string, enabled: boolean): Promise<BrowserStatusResult> {
    return SetBrowserAutostart(id, enabled);
  },
  tabs(id: string): Promise<BrowserTabsResult> {
    return BrowserTabs(id);
  },
  screenshot(id: string): Promise<BrowserScreenshotResult> {
    return BrowserScreenshot(id);
  },
};
