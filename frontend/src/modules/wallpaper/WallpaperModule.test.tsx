import { act, render, screen, fireEvent, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { WallpaperModule } from './WallpaperModule';

// Notices now go through the app-wide toast (lib/notify -> sonner); module
// tests assert the notify text instead of inline DOM messages.
const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));


const getWallpaper = vi.fn();
const setWallpaper = vi.fn();
const getGlass = vi.fn();
const setGlass = vi.fn();
const listCustom = vi.fn();
const pickAndUpload = vi.fn();
const getImage = vi.fn();
const deleteCustom = vi.fn();

vi.mock('@services/wallpaper.service', () => ({
  wallpaperService: {
    get: (...args: unknown[]) => getWallpaper(...args),
    set: (...args: unknown[]) => setWallpaper(...args),
    getGlass: (...args: unknown[]) => getGlass(...args),
    setGlass: (...args: unknown[]) => setGlass(...args),
    listCustom: (...args: unknown[]) => listCustom(...args),
    pickAndUpload: (...args: unknown[]) => pickAndUpload(...args),
    getImage: (...args: unknown[]) => getImage(...args),
    deleteCustom: (...args: unknown[]) => deleteCustom(...args),
  },
  DEFAULT_WALLPAPER_GLASS: 20,
}));

vi.mock('@services/events', () => ({
  onAwEvent: vi.fn(() => vi.fn()),
}));

const webModeControl = vi.hoisted(() => ({ enabled: false }));
vi.mock('@/web/web-bindings', () => ({
  isWebMode: () => webModeControl.enabled,
}));

beforeEach(() => {
  vi.useRealTimers();
  webModeControl.enabled = false;
  getWallpaper.mockResolvedValue('default');
  setWallpaper.mockResolvedValue({ success: true });
  getGlass.mockResolvedValue(20);
  setGlass.mockResolvedValue({ success: true });
  listCustom.mockResolvedValue({ success: true, custom: [] });
  pickAndUpload.mockResolvedValue({ success: true, canceled: true, custom: [] });
  getImage.mockResolvedValue({ success: true, dataUri: 'data:image/png;base64,AAAA' });
  deleteCustom.mockResolvedValue({ success: true, custom: [] });
});

afterEach(() => {
  vi.useRealTimers();
});

describe('WallpaperModule glass slider', () => {
  it('renders the glass slider with the loaded value', async () => {
    getGlass.mockResolvedValue(40);
    render(<WallpaperModule />);
    const slider = await screen.findByRole('slider', { name: /background visibility/i });
    await waitFor(() => expect((slider as HTMLInputElement).value).toBe('40'));
  });

  it('calls setGlass when the slider changes (after debounce)', async () => {
    // Use fake timers so we control the 300ms debounce.
    vi.useFakeTimers({ shouldAdvanceTime: false });
    try {
      render(<WallpaperModule />);
      // Wait for the component to mount with real-timer microtasks flushed.
      await act(async () => {
        await Promise.resolve();
      });
      const slider = screen.getByRole('slider', { name: /background visibility/i });
      act(() => {
        fireEvent.change(slider, { target: { value: '60' } });
      });
      // Before debounce fires: setGlass should NOT have been called yet.
      expect(setGlass).not.toHaveBeenCalled();
      // Advance past the 300ms debounce window.
      act(() => { vi.advanceTimersByTime(400); });
      expect(setGlass).toHaveBeenCalledWith(60);
    } finally {
      vi.useRealTimers();
    }
  });

  it('loads the slider at 0 when glass is off', async () => {
    getGlass.mockResolvedValue(0);
    render(<WallpaperModule />);
    const slider = await screen.findByRole('slider', { name: /background visibility/i });
    await waitFor(() => expect((slider as HTMLInputElement).value).toBe('0'));
  });
});

/**
 * Utility: wallpaperGlassClass maps glass value to the correct CSS class
 * name applied by AppShell. Spec gate: "0 yields the solid class path".
 */
