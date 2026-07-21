export const APP_FONT_FAMILY_STORAGE_KEY = 'aw.font.family';
export const APP_FONT_SIZE_STORAGE_KEY = 'aw.font.size';

export const DEFAULT_APP_FONT_FAMILY = 'system';
export const DEFAULT_APP_FONT_SIZE = 14;
export const APP_FONT_SIZE_MIN = 12;
export const APP_FONT_SIZE_MAX = 22;
export const APP_FONT_SIZE_STEP = 1;

export interface AppFontOption {
  id: string;
  label: string;
  /** Section heading used to group the option in the picker. */
  group: string;
  css: string;
}

// Every non-"system" option maps to a font bundled in src/assets/fonts and
// declared in src/theme/fonts.css, so it renders identically on every OS.
// To add a font, see docs/FONTS.md.
export const APP_FONT_OPTIONS: AppFontOption[] = [
  {
    id: 'system',
    label: 'System Default',
    group: 'System',
    css: 'system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'inter',
    label: 'Inter',
    group: 'Sans-serif',
    css: '"Inter", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'roboto',
    label: 'Roboto',
    group: 'Sans-serif',
    css: '"Roboto", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'open-sans',
    label: 'Open Sans',
    group: 'Sans-serif',
    css: '"Open Sans", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'lato',
    label: 'Lato',
    group: 'Sans-serif',
    css: '"Lato", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'montserrat',
    label: 'Montserrat',
    group: 'Sans-serif',
    css: '"Montserrat", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'poppins',
    label: 'Poppins',
    group: 'Sans-serif',
    css: '"Poppins", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'nunito',
    label: 'Nunito',
    group: 'Sans-serif',
    css: '"Nunito", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'work-sans',
    label: 'Work Sans',
    group: 'Sans-serif',
    css: '"Work Sans", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'dm-sans',
    label: 'DM Sans',
    group: 'Sans-serif',
    css: '"DM Sans", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'manrope',
    label: 'Manrope',
    group: 'Sans-serif',
    css: '"Manrope", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'rubik',
    label: 'Rubik',
    group: 'Sans-serif',
    css: '"Rubik", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'arimo',
    label: 'Arimo (Arial-like)',
    group: 'Sans-serif',
    css: '"Arimo", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'libre-franklin',
    label: 'Libre Franklin',
    group: 'Sans-serif',
    css: '"Libre Franklin", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'jost',
    label: 'Jost (Futura-like)',
    group: 'Sans-serif',
    css: '"Jost", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'comic-neue',
    label: 'Comic Neue (Comic Sans-like)',
    group: 'Sans-serif',
    css: '"Comic Neue", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
  },
  {
    id: 'merriweather',
    label: 'Merriweather',
    group: 'Serif',
    css: '"Merriweather", ui-serif, Georgia, "Times New Roman", serif',
  },
  {
    id: 'lora',
    label: 'Lora',
    group: 'Serif',
    css: '"Lora", ui-serif, Georgia, "Times New Roman", serif',
  },
  {
    id: 'playfair-display',
    label: 'Playfair Display',
    group: 'Serif',
    css: '"Playfair Display", ui-serif, Georgia, "Times New Roman", serif',
  },
  {
    id: 'tinos',
    label: 'Tinos (Times-like)',
    group: 'Serif',
    css: '"Tinos", ui-serif, Georgia, "Times New Roman", serif',
  },
  {
    id: 'gelasio',
    label: 'Gelasio (Georgia-like)',
    group: 'Serif',
    css: '"Gelasio", ui-serif, Georgia, "Times New Roman", serif',
  },
  {
    id: 'libre-baskerville',
    label: 'Libre Baskerville',
    group: 'Serif',
    css: '"Libre Baskerville", ui-serif, Georgia, "Times New Roman", serif',
  },
  {
    id: 'jetbrains-mono',
    label: 'JetBrains Mono',
    group: 'Monospace',
    css: '"JetBrains Mono", ui-monospace, "SFMono-Regular", Consolas, Menlo, monospace',
  },
  {
    id: 'fira-code',
    label: 'Fira Code',
    group: 'Monospace',
    css: '"Fira Code", ui-monospace, "SFMono-Regular", Consolas, Menlo, monospace',
  },
  {
    id: 'ibm-plex-mono',
    label: 'IBM Plex Mono',
    group: 'Monospace',
    css: '"IBM Plex Mono", ui-monospace, "SFMono-Regular", Consolas, Menlo, monospace',
  },
  {
    id: 'cousine',
    label: 'Cousine (Courier-like)',
    group: 'Monospace',
    css: '"Cousine", ui-monospace, "SFMono-Regular", Consolas, Menlo, monospace',
  },
  {
    id: 'inconsolata',
    label: 'Inconsolata (Consolas-like)',
    group: 'Monospace',
    css: '"Inconsolata", ui-monospace, "SFMono-Regular", Consolas, Menlo, monospace',
  },
];

/** Options grouped by section, preserving declaration order, for the picker UI. */
export function getGroupedAppFontOptions(): { group: string; options: AppFontOption[] }[] {
  const groups: { group: string; options: AppFontOption[] }[] = [];
  for (const option of APP_FONT_OPTIONS) {
    let bucket = groups.find((entry) => entry.group === option.group);
    if (!bucket) {
      bucket = { group: option.group, options: [] };
      groups.push(bucket);
    }
    bucket.options.push(option);
  }
  return groups;
}

export function clampAppFontSize(size: number): number {
  if (!Number.isFinite(size)) return DEFAULT_APP_FONT_SIZE;
  return Math.min(APP_FONT_SIZE_MAX, Math.max(APP_FONT_SIZE_MIN, Math.round(size)));
}

export function getAppFontOption(id: string): AppFontOption {
  return APP_FONT_OPTIONS.find((option) => option.id === id) || APP_FONT_OPTIONS[0];
}

export function getSavedAppFontFamily(): string {
  return localStorage.getItem(APP_FONT_FAMILY_STORAGE_KEY) || DEFAULT_APP_FONT_FAMILY;
}

export function getSavedAppFontSize(): number {
  const raw = localStorage.getItem(APP_FONT_SIZE_STORAGE_KEY);
  if (raw === null || raw.trim() === '') return DEFAULT_APP_FONT_SIZE;
  const saved = Number(raw);
  return clampAppFontSize(Number.isNaN(saved) ? DEFAULT_APP_FONT_SIZE : saved);
}

export function applyAppFont(family = getSavedAppFontFamily(), size = getSavedAppFontSize()): { family: string; size: number } {
  const option = getAppFontOption(family);
  const nextSize = clampAppFontSize(size);
  const root = document.documentElement;

  root.style.setProperty('--font-family', option.css);
  root.style.setProperty('--font-size', `${nextSize}px`);
  localStorage.setItem(APP_FONT_FAMILY_STORAGE_KEY, option.id);
  localStorage.setItem(APP_FONT_SIZE_STORAGE_KEY, String(nextSize));

  return { family: option.id, size: nextSize };
}

export function resetAppFont(): { family: string; size: number } {
  localStorage.removeItem(APP_FONT_FAMILY_STORAGE_KEY);
  localStorage.removeItem(APP_FONT_SIZE_STORAGE_KEY);
  return applyAppFont(DEFAULT_APP_FONT_FAMILY, DEFAULT_APP_FONT_SIZE);
}
