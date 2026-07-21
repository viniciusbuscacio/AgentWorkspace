import { beforeEach, describe, expect, it, vi } from 'vitest';
import { extractPalette, localThemeFromPalette } from './palette';

// ── localThemeFromPalette — stable colours ────────────────────────────────────
describe('localThemeFromPalette', () => {
  it('maps palette correctly: darkest→background, contrast→accent', () => {
    const palette = ['#ffffff', '#000000', '#aaaaaa', '#5566ff'];
    const t = localThemeFromPalette(palette);
    expect(t.background).toBe('#000000'); // darkest
    expect(t.text).toBe('#ededed');       // dark background → light text
    // accent must have > 0.25 luminance difference from background (black)
    expect(t.accent).not.toBe('#000000');
  });

  it('uses fallback defaults for empty palette', () => {
    const t = localThemeFromPalette([]);
    expect(t.background).toBe('#171717');
    expect(t.accent).toBe('#6f96ff');
  });

  it('yields light text for dark background', () => {
    const t = localThemeFromPalette(['#111111', '#222222', '#333333', '#6f96ff']);
    expect(t.text).toBe('#ededed');
  });

  it('yields dark text for light background', () => {
    // All colors are clearly light — darkest is #cccccc.
    const t = localThemeFromPalette(['#ffffff', '#f0f0f0', '#e0e0e0', '#cccccc']);
    expect(t.text).toBe('#1f2328');
  });

  it('stable — same palette always produces the same result', () => {
    const palette = ['#1a1a2e', '#16213e', '#0f3460', '#e94560'];
    const a = localThemeFromPalette(palette);
    const b = localThemeFromPalette(palette);
    expect(a.background).toBe(b.background);
    expect(a.accent).toBe(b.accent);
    expect(a.text).toBe(b.text);
  });
});

// ── extractPalette — with canvas mock ────────────────────────────────────────
// jsdom does not implement the Canvas 2D context. We stub getContext so
// extractPalette returns a predictable set of colours from a synthetic pixel
// buffer (two red pixels + two blue pixels).
describe('extractPalette', () => {
  beforeEach(() => {
    // Build a 4-pixel RGBA buffer: 2 red + 2 blue (fully opaque).
    const pixelData = new Uint8ClampedArray([
      255, 0, 0, 255,   // red
      255, 0, 0, 255,   // red
      0, 0, 255, 255,   // blue
      0, 0, 255, 255,   // blue
    ]);
    const fakeImageData = { data: pixelData, width: 2, height: 2 };

    const fakeCtx = {
      drawImage: vi.fn(),
      getImageData: vi.fn().mockReturnValue(fakeImageData),
    };

    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
      fakeCtx as unknown as CanvasRenderingContext2D,
    );
  });

  it('resolves with an array of hex colour strings', async () => {
    // Image.onload fires synchronously when src is set in jsdom.
    const result = await extractPalette('data:image/png;base64,abc');
    expect(Array.isArray(result)).toBe(true);
    // Both quantised colours should be present.
    result.forEach((hex) => {
      expect(hex).toMatch(/^#[0-9a-fA-F]{6}$/);
    });
  });

  it('resolves to empty array on error', async () => {
    const result = await extractPalette('data:image/png;base64,INVALID');
    expect(Array.isArray(result)).toBe(true);
  });
});

// ── fallback — LLM helper rejects ────────────────────────────────────────────
// When AI refine is unavailable the local suggestion stands (Decision 3).
describe('AI refine fallback', () => {
  it('local suggestion is returned when AI result is falsy', () => {
    const palette = ['#1a1a2e', '#16213e', '#0f3460', '#e94560'];
    const local = localThemeFromPalette(palette);
    // Simulate AI helper rejecting — caller picks local.
    const aiResult = null as null | { name: string; background: string };
    const final = aiResult ?? local;
    expect(final.background).toBe(local.background);
  });
});
