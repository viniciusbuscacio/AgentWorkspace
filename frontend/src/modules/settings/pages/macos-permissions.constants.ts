// macos-permissions.constants.ts — UI copy for the macOS Permissions card in
// Settings › Security. All strings live here; the component renders from the
// Go-side permission list (via settingsService.getMacosPermissions()) and this
// copy file — they cannot drift (Decision 5).

// Card title and description.
export const MACOS_PERM_CARD_TITLE = 'macOS permissions';
export const MACOS_PERM_CARD_DESC =
  'macOS requires explicit permission for several aw features. Click "Open System Settings" to enable each one.';

// Column headers (for screen-reader accessibility, not shown as table headers).
export const MACOS_PERM_COL_NAME = 'Permission';
export const MACOS_PERM_COL_WHAT = 'What breaks without it';
export const MACOS_PERM_COL_STATUS = 'Status';

// Status badge copy — shown before the user runs a probe.
export const MACOS_PERM_STATUS_UNKNOWN = 'unknown — test it';
export const MACOS_PERM_STATUS_GRANTED = 'granted';
export const MACOS_PERM_STATUS_DENIED = 'denied';
// Shown after a probe that could not decide either way.
export const MACOS_PERM_STATUS_INCONCLUSIVE = "couldn't determine";

// No-probe row copy — replaces the Test button.
export const MACOS_PERM_NO_PROBE = 'enable if app updates fail';

// Button labels.
export const MACOS_PERM_BTN_OPEN = 'Open System Settings';
export const MACOS_PERM_BTN_TEST = 'Test';
export const MACOS_PERM_BTN_TESTING = 'Testing…';
