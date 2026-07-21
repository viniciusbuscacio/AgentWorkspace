import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@ui/card';
import { Field, FieldLabel } from '@ui/field';
import { Input } from '@ui/input';
import { PasswordInput } from '@ui/password-input';
import { Switch } from '@ui/switch';
import type { ApiServerService, ApiServerStatus } from '@services/mcp.service';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { openSettingsPage } from '@/lib/open-settings';

interface ApiServerSettingsCardsProps {
  service: ApiServerService;
  serverTitle: string;
  description: ReactNode;
  /** Used in data-awid attributes: "mcp-server" / "rest-server". */
  awid: string;
  /** Builds the displayed endpoint URL when the server is stopped. */
  endpointForPort: (port: number) => string;
}

// ApiServerSettingsCards is the shared settings UI of the local API servers
// (MCP, REST). Start/Stop and the token actions take effect immediately;
// port and auto-start are edited as a draft and persisted with Save (or
// reverted with Cancel).
export function ApiServerSettingsCards({
  service,
  serverTitle,
  description,
  awid,
  endpointForPort,
}: ApiServerSettingsCardsProps) {
  const [status, setStatus] = useState<ApiServerStatus | null>(null);
  const [portDraft, setPortDraft] = useState('');
  const [autostartDraft, setAutostartDraft] = useState(false);
  const [token, setToken] = useState('');
  const [busy, setBusy] = useState(false);

  // applyStatus refreshes the saved baseline and resets the draft to it.
  const applyStatus = useCallback((next: ApiServerStatus) => {
    setStatus(next);
    setPortDraft(String(next.port));
    setAutostartDraft(next.autostart);
  }, []);

  const refresh = useCallback(async () => {
    applyStatus(await service.getStatus());
    const result = await service.getToken();
    setToken(result.success ? result.value || '' : '');
  }, [service, applyStatus]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const portDirty = status ? Number(portDraft) !== status.port : false;
  const autostartDirty = status ? autostartDraft !== status.autostart : false;
  const dirty = portDirty || autostartDirty;

  async function toggleRunning() {
    if (!status) return;
    setBusy(true);
    try {
      const wasRunning = status.running;
      const next = wasRunning ? await service.stop() : await service.start();
      // Keep unsaved drafts: only refresh the baseline fields.
      setStatus(next);
      setMessage(next.error || (next.running ? `${serverTitle} started.` : `${serverTitle} stopped.`));
    } finally {
      setBusy(false);
    }
  }

  async function save() {
    if (!status) return;
    const parsed = Number(portDraft);
    if (portDirty && (!Number.isInteger(parsed) || parsed < 1 || parsed > 65535)) {
      setMessage('Port must be an integer between 1 and 65535.');
      return;
    }
    setBusy(true);
    try {
      let next = status;
      if (portDirty) {
        next = await service.setPort(parsed);
        if (next.error) {
          setStatus(next);
          setMessage(next.error);
          return;
        }
      }
      if (autostartDirty) {
        next = await service.setAutostart(autostartDraft);
        if (next.error) {
          setStatus(next);
          setMessage(next.error);
          return;
        }
      }
      applyStatus(next);
      setMessage(
        portDirty && next.running
          ? `Settings saved: server restarted on port ${next.port}.`
          : 'Settings saved.',
      );
    } finally {
      setBusy(false);
    }
  }

  function cancel() {
    if (!status) return;
    setPortDraft(String(status.port));
    setAutostartDraft(status.autostart);
    setMessage('Changes discarded.');
  }

  // HTTPS takes effect immediately (like Start/Stop), restarting a running
  // server. Enabling with no certificate is refused by the backend with an
  // actionable error; we surface a link to the TLS manager.
  const [tlsLinkVisible, setTlsLinkVisible] = useState(false);
  async function toggleTLS(checked: boolean) {
    if (!status) return;
    setBusy(true);
    setTlsLinkVisible(false);
    try {
      const next = await service.setTLSEnabled(checked);
      setStatus(next);
      if (next.error) {
        setMessage(next.error);
        setTlsLinkVisible(checked);
      } else {
        setMessage(next.tlsEnabled ? `HTTPS enabled for ${serverTitle}.` : `HTTPS disabled for ${serverTitle}.`);
      }
    } finally {
      setBusy(false);
    }
  }

  async function regenerateToken() {
    setBusy(true);
    try {
      const result = await service.regenerateToken();
      if (result.success) {
        setToken(result.value || '');
        setMessage('Token regenerated. Connected clients must reconnect with the new token.');
      } else {
        setMessage(result.error || 'Failed to regenerate token.');
      }
      setStatus(await service.getStatus());
    } finally {
      setBusy(false);
    }
  }

  async function copyToClipboard(value: string, label: string) {
    try {
      await navigator.clipboard.writeText(value);
      setMessage(`${label} copied to clipboard.`);
    } catch {
      setMessage(`Could not copy ${label.toLowerCase()} to clipboard.`);
    }
  }

  const running = Boolean(status?.running);
  const url = status?.url || endpointForPort(status?.port ?? (Number(portDraft) || 0));

  return (
    <div className="grid gap-4 xl:grid-cols-2">
      <Card className="rounded-lg">
        <CardHeader><CardTitle>{serverTitle}</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">{description}</p>
          <div className="flex items-center gap-2 text-sm">
            <span
              className={`inline-block h-2 w-2 rounded-full ${running ? 'bg-emerald-500' : 'bg-muted-foreground/40'}`}
              aria-hidden="true"
            />
            <span data-awid={`${awid}-state`}>{running ? 'Running' : 'Stopped'}</span>
          </div>
          {status?.error && (
            <p className="text-sm text-destructive">
              {status.error.includes('Agent Firewall') ? (
                <>
                  {status.error.replace(/ Add one in Settings.*$/, '')}{' '}
                  <button
                    type="button"
                    className="cursor-pointer border-0 bg-transparent p-0 font-medium text-destructive underline underline-offset-2"
                    onClick={() => openSettingsPage('firewall')}
                  >
                    Add one in Settings → Agent Firewall
                  </button>
                </>
              ) : (
                status.error
              )}
            </p>
          )}
          <Field>
            <FieldLabel>Endpoint</FieldLabel>
            <div className="flex items-center gap-2">
              <code className="min-w-0 flex-1 truncate rounded-md border border-border bg-muted/40 px-2 py-1.5 text-sm">{url}</code>
              <Button variant="outline" size="sm" icon="content_copy" onClick={() => copyToClipboard(url, 'Endpoint')}>
                Copy
              </Button>
            </div>
          </Field>
          <Button
            icon={running ? 'stop_circle' : 'play_circle'}
            variant={running ? 'destructive' : 'default'}
            disabled={busy || !status}
            onClick={toggleRunning}
            data-awid={`${awid}-toggle`}
          >
            {running ? 'Stop' : 'Start'}
          </Button>
          <Field>
            <FieldLabel>Port</FieldLabel>
            <Input
              type="number"
              min={1}
              max={65535}
              value={portDraft}
              onChange={(event) => setPortDraft(event.target.value)}
              className="w-32"
              aria-label={`${serverTitle} port`}
              data-awid={`${awid}-port`}
            />
          </Field>
          <div className="flex items-center justify-between gap-3 rounded-md border border-border p-3">
            <div className="flex min-w-0 flex-col">
              <span className="text-sm">Enable HTTPS (TLS)</span>
              <span className="text-xs text-muted-foreground">Serves over the shared TLS certificate. Clients must switch to https:// and may need to trust a self-signed certificate.</span>
            </div>
            <Switch
              checked={Boolean(status?.tlsEnabled)}
              disabled={busy || !status}
              onCheckedChange={(checked) => void toggleTLS(checked)}
              aria-label={`Enable HTTPS for ${serverTitle}`}
              data-awid={`${awid}-tls`}
            />
          </div>
          {tlsLinkVisible && (
            <button
              type="button"
              className="self-start text-sm text-primary underline-offset-2 hover:underline"
              onClick={() => openSettingsPage('tls')}
              data-awid={`${awid}-tls-link`}
            >
              Open the TLS manager
            </button>
          )}
          {status?.tlsEnabled && status?.coverageWarning && (
            <p className="text-sm text-amber-500" data-awid={`${awid}-tls-coverage`}>{status.coverageWarning}</p>
          )}
          <div className="flex items-center justify-between gap-3 rounded-md border border-border p-3">
            <div className="flex min-w-0 flex-col">
              <span className="text-sm">Auto-start {serverTitle} when the app starts</span>
              <span className="text-xs text-muted-foreground">Starts automatically as soon as the vault is unlocked.</span>
            </div>
            <Switch
              checked={autostartDraft}
              disabled={!status}
              onCheckedChange={setAutostartDraft}
              aria-label={`Auto-start ${serverTitle} when the app starts`}
              data-awid={`${awid}-autostart`}
            />
          </div>
          <SaveCancelActions
            onSave={save}
            onCancel={cancel}
            saving={busy}
            disabled={!status || !dirty}
            data-awid={`${awid}-save`}
            />
        </CardContent>
      </Card>
      <Card className="rounded-lg">
        <CardHeader><CardTitle>Bearer token</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            Sent as <code>Authorization: Bearer &lt;token&gt;</code>; regenerating disconnects every client.
          </p>
          <Field>
            <FieldLabel>Token</FieldLabel>
            <PasswordInput value={token} readOnly />
          </Field>
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              icon="content_copy"
              disabled={!token}
              onClick={() => copyToClipboard(token, 'Token')}
            >
              Copy
            </Button>
            <Button variant="outline" icon="autorenew" disabled={busy} onClick={regenerateToken}>
              Regenerate
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
