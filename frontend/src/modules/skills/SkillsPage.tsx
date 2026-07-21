import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { notify as setMessage } from '@/lib/notify';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  Button,
  Switch,
  Textarea,
  ZoomSafeSelect,
} from '@ui/index';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import type { LeaveGuard } from '@modules/module-contract';
import { skillsService, type SkillFileDraft, type SkillView } from '@services/skills.service';
import { writeLastSkillId } from './skills-nav-storage';

const DEFAULT_SKILL_TEMPLATE = `---
name: my-skill
description: Describe when the agent should use this skill.
---

# Skill: My Skill

Use this skill when...

## Procedure

1. ...
`;

// v1 skill id rule (matches application.skillIDPattern); backend stays the
// source of truth, this is cheap pre-validation.
const SKILL_ID_PATTERN = /^[a-z0-9][a-z0-9-]*$/;

const CUSTOMIZED_NOTE =
  'Edits to a builtin skill mark it customized. Use Reset on the Skills list to restore the bundled version.';
const CREATE_NOTE = 'Create a skill by editing its SKILL.md. The name and description come from frontmatter.';

type SkillEditorMode = 'edit' | 'create';

interface SkillEditorState {
  mode: SkillEditorMode;
  id: string;
  title: string;
  note: string;
  files: SkillFileDraft[];
  original: string;
}

interface SkillsPageProps {
  // onSkillDetailTitleChange feeds the Settings breadcrumb: the editor title is
  // owned here and surfaced as `Settings › Skills › <title>`. null = list view.
  onSkillDetailTitleChange?: (title: string | null) => void;
  // listRequest increments when the `Skills` breadcrumb is clicked while an
  // editor is open, asking the page to return to the list.
  listRequest?: number;
  // onRegisterLeaveGuard hands the parents (SettingsModule → AppShell) a guard
  // that intercepts any navigation/close while the editor has unsaved edits.
  onRegisterLeaveGuard?: (guard: LeaveGuard | null) => void;
  // initialSkillId reopens this skill's editor once on mount, so returning to
  // Settings after visiting a chat restores Settings › Skills › <skill>. Only
  // set on a restore (not on manual navigation to the Skills list).
  initialSkillId?: string;
}

