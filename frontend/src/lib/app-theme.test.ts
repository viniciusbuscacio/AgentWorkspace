import { afterEach, describe, expect, it } from 'vitest';
import { applyAppTheme, APP_THEME_STORAGE_KEY, DEFAULT_APP_THEME, getSavedAppTheme } from './app-theme';

describe('app theme', () => {
  afterEach(() => {
    localStorage.clear();
    delete document.documentElement.dataset.theme;
  });

  it('applies the saved theme to documentElement before the app renders', () => {
    localStorage.setItem(APP_THEME_STORAGE_KEY, 'light');

    expect(getSavedAppTheme()).toBe('light');
    expect(applyAppTheme()).toBe('light');
    expect(document.documentElement.dataset.theme).toBe('light');
  });

  it('defaults to midnight when no theme is saved', () => {
    expect(getSavedAppTheme()).toBe(DEFAULT_APP_THEME);
    expect(applyAppTheme()).toBe(DEFAULT_APP_THEME);
    expect(document.documentElement.dataset.theme).toBe(DEFAULT_APP_THEME);
  });
});
