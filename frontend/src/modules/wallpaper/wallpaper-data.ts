const WP = (id: string, name: string, file: string) => ({ id, name, file });

export const WALLPAPERS = [
  { id: 'default', name: 'Deep Space', file: '' },
  WP('mountain-lake', 'Mountain Lake', 'mountain-lake.jpg'),
  WP('northern-lights', 'Northern Lights', 'northern-lights.jpg'),
  WP('ocean-waves', 'Ocean Waves', 'ocean-waves.jpg'),
  WP('forest-path', 'Forest Path', 'forest-path.jpg'),
  WP('desert-dunes', 'Desert Dunes', 'desert-dunes.jpg'),
  WP('starry-night', 'Starry Night', 'starry-night.jpg'),
  WP('misty-mountains', 'Misty Mountains', 'misty-mountains.jpg'),
  WP('tropical-beach', 'Tropical Beach', 'tropical-beach.jpg'),
  WP('autumn-forest', 'Autumn Forest', 'autumn-forest.jpg'),
  WP('cherry-blossoms', 'Cherry Blossoms', 'cherry-blossoms.jpg'),
  WP('waterfall', 'Waterfall', 'waterfall.jpg'),
  WP('lavender-fields', 'Lavender Fields', 'lavender-fields.jpg'),
  WP('city-lights', 'City Lights', 'city-lights.jpg'),
  WP('tokyo-night', 'Tokyo Night', 'tokyo-night.jpg'),
  WP('new-york-skyline', 'New York Skyline', 'new-york-skyline.jpg'),
  WP('hong-kong', 'Hong Kong', 'hong-kong.jpg'),
  WP('london-bridge', 'London Bridge', 'london-bridge.jpg'),
  WP('earth-from-space', 'Earth from Space', 'earth-from-space.jpg'),
  WP('nebula', 'Nebula', 'nebula.jpg'),
  WP('milky-way', 'Milky Way', 'milky-way.jpg'),
  WP('full-moon', 'Full Moon', 'full-moon.jpg'),
  WP('dark-geometry', 'Dark Geometry', 'dark-geometry.jpg'),
  WP('gradient-blur', 'Gradient Blur', 'gradient-blur.jpg'),
] as const;

export type WallpaperID = (typeof WALLPAPERS)[number]['id'];

// Custom uploads are identified as "custom:<filename>" and rendered from a
// backend-provided data URI (they are not bundled assets). Mirrors
// domain.CustomWallpaperPrefix in Go.
export const CUSTOM_WALLPAPER_PREFIX = 'custom:';

export function isCustomWallpaperId(id: string | null | undefined): boolean {
  return typeof id === 'string' && id.startsWith(CUSTOM_WALLPAPER_PREFIX);
}

export function getWallpaper(id: string | null | undefined) {
  return WALLPAPERS.find((wallpaper) => wallpaper.id === id) ?? WALLPAPERS[0];
}

export function getWallpaperBackgroundImage(id: string | null | undefined): string {
  const wallpaper = getWallpaper(id);
  if (!wallpaper.file) {
    return 'linear-gradient(135deg, var(--background) 0%, var(--muted) 55%, var(--card) 100%)';
  }
  return `url(/wallpapers/${wallpaper.file})`;
}

// Class name AppShell puts on the wrapper behind every non-home view
// (modules + chat). This is the single source of truth for the rule that bit
// us once: the wallpaper must stay visible in module/chat views whenever the
// glass slider is above 0. Only an explicit 0 ('fully solid') hides it — any
// positive value uses the frosted veil that reveals the wallpaper.
//
// Locked by wallpaper-data.test.ts: if a refactor ever makes positive glass
// values resolve to the solid wrapper again (the regression that hid the
// wallpaper in Tasks/Chats/Passwords), that test fails and the frontend
// build (npm run build:frontend, part of the build gate) refuses to compile.
export type WallpaperGlassWrapperClass = 'aw-wallpaper-glass-solid' | 'aw-wallpaper-glass-veil';

export function wallpaperGlassWrapperClass(glass: number): WallpaperGlassWrapperClass {
  return glass <= 0 ? 'aw-wallpaper-glass-solid' : 'aw-wallpaper-glass-veil';
}

// wallpaperShowsInModuleViews reports whether, at the given glass value, the
// wallpaper bleeds through the non-home wrapper. True for every positive glass
// value; false only at the explicit solid floor (0). Kept next to the wrapper
// rule so the two can never drift apart.
export function wallpaperShowsInModuleViews(glass: number): boolean {
  return wallpaperGlassWrapperClass(glass) === 'aw-wallpaper-glass-veil';
}

// Applies the glass slider to the CSS vars the veil consumes — the ONE place
// the slider-to-veil math lives. 0 = solid app background; values in between
// = frosted glass (tint and blur both fade as the value rises); 100 = the
// crisp photo, NO veil at all (user decision 2026-06-12, superseding the old
// 35% readability floor — readability at 100 is the user's call).
export function applyWallpaperGlassVars(value: number) {
  const root = document.documentElement;
  root.style.setProperty('--aw-wallpaper-glass-pct', `${Math.max(0, 100 - value)}%`);
  root.style.setProperty(
    '--aw-wallpaper-glass-filter',
    value >= 100 ? 'none' : `blur(${((100 - value) * 0.1).toFixed(1)}px) saturate(1.15)`,
  );
}
