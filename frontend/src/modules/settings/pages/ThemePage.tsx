import { useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@ui/button';
import { Card, CardContent } from '@ui/card';
import { onAwEvent } from '@services/events';
import { uiService } from '@services/ui.service';
import { applyAppTheme, getSavedAppTheme } from '@/lib/app-theme';
import { applyCustomThemeTokens, clearCustomThemeProperties, isCustomThemeId, normalizeCustomThemeTokens, type CustomThemeTokens } from '@/theme/custom-theme';
import { themeService } from '@services/theme.service';
import { ICON_SIZE_MAX, ICON_SIZE_MIN, type OrderMode } from '@modules/home/home-modules';
import { AppGridToolbar } from '@patterns/AppGridToolbar';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { ThemeEditorPage } from './ThemeEditorPage';

const THEME_ICON_SIZE_KEY = 'theme-icon-size';
const THEME_ORDER_MODE_KEY = 'theme-order-mode';
const THEME_ICON_SIZE_DEFAULT = 160;

function getInitialThemeIconSize(): number {
  const saved = Number(localStorage.getItem(THEME_ICON_SIZE_KEY));
  if (!Number.isNaN(saved) && saved >= ICON_SIZE_MIN && saved <= ICON_SIZE_MAX) return saved;
  return THEME_ICON_SIZE_DEFAULT;
}

function getInitialThemeOrderMode(): OrderMode {
  const saved = localStorage.getItem(THEME_ORDER_MODE_KEY);
  return saved === 'most-used' || saved === 'manual' ? saved : 'default';
}

function persistThemeIconSize(size: number): void {
  try { localStorage.setItem(THEME_ICON_SIZE_KEY, String(size)); } catch { /* ignore */ }
}

function persistThemeOrderMode(mode: OrderMode): void {
  try { localStorage.setItem(THEME_ORDER_MODE_KEY, mode); } catch { /* ignore */ }
}

// Keep in sync with awThemes in internal/infrastructure/tools/aw_app.go.
const BUILTIN_THEMES: Array<{ id: string; label: string; bg: string }> = [
  { id: 'midnight', label: 'Midnight', bg: '#171717' },
  { id: 'light', label: 'Light', bg: '#f5f5f5' },
  { id: 'espresso', label: 'Espresso', bg: '#141210' },
  { id: 'violet', label: 'Violet', bg: '#2d2640' },
  { id: 'forest', label: 'Forest', bg: '#1f3320' },
  { id: 'ocean', label: 'Ocean', bg: '#17314f' },
  { id: 'rose', label: 'Rose', bg: '#351d29' },
];

type EditorMode = 'create' | 'edit' | 'duplicate';
interface EditorState {
  mode: EditorMode;
  id?: string;
  prefill?: CustomThemeTokens;
}

export interface ThemePageProps {
  /** Called when the editor opens/closes so the parent can update breadcrumb. */
  onDetailTitleChange?: (title: string | null) => void;
  /** Navigate back to the Settings grid (Cancel). */
  onBack?: () => void;
}

// A previewed (unsaved) theme reverts automatically after this long.
export const THEME_PREVIEW_MS = 30_000;

export function ThemePage({ onDetailTitleChange, onBack }: ThemePageProps = {}) {
  // theme = what is APPLIED right now (may be a preview); savedTheme = what is
  // persisted. Clicking a card only previews; Save persists; leaving without
  // saving (Cancel, unmount or the 30s timer) reverts to savedTheme.
  const [theme, setTheme] = useState(getSavedAppTheme);
  const [savedTheme, setSavedTheme] = useState(getSavedAppTheme);
  const previewTimerRef = useRef<number | null>(null);
  const [customThemes, setCustomThemes] = useState<Record<string, CustomThemeTokens>>({});
  const [editor, setEditor] = useState<EditorState | null>(null);
  // Apps-screen controls (Decision 3 — polish-wave-3-spec)
  const [search, setSearch] = useState('');
  const [orderMode, setOrderMode] = useState<OrderMode>(() => getInitialThemeOrderMode());
  const [iconSize, setIconSize] = useState<number>(() => getInitialThemeIconSize());

  function changeIconSize(size: number) {
    setIconSize(size);
    persistThemeIconSize(size);
  }

  function changeOrder(mode: OrderMode) {
    setOrderMode(mode);
    persistThemeOrderMode(mode);
  }

  // Filtered and ordered themes.
  const filteredBuiltin = useMemo(() => {
    const q = search.trim().toLowerCase();
    const sorted = orderMode === 'most-used'
      // most-used: active theme first, then original order
      ? [...BUILTIN_THEMES].sort((a) => (a.id === theme ? -1 : 0))
      : BUILTIN_THEMES;
    return q ? sorted.filter((t) => t.label.toLowerCase().includes(q)) : sorted;
  }, [search, orderMode, theme]);

  const filteredCustomEntries = useMemo(() => {
    const q = search.trim().toLowerCase();
    const entries = Object.entries(customThemes);
    const sorted = orderMode === 'most-used'
      ? [...entries].sort(([id]) => (id === theme ? -1 : 0))
      : [...entries].sort(([, a], [, b]) => a.name.localeCompare(b.name));
    return q ? sorted.filter(([, t]) => t.name.toLowerCase().includes(q)) : sorted;
  }, [search, orderMode, customThemes, theme]);

  // Card grid column width driven by iconSize slider.
  const gridStyle = { gridTemplateColumns: `repeat(auto-fill, minmax(${Math.round(iconSize * 0.65)}px, 1fr))` };

  // Load active theme + custom library from config.json.
  useEffect(() => {
    let mounted = true;
    Promise.all([themeService.getActiveTheme(), themeService.getCustomThemes()]).then(
      ([activeId, themes]) => {
        if (!mounted) return;
        setCustomThemes(themes);
        // Reconcile: if config says custom theme, apply its tokens.
        if (isCustomThemeId(activeId) && themes[activeId]) {
          applyCustomThemeTokens(themes[activeId], activeId);
          applyAppTheme(activeId);
          setTheme(activeId);
          setSavedTheme(activeId);
        } else if (activeId) {
          if (activeId !== getSavedAppTheme()) applyAppTheme(activeId);
          setTheme(activeId);
          setSavedTheme(activeId);
        }
      },
    );
    return () => { mounted = false; };
  }, []);

  // Stay in sync when the agent switches the theme.
  useEffect(
    () =>
      onAwEvent('ui:set-theme', ({ theme: t }) => {
        setTheme(t);
        setSavedTheme(t);
        if (!isCustomThemeId(t)) {
          clearCustomThemeProperties();
          document.documentElement.setAttribute('data-theme', t);
        }
      }),
    [],
  );

  // Report active theme to the backend state mirror.
  useEffect(() => {
    void uiService.reportUiState({ theme });
  }, [theme]);

  // Sync breadcrumb title with editor state.
  useEffect(() => {
    if (!editor) {
      onDetailTitleChange?.(null);
      return;
    }
    if (editor.mode === 'create') onDetailTitleChange?.('New Theme');
    else if (editor.mode === 'duplicate') onDetailTitleChange?.('Duplicate Theme');
    else {
      const name = editor.id ? customThemes[editor.id]?.name : undefined;
      onDetailTitleChange?.(name ? `Edit: ${name}` : 'Edit Theme');
    }
  }, [editor, customThemes, onDetailTitleChange]);

  function applyThemeVisual(id: string, themes: Record<string, CustomThemeTokens>) {
    if (isCustomThemeId(id) && themes[id]) {
      applyCustomThemeTokens(themes[id], id);
      applyAppTheme(id);
    } else {
      clearCustomThemeProperties();
      applyAppTheme(id);
      document.documentElement.setAttribute('data-theme', id);
    }
  }

  function clearPreviewTimer() {
    if (previewTimerRef.current !== null) {
      window.clearTimeout(previewTimerRef.current);
      previewTimerRef.current = null;
    }
  }

  const savedThemeRef = useRef(savedTheme);
  const themeRef = useRef(theme);
  const customThemesRef = useRef(customThemes);
  useEffect(() => { savedThemeRef.current = savedTheme; }, [savedTheme]);
  useEffect(() => { themeRef.current = theme; }, [theme]);
  useEffect(() => { customThemesRef.current = customThemes; }, [customThemes]);

  function revertPreview() {
    clearPreviewTimer();
    if (themeRef.current === savedThemeRef.current) return;
    applyThemeVisual(savedThemeRef.current, customThemesRef.current);
    setTheme(savedThemeRef.current);
  }

  // Leaving the page without saving reverts the preview.
  useEffect(() => () => {
    clearPreviewTimer();
    if (themeRef.current !== savedThemeRef.current) {
      applyThemeVisual(savedThemeRef.current, customThemesRef.current);
    }
  }, []);

  function previewTheme(id: string) {
    applyThemeVisual(id, customThemes);
    setTheme(id);
    clearPreviewTimer();
    if (id !== savedTheme) {
      previewTimerRef.current = window.setTimeout(revertPreview, THEME_PREVIEW_MS);
    }
  }

  async function saveTheme() {
    clearPreviewTimer();
    await themeService.saveActiveTheme(theme);
    setSavedTheme(theme);
    toast('Theme saved.');
  }

  function cancelTheme() {
    revertPreview();
    onBack?.();
  }

  function openCreate() {
    setEditor({ mode: 'create' });
  }

  function openEdit(id: string) {
    setEditor({ mode: 'edit', id });
  }

  function openDuplicate(id: string) {
    const source = customThemes[id];
    if (!source) return;
    const copy = normalizeCustomThemeTokens({
      ...source,
      name: `${source.name} (copy)`,
    });
    setEditor({ mode: 'duplicate', prefill: copy });
  }

  function handleEditorSaved(id: string, themes: Record<string, CustomThemeTokens>) {
    setCustomThemes(themes);
    setTheme(id);
    setSavedTheme(id);
    setEditor(null);
  }

  function handleEditorCancel() {
    // Restore the last known saved theme.
    applyThemeVisual(savedTheme, customThemes);
    setTheme(savedTheme);
    setEditor(null);
  }

  if (editor) {
    return (
      <ThemeEditorPage
        editId={editor.mode === 'edit' ? editor.id : undefined}
        prefillTokens={editor.prefill}
        onSaved={handleEditorSaved}
        onCancel={handleEditorCancel}
      />
    );
  }

  return (
    <div className="grid gap-4">
      {/* Apps-screen controls: search, order-by, card-size (Decision 3) */}
      <AppGridToolbar
        search={search}
        onSearchChange={setSearch}
        searchPlaceholder="Search themes..."
        orderMode={orderMode}
        onOrderChange={changeOrder}
        iconSize={iconSize}
        onIconSizeChange={changeIconSize}
        sizeLabel="Theme card size"
      />

      {/* Built-in themes */}
      {filteredBuiltin.length > 0 && (
        <section data-testid="theme-builtin-section">
          <h2 className="mb-3 text-xs font-semibold uppercase tracking-[0.08em] text-muted-foreground">
            Built-in themes
          </h2>
          <div className="grid gap-3" style={gridStyle}>
            {filteredBuiltin.map((t) => (
              <button
                key={t.id}
                type="button"
                onClick={() => previewTheme(t.id)}
                className={[
                  'rounded-lg border p-3 text-left transition-colors',
                  'hover:border-primary/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50',
                  theme === t.id ? 'border-primary bg-primary/10 ring-1 ring-primary/40' : 'border-border bg-card',
                ].join(' ')}
                aria-pressed={theme === t.id}
              >
                <span className="flex items-center gap-2">
                  <span
                    className="inline-block h-5 w-5 shrink-0 rounded-full border border-border"
                    style={{ backgroundColor: t.bg }}
                  />
                  <span className="truncate capitalize text-sm">{t.label}</span>
                </span>
              </button>
            ))}
          </div>
        </section>
      )}

      {/* Custom themes */}
      <section data-testid="theme-custom-section">
        <div className="mb-3 flex items-center justify-between gap-3">
          <h2 className="text-xs font-semibold uppercase tracking-[0.08em] text-muted-foreground">
            Custom themes ({Object.keys(customThemes).length})
          </h2>
          <Button
            type="button"
            size="sm"
            variant="outline"
            icon="add"
            onClick={openCreate}
          >
            New theme
          </Button>
        </div>

        {Object.keys(customThemes).length === 0 ? (
          <p className="rounded-lg border border-dashed border-border p-4 text-sm text-muted-foreground">
            No custom themes yet. Click <strong>New theme</strong> to create one.
          </p>
        ) : (
          <div className="grid gap-3" style={gridStyle}>
            {filteredCustomEntries.map(([id, tokens]) => (
              <CustomThemeCard
                key={id}
                id={id}
                tokens={tokens}
                active={theme === id}
                onSelect={() => previewTheme(id)}
                onEdit={() => openEdit(id)}
                onDuplicate={() => openDuplicate(id)}
              />
            ))}
          </div>
        )}
      </section>

      {/* Preview model: a clicked theme is applied for up to 30s as a preview;
          only Save persists it. Cancel reverts and returns to Settings. */}
      <div className="flex flex-wrap items-center gap-2">
        <SaveCancelActions
          onSave={() => void saveTheme()}
          onCancel={cancelTheme}
          disabled={theme === savedTheme}
          status={theme !== savedTheme ? <span className="text-xs text-muted-foreground">Previewing — save to keep, or it reverts in 30s.</span> : null}
        />
      </div>
    </div>
  );
}

// ── Custom theme card ─────────────────────────────────────────────────────────

interface CustomThemeCardProps {
  id: string;
  tokens: CustomThemeTokens;
  active: boolean;
  onSelect: () => void;
  onEdit: () => void;
  onDuplicate: () => void;
}

function CustomThemeCard({ id, tokens, active, onSelect, onEdit, onDuplicate }: CustomThemeCardProps) {
  const slug = id.replace('custom:', '');
  return (
    <Card
      className={[
        'overflow-hidden rounded-lg py-0 shadow-none transition-colors',
        'hover:border-primary/60',
        active ? 'border-primary ring-1 ring-primary/50' : '',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      {/* Palette preview strip */}
      <button
        type="button"
        className="flex h-12 w-full cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
        onClick={onSelect}
        aria-pressed={active}
        aria-label={`Select theme ${tokens.name}`}
        title={`Select ${tokens.name}`}
      >
        {(['background', 'surface', 'accent', 'text'] as const).map((key) => (
          <div
            key={key}
            className="flex-1"
            style={{ backgroundColor: tokens[key] }}
          />
        ))}
      </button>

      <CardContent className="flex items-center justify-between gap-1 p-2">
        <div className="min-w-0">
          <div className="truncate text-sm font-medium">{tokens.name}</div>
          <div className="truncate text-xs text-muted-foreground">{slug}</div>
        </div>
        <div className="flex shrink-0 gap-0.5">
          {active && (
            <span className="material-symbols-outlined text-[16px] text-primary" aria-hidden>check</span>
          )}
          <button
            type="button"
            className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground hover:bg-secondary hover:text-foreground"
            onClick={onDuplicate}
            title="Duplicate theme"
            aria-label={`Duplicate theme ${tokens.name}`}
          >
            <span className="material-symbols-outlined text-[14px]" aria-hidden>content_copy</span>
          </button>
          <button
            type="button"
            className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground hover:bg-secondary hover:text-foreground"
            onClick={onEdit}
            title="Edit theme"
            aria-label={`Edit theme ${tokens.name}`}
          >
            <span className="material-symbols-outlined text-[14px]" aria-hidden>edit</span>
          </button>
        </div>
      </CardContent>
    </Card>
  );
}
