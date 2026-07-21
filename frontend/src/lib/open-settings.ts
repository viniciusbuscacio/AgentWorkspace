import type { SettingsPage } from '@modules/settings/SettingsModule';

// open-settings provides an in-app deep link to a Settings page from views that
// live outside the Settings module (e.g. the server cards in Apps linking to the
// TLS manager). AppShell listens for this window event and opens Settings at the
// requested page.
export const OPEN_SETTINGS_EVENT = 'aw:open-settings-page';

export interface OpenSettingsDetail {
  page: SettingsPage;
}

export function openSettingsPage(page: SettingsPage): void {
  window.dispatchEvent(new CustomEvent<OpenSettingsDetail>(OPEN_SETTINGS_EVENT, { detail: { page } }));
}
