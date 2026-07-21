import { afterEach, describe, expect, it } from 'vitest';
import {
  applyCustomThemeTokens,
  clearCustomThemeProperties,
  customThemeIdForName,
  DEFAULT_CUSTOM_THEME,
  isCustomThemeId,
  luminance,
  normalizeCustomThemeTokens,
  readableTextFor,
  type CustomThemeTokens,
} from './custom-theme';

const VALID_TOKENS: CustomThemeTokens = {
  name: 'Test Theme',
  background: '#111111',
  surface: '#222222',
  surfaceAlt: '#333333',
  text: '#eeeeee',
  mutedText: '#999999',
  border: '#444444',
  accent: '#5566ff',
};

afterEach(() => {
  clearCustomThemeProperties();
  delete document.documentElement.dataset.theme;
});

// ── apply sets CSS variables ─────────────────────────────────────────────────
describe('applyCustomThemeTokens', () => {
  it('sets CSS variables for required tokens', () => {
    applyCustomThemeTokens(VALID_TOKENS, 'custom:test');
    const root = document.documentElement;
    expect(root.style.getPropertyValue('--bg-primary')).toBe('#111111');
    expect(root.style.getPropertyValue('--bg-secondary')).toBe('#222222');
    expect(root.style.getPropertyValue('--bg-tertiary')).toBe('#333333');
    expect(root.style.getPropertyValue('--text-primary')).toBe('#eeeeee');
    expect(root.style.getPropertyValue('--text-secondary')).toBe('#999999');
    expect(root.style.getPropertyValue('--border')).toBe('#444444');
    expect(root.style.getPropertyValue('--accent')).toBe('#5566ff');
  });

  it('sets data-theme to the custom id', () => {
    applyCustomThemeTokens(VALID_TOKENS, 'custom:neon');
    expect(document.documentElement.dataset.theme).toBe('custom:neon');
  });

  it('uses background fallbacks for unset optional tokens', () => {
    applyCustomThemeTokens(VALID_TOKENS, 'custom:t');
    const root = document.documentElement;
    // chatSurface defaults to background
    expect(root.style.getPropertyValue('--chat-surface')).toBe(VALID_TOKENS.background);
    // assistantBubble defaults to surface
    expect(root.style.getPropertyValue('--chat-assistant-bubble')).toBe(VALID_TOKENS.surface);
  });

  it('uses optional sidebar token when supplied', () => {
    applyCustomThemeTokens({ ...VALID_TOKENS, sidebar: '#aabbcc' }, 'custom:t');
    expect(document.documentElement.style.getPropertyValue('--sidebar-bg')).toBe('#aabbcc');
  });
});

// ── clearCustomThemeProperties ───────────────────────────────────────────────
describe('clearCustomThemeProperties', () => {
  it('removes inline CSS overrides after apply', () => {
    applyCustomThemeTokens(VALID_TOKENS, 'custom:test');
    clearCustomThemeProperties();
    expect(document.documentElement.style.getPropertyValue('--bg-primary')).toBe('');
    expect(document.documentElement.style.getPropertyValue('--accent')).toBe('');
  });
});

// ── required-token validation ─────────────────────────────────────────────────
describe('normalizeCustomThemeTokens', () => {
  it('fills in all defaults when called with empty input', () => {
    const t = normalizeCustomThemeTokens({});
    expect(t.background).toBe(DEFAULT_CUSTOM_THEME.background);
    expect(t.surface).toBe(DEFAULT_CUSTOM_THEME.surface);
    expect(t.accent).toBe(DEFAULT_CUSTOM_THEME.accent);
    expect(t.name).toBe(DEFAULT_CUSTOM_THEME.name);
  });

  it('rejects invalid hex colors and uses the default instead', () => {
    const t = normalizeCustomThemeTokens({ background: 'red', accent: '#xyz' });
    expect(t.background).toBe(DEFAULT_CUSTOM_THEME.background);
    expect(t.accent).toBe(DEFAULT_CUSTOM_THEME.accent);
  });

  it('preserves valid required tokens', () => {
    const t = normalizeCustomThemeTokens(VALID_TOKENS);
    expect(t.background).toBe('#111111');
    expect(t.accent).toBe('#5566ff');
    expect(t.name).toBe('Test Theme');
  });

  it('includes optional tokens when valid hex', () => {
    const t = normalizeCustomThemeTokens({ ...VALID_TOKENS, sidebar: '#aabbcc' });
    expect(t.sidebar).toBe('#aabbcc');
  });

  it('drops optional tokens that are invalid hex', () => {
    const t = normalizeCustomThemeTokens({ ...VALID_TOKENS, sidebar: 'blue' });
    expect(t.sidebar).toBeUndefined();
  });
});

// ── duplicate copies tokens ───────────────────────────────────────────────────
describe('duplicating a theme', () => {
  it('normalizeCustomThemeTokens returns an independent copy', () => {
    const original = normalizeCustomThemeTokens(VALID_TOKENS);
    const copy = normalizeCustomThemeTokens({ ...original, name: 'Copy' });
    copy.background = '#ffffff';
    // Mutating copy must not change original
    expect(original.background).toBe('#111111');
    expect(copy.name).toBe('Copy');
  });
});

// ── slug helper ───────────────────────────────────────────────────────────────
describe('customThemeIdForName', () => {
  it('slugifies a name with spaces', () => {
    expect(customThemeIdForName('Neon Dark')).toBe('custom:neon-dark');
  });

  it('strips diacritics', () => {
    expect(customThemeIdForName('Néon')).toBe('custom:neon');
  });

  it('falls back to the default name slug for empty input', () => {
    // Empty name → DEFAULT_CUSTOM_THEME.name "Custom Theme" → slug "custom-theme".
    expect(customThemeIdForName('')).toBe('custom:custom-theme');
  });
});

// ── isCustomThemeId ───────────────────────────────────────────────────────────
describe('isCustomThemeId', () => {
  it('accepts custom: prefix', () => { expect(isCustomThemeId('custom:foo')).toBe(true); });
  it('rejects built-in names', () => { expect(isCustomThemeId('midnight')).toBe(false); });
  it('rejects non-string', () => { expect(isCustomThemeId(42)).toBe(false); });
});

// ── luminance / readableTextFor ───────────────────────────────────────────────
describe('readableTextFor', () => {
  it('returns dark text for light backgrounds', () => {
    expect(readableTextFor('#ffffff')).toBe('#111111');
  });
  it('returns light text for dark backgrounds', () => {
    expect(readableTextFor('#000000')).toBe('#ffffff');
  });
});

describe('luminance', () => {
  it('returns 0 for black', () => { expect(luminance('#000000')).toBeCloseTo(0); });
  it('returns ~1 for white', () => { expect(luminance('#ffffff')).toBeCloseTo(1, 1); });
});