// parseFrontmatterName extracts the `name:` value from a SKILL.md draft so the
// create flow can use it as the explicit skill id.
function parseFrontmatterName(content: string): string {
  const match = content.match(/^---\s*\n([\s\S]*?)\n---/);
  if (!match) return '';
  const line = match[1].split('\n').find((l) => /^\s*name\s*:/.test(l));
  if (!line) return '';
  return line.replace(/^\s*name\s*:/, '').trim().replace(/^["']|["']$/g, '');
}

function Pill({ children, tone = 'muted' }: { children: ReactNode; tone?: 'muted' | 'primary' | 'warning' }) {
  const toneClass =
    tone === 'primary'
      ? 'border-primary text-primary'
      : tone === 'warning'
        ? 'border-warning text-warning'
        : 'border-border text-muted-foreground';
  return (
    <span className={`inline-flex shrink-0 items-center rounded-full border px-2 py-0.5 text-xs font-medium ${toneClass}`}>
      {children}
    </span>
  );
}

type OriginFilter = 'all' | 'user' | 'builtin';

export function SkillsPage({ onSkillDetailTitleChange, listRequest = 0, onRegisterLeaveGuard, initialSkillId }: SkillsPageProps) {
  const [skills, setSkills] = useState<SkillView[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [editor, setEditor] = useState<SkillEditorState | null>(null);
  const [saving, setSaving] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<SkillView | null>(null);
  const [pendingExit, setPendingExit] = useState<(() => void) | null>(null);
  const [search, setSearch] = useState('');
  const [originFilter, setOriginFilter] = useState<OriginFilter>('all');

  // Latest editor in a ref so the registered guard reads fresh dirty state
  // without re-registering on every keystroke.
  const editorRef = useRef<SkillEditorState | null>(null);
  useEffect(() => {
    editorRef.current = editor;
  }, [editor]);

  // Remember which skill's editor is open (session-scoped) so returning to
  // Settings after a chat reopens it. Closing the editor / the create flow
  // clears it.
  useEffect(() => {
    writeLastSkillId(editor && editor.mode === 'edit' ? editor.id : null);
  }, [editor]);

  // tryLeave is the unsaved-changes guard handed up to SettingsModule/AppShell.
  // If the editor is dirty it parks the requested action behind a confirm
  // dialog; otherwise it runs immediately.
  const tryLeave = useCallback<LeaveGuard>((proceed) => {
    const e = editorRef.current;
    const dirty = !!e && JSON.stringify(e.files) !== e.original;
    if (dirty) {
      setPendingExit(() => proceed);
    } else {
      proceed();
    }
  }, []);

  const hasEditor = editor !== null;
  useEffect(() => {
    onRegisterLeaveGuard?.(hasEditor ? tryLeave : null);
    return () => onRegisterLeaveGuard?.(null);
  }, [hasEditor, onRegisterLeaveGuard, tryLeave]);

  // Search + origin filter combine with AND semantics, client-side over the
  // loaded list.
  const filtered = useMemo(() => {
    if (!skills) return [];
    const query = search.trim().toLowerCase();
    return skills.filter((s) => {
      if (originFilter === 'user' && s.origin !== 'user') return false;
      if (originFilter === 'builtin' && s.origin !== 'builtin') return false;
      if (!query) return true;
      return (
        s.id.toLowerCase().includes(query) ||
        (s.name ?? '').toLowerCase().includes(query) ||
        (s.description ?? '').toLowerCase().includes(query)
      );
    });
  }, [skills, search, originFilter]);

  const refresh = useCallback(async () => {
    const result = await skillsService.list();
    if (!result.success) {
      setMessage(result.error ?? 'Could not load skills.');
      setSkills([]);
      return;
    }
    setSkills(result.skills ?? []);
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // One-shot restore: when the parent reopens this page with a skill to restore
  // (returning from a chat), reopen that skill's editor once the list has loaded.
  const restoredRef = useRef(false);
  useEffect(() => {
    if (restoredRef.current) return;
    if (!initialSkillId) {
      restoredRef.current = true;
      return;
    }
    if (skills === null) return; // wait for the list before resolving the id
    restoredRef.current = true;
    const target = skills.find((s) => s.id === initialSkillId);
    if (target) void openEdit(target);
  }, [skills, initialSkillId]);

  // The breadcrumb title is owned by this page: mirror the editor state up, and
  // clear it on unmount so leaving the page never leaves a stale child title.
  useEffect(() => {
    onSkillDetailTitleChange?.(editor ? editor.title : null);
  }, [editor, onSkillDetailTitleChange]);
  useEffect(() => () => onSkillDetailTitleChange?.(null), [onSkillDetailTitleChange]);

  // Clicking the `Skills` breadcrumb asks to return to the list — guarded so a
  // dirty editor confirms first.
  useEffect(() => {
    if (listRequest > 0) tryLeave(() => setEditor(null));
  }, [listRequest, tryLeave]);

  const editorDirty = useMemo(() => {
    if (!editor) return false;
    return JSON.stringify(editor.files) !== editor.original;
  }, [editor]);

  async function runMutation(action: () => Promise<{ success: boolean; error?: string }>, okMsg: string) {
    setBusy(true);
    const result = await action();
    setBusy(false);
    if (!result.success) {
      setMessage(result.error ?? 'Action failed.');
      return false;
    }
    setMessage(okMsg);
    await refresh();
    return true;
  }

  async function toggle(skill: SkillView, enabled: boolean) {
    await runMutation(() => skillsService.setEnabled(skill.id, enabled), enabled ? `Enabled ${skill.id}.` : `Disabled ${skill.id}.`);
  }

  async function openEdit(skill: SkillView) {
    setMessage('');
    const result = await skillsService.detail(skill.id);
    if (!result.success || !result.skill) {
      setMessage(result.error ?? 'Could not open skill.');
      return;
    }
    const files: SkillFileDraft[] = (result.skill.files ?? []).map((f) => ({ path: f.path, content: f.content }));
    setEditor({
      mode: 'edit',
      id: skill.id,
      title: skill.name || skill.id,
      note: skill.origin === 'builtin' ? CUSTOMIZED_NOTE : '',
      files,
      original: JSON.stringify(files),
    });
  }

  function openCreate() {
    setMessage('');
    const files: SkillFileDraft[] = [{ path: 'SKILL.md', content: DEFAULT_SKILL_TEMPLATE }];
    setEditor({ mode: 'create', id: '', title: 'Add Skill', note: CREATE_NOTE, files, original: JSON.stringify(files) });
  }

  function closeEditor() {
    setEditor(null);
  }

  async function saveEditor() {
    if (!editor) return;

    if (editor.mode === 'create') {
      const skillMd = editor.files.find((f) => f.path === 'SKILL.md');
      const id = parseFrontmatterName(skillMd?.content ?? '');
      if (!SKILL_ID_PATTERN.test(id)) {
        setMessage(
          `The frontmatter "name" must be a valid skill id matching ${SKILL_ID_PATTERN.source} (e.g. my-skill). It becomes the skill id.`,
        );
        return;
      }
      setSaving(true);
      const result = await skillsService.create({ id, enabled: true, files: editor.files });
      setSaving(false);
      if (!result.success) {
        setMessage(result.error ?? 'Could not create skill.');
        return; // keep editor open so the user can fix it
      }
      setMessage(`Created ${id}.`);
      setEditor(null);
      await refresh();
      return;
    }

    setSaving(true);
    const result = await skillsService.save(editor.id, editor.files);
    setSaving(false);
    if (!result.success) {
      setMessage(result.error ?? 'Could not save skill.');
      return;
    }
    setMessage(`Saved ${editor.id}.`);
    setEditor(null);
    await refresh();
  }

  // The native file/folder pickers are web-denylisted: over the web bridge the
  // call rejects instead of returning a result, so both imports catch and
  // surface a desktop-only notice.
  async function doImportSkill() {
    setMessage('');
    let result;
    try {
      result = await skillsService.importSkill();
    } catch {
      setMessage('Importing skills needs the native file picker — use the desktop app.');
      return;
    }
    if (result === null) return; // canceled
    if (!result.success) {
      setMessage(`Import failed: ${result.error ?? 'unknown error'}`);
      return;
    }
    setMessage('Skill imported.');
    await refresh();
  }

  async function doImportFolder() {
    setMessage('');
    let result;
    try {
      result = await skillsService.importFolder();
    } catch {
      setMessage('Importing skills needs the native folder picker — use the desktop app.');
      return;
    }
    if (result === null) return; // canceled
    if (!result.success) {
      setMessage(`Import failed: ${result.error ?? 'unknown error'}`);
      return;
    }
    const { imported, skipped } = result.summary;
    let text = `Imported ${imported.length} skill${imported.length === 1 ? '' : 's'}.`;
    if (skipped.length > 0) {
      text += ` Skipped ${skipped.length}.`;
      text += `\nSkipped:\n${skipped.map((s) => `- ${s.id}: ${s.reason}`).join('\n')}`;
    }
    setMessage(text);
    await refresh();
  }

  async function doReset(skill: SkillView) {
    await runMutation(() => skillsService.reset(skill.id), `Reset ${skill.id} to its bundled version.`);
  }

  async function confirmDelete() {
    if (!pendingDelete) return;
    const target = pendingDelete;
    setPendingDelete(null);
    await runMutation(() => skillsService.delete(target.id), `Deleted ${target.id}.`);
  }

  if (editor) {
    return (
      <div className="flex min-h-[560px] flex-col gap-4">
        {editor.note && <p className="text-xs text-muted-foreground">{editor.note}</p>}

        <div className="flex flex-col gap-3">
          {editor.files.map((file, index) => (
            <div key={file.path} className="flex flex-col gap-1.5 rounded-lg border border-border bg-card p-3">
              <code className="text-xs text-muted-foreground">{file.path}</code>
              <Textarea
                className="min-h-[260px] resize-y font-mono text-xs"
                value={file.content}
                spellCheck={false}
                onChange={(e) => {
                  const next = editor.files.slice();
                  next[index] = { ...file, content: e.target.value };
                  setEditor({ ...editor, files: next });
                }}
              />
            </div>
          ))}
        </div>

        <SaveCancelActions
          onSave={() => void saveEditor()}
          onCancel={() => tryLeave(closeEditor)}
          saving={saving}
          disabled={editor.mode === 'edit' && !editorDirty}
          />

        <AlertDialog open={pendingExit !== null} onOpenChange={(open) => !open && setPendingExit(null)}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Discard unsaved skill changes?</AlertDialogTitle>
              <AlertDialogDescription>
                You changed this skill but have not saved it. If you leave now, those edits will be lost.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel onClick={() => setPendingExit(null)}>Keep editing</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => {
                  const proceed = pendingExit;
                  setPendingExit(null);
                  proceed?.();
                }}
              >
                Discard changes
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    );
  }

  return (
    <div className="flex min-h-[560px] flex-col gap-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs text-muted-foreground">
          Skills live in your encrypted vault. Only each skill’s trigger is in the prompt; the agent loads the full
          procedure on demand.
        </p>
        <div className="flex shrink-0 items-center gap-2">
          <Button variant="outline" size="sm" disabled={busy} onClick={openCreate}>
            Add Skill
          </Button>
          <Button variant="outline" size="sm" disabled={busy} onClick={() => void doImportSkill()}>
            Import Skill
          </Button>
          <Button variant="outline" size="sm" disabled={busy} onClick={() => void doImportFolder()}>
            Import Folder
          </Button>
        </div>
      </div>

      {skills !== null && skills.length > 0 && (
        <div className="home-toolbar">
          <div className="home-search">
            <span className="nav-icon home-search-icon material-symbols-outlined" aria-hidden="true">search</span>
            <input
              type="text"
              className="home-search-input"
              placeholder="Search skills..."
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              aria-label="Search skills"
            />
            {search && (
              <button
                type="button"
                className="chat-search-clear"
                onClick={() => setSearch('')}
                aria-label="Clear search"
              >
                <span className="material-symbols-outlined" aria-hidden="true">close</span>
              </button>
            )}
          </div>
          <div className="w-44">
            <ZoomSafeSelect
              value={originFilter}
              onValueChange={(value) => setOriginFilter(value as OriginFilter)}
              options={[
                { value: 'all', label: 'All Skills' },
                { value: 'user', label: 'User Skills' },
                { value: 'builtin', label: 'Built in Skills' },
              ]}
              aria-label="Filter skills by origin"
            />
          </div>
        </div>
      )}

      {skills === null ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : skills.length === 0 ? (
        <p className="text-sm text-muted-foreground">No skills yet.</p>
      ) : filtered.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {search.trim() ? `No skills match "${search.trim()}".` : 'No skills match your filters.'}
        </p>
      ) : (
        <div className="flex flex-col gap-2">
          {filtered.map((skill) => (
            <div
              key={skill.id}
              className={`flex items-start justify-between gap-3 rounded-lg border border-border bg-card p-3 ${skill.deleted ? 'opacity-60' : ''}`}
            >
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-sm font-semibold">{skill.name || skill.id}</span>
                  <code className="text-xs text-muted-foreground">{skill.id}</code>
                  <Pill tone={skill.origin === 'builtin' ? 'muted' : 'primary'}>{skill.origin}</Pill>
                  {skill.customized && <Pill tone="primary">customized</Pill>}
                  {skill.updateAvailable && <Pill tone="warning">update available</Pill>}
                  {skill.deleted && <Pill tone="warning">deleted</Pill>}
                </div>
                {skill.description && (
                  <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">{skill.description}</div>
                )}
              </div>

              <div className="flex shrink-0 items-center gap-2">
                <Switch
                  checked={skill.enabled}
                  disabled={busy || skill.deleted}
                  onCheckedChange={(checked) => void toggle(skill, checked)}
                  aria-label={`Toggle ${skill.id}`}
                />
                <Button variant="ghost" size="sm" disabled={busy} onClick={() => void openEdit(skill)}>
                  Edit
                </Button>
                {skill.origin === 'builtin' && (skill.customized || skill.deleted) && (
                  <Button variant="outline" size="sm" disabled={busy} onClick={() => void doReset(skill)}>
                    Reset
                  </Button>
                )}
                <Button variant="destructive" size="sm" disabled={busy || skill.deleted} onClick={() => setPendingDelete(skill)}>
                  Delete
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <AlertDialog open={pendingDelete !== null} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {pendingDelete?.id}?</AlertDialogTitle>
            <AlertDialogDescription>
              {pendingDelete?.origin === 'builtin'
                ? 'This builtin skill will be hidden. You can restore it later with “Reset”.'
                : 'This user skill will be permanently removed from the vault.'}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => void confirmDelete()}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
