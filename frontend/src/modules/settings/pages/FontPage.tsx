import { useEffect, useState } from 'react';
import { Button } from '@ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@ui/card';
import { ZoomSafeSelect, type ZoomSafeSelectOption } from '@ui/zoom-safe-select';
import { uiService } from '@services/ui.service';
import { notify } from '@/lib/notify';
import { settingsService } from '@services/settings.service';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { APP_ZOOM_MAX, APP_ZOOM_MIN, APP_ZOOM_STEP, clampAppZoom, requestAppZoom } from '@/lib/app-zoom';
import {
  APP_FONT_SIZE_MAX,
  APP_FONT_SIZE_MIN,
  APP_FONT_SIZE_STEP,
  DEFAULT_APP_FONT_FAMILY,
  DEFAULT_APP_FONT_SIZE,
  applyAppFont,
  getAppFontOption,
  getGroupedAppFontOptions,
  getSavedAppFontFamily,
  getSavedAppFontSize,
} from '@/lib/app-font';

export function FontPage() {
  const [savedFamily, setSavedFamily] = useState(getSavedAppFontFamily);
  const [savedSize, setSavedSize] = useState(getSavedAppFontSize);
  const [family, setFamily] = useState(savedFamily);
  const [size, setSize] = useState(savedSize);
  // App zoom applies immediately (no save/cancel): AppShell owns persistence;
  // this page only reads the current value and requests changes.
  const [zoom, setZoom] = useState(100);

  useEffect(() => {
    void settingsService.getAppZoomPercent().then((percent) => setZoom(clampAppZoom(percent)));
  }, []);

  function changeAppZoom(percent: number) {
    const next = clampAppZoom(percent);
    setZoom(next);
    requestAppZoom(next);
  }

  const dirty = family !== savedFamily || size !== savedSize;

  function save() {
    const applied = applyAppFont(family, size);
    setSavedFamily(applied.family);
    setSavedSize(applied.size);
    setFamily(applied.family);
    setSize(applied.size);
    void uiService.reportUiState({ fontFamily: applied.family, fontSize: applied.size });
    notify('Font settings saved.');
  }

  function cancel() {
    setFamily(savedFamily);
    setSize(savedSize);
    applyAppFont(savedFamily, savedSize);
  }

  function resetDraft() {
    setFamily(DEFAULT_APP_FONT_FAMILY);
    setSize(DEFAULT_APP_FONT_SIZE);
  }

  const option = getAppFontOption(family);
  const fontGroups = getGroupedAppFontOptions();

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(280px,420px)]">
      <Card className="rounded-lg">
        <CardHeader>
          <CardTitle>Fonts</CardTitle>
          <CardDescription>Change the application font family and base font size.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-5">
          <div className="grid gap-2 text-sm font-medium text-foreground">
            Font family
            <ZoomSafeSelect
              value={family}
              onValueChange={setFamily}
              options={fontGroups.flatMap((section): ZoomSafeSelectOption[] => [
                // Group headers ride along as disabled options (the zoom-safe
                // panel has no <optgroup> equivalent).
                { value: `__group-${section.group}`, label: section.group, disabled: true },
                ...section.options.map((item) => ({ value: item.id, label: item.label })),
              ])}
              aria-label="Font family"
              className="h-10"
            />
          </div>

          <div className="grid gap-2">
            <div className="flex items-center justify-between gap-3">
              <label className="text-sm font-medium text-foreground" htmlFor="app-font-size">Font size</label>
              <span className="rounded-md border border-border bg-background px-2 py-1 text-xs text-muted-foreground">{size}px</span>
            </div>
            <div className="flex items-center gap-3">
              <Button
                type="button"
                variant="outline"
                size="icon-sm"
                icon="text_decrease"
                aria-label="Decrease font size"
                onClick={() => setSize(size - APP_FONT_SIZE_STEP)}
                disabled={size <= APP_FONT_SIZE_MIN}
              />
              <input
                id="app-font-size"
                type="range"
                min={APP_FONT_SIZE_MIN}
                max={APP_FONT_SIZE_MAX}
                step={APP_FONT_SIZE_STEP}
                value={size}
                onChange={(event) => setSize(Number(event.target.value))}
                className="min-w-0 flex-1 accent-[var(--accent)]"
                aria-label="Font size"
              />
              <Button
                type="button"
                variant="outline"
                size="icon-sm"
                icon="text_increase"
                aria-label="Increase font size"
                onClick={() => setSize(size + APP_FONT_SIZE_STEP)}
                disabled={size >= APP_FONT_SIZE_MAX}
              />
            </div>
            <div className="flex justify-between text-xs text-muted-foreground">
              <span>{APP_FONT_SIZE_MIN}px</span>
              <span>{APP_FONT_SIZE_MAX}px</span>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2">
          <SaveCancelActions
            onSave={save}
            onCancel={cancel}
            disabled={!dirty}
            status={dirty ? <span className="text-xs text-muted-foreground">Unsaved changes</span> : null}
          />
          <Button type="button" variant="outline" icon="restart_alt" onClick={resetDraft}>Reset fonts</Button>
          </div>
        </CardContent>
      </Card>

      <Card className="gap-3 rounded-lg">
        <CardHeader>
          <CardTitle>Preview</CardTitle>
          <CardDescription>{option.label} · {size}px</CardDescription>
        </CardHeader>
        <CardContent>
          <div
            className="rounded-lg border border-border bg-background p-4 text-foreground"
            data-testid="font-preview"
            style={{ fontFamily: option.css, fontSize: `${size}px` }}
          >
            <p className="mb-3 font-semibold leading-tight" style={{ fontSize: '1.08em' }}>Agent Workspace</p>
            <p className="mb-3 leading-relaxed text-muted-foreground" style={{ fontSize: '0.95em' }}>
              This preview follows the active theme and shows how menus, cards and chat text will feel with the selected font.
            </p>
            <div className="rounded-md border border-border bg-secondary px-3 py-2 leading-normal" style={{ fontSize: '1em' }}>
              The quick brown fox jumps over the lazy dog. 0123456789
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="rounded-lg">
        <CardHeader>
          <CardTitle>App zoom</CardTitle>
          <CardDescription>Scale the whole interface. Applies immediately; Ctrl/Cmd +, − and 0 also work anywhere.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-3">
            <Button
              type="button"
              variant="outline"
              size="icon-sm"
              icon="zoom_out"
              aria-label="Zoom out"
              onClick={() => changeAppZoom(zoom - APP_ZOOM_STEP)}
              disabled={zoom <= APP_ZOOM_MIN}
            />
            <Button
              type="button"
              variant="outline"
              className="min-w-20"
              aria-label="Reset zoom to 100%"
              title="Reset to 100%"
              onClick={() => changeAppZoom(100)}
            >
              {zoom}%
            </Button>
            <Button
              type="button"
              variant="outline"
              size="icon-sm"
              icon="zoom_in"
              aria-label="Zoom in"
              onClick={() => changeAppZoom(zoom + APP_ZOOM_STEP)}
              disabled={zoom >= APP_ZOOM_MAX}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
