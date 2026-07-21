// App-wide zoom — faithful port of the vanilla zoom (main.js setAppZoom).
//
// The vanilla applies BOTH the CSS `zoom` (which scales the whole document) and
// the `--aw-app-zoom` variable. The #app container is sized with
// `calc(100vw / var(--aw-app-zoom))` so that, after `zoom` shrinks/grows the
// viewport, the app still fills the screen exactly. Applying only the variable
// (without `zoom`) shrinks #app without scaling the content back — so the app
// stops filling the window. Always apply both together.

export const APP_ZOOM_MIN = 50;
export const APP_ZOOM_MAX = 200;
export const APP_ZOOM_STEP = 10;

export function clampAppZoom(percent: number): number {
  return Math.min(APP_ZOOM_MAX, Math.max(APP_ZOOM_MIN, Math.round(percent)));
}

// requestAppZoom lets views outside the shell (Settings → Fonts) change the
// app zoom through AppShell, which owns persistence and the live zoom state —
// same window-event deep-link pattern as lib/open-settings.
export const APP_ZOOM_EVENT = 'aw:set-app-zoom';

export function requestAppZoom(percent: number): void {
  window.dispatchEvent(new CustomEvent<number>(APP_ZOOM_EVENT, { detail: clampAppZoom(percent) }));
}

/** Apply the app zoom to the document root (both CSS `zoom` and the variable). */
export function applyAppZoom(percent: number): void {
  const zoom = clampAppZoom(percent) / 100;
  const root = document.documentElement;
  // setProperty keeps this type-safe across TS DOM lib versions (zoom is non-standard).
  root.style.setProperty('zoom', String(zoom));
  root.style.setProperty('--aw-app-zoom', String(zoom));
}

// When the document root has a CSS `zoom`, a `position: fixed` element styled
// with `left/top: L` is rendered (and reported by getBoundingClientRect) at
// `L * scale`, while a pointer event reports clientX/Y already in that scaled
// space. So a menu placed at `left: clientX` lands at `clientX * scale` — far
// from the cursor. We measure the actual scale with a throwaway fixed probe
// (engine-agnostic: Blink and WebKit handle `zoom` differently) and divide the
// click coords by it, so the fixed element lands exactly under the cursor.
export function measureFixedScale(): number {
  const probe = document.createElement('div');
  probe.style.cssText = 'position:fixed;left:0;top:1000px;width:0;height:0;visibility:hidden;pointer-events:none;';
  document.body.appendChild(probe);
  const top = probe.getBoundingClientRect().top;
  probe.remove();
  const scale = top / 1000;
  return Number.isFinite(scale) && scale > 0.1 ? scale : 1;
}

/** Map a pointer client point to the left/top a fixed element needs to land there. */
export function clientPointToFixed(clientX: number, clientY: number): { x: number; y: number } {
  const scale = measureFixedScale();
  return { x: clientX / scale, y: clientY / scale };
}
