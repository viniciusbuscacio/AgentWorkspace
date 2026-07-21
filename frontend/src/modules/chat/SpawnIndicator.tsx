import { useState } from 'react';
import type { SubagentTask, SubagentTaskStatus } from './subagent-tasks';

function StatusIcon({ status }: { status: SubagentTaskStatus }) {
  if (status === 'running') {
    return <span className="size-2.5 animate-spin rounded-full border border-muted-foreground/40 border-t-primary" aria-hidden="true" />;
  }
  const icon = status === 'success' ? 'check_circle' : status === 'timeout' ? 'timer' : 'error';
  return <span className="material-symbols-outlined text-[15px]" aria-hidden="true">{icon}</span>;
}

/**
 * Live card for a batch of generic subagents (system.spawn), driven by the
 * backend `chat:subagent` events — ported from aw2's SpawnIndicator. Collapsed
 * by default like the QueuePanel: a summary row with a chevron; expanding shows
 * each task running -> success/timeout/error with its compact output.
 */
export function SpawnIndicator({ tasks }: { tasks: SubagentTask[] }) {
  const [expanded, setExpanded] = useState(false);
  if (!tasks.length) return null;
  const total = tasks.length;
  const done = tasks.filter((task) => task.status !== 'running').length;
  const running = done < total;
  const label = running
    ? `Running ${total} subagent${total > 1 ? 's' : ''}... (${done}/${total})`
    : `${total} subagent${total > 1 ? 's' : ''} completed`;

  return (
    <div className="mx-4 my-2 min-w-0 self-stretch rounded-[8px] border border-border bg-[var(--chat-composer,var(--bg-secondary))] px-3 py-2 text-xs text-muted-foreground shadow-sm">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        aria-expanded={expanded}
        className="flex w-full min-w-0 items-center gap-2 text-left font-medium text-foreground hover:text-primary"
      >
        <span className="material-symbols-outlined text-[16px]" aria-hidden="true">{expanded ? 'expand_more' : 'chevron_right'}</span>
        <span className="material-symbols-outlined text-[16px]" aria-hidden="true">account_tree</span>
        <span className="truncate">{label}</span>
        {running && !expanded && (
          <span className="ml-auto size-2.5 shrink-0 animate-spin rounded-full border border-muted-foreground/40 border-t-primary" aria-hidden="true" />
        )}
      </button>
      {expanded && (
        <div className="mt-1.5 flex flex-col gap-1">
          {tasks.map((task) => (
            <div key={task.id} className="min-w-0">
              <div className="flex min-w-0 items-start gap-2">
                <span className="mt-0.5 flex size-4 shrink-0 items-center justify-center text-primary">
                  <StatusIcon status={task.status} />
                </span>
                {/* Expanded means see EVERYTHING: the full task text the
                    worker received, never a truncated single line. */}
                <span className="min-w-0 flex-1 whitespace-pre-wrap break-words">{task.task}</span>
              </div>
              {task.output ? (
                <div className="ml-6 mt-1 max-h-60 overflow-auto whitespace-pre-wrap break-words rounded-md border border-border/60 bg-[var(--chat-surface,var(--bg-primary))] px-2 py-1 text-foreground">
                  {task.output}
                </div>
              ) : null}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
