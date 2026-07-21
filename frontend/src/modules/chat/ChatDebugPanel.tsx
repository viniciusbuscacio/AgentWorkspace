import { useCallback, useEffect, useState } from 'react';
import type { LLMTurn } from '@services/chat-debug.service';
import { chatDebugService } from '@services/chat-debug.service';
import { Button } from '@ui/button';

interface ChatDebugPanelProps {
  chatId: string | undefined;
  open: boolean;
  onClose: () => void;
}

// prettyJson pretty-prints a raw JSON string, falling back to the raw text when
// it is not valid JSON. Everything is rendered as plain text in <pre> elements,
// so there is no HTML injection surface (no dangerouslySetInnerHTML).
function prettyJson(raw: string | undefined): string {
  if (!raw) return '';
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function usageLabel(turn: LLMTurn): string {
  const parts: string[] = [];
  if (turn.model) parts.push(turn.model);
  if (turn.promptTokens || turn.completionTokens) {
    parts.push(`${turn.promptTokens ?? 0}→${turn.completionTokens ?? 0} tok`);
  }
  if (turn.finishReason) parts.push(turn.finishReason);
  return parts.join(' · ');
}

export function ChatDebugPanel({ chatId, open, onClose }: ChatDebugPanelProps) {
  const [turns, setTurns] = useState<LLMTurn[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!chatId) {
      setTurns([]);
      return;
    }
    setLoading(true);
    setError('');
    try {
      const loaded = await chatDebugService.listChatTurns(chatId);
      setTurns([...loaded].reverse());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load turns');
    } finally {
      setLoading(false);
    }
  }, [chatId]);

  useEffect(() => {
    if (open) void load();
  }, [open, load]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex justify-end" role="dialog" aria-label="Prompt debug">
      <button
        type="button"
        aria-label="Close debug overlay"
        className="absolute inset-0 bg-black/40"
        onClick={onClose}
      />
      <aside className="relative flex h-full w-full max-w-xl flex-col border-l border-border bg-[var(--chat-surface)] shadow-xl">
        <header className="flex h-14 shrink-0 items-center justify-between border-b border-border px-4">
          <div className="min-w-0">
            <h2 className="text-sm font-semibold">Prompt debug</h2>
            <p className="truncate text-xs text-muted-foreground">
              {turns.length} turn{turns.length === 1 ? '' : 's'} sent to the model
            </p>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" icon="refresh" onClick={() => void load()}>Refresh</Button>
            <Button variant="outline" size="sm" icon="close" onClick={onClose}>Close</Button>
          </div>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {error && (
            <div className="mb-3 rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </div>
          )}
          {loading && <p className="text-sm text-muted-foreground">Loading…</p>}
          {!loading && turns.length === 0 && !error && (
            <p className="text-sm text-muted-foreground">No turns recorded for this chat yet.</p>
          )}
          <ul className="flex flex-col gap-3">
            {turns.map((turn) => (
              <li key={turn.id} className="rounded-md border border-border">
                <details>
                  <summary className="cursor-pointer list-none px-3 py-2 text-sm font-medium">
                    <span className="text-muted-foreground">#{turn.turnIndex}</span>{' '}
                    {usageLabel(turn) || 'turn'}
                  </summary>
                  <div className="flex flex-col gap-3 border-t border-border px-3 py-3 text-xs">
                    {turn.responseText && (
                      <section>
                        <h3 className="mb-1 font-semibold uppercase text-muted-foreground">Response</h3>
                        <pre className="whitespace-pre-wrap break-words rounded bg-muted/40 p-2">{turn.responseText}</pre>
                      </section>
                    )}
                    {turn.toolCallsJson && turn.toolCallsJson !== '[]' && (
                      <section>
                        <h3 className="mb-1 font-semibold uppercase text-muted-foreground">Tool calls</h3>
                        <pre className="whitespace-pre-wrap break-words rounded bg-muted/40 p-2">{prettyJson(turn.toolCallsJson)}</pre>
                      </section>
                    )}
                    <details>
                      <summary className="cursor-pointer font-semibold uppercase text-muted-foreground">Raw request</summary>
                      <pre className="mt-1 max-h-96 overflow-auto whitespace-pre-wrap break-words rounded bg-muted/40 p-2">{prettyJson(turn.requestJson)}</pre>
                    </details>
                  </div>
                </details>
              </li>
            ))}
          </ul>
        </div>
      </aside>
    </div>
  );
}
