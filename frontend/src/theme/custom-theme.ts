// Custom theme system for aw. Ported from AW2 src/renderer/theme/custom-theme.ts.
// Persistence: tokens and active theme id live in config.json via app_theme.go
// Wails bindings (not localStorage). This module only handles type definitions,
// normalization, the apply/clear CSS-var path, and slug utilities.

export interface CustomThemeTokens {
  name: string;
  // 7 required
  background: string;
  surface: string;
  surfaceAlt: string;
  text: string;
  mutedText: string;
  border: string;
  accent: string;
  // 10 optional
  sidebar?: string;
  sidebarHover?: string;
  sidebarActive?: string;
  sidebarActiveText?: string;
  chatSurface?: string;
  chatComposer?: string;
  userBubble?: string;
  userBubbleText?: string;
  assistantBubble?: string;
  codeSurface?: string;
}

export const CUSTOM_THEME_PREFIX = 'custom:';

/** CSS-variable names managed by applyCustomThemeTokens / clearCustomThemeProperties. */
const CUSTOM_THEME_CSS_VARS = [
  '--bg-primary', '--bg-secondary', '--bg-tertiary', '--bg-elevated',
  '--bg-input', '--bg-card', '--bg-muted', '--bg-hover',
  '--text-primary', '--text-secondary', '--text-muted', '--text-faint',
  '--text-foreground', '--text-muted-foreground',
  '--border', '--border-subtle', '--border-focus',
  '--accent', '--accent-hover', '--accent-foreground', '--ring',
  '--sidebar-bg', '--sidebar-hover', '--sidebar-active', '--sidebar-active-text',
  '--chat-surface', '--chat-composer',
  '--chat-user-bubble', '--chat-user-bubble-text',
  '--chat-assistant-bubble', '--chat-code-surface',
  '--scrollbar-thumb', '--scrollbar-track',
] as const;

export const DEFAULT_CUSTOM_THEME: CustomThemeTokens = {
  name: 'Custom Theme',
  background: '#171717',
  surface: '#1f1f1f',
  surfaceAlt: '#2d2d2d',
  text: '#ededed',
  mutedText: '#a3a3a3',
  border: '#333333',
  accent: '#6f96ff',
};

// ---------- utilities -------------------------------------------------------

export function isCustomThemeId(value: unknown): value is string {
  return typeof value === 'string' && value.startsWith(CUSTOM_THEME_PREFIX);
}

/** Slug a display name into a "custom:slug" id. */
export function customThemeIdForName(name: string): string {
  const slug = String(name || DEFAULT_CUSTOM_THEME.name)
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 40) || 'theme';
  return `${CUSTOM_THEME_PREFIX}${slug}`;
}

function isHexColor(v: unknown): v is string {
  return typeof v === 'string' && /^#[0-9a-fA-F]{6}$/.test(v);
}

/** Compute relative luminance (WCAG 2.1). */
export function luminance(hex: string): number {
  const rgb = [1, 3, 5]
    .map((s) => parseInt(hex.slice(s, s + 2), 16) / 255)
    .map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return 0.2126 * rgb[0] + 0.7152 * rgb[1] + 0.0722 * rgb[2];
}

/**
 * Return a high-contrast text colour (#111111 or #ffffff) for a given
 * background hex. Used by the editor to show live contrast hints.
 */
export function readableTextFor(background: string): string {
  return luminance(background) > 0.45 ? '#111111' : '#ffffff';
}

// ---------- normalization ---------------------------------------------------

/**
 * Fill missing or invalid required tokens with defaults from
 * DEFAULT_CUSTOM_THEME. Optional tokens are included only when valid hex.
 */
export function normalizeCustomThemeTokens(
  input: Partial<CustomThemeTokens> = {},
): CustomThemeTokens {
  const tokens: CustomThemeTokens = {
    name: typeof input.name === 'string' && input.name.trim() ? input.name : DEFAULT_CUSTOM_THEME.name,
    background: isHexColor(input.background) ? input.background : DEFAULT_CUSTOM_THEME.background,
    surface: isHexColor(input.surface) ? input.surface : DEFAULT_CUSTOM_THEME.surface,
    surfaceAlt: isHexColor(input.surfaceAlt) ? input.surfaceAlt : DEFAULT_CUSTOM_THEME.surfaceAlt,
    text: isHexColor(input.text) ? input.text : DEFAULT_CUSTOM_THEME.text,
    mutedText: isHexColor(input.mutedText) ? input.mutedText : DEFAULT_CUSTOM_THEME.mutedText,
    border: isHexColor(input.border) ? input.border : DEFAULT_CUSTOM_THEME.border,
    accent: isHexColor(input.accent) ? input.accent : DEFAULT_CUSTOM_THEME.accent,
  };
  for (const key of [
    'sidebar', 'sidebarHover', 'sidebarActive', 'sidebarActiveText',
    'chatSurface', 'chatComposer', 'userBubble', 'userBubbleText',
    'assistantBubble', 'codeSurface',
  ] as const) {
    if (isHexColor(input[key])) tokens[key] = input[key];
  }
  return tokens;
}

