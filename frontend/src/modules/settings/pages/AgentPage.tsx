import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { Card, CardContent, CardHeader, CardTitle } from '@ui/card';
import { ZoomSafeSelect } from '@ui/zoom-safe-select';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { settingsService } from '@services/settings.service';

type SubagentMode = 'off' | 'balanced' | 'aggressive';

const modes: Array<{ value: SubagentMode; label: string; description: string }> = [
  { value: 'off', label: 'Off', description: 'No automatic subagent delegation.' },
  { value: 'balanced', label: 'Balanced', description: 'Use generic subagents only when the benefit is clear.' },
  { value: 'aggressive', label: 'Aggressive', description: 'For large tasks, use up to 5 parallel generic subagents.' },
];

function normalizeMode(value: string | undefined): SubagentMode {
  if (value === 'off' || value === 'aggressive') return value;
  return 'balanced';
}

export function AgentPage({ onBack }: { onBack?: () => void } = {}) {
  const [savedMode, setSavedMode] = useState<SubagentMode>('balanced');
  const [mode, setMode] = useState<SubagentMode>('balanced');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const dirty = mode !== savedMode;
  const selected = modes.find((item) => item.value === mode) ?? modes[1];

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    settingsService.getSubagentMode()
      .then((res) => {
        if (cancelled) return;
        const next = normalizeMode(res.mode);
        setSavedMode(next);
        setMode(next);
      })
      .catch(() => {
        if (cancelled) return;
        setSavedMode('balanced');
        setMode('balanced');
      })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  async function saveMode() {
    if (loading || saving || !dirty) return;
    setSaving(true);
    try {
      const result = await settingsService.setSubagentMode(mode);
      if (!result.success) throw new Error(result.error || 'Failed to save subagent mode.');
      setSavedMode(mode);
      toast('Subagent mode saved.');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save subagent mode.');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="grid max-w-4xl gap-4">
      <Card className="rounded-lg">
        <CardHeader><CardTitle>Subagents</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex items-center justify-between gap-4 rounded-lg border border-border bg-secondary p-3 text-sm">
            <div className="flex min-w-0 flex-col gap-1">
              <span className="inline-flex items-center gap-2 font-semibold">
                <span className="material-symbols-outlined text-[18px]" aria-hidden="true">account_tree</span>
                Generic subagents
              </span>
              <span className="text-xs text-muted-foreground">{selected.description}</span>
            </div>
            <ZoomSafeSelect
              aria-label="Subagent mode"
              className="w-[150px]"
              value={mode}
              onValueChange={(next) => setMode(normalizeMode(next))}
              disabled={loading || saving}
              options={modes.map((item) => ({ value: item.value, label: item.label }))}
            />
          </div>
          <div className="rounded-lg border border-border bg-muted/40 px-3 py-2 text-xs leading-5 text-muted-foreground">
            Subagents are generic, receive small scoped tasks, and work under the main agent. They do not create branches, commits, worktrees, threads, or PRs.
          </div>
          <SaveCancelActions
            onCancel={() => { setMode(savedMode); onBack?.(); }}
            onSave={() => void saveMode()}
            disabled={loading || saving}
            saving={saving}
          />
        </CardContent>
      </Card>
    </div>
  );
}
