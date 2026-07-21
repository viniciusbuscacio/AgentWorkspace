import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@ui/button';
import { Input } from '@ui/input';
import { Textarea } from '@ui/textarea';
import { notesService, type Note } from '@services/notes.service';
import { onAwEvent } from '@services/events';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { ModuleHelpButton } from '@/components/patterns/ModuleHelpButton';

// Autosave debounce — matches the sidebar spec and AW2 (Decision 3).
export const NOTES_AUTOSAVE_DEBOUNCE_MS = 500;

// Notes — the workspace-module template: a vault-backed list + editor the
// agent shares through the notes.* aw actions.
export function NotesModule() {
  const [notes, setNotes] = useState<Note[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [showArchived, setShowArchived] = useState(false);
  const [search, setSearch] = useState('');
  const [inPrompt, setInPrompt] = useState(true);
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  // True after "New note" is clicked and before the draft is saved — the left
  // list shows a synthetic "New note" card while this is on.
  const [creating, setCreating] = useState(false);
  const lastLoadedRef = useRef<{ title: string; content: string }>({ title: '', content: '' });

  const refresh = useCallback(async () => {
    const result = await notesService.list();
    if (!result.success) {
      toast.error(result.error || 'Could not load notes.');
      return [] as Note[];
    }
    setNotes(result.notes ?? []);
    return result.notes ?? [];
  }, []);

  const selectedNote = useMemo(() => notes.find((note) => note.id === selectedId) ?? null, [notes, selectedId]);
  const visibleNotes = useMemo(() => {
    const base = notes.filter((note) => showArchived || !note.archived);
    const q = search.trim().toLowerCase();
    if (!q) return base;
    return base.filter(
      (note) => note.title.toLowerCase().includes(q) || note.content.toLowerCase().includes(q),
    );
  }, [notes, showArchived, search]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Refresh live when the agent mutates notes (notes.* aw actions). Safe
  // mid-edit: the editor fields are set imperatively in select()/startNew(),
  // never re-synced from the list.
  useEffect(() => {
    try {
      return onAwEvent('notes:changed', () => { void refresh(); });
    } catch {
      return undefined; // event bus unavailable (e.g. tests)
    }
  }, [refresh]);

  function select(note: Note) {
    setSelectedId(note.id);
    setCreating(false);
    setTitle(note.title);
    setContent(note.content);
    setInPrompt(note.inPrompt ?? true);
    lastLoadedRef.current = { title: note.title, content: note.content };
    setDirty(false);
  }

  function startNew() {
    setSelectedId(null);
    setCreating(true);
    setTitle('');
    setContent('');
    setInPrompt(true);
    lastLoadedRef.current = { title: '', content: '' };
    setDirty(false);
  }

  function cancelEdit() {
    if (!selectedId) {
      // New note: discard draft and leave creation mode.
      startNew();
      setCreating(false);
      return;
    }
    // Existing note: revert to last-loaded state
    setTitle(lastLoadedRef.current.title);
    setContent(lastLoadedRef.current.content);
    setDirty(false);
  }

  function editTitle(value: string) {
    setTitle(value);
    setDirty(true);
  }

  function editContent(value: string) {
    setContent(value);
    setDirty(true);
  }

  const saveExisting = useCallback(async (successMessage = 'Saved.') => {
    if (!selectedId) return;
    if (title === lastLoadedRef.current.title && content === lastLoadedRef.current.content) {
      setDirty(false);
      return;
    }
    setSaving(true);
    const result = await notesService.update(selectedId, title.trim() || 'New note', content);
    setSaving(false);
    if (!result.success) {
      toast.error(result.error || 'Could not save the note.');
      return;
    }
    setDirty(false);
    if (successMessage === 'Saved.') toast('Note saved.');
    if (result.note) {
      lastLoadedRef.current = { title: result.note.title, content: result.note.content };
      setInPrompt(result.note.inPrompt ?? true);
      setNotes((current) => current.map((note) => (note.id === result.note?.id ? result.note : note)));
    }
  }, [content, selectedId, title]);

  useEffect(() => {
    if (!selectedId || !dirty) return undefined;
    const handle = window.setTimeout(() => {
      void saveExisting('Autosaved.');
    }, NOTES_AUTOSAVE_DEBOUNCE_MS);
    return () => window.clearTimeout(handle);
  }, [dirty, saveExisting, selectedId]);

  async function saveNew() {
    const result = await notesService.create(title.trim() || 'New note', content, inPrompt);
    if (!result.success) {
      toast.error(result.error || 'Could not save the note.');
      return;
    }
    toast('Note saved.');
    setCreating(false);
    if (result.note) select(result.note);
    await refresh();
  }

  async function save() {
    if (selectedId) {
      await saveExisting('Saved.');
      await refresh();
      return;
    }
    await saveNew();
  }

  async function updateFlags(note: Note, pinned: boolean, archived: boolean) {
    const result = await notesService.updateFlags(note.id, pinned, archived);
    if (!result.success) {
      toast.error(result.error || 'Could not update the note.');
      return;
    }
    if (result.note) {
      setNotes((current) => current.map((entry) => (entry.id === result.note?.id ? result.note : entry)));
      if (selectedId === result.note.id) select(result.note);
    }
    toast(archived ? 'Note archived.' : pinned ? 'Note pinned.' : 'Note updated.');
    await refresh();
  }

  async function remove(note: Note) {
    const result = await notesService.delete(note.id);
    if (result.success) toast(`Deleted "${note.title}".`);
    else toast.error(result.error || 'Could not delete.');
    if (selectedId === note.id) startNew();
    await refresh();
  }

  return (
    <section className="home-screen" data-awid="notes-screen">
      <div className="home-header">
        <div className="flex items-start justify-between gap-3">
          <h1 className="home-title">Notes</h1>
          <ModuleHelpButton module="Notes" />
        </div>
        <p className="home-subtitle">Personal notes you share with the agent.</p>
      </div>
      <div className="grid min-h-[560px] gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
        <aside className="flex min-h-0 flex-col overflow-hidden rounded-lg border border-border bg-card">
          <div className="flex items-center justify-between gap-2 border-b border-border p-3">
            <div className="min-w-0 text-sm font-semibold">All notes</div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" icon="refresh" onClick={() => void refresh()} aria-label="Refresh notes" />
              <Button size="sm" icon="add" onClick={startNew} aria-label="New note" />
            </div>
          </div>
          <label className="flex items-center gap-2 border-b border-border px-3 py-2 text-xs text-muted-foreground">
            <input
              type="checkbox"
              checked={showArchived}
              onChange={(event) => setShowArchived(event.target.checked)}
            />
            Show archived
          </label>
          <div className="border-b border-border px-2 py-1">
            <div className="chat-search">
              <span className="chat-search-icon"><span className="material-symbols-outlined" aria-hidden="true">search</span></span>
              <input
                type="text"
                className="chat-search-input"
                placeholder="Search notes..."
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                aria-label="Search notes"
              />
              {search && (
                <button type="button" className="chat-search-clear" onClick={() => setSearch('')} aria-label="Clear note search">
                  <span className="material-symbols-outlined" aria-hidden="true">close</span>
                </button>
              )}
            </div>
          </div>
          <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-3">
            {creating && (
              <article className="flex flex-col gap-1 rounded-md border border-ring bg-muted/40 p-3 text-left">
                <div className="truncate text-sm font-medium">{title.trim() || 'New note'}</div>
                <div className="text-xs text-muted-foreground">Unsaved draft</div>
              </article>
            )}
            {visibleNotes.length === 0 && !creating && (
              <p className="text-sm text-muted-foreground">No notes here.</p>
            )}
            {visibleNotes.map((note) => (
              <article
                key={note.id}
                className={`flex flex-col gap-2 rounded-md border p-3 text-left${note.archived ? ' opacity-60' : ''} ${selectedId === note.id ? 'border-ring bg-muted/40' : 'border-border bg-background/30'}`}
              >
                <button
                type="button"
                className="min-w-0 text-left"
                onClick={() => select(note)}
                aria-label={`Open note ${note.title}`}
              >
                  <div className="flex min-w-0 items-center gap-2">
                    {note.pinned && <span className="material-symbols-outlined text-[15px] text-primary" aria-hidden="true">keep</span>}
                    <div className="truncate text-sm font-medium">{note.title}</div>
                  </div>
                  <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">{note.content || note.updatedAt}</div>
                </button>
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate text-xs text-muted-foreground">{note.updatedAt}</span>
                  <div className="flex gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      icon={note.pinned ? 'keep_off' : 'keep'}
                      onClick={() => void updateFlags(note, !note.pinned, note.archived)}
                      aria-label={`${note.pinned ? 'Unpin' : 'Pin'} note ${note.title}`}
                    />
                    <Button
                      variant="ghost"
                      size="sm"
                      icon={note.archived ? 'unarchive' : 'archive'}
                      onClick={() => void updateFlags(note, note.pinned, !note.archived)}
                      aria-label={`${note.archived ? 'Unarchive' : 'Archive'} note ${note.title}`}
                    />
                    <Button
                      variant="ghost"
                      size="sm"
                      icon="delete"
                      onClick={() => void remove(note)}
                      aria-label={`Delete note ${note.title}`}
                    />
                  </div>
                </div>
              </article>
            ))}
          </div>
        </aside>
        <main className="flex min-h-0 flex-col overflow-hidden rounded-lg border border-border bg-card">
          <div className="flex min-h-[58px] items-center justify-between gap-3 border-b border-border p-3">
            <div className="min-w-0">
              <div className="text-sm font-semibold">{selectedId ? 'Edit note' : 'New note'}</div>
              <div className="text-xs text-muted-foreground">
                {selectedId ? (saving ? 'Saving...' : dirty ? 'Unsaved changes' : 'Autosaved') : 'Manual save'}
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {selectedNote && (
                <>
                  <Button variant="outline" size="sm" icon={selectedNote.pinned ? 'keep_off' : 'keep'} onClick={() => void updateFlags(selectedNote, !selectedNote.pinned, selectedNote.archived)}>
                    {selectedNote.pinned ? 'Unpin' : 'Pin'}
                  </Button>
                  <Button variant="outline" size="sm" icon={selectedNote.archived ? 'unarchive' : 'archive'} onClick={() => void updateFlags(selectedNote, selectedNote.pinned, !selectedNote.archived)}>
                    {selectedNote.archived ? 'Unarchive' : 'Archive'}
                  </Button>
                </>
              )}
              <label className="flex items-center gap-2 text-sm text-muted-foreground">
                <input
                  type="checkbox"
                  checked={inPrompt}
                  onChange={(event) => {
                    const next = event.target.checked;
                    setInPrompt(next);
                    if (selectedId) void notesService.updateInPrompt(selectedId, next);
                  }}
                  aria-label="Insert into Agent prompt"
                />
                Insert into Agent prompt
              </label>
              <SaveCancelActions
                onSave={() => void save()}
                onCancel={cancelEdit}
                saving={saving}
                disabled={!dirty && !!selectedId}
                size="sm"
              />
            </div>
          </div>
          <div className="flex min-h-0 flex-1 flex-col gap-3 p-3">
            <Input
              value={title}
              onChange={(event) => editTitle(event.target.value)}
              placeholder="Title"
              aria-label="Note title"
              className="h-11 text-lg font-semibold"
            />
            <Textarea
              value={content}
              onChange={(event) => editContent(event.target.value)}
              placeholder="Write your note..."
              aria-label="Note content"
              className="min-h-0 flex-1 resize-none overflow-x-hidden break-words [overflow-wrap:anywhere]"
              wrap="soft"
            />
          </div>
        </main>
      </div>
    </section>
  );
}