// ---------- apply / clear ---------------------------------------------------

/**
 * Remove all custom-theme inline style overrides from the document root.
 * Call before switching back to a built-in theme.
 */
export function clearCustomThemeProperties(): void {
  const root = document.documentElement;
  for (const v of CUSTOM_THEME_CSS_VARS) root.style.removeProperty(v);
  root.style.removeProperty('color');
  root.style.removeProperty('background-color');
  document.body.style.removeProperty('color');
  document.body.style.removeProperty('background-color');
}

/**
 * THE single translation table: maps CustomThemeTokens → aw CSS variables.
 * Never add per-component overrides; change this function instead (Decision 6).
 */
export function applyCustomThemeTokens(
  tokens = DEFAULT_CUSTOM_THEME,
  themeId = 'custom',
): void {
  const t = normalizeCustomThemeTokens(tokens);
  const root = document.documentElement;

  const chatSurface = t.chatSurface ?? t.background;
  const chatComposer = t.chatComposer ?? t.surfaceAlt;
  const assistantBubble = t.assistantBubble ?? t.surface;
  const codeSurface = t.codeSurface ?? t.surfaceAlt;
  const sidebarActive = t.sidebarActive ?? t.userBubble ?? t.accent;
  const sidebarActiveText = t.sidebarActiveText ?? readableTextFor(sidebarActive);

  root.setAttribute('data-theme', isCustomThemeId(themeId) ? themeId : CUSTOM_THEME_PREFIX + 'default');
  root.style.color = t.text;
  root.style.backgroundColor = t.background;
  document.body.style.color = t.text;
  document.body.style.backgroundColor = t.background;

  root.style.setProperty('--bg-primary', t.background);
  root.style.setProperty('--bg-secondary', t.surface);
  root.style.setProperty('--bg-tertiary', t.surfaceAlt);
  root.style.setProperty('--bg-elevated', t.surface);
  root.style.setProperty('--bg-input', t.surfaceAlt);
  root.style.setProperty('--bg-card', t.surface);
  root.style.setProperty('--bg-muted', t.surfaceAlt);
  root.style.setProperty('--bg-hover', t.surfaceAlt);
  root.style.setProperty('--text-primary', t.text);
  root.style.setProperty('--text-secondary', t.mutedText);
  root.style.setProperty('--text-muted', t.mutedText);
  root.style.setProperty('--text-faint', t.mutedText);
  root.style.setProperty('--text-foreground', t.text);
  root.style.setProperty('--text-muted-foreground', t.mutedText);
  root.style.setProperty('--border', t.border);
  root.style.setProperty('--border-subtle', t.border);
  root.style.setProperty('--border-focus', t.accent);
  root.style.setProperty('--accent', t.accent);
  root.style.setProperty('--accent-hover', t.accent);
  root.style.setProperty('--accent-foreground', '#ffffff');
  root.style.setProperty('--ring', t.accent);
  root.style.setProperty('--sidebar-bg', t.sidebar ?? t.surface);
  root.style.setProperty('--sidebar-hover', t.sidebarHover ?? t.surfaceAlt);
  root.style.setProperty('--sidebar-active', sidebarActive);
  root.style.setProperty('--sidebar-active-text', sidebarActiveText);
  root.style.setProperty('--chat-surface', chatSurface);
  root.style.setProperty('--chat-composer', chatComposer);
  root.style.setProperty('--chat-user-bubble', t.userBubble ?? t.accent);
  root.style.setProperty('--chat-user-bubble-text', t.userBubbleText ?? '#ffffff');
  root.style.setProperty('--chat-assistant-bubble', assistantBubble);
  root.style.setProperty('--chat-code-surface', codeSurface);
  root.style.setProperty('--scrollbar-thumb', t.border);
  root.style.setProperty('--scrollbar-track', 'transparent');
}
