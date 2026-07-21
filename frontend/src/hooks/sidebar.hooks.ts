import { useCallback, useEffect, useRef, useState } from 'react';

// Sidebar geometry ported from the vanilla AW2 frontend
// (codex/aw-chat-pip-fixes:frontend/src/main.js). Left position only for now;
// top/bottom/right positions are a later phase.
const SB_MIN_WIDTH = 12;
const SB_PEEK_WIDTH = 12;
const SB_ICON_WIDTH = 84;
const SB_MAX_WIDTH = 480;
const SB_DEFAULT_WIDTH = 260;
const SB_ICON_ONLY_THRESHOLD = 108;
const SB_ICON_SNAP_THRESHOLD = 140;
const SB_PEEK_SNAP_THRESHOLD = 40;

const WIDTH_KEY = 'aw-sidebar-width';

export type SidebarMode = 'peek' | 'icon' | 'full';

function clamp(min: number, value: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function readInitialWidth(): number {
  const saved = Number(localStorage.getItem(WIDTH_KEY));
  return clamp(SB_MIN_WIDTH, Number.isFinite(saved) && saved > 0 ? saved : SB_DEFAULT_WIDTH, SB_MAX_WIDTH);
}

export function sidebarModeForWidth(width: number): SidebarMode {
  if (width <= SB_PEEK_WIDTH + 1) return 'peek';
  if (width <= SB_ICON_ONLY_THRESHOLD) return 'icon';
  return 'full';
}

export interface UseSidebar {
  width: number;
  mode: SidebarMode;
  /** Start a drag-resize from the grip (pointer down on the inner edge). */
  beginResize: (event: React.PointerEvent) => void;
  /** Toggle between peek and the default width (double-click / collapse button). */
  toggleCollapse: () => void;
  /** Expand from peek/icon to the default full width. */
  expand: () => void;
}

export function useSidebar(): UseSidebar {
  const [width, setWidth] = useState<number>(readInitialWidth);
  const widthRef = useRef(width);

  const persist = useCallback((value: number) => {
    localStorage.setItem(WIDTH_KEY, String(Math.round(value)));
  }, []);

  const beginResize = useCallback((event: React.PointerEvent) => {
    event.preventDefault();
    const startX = event.clientX;
    const startW = widthRef.current;
    let latest = startW;
    document.body.style.userSelect = 'none';
    document.body.style.cursor = 'col-resize';

    const onMove = (ev: PointerEvent) => {
      let next = clamp(SB_MIN_WIDTH, startW + (ev.clientX - startX), SB_MAX_WIDTH);
      // Auto-collapse snapping (matches vanilla): collapse to peek/icon when
      // dragging the edge past the snap thresholds.
      if (startW <= SB_ICON_ONLY_THRESHOLD && next <= SB_PEEK_SNAP_THRESHOLD) next = SB_PEEK_WIDTH;
      else if (startW > SB_ICON_ONLY_THRESHOLD && next <= SB_ICON_SNAP_THRESHOLD) next = SB_ICON_WIDTH;
      latest = next;
      setWidth(next);
    };
    const onUp = () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      document.body.style.userSelect = '';
      document.body.style.cursor = '';
      persist(latest);
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  }, [persist]);

  const expand = useCallback(() => {
    setWidth(SB_DEFAULT_WIDTH);
    persist(SB_DEFAULT_WIDTH);
  }, [persist]);

  const toggleCollapse = useCallback(() => {
    // Vanilla collapse button toggles icon-width <-> full-width (not peek).
    const next = widthRef.current <= SB_ICON_ONLY_THRESHOLD ? SB_DEFAULT_WIDTH : SB_ICON_WIDTH;
    setWidth(next);
    persist(next);
  }, [persist]);

  // Keep the persisted value in sync if width changes programmatically.
  useEffect(() => {
    widthRef.current = width;
  }, [width]);

  return { width, mode: sidebarModeForWidth(width), beginResize, toggleCollapse, expand };
}

export const sidebarConstants = {
  SB_MIN_WIDTH,
  SB_PEEK_WIDTH,
  SB_ICON_WIDTH,
  SB_MAX_WIDTH,
  SB_DEFAULT_WIDTH,
  SB_ICON_ONLY_THRESHOLD,
};

// --- Sidebar preferences (context menu): position + icon size + toggles ---
// Ported from the AW2 Sidebar context menu. Keys match the vanilla aw
// (aw-sidebar-*) so they stay consistent with the existing width key.
export const SB_ICON_SIZES = { small: 16, medium: 18, large: 22, extraLarge: 26 } as const;
export type SidebarIconSizeKey = keyof typeof SB_ICON_SIZES;
export const SB_POSITIONS = ['left', 'right', 'top', 'bottom'] as const;
export const SB_MENU_POSITIONS = ['left', 'right'] as const;
export type SidebarPosition = (typeof SB_POSITIONS)[number];

const POSITION_KEY = 'aw-sidebar-position';
const ICON_SIZE_KEY = 'aw-sidebar-icon-size';
const AUTOCOLLAPSE_KEY = 'aw-sidebar-autocollapse';
const SHOW_NAMES_KEY = 'aw-sidebar-show-icon-names';

function readPosition(): SidebarPosition {
  const value = localStorage.getItem(POSITION_KEY);
  // Top/bottom exist in the CSS for later, but are intentionally disabled in
  // the UI for now. If an older value is persisted, fall back to left.
  return SB_MENU_POSITIONS.includes(value as (typeof SB_MENU_POSITIONS)[number]) ? (value as SidebarPosition) : 'left';
}

function readIconSizeKey(): SidebarIconSizeKey {
  const value = localStorage.getItem(ICON_SIZE_KEY);
  return value && value in SB_ICON_SIZES ? (value as SidebarIconSizeKey) : 'medium';
}

function readBoolean(key: string, fallback: boolean): boolean {
  const value = localStorage.getItem(key);
  if (value === 'true') return true;
  if (value === 'false') return false;
  return fallback;
}

export interface UseSidebarPrefs {
  position: SidebarPosition;
  iconSizeKey: SidebarIconSizeKey;
  autoCollapse: boolean;
  showIconNames: boolean;
  setPosition: (position: SidebarPosition) => void;
  setIconSizeKey: (key: SidebarIconSizeKey) => void;
  toggleAutoCollapse: () => void;
  toggleShowIconNames: () => void;
}

export function useSidebarPrefs(): UseSidebarPrefs {
  const [position, setPositionState] = useState<SidebarPosition>(readPosition);
  const [iconSizeKey, setIconSizeState] = useState<SidebarIconSizeKey>(readIconSizeKey);
  const [autoCollapse, setAutoCollapse] = useState<boolean>(() => readBoolean(AUTOCOLLAPSE_KEY, true));
  const [showIconNames, setShowIconNames] = useState<boolean>(() => readBoolean(SHOW_NAMES_KEY, false));

  const setPosition = useCallback((next: SidebarPosition) => {
    const normalized = SB_MENU_POSITIONS.includes(next as (typeof SB_MENU_POSITIONS)[number]) ? next : 'left';
    localStorage.setItem(POSITION_KEY, normalized);
    setPositionState(normalized);
  }, []);

  const setIconSizeKey = useCallback((next: SidebarIconSizeKey) => {
    localStorage.setItem(ICON_SIZE_KEY, next);
    setIconSizeState(next);
  }, []);

  const toggleAutoCollapse = useCallback(() => {
    setAutoCollapse((current) => {
      const next = !current;
      localStorage.setItem(AUTOCOLLAPSE_KEY, String(next));
      return next;
    });
  }, []);

  const toggleShowIconNames = useCallback(() => {
    setShowIconNames((current) => {
      const next = !current;
      localStorage.setItem(SHOW_NAMES_KEY, String(next));
      return next;
    });
  }, []);

  return { position, iconSizeKey, autoCollapse, showIconNames, setPosition, setIconSizeKey, toggleAutoCollapse, toggleShowIconNames };
}
