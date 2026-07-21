import { useEffect, useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@ui/card';
import { Switch } from '@ui/switch';
import { settingsService } from '@services/settings.service';
import { SaveCancelActions } from '@patterns/SaveCancelActions';

export function DebugPage() {
  const [savedEnabled, setSavedEnabled] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const dirty = enabled !== savedEnabled;

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    settingsService.getPromptDebugMode()
      .then((res) => {
        if (cancelled) return;
        const next = Boolean(res.enabled);
        setSavedEnabled(next);
        setEnabled(next);
      })
      .catch(() => {
        if (cancelled) return;
        setSavedEnabled(false);
        setEnabled(false);
      })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  async function saveDebugMode() {
    if (loading || saving || !dirty) return;
    setSaving(true);
    try {
      const result = await settingsService.setPromptDebugMode(enabled);
      if (!result.success) throw new Error(result.error || 'Failed to save Prompt Debug Mode.');
      setSavedEnabled(enabled);
    } catch (err) {
      console.error(err instanceof Error ? err.message : 'Failed to save Prompt Debug Mode.');
    } finally {
      setSaving(false);
    }
  }

  function cancelDebugMode() {
    setEnabled(savedEnabled);
  }

  return (
    <div className="grid max-w-4xl gap-4">
      <Card className="rounded-lg">
        <CardHeader><CardTitle>Debug Mode</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex items-center justify-between gap-4 rounded-lg border border-border bg-secondary p-3 text-sm">
            <div className="flex min-w-0 flex-col gap-1">
              <span className="inline-flex items-center gap-2 font-semibold">
                <span className="material-symbols-outlined text-[18px]" aria-hidden="true">bug_report</span>
                Prompt Debug Mode
              </span>
              <span className="text-xs text-muted-foreground">
                Shows a collapsible prompt payload card in the chat for each LLM turn.
              </span>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <span className="min-w-8 text-right text-xs font-bold text-muted-foreground">{enabled ? 'ON' : 'OFF'}</span>
              <Switch
                aria-label="Toggle Prompt Debug Mode"
                checked={enabled}
                disabled={loading || saving}
                onCheckedChange={(next) => {
                  setEnabled(next);
                }}
              />
            </div>
          </div>
          <div className="rounded-lg border border-border bg-muted/40 px-3 py-2 text-xs leading-5 text-muted-foreground">
            When enabled, Agent Workspace shows the exact prompt payload sent to the LLM for each turn. This may include sensitive chat, vault, context, and file information.
          </div>
          <SaveCancelActions
            onCancel={cancelDebugMode}
            onSave={() => void saveDebugMode()}
            disabled={loading || saving || !dirty}
            saving={saving}
          />
        </CardContent>
      </Card>
    </div>
  );
}
