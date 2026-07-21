import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { Input } from '@ui/input';
import { Textarea } from '@ui/textarea';
import { ZoomSafeSelect } from '@ui/zoom-safe-select';
import {
  BACKLOG_STATUSES,
  tasksService,
  type TasksAttachment,
  type TasksItem,
  type TasksStatus,
} from '@services/tasks.service';
import type { ModuleRuntimeProps } from '@modules/module-contract';
import { onAwEvent } from '@services/events';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { ModuleHelpButton } from '@/components/patterns/ModuleHelpButton';

// Tasks — copy of the Notes module recipe: a vault-backed task list the
// agent shares through the tasks.* aw actions.
export function TasksModule({ createSpecChat }: ModuleRuntimeProps) {
  const [items, setItems] = useState<TasksItem[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [newTitle, setNewTitle] = useState('');
  const [detailTitle, setDetailTitle] = useState('');
  const [detailBody, setDetailBody] = useState('');
  const [detailStatus, setDetailStatus] = useState<TasksStatus>('open');
  const [attachmentPreview, setAttachmentPreview] = useState<{ name: string; dataUri: string } | null>(null);
  const [uploading, setUploading] = useState(false);
  const [saving, setSaving] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const lastLoadedRef = useRef<{ title: string; body: string; status: TasksStatus }>({ title: '', body: '', status: 'open' });
  // Id whose values are currently loaded in the editor. refresh() replaces the
  // items with fresh objects, so the editor must re-sync only when a DIFFERENT
  // item is selected — never on mere identity change, or a refresh mid-edit
  // (attachment upload, list status toggle) would wipe unsaved text.
  const loadedItemIdRef = useRef<string | null>(null);

  const refresh = useCallback(async () => {
    const result = await tasksService.list();
    if (!result.success) {
      setMessage(result.error || 'Could not load the tasks.');
      return;
    }
    setItems(result.items ?? []);
  }, []);

  const selectedItem = useMemo(() => items.find((item) => item.id === selectedId) ?? null, [items, selectedId]);
  const openCount = items.filter((item) => item.status !== 'completed').length;
  const completedCount = items.length - openCount;

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Refresh live when the agent mutates the tasks (tasks.* aw actions).
  // Safe mid-edit: the editor only re-syncs when a different item is selected.
  useEffect(() => {
    try {
      return onAwEvent('tasks:changed', () => { void refresh(); });
    } catch {
      return undefined; // event bus unavailable (e.g. tests)
    }
  }, [refresh]);

  useEffect(() => {
    if (!selectedItem) {
      loadedItemIdRef.current = null;
      setDetailTitle('');
      setDetailBody('');
      setDetailStatus('open');
      return;
    }
    if (loadedItemIdRef.current === selectedItem.id) return;
    loadedItemIdRef.current = selectedItem.id;
    setDetailTitle(selectedItem.title);
    setDetailBody(selectedItem.body ?? '');
    const status = (BACKLOG_STATUSES.includes(selectedItem.status as TasksStatus) ? selectedItem.status : 'open') as TasksStatus;
    setDetailStatus(status);
    lastLoadedRef.current = { title: selectedItem.title, body: selectedItem.body ?? '', status };
  }, [selectedItem]);

  async function add() {
    const title = newTitle.trim();
    if (!title) return;
    const result = await tasksService.add(title);
    if (!result.success) {
      setMessage(result.error || 'Could not add the item.');
      return;
    }
    setNewTitle('');
    if (result.item) setSelectedId(result.item.id);
    setMessage('');
    await refresh();
  }

  function select(item: TasksItem) {
    setSelectedId(item.id);
    setMessage('');
  }

  async function setStatus(item: TasksItem, status: TasksStatus) {
    const result = await tasksService.update(item.id, '', item.body ?? '', status);
    if (!result.success) {
      setMessage(result.error || 'Could not update the item.');
    } else if (item.id === selectedId) {
      // The editor no longer re-syncs on refresh (it would wipe unsaved
      // edits), so a list-side toggle of the open item must update the
      // editor's status itself — otherwise Save would revert it.
      setDetailStatus(status);
      lastLoadedRef.current = { ...lastLoadedRef.current, status };
    }
    await refresh();
  }

  async function saveDetail() {
    if (!selectedItem) return;
    setSaving(true);
    const result = await tasksService.update(selectedItem.id, detailTitle, detailBody, detailStatus);
    setSaving(false);
    setMessage(result.success ? 'Saved.' : result.error || 'Could not save the item.');
    if (result.success) {
      lastLoadedRef.current = { title: detailTitle, body: detailBody, status: detailStatus };
    }
    await refresh();
  }

  function cancelDetail() {
    setDetailTitle(lastLoadedRef.current.title);
    setDetailBody(lastLoadedRef.current.body);
    setDetailStatus(lastLoadedRef.current.status);
    setMessage('');
  }

  async function createSpecInChat() {
    if (!selectedItem || !createSpecChat) return;
    const result = await tasksService.update(selectedItem.id, detailTitle, detailBody, detailStatus);
    if (!result.success) {
      setMessage(result.error || 'Could not save the item.');
      return;
    }
    setMessage('Opening spec chat...');
    await refresh();
    await createSpecChat(detailTitle, detailBody);
  }

  async function remove(item: TasksItem) {
    const result = await tasksService.delete(item.id);
    setMessage(result.success ? `Deleted "${item.title}".` : result.error || 'Could not delete.');
    if (selectedId === item.id) setSelectedId(null);
    await refresh();
  }

  async function uploadFiles(fileList: FileList | File[]) {
    if (!selectedItem) return;
    const files = Array.from(fileList).filter((file) => file.type.startsWith('image/'));
    if (files.length === 0) return;
    setUploading(true);
    for (const file of files) {
      const dataUri = await readFileAsDataUri(file);
      const result = await tasksService.addAttachment(selectedItem.id, file.name, file.type, dataUri);
      if (!result.success) {
        setMessage(result.error || `Could not attach ${file.name}.`);
        break;
      }
      setMessage(`Attached ${file.name}.`);
    }
    setUploading(false);
    await refresh();
  }

  async function previewAttachment(attachment: TasksAttachment) {
    const result = await tasksService.getAttachmentData(attachment.id);
    if (!result.success || !result.dataUri) {
      setMessage(result.error || 'Could not load the attachment.');
      return;
    }
    setAttachmentPreview({ name: attachment.name, dataUri: result.dataUri });
  }

  async function removeAttachment(attachment: TasksAttachment) {
    const result = await tasksService.deleteAttachment(attachment.id);
    setMessage(result.success ? `Removed ${attachment.name}.` : result.error || 'Could not remove the attachment.');
    await refresh();
  }

  return (
    <section className="home-screen" data-awid="tasks-screen">
      <div className="home-header">
        <div className="flex items-start justify-between gap-3">
          <h1 className="home-title">Tasks</h1>
          <ModuleHelpButton module="Tasks" />
        </div>
        <p className="home-subtitle">{openCount} open · {completedCount} completed</p>
      </div>
      <div className="grid min-h-[590px] gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
        <aside className="flex min-h-0 flex-col overflow-hidden rounded-lg border border-border bg-card">
          <div className="flex gap-2 border-b border-border p-3">
            <Input
              value={newTitle}
              onChange={(event) => setNewTitle(event.target.value)}
              onKeyDown={(event) => { if (event.key === 'Enter') void add(); }}
              placeholder="Add a task..."
              aria-label="New task"
              className="min-w-24 flex-1"
            />
            <Button icon="add" onClick={() => void add()}>Add</Button>
          </div>
          <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-3">
            {items.length === 0 && (
              <p className="text-sm text-muted-foreground">No tasks yet. Add one or ask the agent to track something.</p>
            )}
            {items.map((item) => (
              <article key={item.id} className={`rounded-md border p-3 ${selectedId === item.id ? 'border-ring bg-muted/40' : 'border-border bg-background/30'}`}>
                <button type="button" className="block w-full min-w-0 text-left" onClick={() => select(item)} aria-label={`Open task ${item.title}`}>
                  <div className={`truncate text-sm font-medium ${item.status === 'completed' ? 'text-muted-foreground line-through' : ''}`}>{item.title}</div>
                  {item.body && <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">{item.body}</div>}
                </button>
                <div className="mt-3 flex items-center justify-between gap-2">
                  <span className="rounded-sm border border-border px-2 py-1 text-xs text-muted-foreground">{statusLabel(item.status)}</span>
                  <div className="flex gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      icon={item.status === 'completed' ? 'radio_button_unchecked' : 'check_circle'}
                      onClick={() => void setStatus(item, item.status === 'completed' ? 'open' : 'completed')}
                      aria-label={`Mark ${item.title} as ${item.status === 'completed' ? 'open' : 'completed'}`}
                    />
                    <Button variant="ghost" size="sm" icon="delete" onClick={() => void remove(item)} aria-label={`Delete ${item.title}`} />
                  </div>
                </div>
              </article>
            ))}
          </div>
        </aside>
        <main
          className="flex min-h-0 flex-col overflow-hidden rounded-lg border border-border bg-card"
          onDragOver={(event) => {
            if (selectedItem) event.preventDefault();
          }}
          onDrop={(event) => {
            if (!selectedItem) return;
            event.preventDefault();
            void uploadFiles(event.dataTransfer.files);
          }}
          onPaste={(event) => {
            if (!selectedItem) return;
            const files = Array.from(event.clipboardData.files);
            if (files.length > 0) void uploadFiles(files);
          }}
        >
          {/* flex-wrap: the actions row is wider than a narrow pane (or a
              zoomed app); it must wrap under the title instead of overflowing
              past the pane's clipped edge. Actions only render with an item
              selected (Notes pattern) — disabled ghosts next to "Select an
              item" were pure noise. */}
          <div className="flex min-h-[58px] flex-wrap items-center justify-between gap-3 border-b border-border p-3">
            <div className="min-w-0">
              <div className="text-sm font-semibold">{selectedItem ? 'Item detail' : 'Select an item'}</div>
              <div className="text-xs text-muted-foreground">{selectedItem ? `${selectedItem.attachments?.length ?? 0} attachments` : 'Details appear here'}</div>
            </div>
            {selectedItem && (
              <div className="flex flex-wrap justify-end gap-2">
                <Button variant="outline" size="sm" icon="chat" disabled={!createSpecChat} onClick={() => void createSpecInChat()}>
                  Create spec in chat
                </Button>
                <Button variant="outline" size="sm" icon="attach_file" disabled={uploading} onClick={() => fileInputRef.current?.click()}>
                  Attach
                </Button>
                <SaveCancelActions
                  onSave={() => void saveDetail()}
                  onCancel={cancelDetail}
                  saving={saving}
                  disabled={false}
                  size="sm"
                />
              </div>
            )}
          </div>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/png,image/jpeg,image/gif,image/webp"
            multiple
            className="hidden"
            onChange={(event) => {
              if (event.currentTarget.files) void uploadFiles(event.currentTarget.files);
              event.currentTarget.value = '';
            }}
            aria-label="Task attachment files"
          />
          {selectedItem ? (
            <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-3">
              <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_180px]">
                <Input value={detailTitle} onChange={(event) => setDetailTitle(event.target.value)} aria-label="Tasks item title" />
                <ZoomSafeSelect
                  aria-label="Tasks item status"
                  value={detailStatus}
                  onValueChange={(value) => setDetailStatus(value as TasksStatus)}
                  options={BACKLOG_STATUSES.map((status) => ({ value: status, label: statusLabel(status) }))}
                />
              </div>
              <Textarea
                value={detailBody}
                onChange={(event) => setDetailBody(event.target.value)}
                aria-label="Tasks item body"
                placeholder="Add implementation notes, acceptance criteria, links, or validation details..."
                className="min-h-[220px] resize-y"
              />
              <section className="flex flex-col gap-2">
                <div className="flex items-center justify-between">
                  <h2 className="text-sm font-semibold">Attachments</h2>
                  <span className="text-xs text-muted-foreground">Paste, drop, or choose images</span>
                </div>
                {(selectedItem.attachments?.length ?? 0) === 0 ? (
                  <div className="rounded-md border border-dashed border-border p-4 text-sm text-muted-foreground">No attachments yet.</div>
                ) : (
                  <div className="grid gap-2 md:grid-cols-2">
                    {selectedItem.attachments.map((attachment) => (
                      <div key={attachment.id} className="flex items-center justify-between gap-2 rounded-md border border-border p-2">
                        <button type="button" className="min-w-0 text-left" onClick={() => void previewAttachment(attachment)} aria-label={`Preview ${attachment.name}`}>
                          <div className="truncate text-sm">{attachment.name}</div>
                          <div className="text-xs text-muted-foreground">{formatBytes(attachment.size)}</div>
                        </button>
                        <Button variant="ghost" size="sm" icon="delete" onClick={() => void removeAttachment(attachment)} aria-label={`Delete attachment ${attachment.name}`} />
                      </div>
                    ))}
                  </div>
                )}
              </section>
            </div>
          ) : (
            <div className="flex flex-1 items-center justify-center p-6 text-sm text-muted-foreground">Select a tasks item to edit title, body, status, and attachments.</div>
          )}
        </main>
      </div>
      {attachmentPreview && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-6" role="dialog" aria-label={`Attachment preview ${attachmentPreview.name}`}>
          <div className="flex max-h-full max-w-4xl flex-col overflow-hidden rounded-lg border border-border bg-card">
            <div className="flex items-center justify-between gap-3 border-b border-border p-3">
              <div className="truncate text-sm font-medium">{attachmentPreview.name}</div>
              <Button variant="ghost" size="sm" icon="close" onClick={() => setAttachmentPreview(null)} aria-label="Close attachment preview" />
            </div>
            <img src={attachmentPreview.dataUri} alt={attachmentPreview.name} className="max-h-[75vh] max-w-full object-contain" />
          </div>
        </div>
      )}
    </section>
  );
}

function statusLabel(status: string): string {
  switch (status) {
    case 'in-progress':
      return 'In progress';
    case 'needs-validation':
      return 'Needs validation';
    case 'completed':
      return 'Completed';
    default:
      return 'Open';
  }
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB'];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

function readFileAsDataUri(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ''));
    reader.onerror = () => reject(reader.error ?? new Error('Could not read attachment.'));
    reader.readAsDataURL(file);
  });
}
