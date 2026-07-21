import { clearCustomThemeProperties, isCustomThemeId } from '@/theme/custom-theme';

export const APP_THEME_STORAGE_KEY = 'aw.theme';
export const DEFAULT_APP_THEME = 'midnight';

export function getSavedAppTheme(): string {
  return localStorage.getItem(APP_THEME_STORAGE_KEY) || DEFAULT_APP_THEME;
}

/**
 * Apply a built-in theme: sets data-theme + persists to localStorage.
 * For custom themes ("custom:slug"), call applyCustomThemeTokens AFTER this
 * to set the CSS variables; this function only handles the id tracking.
 * When switching away from a custom theme to a built-in one, the inline
 * CSS-variable overrides are cleared.
 */
export function applyAppTheme(theme = getSavedAppTheme()): string {
  document.documentElement.dataset.theme = theme;
  localStorage.setItem(APP_THEME_STORAGE_KEY, theme);
  if (!isCustomThemeId(theme)) {
    clearCustomThemeProperties();
  }
  return theme;
}
