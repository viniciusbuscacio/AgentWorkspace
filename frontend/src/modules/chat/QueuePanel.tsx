import { useState } from 'react';
import type { domain } from '@services/models';

export interface QueuedMessage {
  id: string;
  // The chat this message was typed in; the flush only ever delivers it
  // there, never to whichever chat is currently displayed (AW2's queueId).
  chatId: string;
  content: string;
  attachments: domain.Attachment[];
}

interface QueuePanelProps {
  queue: QueuedMessage[];
  onEdit: (id: string, content: string) => void;
  onDelete: (id: string) => void;
}

/** Messages waiting to be sent while the agent is busy (AW2 parity). */
export function QueuePanel({ queue, onEdit, onDelete }: QueuePanelProps) {
  // Collapsed by default: show only the "N messages waiting" row; click to
  // expand the list, click again to collapse (user request 2026-06-12).
  const [expanded, setExpanded] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');

  if (queue.length === 0) return null;

  function startEdit(message: QueuedMessage) {
    setEditingId(message.id);
    setEditValue(message.content);
  }

  function saveEdit(id: string) {
    const trimmed = editValue.trim();
    // An empty edit would delete the queued message (empty content is filtered
    // out on save). Clearing the field to retype and hitting Save should not
    // destroy the item — treat empty as a cancel that keeps the original.
    if (!trimmed) {
      setEditingId(null);
      setEditValue('');
      return;
    }
    onEdit(id, trimmed);
    setEditingId(null);
    setEditValue('');
  }

  return (
    <div className="border-t border-border bg-background/60 px-4 py-2">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        aria-expanded={expanded}
        className="flex items-center gap-2 text-xs font-medium text-muted-foreground hover:text-foreground"
      >
        <span className="material-symbols-outlined text-[16px]" aria-hidden="true">{expanded ? 'expand_more' : 'chevron_right'}</span>
        {queue.length} {queue.length === 1 ? 'message' : 'messages'} waiting to be sent
      </button>
      {expanded && (
        <div className="mt-2 flex flex-col gap-1">
          {queue.map((message, index) => {
            const imageCount = message.attachments.filter((a) => String(a.type || '').startsWith('image/')).length;
            const isEditing = editingId === message.id;
            return (
              <div key={message.id} className="flex items-center gap-2 rounded-md border border-border bg-background px-2 py-1.5 text-sm">
                <span className="material-symbols-outlined text-[16px] text-muted-foreground" aria-hidden="true">
                  {index === 0 ? 'chevron_right' : 'list'}
                </span>
                <div className="min-w-0 flex-1">
                  {isEditing ? (
                    <textarea
                      value={editValue}
                      onChange={(event) => setEditValue(event.target.value)}
                      rows={1}
                      className="w-full resize-none rounded border border-border bg-background px-2 py-1 text-sm"
                      autoFocus
                    />
                  ) : (
                    <div className="truncate">{message.content}</div>
                  )}
                </div>
                {imageCount > 0 && <span className="shrink-0 text-xs text-muted-foreground">{imageCount} img</span>}
                {isEditing ? (
                  <>
                    <IconAction icon="check" label="Save queued message" onClick={() => saveEdit(message.id)} />
                    <IconAction icon="close" label="Cancel edit" onClick={() => { setEditingId(null); setEditValue(''); }} />
                  </>
                ) : (
                  <>
                    <IconAction icon="edit" label="Edit queued message" onClick={() => startEdit(message)} />
                    <IconAction icon="delete" label="Delete queued message" danger onClick={() => onDelete(message.id)} />
                  </>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

function IconAction({ icon, label, onClick, danger = false }: { icon: string; label: string; onClick: () => void; danger?: boolean }) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      onClick={onClick}
      className={`shrink-0 rounded p-1 hover:bg-accent ${danger ? 'text-destructive' : 'text-muted-foreground hover:text-foreground'}`}
    >
      <span className="material-symbols-outlined text-[16px]" aria-hidden="true">{icon}</span>
    </button>
  );
}
