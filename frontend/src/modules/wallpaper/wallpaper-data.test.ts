import { describe, it, expect, beforeEach } from 'vitest';
import {
  wallpaperGlassWrapperClass,
  wallpaperShowsInModuleViews,
  applyWallpaperGlassVars,
} from './wallpaper-data';

// Regression lock for the 2026-06-13 incident: the wallpaper stopped showing
// in the module/chat views (Tasks, Chats, Passwords, ...) while it still
// showed on the Home/"Open Apps and Modules" screen. Root cause was the glass
// slider sitting at 0, which by design uses a fully solid wrapper. These tests
// pin the contract so a CODE change can never silently make a POSITIVE glass
// value resolve to "solid" (wallpaper hidden) again. A failure here fails
// `npm run build:frontend` (part of the build gate), so the frontend will not
// compile/ship with the regression.
describe('wallpaper glass — module-view visibility contract', () => {
  it('uses the solid wrapper ONLY at the explicit 0 floor', () => {
    expect(wallpaperGlassWrapperClass(0)).toBe('aw-wallpaper-glass-solid');
    expect(wallpaperShowsInModuleViews(0)).toBe(false);
  });

  it('shows the wallpaper (frosted veil) for every positive glass value', () => {
    for (const glass of [1, 5, 20, 50, 70, 99, 100]) {
      expect(wallpaperGlassWrapperClass(glass)).toBe('aw-wallpaper-glass-veil');
      expect(wallpaperShowsInModuleViews(glass)).toBe(true);
    }
  });

  it('never treats the restored default (70) as solid — the exact bug that hid it', () => {
    expect(wallpaperGlassWrapperClass(70)).not.toBe('aw-wallpaper-glass-solid');
    expect(wallpaperShowsInModuleViews(70)).toBe(true);
  });
});

describe('wallpaper glass — CSS var math reveals the wallpaper', () => {
  beforeEach(() => {
    document.documentElement.style.removeProperty('--aw-wallpaper-glass-pct');
    document.documentElement.style.removeProperty('--aw-wallpaper-glass-filter');
  });

  const pct = () =>
    document.documentElement.style.getPropertyValue('--aw-wallpaper-glass-pct');
  const filter = () =>
    document.documentElement.style.getPropertyValue('--aw-wallpaper-glass-filter');

  it('makes the veil partly transparent for positive glass (wallpaper bleeds through)', () => {
    applyWallpaperGlassVars(70);
    // pct is the OPAQUE share of the tint; below 100% means the wallpaper shows.
    expect(pct()).toBe('30%');
    expect(Number.parseInt(pct(), 10)).toBeLessThan(100);
  });

  it('is fully crisp (no veil) at 100', () => {
    applyWallpaperGlassVars(100);
    expect(pct()).toBe('0%');
    expect(filter()).toBe('none');
  });

  it('is monotonic: more glass => less tint => more wallpaper', () => {
    applyWallpaperGlassVars(20);
    const low = Number.parseInt(pct(), 10);
    applyWallpaperGlassVars(80);
    const high = Number.parseInt(pct(), 10);
    expect(high).toBeLessThan(low);
  });
});
