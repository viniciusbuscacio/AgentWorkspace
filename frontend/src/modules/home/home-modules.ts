// Apps/Home — card metrics + ordering, ported from the AW2 HomeModule (and
// the faithful vanilla main.js). The module catalog itself comes from the
// backend registry (modulesService.list) — never hardcoded here.

export interface AppModule {
  type: string;
  name: string;
  icon: string;
  description: string;
  added?: boolean;
  core?: boolean;
}

export const ICON_SIZE_MIN = 80;
export const ICON_SIZE_MAX = 300;
export const ICON_SIZE_DEFAULT = 220;
export const ICON_SIZE_STEP = 10;

const ICON_SIZE_KEY = 'home-icon-size';
const ORDER_MODE_KEY = 'home-order-mode';
const USAGE_KEY = 'home-module-usage';

export type OrderMode = 'default' | 'most-used' | 'manual';
export const ORDER_LABELS: Record<OrderMode, string> = {
  default: 'Default',
  'most-used': 'Most used',
  manual: 'Manual',
};

function clamp(min: number, value: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

export function getInitialIconSize(): number {
  const saved = Number(localStorage.getItem(ICON_SIZE_KEY));
  if (!Number.isNaN(saved) && saved >= ICON_SIZE_MIN && saved <= ICON_SIZE_MAX) return saved;
  return ICON_SIZE_DEFAULT;
}

export function persistIconSize(size: number): void {
  try { localStorage.setItem(ICON_SIZE_KEY, String(size)); } catch { /* ignore */ }
}

export function getInitialOrderMode(): OrderMode {
  const saved = localStorage.getItem(ORDER_MODE_KEY);
  return saved === 'most-used' || saved === 'manual' ? saved : 'default';
}

export function persistOrderMode(mode: OrderMode): void {
  try { localStorage.setItem(ORDER_MODE_KEY, mode); } catch { /* ignore */ }
}

function getUsageCounts(): Record<string, number> {
  try { return JSON.parse(localStorage.getItem(USAGE_KEY) || '{}') as Record<string, number>; } catch { return {}; }
}

export function recordModuleUsage(type: string): void {
  const counts = getUsageCounts();
  counts[type] = (counts[type] || 0) + 1;
  try { localStorage.setItem(USAGE_KEY, JSON.stringify(counts)); } catch { /* ignore */ }
}

export interface CardMetrics {
  mini: boolean;
  compact: boolean;
  gridMin: number;
  iconPx: number;
  titleRem: number;
  descRem: number;
  padRem: number;
  gapRem: number;
}

// Identical formula to the AW2 HomeModuleCard / vanilla cardMetrics.
export function cardMetrics(size: number): CardMetrics {
  const ratio = size / ICON_SIZE_DEFAULT;
  const mini = size < 130;
  const compact = size >= 130 && size < 180;
  return {
    mini,
    compact,
    gridMin: size,
    iconPx: mini ? 24 : Math.round(clamp(18, 22 * ratio, 40)),
    titleRem: compact ? 0.85 : clamp(0.9, 1 * ratio, 1.4),
    descRem: clamp(0.78, 0.82 * ratio, 1.1),
    padRem: compact ? 0.6 : clamp(0.6, 1.25 * ratio, 1.25),
    gapRem: compact ? 0.35 : clamp(0.25, 0.75 * ratio, 0.75),
  };
}

export function cardStateClass(m: CardMetrics): string {
  return m.mini ? 'is-mini' : m.compact ? 'is-compact' : '';
}

export function cardStyleVars(m: CardMetrics): React.CSSProperties {
  return {
    '--card-pad': `${m.padRem}rem`,
    '--card-gap': `${m.gapRem}rem`,
    '--card-icon-px': `${m.iconPx}px`,
    '--card-title-rem': `${m.titleRem}rem`,
    '--card-desc-rem': `${m.descRem}rem`,
  } as React.CSSProperties;
}

// orderModules keeps the input (catalog) order as the default; "most-used"
// reorders by local usage counts.
export function orderModules(modules: AppModule[], mode: OrderMode): AppModule[] {
  if (mode !== 'most-used') return [...modules];
  const counts = getUsageCounts();
  return modules
    .map((option, index) => ({ option, index }))
    .sort((a, b) => (counts[b.option.type] || 0) - (counts[a.option.type] || 0) || a.index - b.index)
    .map((item) => item.option);
}
