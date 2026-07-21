// Palette extraction and local theme suggestion from a reference image.
// Ported from AW2 src/renderer/theme/palette.ts — algorithm is verbatim:
// 96×96 resample, luminance buckets (32-step quantisation), top-8 colours,
// then map darkest→background and high-contrast→accent for the local suggestion.
// No image data ever leaves the machine (Decision 3): only hex strings are
// sent to the LLM in the optional AI-refine step.

import { luminance, type CustomThemeTokens } from './custom-theme';

const DARK_FALLBACK = ['#171717', '#1f1f1f', '#2d2d2d', '#6f96ff'];
const LIGHT_TEXT = '#1f2328';
const DARK_TEXT = '#ededed';
const LIGHT_MUTED = '#57606a';
const DARK_MUTED = '#a3a3a3';
const ACCENT_FALLBACK = '#6f96ff';

function rgbToHex(r: number, g: number, b: number): string {
  return `#${[r, g, b]
    .map((v) => Math.max(0, Math.min(255, v)).toString(16).padStart(2, '0'))
    .join('')}`;
}

function readableTextForPalette(background: string): string {
  return luminance(background) > 0.45 ? LIGHT_TEXT : DARK_TEXT;
}

/** Read a File as a base64 data URI. */
export function fileToBase64(file: File): Promise<{ data: string; dataUri: string }> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const dataUri = String(reader.result ?? '');
      resolve({ dataUri, data: dataUri.split(',')[1] ?? '' });
    };
    reader.onerror = () => reject(reader.error ?? new Error('Failed to read image'));
    reader.readAsDataURL(file);
  });
}

/**
 * Extract up to 8 dominant colours from a data URI using a 96×96 canvas
 * resample and 32-step RGB quantisation — identical to AW2's algorithm.
 * Resolves to an empty array on any failure.
 */
export function extractPalette(dataUri: string): Promise<string[]> {
  return new Promise((resolve) => {
    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement('canvas');
      const size = 96;
      canvas.width = size;
      canvas.height = Math.max(1, Math.round((size * img.height) / Math.max(1, img.width)));
      const ctx = canvas.getContext('2d');
      if (!ctx) { resolve([]); return; }
      ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
      const pixels = ctx.getImageData(0, 0, canvas.width, canvas.height).data;
      const buckets = new Map<string, number>();
      for (let i = 0; i < pixels.length; i += 16) {
        if (pixels[i + 3] < 180) continue; // skip near-transparent
        const r = Math.round(pixels[i] / 32) * 32;
        const g = Math.round(pixels[i + 1] / 32) * 32;
        const b = Math.round(pixels[i + 2] / 32) * 32;
        const key = rgbToHex(r, g, b);
        buckets.set(key, (buckets.get(key) ?? 0) + 1);
      }
      resolve(
        [...buckets.entries()]
          .sort((a, b) => b[1] - a[1])
          .slice(0, 8)
          .map(([hex]) => hex),
      );
    };
    img.onerror = () => resolve([]);
    img.src = dataUri;
  });
}

/**
 * Map an extracted palette to a CustomThemeTokens suggestion — purely local,
 * no network call. Darkest colour becomes background; third-darkest is
 * surfaceAlt; the colour with highest luminance contrast to background becomes
 * accent.
 */
export function localThemeFromPalette(palette: string[]): CustomThemeTokens {
  const colours = palette.length ? palette : DARK_FALLBACK;
  const sorted = [...colours].sort((a, b) => luminance(a) - luminance(b));
  const background = sorted[0];
  const surface = sorted[1] ?? background;
  const surfaceAlt = sorted[2] ?? surface;
  const accent =
    colours.find((c) => Math.abs(luminance(c) - luminance(background)) > 0.25) ??
    sorted[sorted.length - 1] ??
    ACCENT_FALLBACK;
  return {
    name: 'Reference Theme',
    background,
    surface,
    surfaceAlt,
    text: readableTextForPalette(background),
    mutedText: luminance(background) > 0.45 ? LIGHT_MUTED : DARK_MUTED,
    border: surfaceAlt,
    accent,
  };
}