describe('wallpaperGlassClass', () => {
  function wallpaperGlassClass(glass: number): string {
    return glass === 0 ? 'aw-wallpaper-glass-solid' : 'aw-wallpaper-glass-veil';
  }

  it('returns aw-wallpaper-glass-solid for 0', () => {
    expect(wallpaperGlassClass(0)).toBe('aw-wallpaper-glass-solid');
  });

  it('returns aw-wallpaper-glass-veil for any value > 0', () => {
    expect(wallpaperGlassClass(1)).toBe('aw-wallpaper-glass-veil');
    expect(wallpaperGlassClass(20)).toBe('aw-wallpaper-glass-veil');
    expect(wallpaperGlassClass(100)).toBe('aw-wallpaper-glass-veil');
  });
});

// Save/Cancel (user request 2026-06-12): changes preview live; Save commits
// the snapshot, Cancel reverts wallpaper AND glass to the last saved state.
// The pair is always rendered (user request 2026-06-13) and disabled until a
// change makes the view dirty.
describe('WallpaperModule save/cancel', () => {
  it('keeps Save/Cancel visible, disabled when clean, and Cancel reverts to the saved state', async () => {
    getWallpaper.mockResolvedValue('default');
    getGlass.mockResolvedValue(20);
    render(<WallpaperModule />);
    await screen.findByRole('slider', { name: /background visibility/i });

    // Clean state: the pair is present but disabled.
    const saveWhenClean = screen.getByRole('button', { name: /save/i });
    expect(saveWhenClean).toBeTruthy();
    expect((saveWhenClean as HTMLButtonElement).disabled).toBe(true);

    // Preview a different wallpaper → dirty → the pair enables.
    fireEvent.click(await screen.findByRole('button', { name: /Ocean Waves/i }));
    await waitFor(() => expect(setWallpaper).toHaveBeenCalledWith('ocean-waves'));
    const cancel = await screen.findByRole('button', { name: /cancel/i });
    await waitFor(() => expect((screen.getByRole('button', { name: /save/i }) as HTMLButtonElement).disabled).toBe(false));

    // Cancel restores the snapshot: wallpaper and glass re-persisted.
    setWallpaper.mockClear();
    fireEvent.click(cancel);
    await waitFor(() => expect(setWallpaper).toHaveBeenCalledWith('default'));
    expect(setGlass).toHaveBeenCalledWith(20);
    await waitFor(() => expect((screen.getByRole('button', { name: /save/i }) as HTMLButtonElement).disabled).toBe(true));
  });

  it('Save commits the new snapshot and disables the pair again', async () => {
    render(<WallpaperModule />);
    await screen.findByRole('slider', { name: /background visibility/i });
    fireEvent.click(await screen.findByRole('button', { name: /Starry Night/i }));
    const save = await screen.findByRole('button', { name: /save/i });
    await waitFor(() => expect((save as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(save);
    await waitFor(() => expect((screen.getByRole('button', { name: /save/i }) as HTMLButtonElement).disabled).toBe(true));
    expect(notifyMock).toHaveBeenCalledWith('Wallpaper saved.');
  });

  it('uploads a custom wallpaper via the picker and selects it', async () => {
    pickAndUpload.mockResolvedValue({ success: true, id: 'custom:my.png', custom: ['custom:my.png'] });
    getImage.mockResolvedValue({ success: true, dataUri: 'data:image/png;base64,AAAA' });
    render(<WallpaperModule />);
    await screen.findByRole('slider', { name: /background visibility/i });
    fireEvent.click(screen.getByRole('button', { name: /upload new wallpaper/i }));
    await waitFor(() => expect(pickAndUpload).toHaveBeenCalled());
    await waitFor(() => expect(notifyMock).toHaveBeenCalledWith('Wallpaper uploaded.'));
  });

  it('web mode hides the upload button (native picker cannot open remotely)', async () => {
    webModeControl.enabled = true;
    render(<WallpaperModule />);
    await screen.findByRole('slider', { name: /background visibility/i });
    expect(screen.queryByRole('button', { name: /upload new wallpaper/i })).toBeNull();
  });

  it('upload failure re-enables the button and shows the error', async () => {
    pickAndUpload.mockResolvedValue({ success: false, error: 'not available in web mode' });
    render(<WallpaperModule />);
    await screen.findByRole('slider', { name: /background visibility/i });
    const button = screen.getByRole('button', { name: /upload new wallpaper/i });
    fireEvent.click(button);
    await waitFor(() => expect(notifyMock).toHaveBeenCalledWith('not available in web mode'));
    expect((button as HTMLButtonElement).disabled).toBe(false);
  });
});
