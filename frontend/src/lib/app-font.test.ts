import { afterEach, describe, expect, it } from 'vitest';
import {
  APP_FONT_FAMILY_STORAGE_KEY,
  APP_FONT_SIZE_STORAGE_KEY,
  DEFAULT_APP_FONT_FAMILY,
  DEFAULT_APP_FONT_SIZE,
  applyAppFont,
  getSavedAppFontFamily,
  getSavedAppFontSize,
  resetAppFont,
} from './app-font';

describe('app font', () => {
  afterEach(() => {
    localStorage.clear();
    document.documentElement.style.removeProperty('--font-family');
    document.documentElement.style.removeProperty('--font-size');
  });

  it('applies saved font settings to documentElement', () => {
    localStorage.setItem(APP_FONT_FAMILY_STORAGE_KEY, 'jetbrains-mono');
    localStorage.setItem(APP_FONT_SIZE_STORAGE_KEY, '18');

    expect(getSavedAppFontFamily()).toBe('jetbrains-mono');
    expect(getSavedAppFontSize()).toBe(18);

    const applied = applyAppFont();
    expect(applied).toEqual({ family: 'jetbrains-mono', size: 18 });
    expect(document.documentElement.style.getPropertyValue('--font-family')).toContain('monospace');
    expect(document.documentElement.style.getPropertyValue('--font-size')).toBe('18px');
  });

  it('resets to default font settings', () => {
    applyAppFont('lora', 20);

    const applied = resetAppFont();
    expect(applied).toEqual({ family: DEFAULT_APP_FONT_FAMILY, size: DEFAULT_APP_FONT_SIZE });
    expect(localStorage.getItem(APP_FONT_FAMILY_STORAGE_KEY)).toBe(DEFAULT_APP_FONT_FAMILY);
    expect(localStorage.getItem(APP_FONT_SIZE_STORAGE_KEY)).toBe(String(DEFAULT_APP_FONT_SIZE));
  });
});
