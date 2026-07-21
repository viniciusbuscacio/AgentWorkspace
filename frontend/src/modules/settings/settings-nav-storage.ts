// Settings sub-navigation memory. When the user leaves Settings (e.g. opens a
// chat) the Settings module unmounts and loses its in-component state; coming
// back used to dump them on the grid. We persist the last sub-page for the
// session so returning lands exactly where they were.
//
// sessionStorage (not localStorage) on purpose: this is a within-session
// convenience, not a cross-restart preference. A fresh app launch starts clean.

const PAGE_KEY = 'settings-last-page';

export function readLastSettingsPage(): string | null {
  try {
    return sessionStorage.getItem(PAGE_KEY) || null;
  } catch {
    return null;
  }
}

export function writeLastSettingsPage(page: string | null): void {
  try {
    sessionStorage.setItem(PAGE_KEY, page ?? '');
  } catch {
    /* ignore */
  }
}

