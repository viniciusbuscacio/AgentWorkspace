import { useCallback, useEffect, useMemo, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@ui/card';
import { Input } from '@ui/input';
import {
  permissionsService,
  type SandboxMode,
  type SandboxSettings,
} from '@services/permissions.service';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import {
  BUILTIN_BADGE_LABEL,
  BUILTIN_HINT,
  FOLDER_PLACEHOLDER,
  MODES,
  PERM_TITLE,
  SAVE_CANCELED_MSG,
  SAVE_FOOTER,
  SELF_DEV_BANNER,
} from './permissions.constants';

interface EditableState {
  mode: SandboxMode;
  allowedFolders: string[];
}

const EMPTY: EditableState = { mode: 'permit_list', allowedFolders: [] };

function sameList(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((item, index) => item === b[index]);
}

export function PermissionsPage() {
  const [settings, setSettings] = useState<SandboxSettings | null>(null);
  const [draft, setDraft] = useState<EditableState>(EMPTY);
  const [snapshot, setSnapshot] = useState<EditableState>(EMPTY);
  const [newPath, setNewPath] = useState('');
  const [saving, setSaving] = useState(false);

  const dirty = useMemo(
    () => draft.mode !== snapshot.mode || !sameList(draft.allowedFolders, snapshot.allowedFolders),
    [draft, snapshot],
  );

  const selectedMode = MODES.find((m) => m.value === draft.mode) ?? MODES[1];

  const refresh = useCallback(async () => {
    const result = await permissionsService.load();
    if (!result.success || !result.settings) {
      setMessage(result.error ?? 'Could not load permissions.');
      return;
    }
    const loaded: EditableState = {
      mode: (result.settings.mode as SandboxMode) ?? 'permit_list',
      allowedFolders: result.settings.allowedFolders ?? [],
    };
    setSettings(result.settings);
    setDraft(loaded);
    setSnapshot(loaded);
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  function selectMode(mode: SandboxMode) {
    setDraft((current) => ({ ...current, mode }));
    setNewPath('');
    setMessage('');
  }

  function addPath() {
    const path = newPath.trim();
    if (!path) return;
    if (draft.allowedFolders.includes(path)) {
      setNewPath('');
      return;
    }
    setDraft((current) => ({ ...current, allowedFolders: [...current.allowedFolders, path] }));
    setNewPath('');
    setMessage('');
  }

  function removePath(path: string) {
    setDraft((current) => ({
      ...current,
      allowedFolders: current.allowedFolders.filter((item) => item !== path),
    }));
    setMessage('');
  }

  async function browseFolder() {
    try {
      const path = await permissionsService.selectFolder();
      if (path) setNewPath(path);
    } catch {
      // The native folder picker binding is web-denylisted: the bridge rejects
      // instead of returning a result. Type the path manually there.
      setMessage('The folder picker is only available in the desktop app — type the path instead.');
    }
  }

  async function save() {
    setSaving(true);
    setMessage('');
    try {
      const result = await permissionsService.save(draft.mode, draft.allowedFolders);
      if (result.canceled) {
        setMessage(SAVE_CANCELED_MSG);
        return;
      }
      if (!result.success) {
        setMessage(result.error ?? 'Could not save permissions.');
        return;
      }
      setMessage('Permissions saved.');
      await refresh();
    } catch (err: unknown) {
      // Saving permissions requires the native confirmation dialog, which the
      // web bridge denies — the rejection lands here instead of a result.
      setMessage(err instanceof Error ? err.message : 'Permissions can only be changed in the desktop app.');
    } finally {
      setSaving(false);
    }
  }

  function cancelChanges() {
    setDraft(snapshot);
    setNewPath('');
    setMessage('');
  }

  // The Balanced list shows the always-included rows (workspace + built-in
  // allows) as non-removable, then the user's folders with a remove control.
  const builtinRows = [settings?.workspaceRoot ?? '', ...(settings?.builtinAllowed ?? [])].filter(
    (path) => path.trim() !== '',
  );

  return (
    <div className="flex flex-col gap-6">
      {settings?.selfDev && (
        <div className="rounded-lg border border-border bg-secondary px-3 py-2 text-sm">
          {SELF_DEV_BANNER(draft.mode)}
        </div>
      )}

      <Card className="rounded-lg">
        <CardHeader>
          <CardTitle>{PERM_TITLE}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {/* Mode radios — three, one per line. */}
          <div role="radiogroup" aria-label={PERM_TITLE} className="flex flex-col">
            {MODES.map((mode) => (
              <label
                key={mode.value}
                className="flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-sm hover:bg-secondary"
              >
                <input
                  type="radio"
                  name="access-mode"
                  value={mode.value}
                  checked={draft.mode === mode.value}
                  onChange={() => selectMode(mode.value as SandboxMode)}
                  className="accent-primary"
                />
                <span className={draft.mode === mode.value ? 'font-semibold' : ''}>{mode.label}</span>
              </label>
            ))}
          </div>

          {/* Panel: what the selected mode means (and, for Balanced, the folders). */}
          <div className="flex flex-col gap-3 rounded-lg border border-border px-4 py-3">
            <div className="flex items-baseline justify-between gap-2">
              <span className="text-sm font-semibold">{selectedMode.label}</span>
              <span className="font-mono text-[10px] text-muted-foreground/60">{selectedMode.subtitle}</span>
            </div>
            <p className="text-sm text-muted-foreground">{selectedMode.description}</p>

            {selectedMode.editable && (
              <>
                <div className="flex flex-col divide-y divide-border rounded-md border border-border">
                  {builtinRows.map((path) => (
                    <div
                      key={path}
                      className="flex items-center justify-between gap-2 px-3 py-2"
                      title={BUILTIN_HINT}
                    >
                      <span className="min-w-0 truncate font-mono text-xs opacity-60">{path}</span>
                      <span className="shrink-0 rounded bg-muted px-1.5 text-[10px] uppercase text-muted-foreground">
                        {BUILTIN_BADGE_LABEL}
                      </span>
                    </div>
                  ))}
                  {draft.allowedFolders.map((path) => (
                    <div key={path} className="flex items-center justify-between gap-2 px-3 py-2">
                      <span className="min-w-0 truncate font-mono text-xs">{path}</span>
                      <button
                        type="button"
                        className="shrink-0 rounded-sm opacity-60 hover:opacity-100 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                        onClick={() => removePath(path)}
                        aria-label={`Remove ${path}`}
                      >
                        <span className="material-symbols-outlined text-[14px]" aria-hidden="true">
                          close
                        </span>
                      </button>
                    </div>
                  ))}
                </div>

                <div className="flex gap-2">
                  <Input
                    value={newPath}
                    onChange={(e) => setNewPath(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') addPath();
                    }}
                    placeholder={FOLDER_PLACEHOLDER}
                    aria-label="New path"
                  />
                  <Button variant="outline" icon="folder_open" onClick={browseFolder}>
                    Browse
                  </Button>
                  <Button icon="add" onClick={addPath} disabled={!newPath.trim()}>
                    Add
                  </Button>
                </div>
              </>
            )}
          </div>

          <SaveCancelActions
            onSave={() => void save()}
            onCancel={cancelChanges}
            saving={saving}
            disabled={!dirty}
          />
          <p className="text-xs text-muted-foreground">{SAVE_FOOTER}</p>
        </CardContent>
      </Card>
    </div>
  );
}
