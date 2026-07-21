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
  Textarea,
} from '@ui/index';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import type { LeaveGuard } from '@modules/module-contract';
import {
  instructionsService,
  type EffectiveInstructions,
  type InstructionDocument,
} from '@services/instructions.service';

interface AgentInstructionsPageProps {
  // onDetailTitleChange feeds the Settings breadcrumb: the open document / the
  // Effective Instructions view surfaces as `Settings › Agent Instructions › X`.
  onDetailTitleChange?: (title: string | null) => void;
  // listRequest increments when the `Agent Instructions` breadcrumb is clicked
  // while a sub-view is open, asking the page to return to the list.
  listRequest?: number;
  // onRegisterLeaveGuard hands up a guard intercepting navigation while an
  // editor has unsaved edits.
  onRegisterLeaveGuard?: (guard: LeaveGuard | null) => void;
}

interface EditorState {
  id: string;
  content: string;
  original: string;
  editable: boolean;
  resettable: boolean;
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

export function AgentInstructionsPage({ onDetailTitleChange, listRequest = 0, onRegisterLeaveGuard }: AgentInstructionsPageProps) {
  const [docs, setDocs] = useState<InstructionDocument[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editor, setEditor] = useState<EditorState | null>(null);
  const [effective, setEffective] = useState<EffectiveInstructions | null>(null);
  const [effectiveOpen, setEffectiveOpen] = useState(false);
  const [effectiveAt, setEffectiveAt] = useState('');
  const [pendingExit, setPendingExit] = useState<(() => void) | null>(null);
  const [pendingReset, setPendingReset] = useState<string | null>(null);

  const editorRef = useRef<EditorState | null>(null);
  useEffect(() => {
    editorRef.current = editor;
  }, [editor]);

  const editorDirty = useMemo(() => !!editor && editor.content !== editor.original, [editor]);

  // tryLeave guards unsaved editor changes, like Settings → Skills.
  const tryLeave = useCallback<LeaveGuard>((proceed) => {
    const e = editorRef.current;
    if (e && e.content !== e.original) setPendingExit(() => proceed);
    else proceed();
  }, []);

  const hasEditor = editor !== null;
  useEffect(() => {
    onRegisterLeaveGuard?.(hasEditor ? tryLeave : null);
    return () => onRegisterLeaveGuard?.(null);
  }, [hasEditor, onRegisterLeaveGuard, tryLeave]);

  const refresh = useCallback(async () => {
    const listRes = await instructionsService.list();
    if (!listRes.success) {
      setMessage(listRes.error ?? 'Could not load instruction documents.');
      setDocs([]);
      return;
    }
    setDocs(listRes.documents ?? []);
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Breadcrumb child title: editor id, or the Effective view label, else null.
  useEffect(() => {
    const title = editor ? editor.id : effectiveOpen ? 'Effective Instructions' : null;
    onDetailTitleChange?.(title);
  }, [editor, effectiveOpen, onDetailTitleChange]);
  useEffect(() => () => onDetailTitleChange?.(null), [onDetailTitleChange]);

  // Clicking the `Agent Instructions` breadcrumb returns to the list (guarded).
  useEffect(() => {
    if (listRequest > 0) tryLeave(() => {
      setEditor(null);
      setEffectiveOpen(false);
    });
  }, [listRequest, tryLeave]);

  async function openEdit(doc: InstructionDocument) {
    setMessage('');
    const res = await instructionsService.read(doc.id);
    if (!res.success || !res.document) {
      setMessage(res.error ?? 'Could not open document.');
      return;
    }
    const content = res.document.content ?? '';
    setEffectiveOpen(false);
    setEditor({
      id: doc.id,
      content,
      original: content,
      editable: res.document.editable,
      resettable: res.document.resettable,
    });
  }

  async function openEffective() {
    setMessage('');
    setBusy(true);
    const res = await instructionsService.effective(true);
    setBusy(false);
    if (!res.success || !res.effective) {
      setMessage(res.error ?? 'Could not load effective instructions.');
      return;
    }
    setEditor(null);
    setEffective(res.effective);
    setEffectiveAt(res.generatedAt ?? '');
    setEffectiveOpen(true);
  }

  async function saveEditor() {
    if (!editor) return;
    setSaving(true);
    const res = await instructionsService.save(editor.id, editor.content);
    setSaving(false);
    if (!res.success) {
      setMessage(res.error ?? 'Could not save document.');
      return;
    }
    if (res.devOverrideActive && res.warning) setMessage(res.warning);
    else setMessage(`Saved ${editor.id}.`);
    setEditor(null);
    await refresh();
  }

  async function confirmReset() {
    if (!pendingReset) return;
    const id = pendingReset;
    setPendingReset(null);
    setBusy(true);
    const res = await instructionsService.reset(id);
    setBusy(false);
    if (!res.success) {
      setMessage(res.error ?? 'Could not reset document.');
      return;
    }
    setMessage(`Reset ${id} to its built-in version.`);
    setEditor(null);
    await refresh();
  }

  // --- editor view ---
  if (editor) {
    return (
      <div className="flex min-h-[560px] flex-col gap-4">
        {!editor.editable && (
          <p className="text-xs text-warning">
            This document is read-only right now (a dev override via AW_SKILLS_DIR is active).
          </p>
        )}
        <Textarea
          className="min-h-[420px] resize-y font-mono text-xs"
          value={editor.content}
          spellCheck={false}
          readOnly={!editor.editable}
          onChange={(e) => setEditor({ ...editor, content: e.target.value })}
        />
        <div className="flex flex-wrap items-center justify-between gap-2">
          <SaveCancelActions
            onSave={() => void saveEditor()}
            onCancel={() => tryLeave(() => setEditor(null))}
            saving={saving}
            disabled={!editor.editable || !editorDirty}
            />
          {editor.resettable && (
            <Button variant="outline" size="sm" disabled={busy} onClick={() => setPendingReset(editor.id)}>
              Reset to built-in
            </Button>
          )}
        </div>

        <AlertDialog open={pendingExit !== null} onOpenChange={(open) => !open && setPendingExit(null)}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Discard unsaved changes?</AlertDialogTitle>
              <AlertDialogDescription>
                You changed this instruction document but have not saved it. If you leave now, those edits will be lost.
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

        {resetDialog(pendingReset, setPendingReset, confirmReset)}
      </div>
    );
  }

  // --- effective instructions view ---
  if (effectiveOpen) {
    return (
      <div className="flex min-h-[560px] flex-col gap-4">
        <p className="rounded-lg border border-border bg-card p-3 text-xs text-muted-foreground">
          This is trusted configuration loaded into the agent — not web or email content. Secret-looking values are
          scrubbed for display.
        </p>
        <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
          <span>{effective?.metadata?.charCount ?? 0} chars</span>
          {effectiveAt && <span>Generated {effectiveAt}</span>}
          {effective?.metadata?.devOverrideActive && <Pill tone="primary">dev override</Pill>}
          <div className="ml-auto flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => void navigator.clipboard?.writeText(effective?.content ?? '')}
            >
              Copy
            </Button>
            <Button variant="outline" size="sm" disabled={busy} onClick={() => void openEffective()}>
              Refresh
            </Button>
          </div>
        </div>
        <div className="flex flex-col gap-1.5">
          <span className="text-xs font-semibold text-muted-foreground">Sources</span>
          <div className="flex flex-col gap-1">
            {(effective?.sources ?? []).map((s) => (
              <div key={s.id} className="flex items-center justify-between rounded-md border border-border bg-card px-3 py-1.5 text-xs">
                <span className="font-medium">{s.id}</span>
                <span className="text-muted-foreground">
                  {s.origin} · {s.chars} chars
                </span>
              </div>
            ))}
          </div>
        </div>
        <pre className="max-h-[360px] overflow-auto whitespace-pre-wrap rounded-lg border border-border bg-card p-3 font-mono text-xs">
          {effective?.content || '(empty)'}
        </pre>
      </div>
    );
  }

  // --- list view ---
  return (
    <div className="flex min-h-[560px] flex-col gap-4">
      <p className="text-xs text-muted-foreground">
        Agent Instructions are always-on rules. Skills are on-demand procedures (Settings → Skills) and Memory is curated
        facts (Settings → Memory).
      </p>

      <div className="flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-3">
        <div className="min-w-0">
          <div className="text-sm font-semibold">Effective Instructions</div>
          <div className="text-xs text-muted-foreground">The final instruction block currently loaded into the agent.</div>
        </div>
        <Button variant="outline" size="sm" disabled={busy} onClick={() => void openEffective()}>
          View
        </Button>
      </div>

      {docs === null ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : (
        <div className="flex flex-col gap-2">
          {docs.map((doc) => (
            <div key={doc.id} className="flex items-start justify-between gap-3 rounded-lg border border-border bg-card p-3">
              <div className="min-w-0 flex-1">
                <span className="text-sm font-semibold">{doc.title || doc.id}</span>
                {doc.description && <div className="mt-1 text-xs text-muted-foreground">{doc.description}</div>}
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <Button variant="ghost" size="sm" disabled={busy} onClick={() => void openEdit(doc)}>
                  {doc.editable ? 'Edit' : 'View'}
                </Button>
                {doc.resettable && (
                  <Button variant="outline" size="sm" disabled={busy} onClick={() => setPendingReset(doc.id)}>
                    Reset
                  </Button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {resetDialog(pendingReset, setPendingReset, confirmReset)}
    </div>
  );
}

// resetDialog renders the destructive reset confirmation (spec §10.4).
function resetDialog(
  pendingReset: string | null,
  setPendingReset: (v: string | null) => void,
  confirmReset: () => void | Promise<void>,
) {
  return (
    <AlertDialog open={pendingReset !== null} onOpenChange={(open) => !open && setPendingReset(null)}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Reset {pendingReset} to the default settings?</AlertDialogTitle>
          <AlertDialogDescription>
            This restores the default {pendingReset} and discards your changes. This cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction onClick={() => void confirmReset()}>Reset</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
