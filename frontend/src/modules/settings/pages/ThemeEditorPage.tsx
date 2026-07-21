import { useEffect, useState } from 'react';
import { Button } from '@ui/button';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@ui/card';
import { Input } from '@ui/input';
import { Field, FieldDescription, FieldLabel } from '@ui/field';
import { FileInput } from '@ui/file-input';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { applyAppTheme } from '@/lib/app-theme';
import {
  applyCustomThemeTokens,
  customThemeIdForName,
  DEFAULT_CUSTOM_THEME,
  isCustomThemeId,
  normalizeCustomThemeTokens,
  readableTextFor,
  type CustomThemeTokens,
} from '@/theme/custom-theme';
import { extractPalette, fileToBase64, localThemeFromPalette } from '@/theme/palette';
import { themeService } from '@services/theme.service';

// Color fields in the required group.
const REQUIRED_COLOR_FIELDS: Array<[keyof CustomThemeTokens, string]> = [
  ['background', 'Background'],
  ['surface', 'Surface'],
  ['surfaceAlt', 'Surface Alt'],
  ['text', 'Text'],
  ['mutedText', 'Muted Text'],
  ['border', 'Border'],
  ['accent', 'Accent'],
];

// Optional color fields shown in the expanded panel.
const OPTIONAL_COLOR_FIELDS: Array<[keyof CustomThemeTokens, string]> = [
  ['sidebar', 'Sidebar'],
  ['sidebarHover', 'Sidebar Hover'],
  ['sidebarActive', 'Sidebar Active'],
  ['sidebarActiveText', 'Sidebar Active Text'],
  ['chatSurface', 'Chat Surface'],
  ['chatComposer', 'Chat Composer'],
  ['userBubble', 'User Bubble'],
  ['userBubbleText', 'User Bubble Text'],
  ['assistantBubble', 'Assistant Bubble'],
  ['codeSurface', 'Code Surface'],
];

export interface ThemeEditorPageProps {
  /** When set, the editor opens in edit mode for this theme id. */
  editId?: string;
  /** Tokens to pre-fill the editor (for duplicate mode). */
  prefillTokens?: CustomThemeTokens;
  /** Called after a successful save (passes the new/updated id). */
  onSaved: (id: string, themes: Record<string, CustomThemeTokens>) => void;
  /** Called when the user cancels without saving. */
  onCancel: () => void;
}

