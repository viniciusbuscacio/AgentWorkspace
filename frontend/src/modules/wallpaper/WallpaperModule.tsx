import { useCallback, useEffect, useRef, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { SaveCancelActions } from '@/components/patterns/SaveCancelActions';
import { onAwEvent } from '@services/events';
import { wallpaperService, DEFAULT_WALLPAPER_GLASS } from '@services/wallpaper.service';
import { WALLPAPERS, getWallpaperBackgroundImage, applyWallpaperGlassVars } from './wallpaper-data';
import { isWebMode } from '@/web/web-bindings';

export function WallpaperModule() {
  const [wallpaperId, setWallpaperId] = useState('default');
  const [saving, setSaving] = useState(false);
  const [glass, setGlass] = useState(DEFAULT_WALLPAPER_GLASS);
  // User uploads: ids ("custom:<file>") and their data URIs for the thumbnails.
  const [customIds, setCustomIds] = useState<string[]>([]);
  const [customImages, setCustomImages] = useState<Record<string, string>>({});
  // The last CONFIRMED state: selections and slider moves apply live as a
  // preview, and Save/Cancel commit or revert to this snapshot.
  const savedRef = useRef<{ id: string; glass: number } | null>(null);
  const [dirty, setDirty] = useState(false);
  // Debounce glass persistence so sliding is smooth.
  const glassTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // loadCustom refreshes the upload list and fetches any missing thumbnails.
  const loadCustom = useCallback(async (ids: string[]) => {
    setCustomIds(ids);
    const missing = ids.filter((id) => !customImages[id]);
    if (missing.length === 0) return;
    const entries = await Promise.all(
      missing.map(async (id) => {
        const result = await wallpaperService.getImage(id);
        return [id, result.success ? result.dataUri || '' : ''] as const;
      }),
    );
    setCustomImages((prev) => {
      const next = { ...prev };
      for (const [id, uri] of entries) if (uri) next[id] = uri;
      return next;
    });
  }, [customImages]);

  useEffect(() => {
    void Promise.all([wallpaperService.get(), wallpaperService.getGlass()]).then(([id, glassValue]) => {
      setWallpaperId(id);
      setGlass(glassValue);
      if (!savedRef.current) savedRef.current = { id, glass: glassValue };
    });
    void wallpaperService.listCustom().then((result) => {
      if (result.success) void loadCustom(result.custom ?? []);
    });
    const offWallpaper = onAwEvent('ui:set-wallpaper', ({ id }) => setWallpaperId(id));
    // Agent-driven glass changes (app.wallpaper.glass.set) must move the
    // slider and the Save/Cancel baseline too, or they go stale.
    const offGlass = onAwEvent('ui:set-wallpaper-glass', ({ opacity }) => {
      setGlass(opacity);
      if (savedRef.current) savedRef.current = { ...savedRef.current, glass: opacity };
    });
    return () => {
      offWallpaper();
      offGlass();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- boot once
  }, []);

  function markDirty(nextId: string, nextGlass: number) {
    const saved = savedRef.current;
    setDirty(!saved || saved.id !== nextId || saved.glass !== nextGlass);
  }

  async function select(id: string) {
    setSaving(true);
    const result = await wallpaperService.set(id);
    setSaving(false);
    if (!result.success) {
      setMessage(result.error || 'Could not set wallpaper.');
      return;
    }
    setWallpaperId(id);
    setMessage('');
    markDirty(id, glass);
  }

  async function uploadNew() {
    setSaving(true);
    setMessage('');
    let result;
    try {
      result = await wallpaperService.pickAndUpload();
    } finally {
      setSaving(false);
    }
    if (result.canceled) return;
    if (!result.success) {
      setMessage(result.error || 'Could not upload the image.');
      return;
    }
    await loadCustom(result.custom ?? []);
    if (result.id) {
      setWallpaperId(result.id);
      markDirty(result.id, glass);
    }
    setMessage('Wallpaper uploaded.');
  }

  async function removeCustom(id: string) {
    const result = await wallpaperService.deleteCustom(id);
    if (!result.success) {
      setMessage(result.error || 'Could not delete the wallpaper.');
      return;
    }
    setCustomImages((prev) => {
      const next = { ...prev };
      delete next[id];
      return next;
    });
    await loadCustom(result.custom ?? []);
    // The backend falls back to default when the active upload is deleted.
    if (wallpaperId === id) {
      setWallpaperId('default');
      markDirty('default', glass);
    }
    setMessage('Wallpaper deleted.');
  }

  function onGlassChange(value: number) {
    setGlass(value);
    markDirty(wallpaperId, value);
    // Update the CSS variables immediately for instant preview.
    applyWallpaperGlassVars(value);
    if (glassTimerRef.current) clearTimeout(glassTimerRef.current);
    glassTimerRef.current = setTimeout(() => {
      void wallpaperService.setGlass(value);
    }, 300);
  }

  function onSave() {
    savedRef.current = { id: wallpaperId, glass };
    setDirty(false);
    setMessage('Wallpaper saved.');
  }

  function onCancel() {
    const saved = savedRef.current;
    if (!saved) return;
    void wallpaperService.set(saved.id);
    void wallpaperService.setGlass(saved.glass);
    setWallpaperId(saved.id);
    setGlass(saved.glass);
    applyWallpaperGlassVars(saved.glass);
    setDirty(false);
    setMessage('Reverted to the saved wallpaper.');
  }

  return (
    // No aw-wallpaper-surface here: painting the full image inside this view
    // covered the glass veil and made the slider look dead. The module sits
    // behind the same veil as every other view, so dragging the slider IS
    // the live preview of the effect (the var updates on input).
    <section className="home-screen" data-awid="wallpaper-screen">
      <div className="home-header flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="home-title">Wallpaper</h1>
          <p className="home-subtitle">Choose the background for Home and Apps.</p>
        </div>
        <div className="flex flex-wrap items-end gap-4">
          {/* Glass slider — how much wallpaper shows behind app panels */}
          <div className="min-w-[220px]">
            <label htmlFor="wallpaper-glass-slider" className="mb-1.5 block text-sm font-medium text-foreground">
              Background visibility behind apps
            </label>
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">Off</span>
              <input
                id="wallpaper-glass-slider"
                type="range"
                min={0}
                max={100}
                step={5}
                value={glass}
                className="flex-1 accent-primary"
                aria-label="Background visibility behind apps"
                onChange={(e) => onGlassChange(Number(e.target.value))}
              />
              <span className="text-xs text-muted-foreground">Full</span>
              <span className="w-7 text-right text-xs tabular-nums text-muted-foreground">{glass}</span>
            </div>
          </div>
          {/* The native file picker cannot open on a remote browser (the
              binding is web-denylisted); hide the button there. */}
          {!isWebMode() && (
          <Button
            type="button"
            icon="upload"
            variant="outline"
            disabled={saving}
            onClick={() => void uploadNew()}
            data-awid="wallpaper-upload"
          >
            Upload New Wallpaper
          </Button>
          )}
        </div>
      </div>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-3 pb-8">
        {customIds.map((id) => {
          const active = wallpaperId === id;
          return (
            <div
              key={id}
              className={`group relative aspect-[16/10] overflow-hidden rounded-lg border-2 bg-card/40 shadow-lg transition-transform hover:scale-[1.03] ${active ? 'border-primary' : 'border-transparent'}`}
            >
              <button
                type="button"
                className="block size-full text-left"
                aria-pressed={active}
                aria-label={`Custom wallpaper${active ? ' (selected)' : ''}`}
                disabled={saving}
                onClick={() => void select(id)}
              >
                {customImages[id] ? (
                  <img className="size-full object-cover" src={customImages[id]} alt="Custom wallpaper" loading="lazy" />
                ) : (
                  <div className="size-full animate-pulse bg-muted" />
                )}
              </button>
              <span className="pointer-events-none absolute inset-x-0 bottom-0 bg-background/85 px-2 py-1.5 text-center text-xs text-foreground">
                My wallpaper
              </span>
              <button
                type="button"
                className="absolute right-1 top-1 hidden rounded-md bg-background/85 p-1 text-foreground hover:bg-destructive hover:text-destructive-foreground group-hover:block"
                aria-label="Delete this wallpaper"
                onClick={() => void removeCustom(id)}
              >
                <span className="material-symbols-outlined text-[18px] leading-none">delete</span>
              </button>
            </div>
          );
        })}
        {WALLPAPERS.map((wallpaper) => {
          const active = wallpaperId === wallpaper.id;
          return (
            <Button
              key={wallpaper.id}
              type="button"
              variant="ghost"
              className={`group relative aspect-[16/10] h-auto overflow-hidden rounded-lg border-2 bg-card/40 p-0 text-left shadow-lg transition-transform hover:scale-[1.03] hover:bg-card/40 ${active ? 'border-primary' : 'border-transparent'}`}
              aria-pressed={active}
              disabled={saving}
              onClick={() => void select(wallpaper.id)}
            >
              {wallpaper.file ? (
                <img className="size-full object-cover" src={`/wallpapers/${wallpaper.file}`} alt={wallpaper.name} loading="lazy" />
              ) : (
                <div className="size-full" style={{ backgroundImage: getWallpaperBackgroundImage('default') }} />
              )}
              <span className="absolute inset-x-0 bottom-0 bg-background/85 px-2 py-1.5 text-center text-xs text-foreground">
                {wallpaper.name}
              </span>
            </Button>
          );
        })}
      </div>

      {/* Glass slider lives in the header (right of Upload). */}

      <SaveCancelActions
        onSave={onSave}
        onCancel={onCancel}
        saving={saving}
        disabled={!dirty}
        />
    </section>
  );
}
