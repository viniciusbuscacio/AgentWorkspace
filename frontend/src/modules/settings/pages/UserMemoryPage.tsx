import { useCallback, useEffect, useRef, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import {
  USER_MEMORY_CONDENSE_THRESHOLD,
  USER_MEMORY_PROMPT_CAP,
  userMemoryService,
  type UserMemoryDoc,
} from '@services/user-memory.service';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { onAwEvent } from '@services/events';

const CHAR_COUNT_WARN = USER_MEMORY_CONDENSE_THRESHOLD;
const CHAR_COUNT_CAP = USER_MEMORY_PROMPT_CAP;

export function UserMemoryPage() {
  const [doc, setDoc] = useState<UserMemoryDoc | null>(null);
  const [draft, setDraft] = useState('');
  const [saving, setSaving] = useState(false);
  const [hasBackup, setHasBackup] = useState(false);
  const [undoing, setUndoing] = useState(false);

  const dirty = doc !== null && draft !== doc.content;
  // Live view of dirty for the memory:changed subscription (subscribing per
  // dirty-change would drop events in the gap between unsubscribe/resubscribe).
  const dirtyRef = useRef(dirty);
  useEffect(() => {
    dirtyRef.current = dirty;
  }, [dirty]);

  const refresh = useCallback(async () => {
    const result = await userMemoryService.getDoc();
    if (!result.success) {
      setMessage(result.error ?? 'Could not load memory document.');
      return;
    }
    const loaded = result.doc ?? { content: '', updatedAt: '' };
    setDoc(loaded);
    setDraft(loaded.content);
    setHasBackup(!!loaded.backup);
  }, []);

  useEffect(() => {
    void refresh();
    // The agent's memory.remember mutates the same document; without this an
    // open panel goes stale and Save would clobber what the agent recorded.
    return onAwEvent('memory:changed', () => {
      if (dirtyRef.current) {
        setMessage('The agent added to the memory while you were editing — Cancel reloads it; Save overwrites it.');
        return;
      }
      void refresh();
    });
  }, [refresh]);

  async function save() {
    setSaving(true);
    const result = await userMemoryService.setDoc(draft);
    setSaving(false);
    if (!result.success) {
      setMessage(result.error ?? 'Could not save.');
      return;
    }
    const updated = result.doc ?? { content: draft, updatedAt: '' };
    setDoc(updated);
    setDraft(updated.content);
    setHasBackup(!!updated.backup);
    setMessage('Saved.');
  }

  function cancel() {
    setMessage('');
    // Reload from the vault (not the local snapshot) so discarding also picks
    // up anything the agent recorded while the draft was open.
    void refresh();
  }

  async function undoCondensation() {
    setUndoing(true);
    const backupResult = await userMemoryService.getBackup();
    setUndoing(false);
    if (!backupResult.success || !backupResult.doc?.content) {
      setMessage('No backup available to restore.');
      return;
    }
    setDraft(backupResult.doc.content);
    setMessage('Backup loaded into the editor — save to apply.');
  }

  const charCount = draft.length;
  const charCountColor =
    charCount >= CHAR_COUNT_CAP
      ? 'text-destructive'
      : charCount >= CHAR_COUNT_WARN
        ? 'text-warning'
        : 'text-muted-foreground';

  return (
    <div className="flex min-h-[560px] flex-col gap-4">
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-border bg-card">
        {/* Header */}
        <div className="flex items-center justify-between gap-2 border-b border-border p-3">
          <div className="min-w-0">
            <div className="text-sm font-semibold">Memory document</div>
            <div className="text-xs text-muted-foreground">
              The agent writes here as it learns about you; you can edit directly.
              {doc?.lastCondensedAt ? ` Last condensed: ${doc.lastCondensedAt}.` : ''}
            </div>
          </div>
          {hasBackup && (
            <button
              type="button"
              className="shrink-0 text-xs text-muted-foreground underline-offset-2 hover:underline disabled:opacity-50"
              disabled={undoing}
              onClick={() => void undoCondensation()}
              data-awid="memory-undo-btn"
            >
              {undoing ? 'Loading…' : 'Undo condensation'}
            </button>
          )}
        </div>

        {/* Textarea */}
        <div className="flex min-h-0 flex-1 flex-col gap-2 p-3">
          {doc === null ? (
            <p className="text-sm text-muted-foreground">Loading…</p>
          ) : (
            <textarea
              data-awid="memory-doc-textarea"
              className="min-h-0 flex-1 resize-none rounded-md border border-input bg-background p-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
              value={draft}
              onChange={(e) => {
                setDraft(e.target.value);
                setMessage('');
              }}
              placeholder={
                'Nothing saved yet.\nThe agent records what it learns about you during chats.\nYou can also write here directly.'
              }
              spellCheck={false}
              rows={16}
            />
          )}

          {/* Character count */}
          <div className={`text-right text-xs ${charCountColor}`} data-awid="memory-char-count">
            {charCount.toLocaleString()} chars
            {charCount >= CHAR_COUNT_WARN && charCount < CHAR_COUNT_CAP
              ? ` — will condense on next unlock`
              : ''}
            {charCount >= CHAR_COUNT_CAP
              ? ` — exceeds ${CHAR_COUNT_CAP.toLocaleString()} char prompt cap`
              : ''}
          </div>

          <SaveCancelActions
            onSave={() => void save()}
            onCancel={cancel}
            saving={saving}
            disabled={!dirty}
            />
        </div>
      </div>
    </div>
  );
}
