import { useCallback, useEffect, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@ui/card';
import { Input } from '@ui/input';
import { Switch } from '@ui/switch';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { browserService, type BrowserStatusInfo, type BrowserTab } from '@services/browser.service';
import { ModuleHelpButton } from '@/components/patterns/ModuleHelpButton';

// Agent Browser — v1 status panel over the shared CDP core: running state,
// tab list, screenshot preview and start/stop. The agent drives the pages
// through the browser.* aw actions; this view is the user's window into it.
export function BrowserModule({ browserId, title }: { browserId: string; title: string }) {
  const browserName = browserId === 'browser-edge' ? 'Edge' : 'Chrome';
  const [status, setStatus] = useState<BrowserStatusInfo | null>(null);
  const [tabs, setTabs] = useState<BrowserTab[]>([]);
  const [shot, setShot] = useState('');
  const [busy, setBusy] = useState(false);
  // Saved baseline of the settings, plus their editable drafts: the toggle is
  // a draft persisted with Save (or reverted with Cancel), the same pattern
  // as the MCP/REST server settings.
  const [autostart, setAutostart] = useState(false);
  const [autostartDraft, setAutostartDraft] = useState(false);
  const [executable, setExecutable] = useState('');
  const [executableDraft, setExecutableDraft] = useState('');
  const [defaultExecutable, setDefaultExecutable] = useState('');

  const refresh = useCallback(async () => {
    const result = await browserService.status(browserId);
    if (!result.success) {
      setMessage(result.error || 'Could not read the browser status.');
      return;
    }
    setStatus(result.status ?? null);
    setAutostart(result.autostart);
    setAutostartDraft(result.autostart);
    const executableResult = await browserService.executable(browserId);
    if (executableResult.success) {
      setExecutable(executableResult.path || result.status?.binary || '');
      setExecutableDraft(executableResult.path || result.status?.binary || '');
      setDefaultExecutable(executableResult.defaultPath || result.status?.binary || '');
    }
    if (result.status?.running) {
      const tabsResult = await browserService.tabs(browserId);
      setTabs(tabsResult.success ? tabsResult.tabs ?? [] : []);
    } else {
      setTabs([]);
      setShot('');
    }
  }, [browserId]);

  useEffect(() => {
    void refresh();
    const timer = setInterval(() => void refresh(), 5000);
    return () => clearInterval(timer);
  }, [refresh]);

  async function start() {
    setBusy(true);
    setMessage('Connecting...');
    const result = await browserService.start(browserId);
    setMessage(result.success ? 'Connected.' : result.error || 'Could not connect.');
    setBusy(false);
    await refresh();
  }

  async function stop() {
    setBusy(true);
    const result = await browserService.stop(browserId);
    setMessage(result.success ? 'Browser stopped.' : result.error || 'Could not stop.');
    setBusy(false);
    await refresh();
  }

  async function saveSettings() {
    setBusy(true);
    try {
      if (executableDraft.trim() !== executable.trim()) {
        const executableResult = await browserService.setExecutable(browserId, executableDraft.trim());
        if (!executableResult.success) {
          setMessage(executableResult.error || 'Could not save the browser executable.');
          return;
        }
        setExecutable(executableResult.path || executableDraft.trim());
        setExecutableDraft(executableResult.path || executableDraft.trim());
        setDefaultExecutable(executableResult.defaultPath || defaultExecutable);
      }
      if (autostartDraft !== autostart) {
        const result = await browserService.setAutostart(browserId, autostartDraft);
        if (!result.success) {
          setMessage(result.error || 'Could not save the autostart setting.');
          return;
        }
      }
      // Commit the baseline from the draft we just persisted — not from a
      // stale status snapshot.
      setAutostart(autostartDraft);
      setMessage('Settings saved.');
    } finally {
      setBusy(false);
    }
  }

  function cancelSettings() {
    setAutostartDraft(autostart);
    setExecutableDraft(executable);
    setMessage('Changes discarded.');
  }

  async function browseExecutable() {
    setBusy(true);
    try {
      const result = await browserService.pickExecutable(browserId);
      if (result.canceled) return;
      if (!result.success) {
        setMessage(result.error || 'Could not choose browser executable.');
        return;
      }
      setExecutableDraft(result.path || '');
    } catch (err: unknown) {
      // The native picker binding is web-denylisted: over the web bridge the
      // call rejects instead of returning {success:false}. Type the path
      // manually there.
      setMessage(err instanceof Error ? err.message : 'The file picker is only available in the desktop app — type the path instead.');
    } finally {
      setBusy(false);
    }
  }

  async function takeScreenshot() {
    const result = await browserService.screenshot(browserId);
    if (!result.success) {
      setMessage(result.error || 'Could not capture the page.');
      return;
    }
    setShot(result.dataUri || '');
  }

  const running = Boolean(status?.running);
  const settingsDirty = autostartDraft !== autostart || executableDraft.trim() !== executable.trim();

  return (
    <section className="home-screen" data-awid={`${browserId}-screen`}>
      <div className="home-header">
        <div className="flex items-start justify-between gap-3">
          <h1 className="home-title">{title}</h1>
          <ModuleHelpButton module={title} />
        </div>
        <p className="home-subtitle">The agent&apos;s own {browserName === 'Edge' ? 'Microsoft Edge' : 'Google Chrome'} — an isolated profile, fully separate from your browser and tabs.</p>
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <Card className="rounded-lg">
          <CardHeader><CardTitle>Status</CardTitle></CardHeader>
          <CardContent className="flex flex-col gap-3">
            <div className="flex items-center gap-2 text-sm">
              <span
                className={`inline-block h-2.5 w-2.5 rounded-full ${running ? 'bg-emerald-500' : 'bg-muted-foreground/40'}`}
                aria-hidden="true"
              />
              <span>{running ? 'Connected' : 'Stopped'}</span>
            </div>
            {status && (
              <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
                <dt className="text-muted-foreground">CDP port</dt><dd>{status.port}</dd>
                <dt className="text-muted-foreground">Binary</dt>
                <dd className="min-w-0 break-words [overflow-wrap:anywhere]" title={status.binary}>{status.binary}</dd>
              </dl>
            )}
            <div className="flex flex-wrap gap-2">
              {running ? (
                <Button variant="destructive" icon="stop_circle" disabled={busy} onClick={() => void stop()}>Disconnect</Button>
              ) : (
                <Button icon="play_circle" disabled={busy} onClick={() => void start()}>Connect</Button>
              )}
              <Button variant="outline" icon="refresh" disabled={busy} onClick={() => void refresh()} aria-label="Refresh status">Refresh</Button>
              {running && (
                <Button variant="outline" icon="photo_camera" disabled={busy} onClick={() => void takeScreenshot()}>Screenshot</Button>
              )}
            </div>
            <div className="rounded-md border border-border p-3">
              <div className="mb-2 flex min-w-0 flex-col">
                <span className="text-sm">Browser executable</span>
                <span className="text-xs text-muted-foreground">Choose a custom {browserName} executable when the default path is wrong.</span>
              </div>
              <div className="flex flex-wrap gap-2">
                <Input
                  value={executableDraft}
                  onChange={(event) => setExecutableDraft(event.target.value)}
                  placeholder={defaultExecutable}
                  aria-label={`${browserName} executable path`}
                  className="min-w-40 flex-1 basis-64 font-mono text-xs"
                />
                <Button variant="outline" icon="folder_open" disabled={busy} onClick={() => void browseExecutable()}>Browse</Button>
                <Button variant="outline" icon="restart_alt" disabled={busy || !defaultExecutable} onClick={() => setExecutableDraft('')}>Reset</Button>
              </div>
            </div>
            <div className="flex items-center justify-between gap-3 rounded-md border border-border p-3">
              <div className="flex min-w-0 flex-col">
                <span className="text-sm">Connect automatically when the app starts</span>
                <span className="text-xs text-muted-foreground">When the vault unlocks, aw opens the agent&apos;s {browserName} window automatically.</span>
              </div>
              <Switch
                checked={autostartDraft}
                onCheckedChange={setAutostartDraft}
                aria-label="Connect automatically when the app starts"
                data-awid={`${browserId}-autostart`}
              />
            </div>
            <SaveCancelActions
              onSave={() => void saveSettings()}
              onCancel={cancelSettings}
              saving={busy}
              disabled={!settingsDirty}
              data-awid={`${browserId}-settings-save`}
              />
          </CardContent>
        </Card>
        <Card className="rounded-lg">
          <CardHeader><CardTitle>Tabs</CardTitle></CardHeader>
          <CardContent className="flex flex-col gap-2">
            {!running && <p className="text-sm text-muted-foreground">Start the browser to see its tabs.</p>}
            {running && tabs.length === 0 && <p className="text-sm text-muted-foreground">No open tabs.</p>}
            {tabs.map((tab) => (
              <div key={tab.id} className="rounded-md border border-border p-2">
                <div className="truncate text-sm font-medium">{tab.title || '(untitled)'}</div>
                <div className="truncate text-xs text-muted-foreground" title={tab.url}>{tab.url}</div>
              </div>
            ))}
            {shot && (
              <img src={shot} alt="Latest page screenshot" className="mt-2 w-full rounded-md border border-border" />
            )}
          </CardContent>
        </Card>
      </div>
    </section>
  );
}
