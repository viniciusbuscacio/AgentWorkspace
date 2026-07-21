import { useEffect, useMemo, useRef, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { Input } from '@ui/input';
import { PasswordInput } from '@ui/password-input';
import { Textarea } from '@ui/textarea';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@ui/alert-dialog';
import { passwordsService, type PasswordEntry } from '@services/passwords.service';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { isWebMode } from '@/web/web-bindings';
import { ModuleHelpButton } from '@/components/patterns/ModuleHelpButton';

type Draft = Pick<PasswordEntry, 'name' | 'username' | 'password' | 'url' | 'notes'>;

const emptyDraft: Draft = { name: '', username: '', password: '', url: '', notes: '' };

// How long a copied password stays on the OS clipboard before the module
// clears it (only if the clipboard still holds that exact value).
const CLIPBOARD_CLEAR_MS = 25_000;

// pendingConfirm drives the themed confirmation modal: deleting a credential,
// or discarding unsaved edits when the user switches selection / starts new.
type PendingConfirm =
  | { kind: 'delete' }
  | { kind: 'discard'; proceed: () => void };

export function PasswordsModule() {
  const [items, setItems] = useState<PasswordEntry[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [dirty, setDirty] = useState(false);
  const [search, setSearch] = useState('');
  // The editor panel opens on demand (pick a credential / Add Password) and
  // closes via its X — the module starts with the right side empty.
  const [editorOpen, setEditorOpen] = useState(false);
  const [pendingConfirm, setPendingConfirm] = useState<PendingConfirm | null>(null);
  // Id whose values are loaded in the editor ('' = the New form). The editor
  // re-syncs only when the selection actually changes — never because a
  // refresh replaced the list objects (that wiped unsaved edits in Tasks).
  const loadedIdRef = useRef<string | null>(null);
  const clipboardTimerRef = useRef<number | null>(null);

  async function refresh() {
    const result = await passwordsService.list();
    if (!result.success) {
      setMessage(result.error || 'Could not load passwords.');
      return;
    }
    const next = result.passwords ?? [];
    setItems(next);
    // No auto-select: the editor stays closed until the user picks a
    // credential or clicks Add Password.
    setSelectedId((current) => current && next.some((item) => item.id === current) ? current : '');
  }

  useEffect(() => {
    // In web mode the view renders a notice and must not call the password
    // bindings at all (they are denylisted on the bridge anyway).
    if (isWebMode()) return;
    void refresh();
  }, []);

  useEffect(() => {
    if (loadedIdRef.current === selectedId) return;
    loadedIdRef.current = selectedId;
    const selected = items.find((item) => item.id === selectedId);
    setDraft(selected ? {
      name: selected.name,
      username: selected.username,
      password: selected.password,
      url: selected.url,
      notes: selected.notes,
    } : emptyDraft);
    setDirty(false);
  }, [items, selectedId]);

  // Cancel a pending clipboard clear when the module unmounts (the copied
  // value stays; clearing without the compare below could eat newer content).
  useEffect(() => () => {
    if (clipboardTimerRef.current) window.clearTimeout(clipboardTimerRef.current);
  }, []);

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) return items;
    return items.filter((item) => `${item.name} ${item.username} ${item.url}`.toLowerCase().includes(query));
  }, [items, search]);

  function update<K extends keyof Draft>(key: K, value: string) {
    setDraft((current) => ({ ...current, [key]: value }));
    setDirty(true);
  }

  // guardDirty runs the navigation immediately when the editor is clean, and
  // asks for confirmation first when it would discard unsaved edits.
  function guardDirty(proceed: () => void) {
    if (!dirty) {
      proceed();
      return;
    }
    setPendingConfirm({ kind: 'discard', proceed });
  }

  function select(id: string) {
    guardDirty(() => {
      setSelectedId(id);
      setMessage('');
      setEditorOpen(true);
    });
  }

  function startNew() {
    guardDirty(() => {
      loadedIdRef.current = null; // force the editor to load the empty form
      setSelectedId('');
      setMessage('');
      setEditorOpen(true);
    });
  }

  function closeEditor() {
    guardDirty(() => {
      loadedIdRef.current = null; // next open re-syncs the draft cleanly
      setSelectedId('');
      setMessage('');
      setEditorOpen(false);
    });
  }

  function cancelEdit() {
    const selected = items.find((item) => item.id === selectedId);
    setDraft(selected ? {
      name: selected.name,
      username: selected.username,
      password: selected.password,
      url: selected.url,
      notes: selected.notes,
    } : emptyDraft);
    setDirty(false);
    setMessage('');
  }

  async function save() {
    const result = await passwordsService.save({ id: selectedId || undefined, ...draft });
    if (!result.success || !result.password) {
      setMessage(result.error || 'Could not save password.');
      return;
    }
    setItems((current) => [result.password!, ...current.filter((item) => item.id !== result.password!.id)]);
    loadedIdRef.current = result.password.id; // editor already shows these values
    setSelectedId(result.password.id);
    setDirty(false);
    setMessage('Saved.');
  }

  async function confirmedRemove() {
    if (!selectedId) return;
    const result = await passwordsService.delete(selectedId);
    if (!result.success) {
      setMessage(result.error || 'Could not delete password.');
      return;
    }
    setSelectedId('');
    setDirty(false);
    setEditorOpen(false);
    setMessage('Deleted.');
    await refresh();
  }

  async function copyPassword() {
    const value = draft.password;
    if (!value) {
      setMessage('Nothing to copy.');
      return;
    }
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      setMessage('Could not copy to the clipboard.');
      return;
    }
    setMessage(`Copied. The clipboard clears in ${CLIPBOARD_CLEAR_MS / 1000}s.`);
    if (clipboardTimerRef.current) window.clearTimeout(clipboardTimerRef.current);
    clipboardTimerRef.current = window.setTimeout(() => {
      clipboardTimerRef.current = null;
      void (async () => {
        try {
          // Clear only if the clipboard still holds the copied password, so a
          // newer copy of something else is never destroyed.
          if (await navigator.clipboard.readText() === value) {
            await navigator.clipboard.writeText('');
          }
        } catch {
          // Cannot read the clipboard: leave it untouched.
        }
      })();
    }, CLIPBOARD_CLEAR_MS);
  }

  function confirmPending() {
    const pending = pendingConfirm;
    setPendingConfirm(null);
    if (!pending) return;
    if (pending.kind === 'delete') {
      void confirmedRemove();
      return;
    }
    setDirty(false);
    pending.proceed();
  }

  // Credential plaintext must never ride the web bridge (the backend denies
  // the password methods remotely); the web UI explains instead of erroring.
  if (isWebMode()) {
    return (
      <section className="home-screen" data-awid="passwords-screen">
        <div className="home-header">
          <div className="flex items-start justify-between gap-3">
            <h1 className="home-title">Passwords</h1>
            <ModuleHelpButton module="Passwords" />
          </div>
          <p className="home-subtitle">Encrypted, local-only, shared with the agent.</p>
        </div>
        <p className="rounded-md border border-border bg-muted p-3 text-sm text-muted-foreground">
          Passwords are only available on the desktop app. Web access never transports credential plaintext over the network.
        </p>
      </section>
    );
  }

  return (
    <section className="home-screen" data-awid="passwords-screen">
      <div className="home-header">
        <div className="flex items-start justify-between gap-3">
          <h1 className="home-title">Passwords</h1>
          <ModuleHelpButton module="Passwords" />
        </div>
        <p className="home-subtitle">Encrypted, local-only, shared with the agent.</p>
      </div>
      <div className="grid min-h-[560px] gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
        <aside className="flex min-h-0 flex-col overflow-hidden rounded-lg border border-border bg-card">
          <div className="flex items-center gap-2 border-b border-border p-3">
            <Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search credentials..." aria-label="Search credentials" className="min-w-24 flex-1" />
            <Button size="sm" icon="add" className="shrink-0" onClick={startNew}>Add Password</Button>
          </div>
          <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-3">
            {filtered.length === 0 && <p className="text-sm text-muted-foreground">No passwords saved.</p>}
            {filtered.map((item) => (
              <button
                key={item.id}
                type="button"
                className={`rounded-md border p-3 text-left ${item.id === selectedId ? 'border-ring bg-muted/40' : 'border-border bg-background/30'}`}
                onClick={() => select(item.id)}
              >
                <div className="truncate text-sm font-medium">{item.name}</div>
                <div className="mt-1 truncate text-xs text-muted-foreground">{item.username || item.url || 'No username'}</div>
              </button>
            ))}
          </div>
        </aside>
        {editorOpen && (
        <main className="relative flex min-h-0 flex-col overflow-hidden rounded-lg border border-border bg-card">
          {/* Close pinned to the panel's top-right corner (like the sidebar
              toggle), visually apart from the action buttons. */}
          <Button variant="ghost" size="icon-sm" icon="close" aria-label="Close panel" title="Close panel" className="absolute right-1.5 top-1.5 z-10" onClick={closeEditor} />
          <div className="flex min-h-[58px] flex-wrap items-center justify-between gap-3 border-b border-border p-3 pr-10">
            <div className="min-w-0">
              <div className="text-sm font-semibold">{selectedId ? 'Credential detail' : 'New credential'}</div>
              <div className="text-xs text-muted-foreground">
                {dirty ? 'Unsaved changes' : 'Stored as an encrypted vault secret.'}
              </div>
            </div>
            <div className="flex flex-wrap items-center justify-end gap-2">
              <Button variant="outline" size="sm" icon="delete" disabled={!selectedId} onClick={() => setPendingConfirm({ kind: 'delete' })}>Delete</Button>
              <SaveCancelActions
                onSave={() => void save()}
                onCancel={cancelEdit}
                disabled={!draft.name.trim()}
                size="sm"
              />
            </div>
          </div>
          <div className="grid content-start gap-3 overflow-y-auto p-3">
            <Input value={draft.name} onChange={(event) => update('name', event.target.value)} placeholder="Name" aria-label="Password name" />
            <Input value={draft.username} onChange={(event) => update('username', event.target.value)} placeholder="Username / email" aria-label="Username" />
            <div className="flex gap-2">
              {/* key remounts the field on selection change so the eye-reveal
                  resets — switching credentials never starts revealed. */}
              <PasswordInput
                key={selectedId || 'new'}
                className="flex-1"
                containerClassName="flex-1"
                value={draft.password}
                onChange={(event) => update('password', event.target.value)}
                placeholder="Password"
                aria-label="Password"
              />
              <Button variant="outline" icon="content_copy" disabled={!draft.password} onClick={() => void copyPassword()}>Copy</Button>
            </div>
            <Input value={draft.url} onChange={(event) => update('url', event.target.value)} placeholder="URL" aria-label="URL" />
            <Textarea className="min-h-36" value={draft.notes} onChange={(event) => update('notes', event.target.value)} placeholder="Notes" aria-label="Notes" />
          </div>
        </main>
        )}
      </div>
      <AlertDialog open={pendingConfirm !== null} onOpenChange={(open) => !open && setPendingConfirm(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingConfirm?.kind === 'delete' ? 'Delete credential?' : 'Discard unsaved changes?'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingConfirm?.kind === 'delete'
                ? 'This permanently removes the credential from the encrypted vault. A deleted password cannot be recovered.'
                : 'The edits in this form were not saved and will be lost.'}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setPendingConfirm(null)}>Keep editing</AlertDialogCancel>
            <AlertDialogAction onClick={confirmPending}>
              {pendingConfirm?.kind === 'delete' ? 'Delete' : 'Discard'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  );
}