/** Full-page custom-theme editor: create, edit and duplicate custom themes. */
export function ThemeEditorPage({
  editId,
  prefillTokens,
  onSaved,
  onCancel,
}: ThemeEditorPageProps) {
  const [tokens, setTokens] = useState<CustomThemeTokens>(() =>
    normalizeCustomThemeTokens(prefillTokens ?? {}),
  );
  const [showOptional, setShowOptional] = useState(false);
  const [status, setStatus] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [refPreview, setRefPreview] = useState<string | null>(null);
  const [analyzing, setAnalyzing] = useState(false);

  // Pre-fill editor when editId is provided (load from config).
  useEffect(() => {
    if (!editId || prefillTokens) return;
    let mounted = true;
    void themeService.getCustomThemes().then((themes) => {
      if (!mounted) return;
      const t = themes[editId];
      if (t) setTokens(normalizeCustomThemeTokens(t));
    });
    return () => { mounted = false; };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editId]);

  async function analyzeReference(file: File) {
    setAnalyzing(true);
    setStatus('Analyzing reference…');
    try {
      const { dataUri } = await fileToBase64(file);
      setRefPreview(dataUri);
      const palette = await extractPalette(dataUri);
      const local = localThemeFromPalette(palette);
      // Optional AI refine — only hex strings sent to the LLM (Decision 3).
      let suggestion: CustomThemeTokens = local;
      const aiResult = await themeService.suggestFromPalette(palette);
      if (aiResult.tokens) {
        suggestion = aiResult.tokens;
        setStatus('AI suggestion applied. Review and save.');
      } else {
        setStatus(`Local suggestion applied (${aiResult.error ?? 'AI unavailable'}).`);
      }
      setTokens(suggestion);
      applyCustomThemeTokens(suggestion, customThemeIdForName(suggestion.name));
    } catch (err) {
      setStatus(err instanceof Error ? err.message : 'Failed to analyze reference');
    } finally {
      setAnalyzing(false);
    }
  }

  function update<K extends keyof CustomThemeTokens>(key: K, value: string) {
    setTokens((prev) => ({ ...prev, [key]: value }));
  }

  function previewLive() {
    const normalized = normalizeCustomThemeTokens(tokens);
    const id = editId ?? customThemeIdForName(normalized.name);
    applyCustomThemeTokens(normalized, id);
    setStatus('Preview applied — not yet saved.');
  }

  async function save() {
    setSaving(true);
    setStatus(null);
    try {
      const normalized = normalizeCustomThemeTokens(tokens);
      const id = editId && isCustomThemeId(editId)
        ? editId
        : customThemeIdForName(normalized.name);

      const err = await themeService.saveCustomTheme(id, normalized);
      if (err) {
        setStatus(`Error: ${err}`);
        return;
      }
      // Apply + persist as active theme.
      applyCustomThemeTokens(normalized, id);
      applyAppTheme(id);
      await themeService.saveActiveTheme(id);
      setStatus('Saved.');
      // Reload updated library for caller.
      const themes = await themeService.getCustomThemes();
      onSaved(id, themes);
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!editId) return;
    setSaving(true);
    try {
      const err = await themeService.deleteCustomTheme(editId);
      if (err) { setStatus(`Error: ${err}`); return; }
      // Revert to midnight if this was the active theme.
      const current = await themeService.getActiveTheme();
      if (current === editId) {
        applyAppTheme('midnight');
        await themeService.saveActiveTheme('midnight');
      }
      const themes = await themeService.getCustomThemes();
      onSaved(editId, themes);
    } finally {
      setSaving(false);
    }
  }

  // Live contrast hint for the active background color.
  const bgContrast = readableTextFor(tokens.background);

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(260px,380px)]">
      {/* Editor column */}
      <Card className="rounded-lg">
        <CardHeader>
          <CardTitle>{editId ? 'Edit custom theme' : 'New custom theme'}</CardTitle>
          <CardDescription>
            Required colors define the workspace palette. Optional slots override
            specific surfaces — leave blank to inherit from required colors.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-5">
          {/* Reference image upload */}
          <div>
            <div className="flex items-center gap-2">
              <label
                htmlFor="theme-ref-upload"
                className={[
                  'flex cursor-pointer items-center gap-1.5 rounded-md border border-border bg-secondary px-3 py-1.5 text-xs font-medium hover:bg-secondary/80',
                  analyzing ? 'cursor-not-allowed opacity-50' : '',
                ]
                  .filter(Boolean)
                  .join(' ')}
              >
                <span className="material-symbols-outlined text-[14px]" aria-hidden>upload</span>
                {analyzing ? 'Analyzing…' : 'Theme from image'}
              </label>
              <FileInput
                id="theme-ref-upload"
                className="sr-only"
                accept="image/png,image/jpeg,image/webp"
                disabled={analyzing}
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) void analyzeReference(file);
                  e.currentTarget.value = '';
                }}
              />
              <span className="text-xs text-muted-foreground">
                PNG/JPEG/WebP — palette extracted locally, only hex strings sent to AI.
              </span>
            </div>
            {refPreview && (
              <div className="mt-2 overflow-hidden rounded-lg border border-border">
                <img src={refPreview} alt="Reference" className="max-h-36 w-full object-cover" />
              </div>
            )}
          </div>

          {/* Name */}
          <Field>
            <FieldLabel>Name</FieldLabel>
            <Input
              value={tokens.name}
              onChange={(e) => update('name', e.target.value)}
              placeholder="My Theme"
              maxLength={60}
            />
          </Field>

          {/* Required color tokens */}
          <div>
            <p className="mb-3 text-xs font-semibold uppercase tracking-[0.08em] text-muted-foreground">
              Required colors
            </p>
            <div className="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-3">
              {REQUIRED_COLOR_FIELDS.map(([key, label]) => (
                <Field key={key}>
                  <FieldLabel>{label}</FieldLabel>
                  <div className="flex items-center gap-2">
                    <input
                      type="color"
                      value={tokens[key] ?? DEFAULT_CUSTOM_THEME[key as keyof typeof DEFAULT_CUSTOM_THEME] ?? '#000000'}
                      onChange={(e) => update(key, e.target.value)}
                      className="h-8 w-10 cursor-pointer rounded border border-border bg-transparent p-0.5"
                      aria-label={label}
                    />
                    <span className="font-mono text-xs text-muted-foreground">
                      {tokens[key] ?? ''}
                    </span>
                  </div>
                </Field>
              ))}
            </div>
          </div>

          {/* Optional color tokens */}
          <div>
            <button
              type="button"
              className="flex items-center gap-1 text-xs font-semibold uppercase tracking-[0.08em] text-muted-foreground hover:text-foreground"
              onClick={() => setShowOptional((s) => !s)}
              aria-expanded={showOptional}
            >
              <span className="material-symbols-outlined text-[14px]" aria-hidden="true">
                {showOptional ? 'expand_less' : 'expand_more'}
              </span>
              Optional overrides
            </button>
            {showOptional && (
              <div className="mt-3 grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-3">
                {OPTIONAL_COLOR_FIELDS.map(([key, label]) => (
                  <Field key={key}>
                    <FieldLabel>{label}</FieldLabel>
                    <div className="flex items-center gap-2">
                      <input
                        type="color"
                        value={tokens[key] ?? DEFAULT_CUSTOM_THEME[key as keyof typeof DEFAULT_CUSTOM_THEME] ?? '#000000'}
                        onChange={(e) => update(key, e.target.value)}
                        className="h-8 w-10 cursor-pointer rounded border border-border bg-transparent p-0.5"
                        aria-label={label}
                      />
                      <span className="font-mono text-xs text-muted-foreground">
                        {tokens[key] ?? '(inherit)'}
                      </span>
                    </div>
                  </Field>
                ))}
              </div>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-2 pt-1">
            <Button
              type="button"
              variant="outline"
              size="sm"
              icon="visibility"
              onClick={previewLive}
            >
              Preview
            </Button>
            <SaveCancelActions
              size="sm"
              saving={saving}
              onSave={() => { void save(); }}
              onCancel={onCancel}
              status={status ? (
                <span className="text-xs text-muted-foreground">{status}</span>
              ) : null}
            />
            {editId && (
              <Button
                type="button"
                variant="destructive"
                size="sm"
                icon="delete"
                disabled={saving}
                onClick={() => { void handleDelete(); }}
              >
                Delete
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Live preview column */}
      <Card className="h-fit rounded-lg">
        <CardHeader>
          <CardTitle>Preview</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          {/* Palette swatches */}
          <div className="grid grid-cols-4 gap-1 rounded-md overflow-hidden border border-border">
            {REQUIRED_COLOR_FIELDS.slice(0, 7).map(([key, label]) => (
              <div
                key={key}
                className="h-8"
                style={{ backgroundColor: tokens[key] ?? '#000' }}
                title={`${label}: ${tokens[key]}`}
              />
            ))}
          </div>

          {/* Background / text contrast preview */}
          <div
            className="rounded-md border p-3 text-sm"
            style={{
              backgroundColor: tokens.background,
              borderColor: tokens.border,
              color: tokens.text,
            }}
          >
            <div className="font-semibold">{tokens.name}</div>
            <div style={{ color: tokens.mutedText }} className="mt-1 text-xs">
              Surface text and muted copy
            </div>
            <div
              className="mt-2 inline-block rounded px-2 py-1 text-xs font-semibold"
              style={{ backgroundColor: tokens.accent, color: bgContrast }}
            >
              Accent button
            </div>
          </div>

          {/* Contrast hint */}
          <FieldDescription>
            Auto text color for accent:{' '}
            <span
              className="inline-block h-3 w-3 rounded-full border border-border align-middle"
              style={{ backgroundColor: bgContrast }}
            />{' '}
            <code className="text-xs">{bgContrast}</code>
          </FieldDescription>
        </CardContent>
      </Card>
    </div>
  );
}
