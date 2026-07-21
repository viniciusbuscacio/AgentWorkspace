import { useCallback, useEffect, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@ui/card';
import { Field, FieldLabel } from '@ui/field';
import { Input } from '@ui/input';
import { Switch } from '@ui/switch';
import { ZoomSafeSelect } from '@ui/zoom-safe-select';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { webaccessService, type WebBindCandidate, type WebServerStatus } from '@services/webaccess.service';
import { openSettingsPage } from '@/lib/open-settings';

const BIND_TAILSCALE = 'tailscale';
const BIND_MANUAL = 'manual';
const ALL_INTERFACES = '0.0.0.0';
const CUSTOM_BIND = '__custom__';

// kindLabel tags a detected interface address by reachability so the user can
// judge the exposure of a plain-HTTP bind at a glance.
function kindLabel(kind: string): string {
  switch (kind) {
    case 'tailscale': return 'Tailscale';
    case 'loopback': return 'loopback';
    case 'private': return 'LAN';
    case 'public': return 'public (exposed)';
    default: return kind;
  }
}

// WebAccessPage configures remote-browser access to the full app UI. It mirrors
// the MCP/REST server cards (ApiServerSettingsCards): Start now / Stop now and
// the signing-key reset take effect immediately, while port, bind mode, session
// TTL and the auto-start preference are edited as a draft and persisted with
// Save (or reverted with Cancel). The auto-start switch only saves the
// preference; it never starts or stops the live server on its own. In manual
// bind mode the bind address is chosen from the detected network interfaces
// (with "all interfaces" and a custom escape hatch), warning proportionally to
// how exposed a plain-HTTP bind would be.
export function WebAccessPage() {
  const [status, setStatus] = useState<WebServerStatus | null>(null);
  const [enabledDraft, setEnabledDraft] = useState(false);
  const [portDraft, setPortDraft] = useState('');
  const [bindDraft, setBindDraft] = useState(BIND_TAILSCALE);
  const [bindAddrDraft, setBindAddrDraft] = useState('');
  const [bindCustom, setBindCustom] = useState(false);
  const [cidrDraft, setCidrDraft] = useState('');
  const [ttlDraft, setTtlDraft] = useState('0');
  const [interfaces, setInterfaces] = useState<WebBindCandidate[]>([]);
  const [busy, setBusy] = useState(false);
  const [tlsLinkVisible, setTlsLinkVisible] = useState(false);

  const manual = bindDraft === BIND_MANUAL;

  const applyStatus = useCallback((next: WebServerStatus) => {
    setStatus(next);
    setEnabledDraft(Boolean(next.enabled));
    setPortDraft(String(next.port));
    setBindDraft(next.bindMode || BIND_TAILSCALE);
    setBindAddrDraft(next.bindAddr || '');
    setBindCustom(false);
    setCidrDraft((next.allowedCIDRs || []).join(', '));
    setTtlDraft(String(next.sessionTTLMinutes ?? 0));
  }, []);

  const refresh = useCallback(async () => {
    applyStatus(await webaccessService.getStatus());
  }, [applyStatus]);

  const loadInterfaces = useCallback(async () => {
    setInterfaces(await webaccessService.listInterfaces());
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);
  // Interfaces change as Wi-Fi/VPN come and go, so (re)load them whenever the
  // user enters manual mode rather than once at mount.
  useEffect(() => { if (manual) void loadInterfaces(); }, [manual, loadInterfaces]);

  const dirty = status
    ? enabledDraft !== Boolean(status.enabled) ||
      Number(portDraft) !== status.port ||
      bindDraft !== (status.bindMode || BIND_TAILSCALE) ||
      bindAddrDraft !== (status.bindAddr || '') ||
      cidrDraft !== (status.allowedCIDRs || []).join(', ') ||
      Number(ttlDraft) !== (status.sessionTTLMinutes ?? 0)
    : false;

  // toggleRunning starts/stops the live server now without touching the saved
  // auto-start preference or any unsaved draft (mirrors the MCP/REST Start/Stop).
  async function toggleRunning() {
    if (!status) return;
    setBusy(true);
    try {
      const next = status.running ? await webaccessService.stop() : await webaccessService.start();
      setStatus(next);
      setMessage(next.error || (next.running ? 'Web server started.' : 'Web server stopped.'));
    } finally {
      setBusy(false);
    }
  }

  // HTTPS takes effect immediately, restarting a running server. Enabling with
  // no certificate is refused by the backend with an actionable error.
  async function toggleTLS(checked: boolean) {
    if (!status) return;
    setBusy(true);
    setTlsLinkVisible(false);
    try {
      const next = await webaccessService.setTLSEnabled(checked);
      setStatus(next);
      if (next.error) {
        setMessage(next.error);
        setTlsLinkVisible(checked);
      } else {
        setMessage(next.tlsEnabled ? 'HTTPS enabled for web access.' : 'HTTPS disabled for web access.');
      }
    } finally {
      setBusy(false);
    }
  }

  async function save() {
    if (!status) return;
    const port = Number(portDraft);
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      setMessage('Port must be an integer between 1 and 65535.');
      return;
    }
    if (bindDraft === BIND_MANUAL && bindAddrDraft.trim() === '') {
      setMessage('Manual bind mode needs a bind address — pick an interface or enter one.');
      return;
    }
    const ttl = Number(ttlDraft);
    if (!Number.isInteger(ttl) || ttl < 0) {
      setMessage('Session timeout must be 0 (no expiry) or a positive number of minutes.');
      return;
    }
    setBusy(true);
    try {
      let next = status;
      if (port !== status.port) {
        next = await webaccessService.setPort(port);
        if (next.error) { setStatus(next); setMessage(next.error); return; }
      }
      const cidrs = cidrDraft.split(',').map((c) => c.trim()).filter(Boolean);
      if (bindDraft !== (status.bindMode || BIND_TAILSCALE) ||
          bindAddrDraft !== (status.bindAddr || '') ||
          cidrDraft !== (status.allowedCIDRs || []).join(', ')) {
        next = await webaccessService.setBindMode(bindDraft, bindAddrDraft, cidrs);
        if (next.error) { setStatus(next); setMessage(next.error); return; }
      }
      if (ttl !== (status.sessionTTLMinutes ?? 0)) {
        next = await webaccessService.setSessionTTL(ttl);
        if (next.error) { setStatus(next); setMessage(next.error); return; }
      }
      if (enabledDraft !== Boolean(status.enabled)) {
        next = await webaccessService.setEnabled(enabledDraft);
        if (next.error) { setStatus(next); setMessage(next.error); return; }
      }
      applyStatus(next);
      setMessage('Settings saved.');
    } finally {
      setBusy(false);
    }
  }

  function cancel() {
    if (status) applyStatus(status);
    setMessage('Changes discarded.');
  }

  async function regenerate() {
    setBusy(true);
    try {
      const next = await webaccessService.regenerateSessionKey();
      setStatus(next);
      setMessage(next.error || 'Signing key rotated. All web sessions were signed out.');
    } finally {
      setBusy(false);
    }
  }

  function onBindSelect(value: string) {
    if (value === CUSTOM_BIND) {
      setBindCustom(true);
      return; // reveal the text input; keep whatever address is drafted
    }
    setBindCustom(false);
    setBindAddrDraft(value);
  }

  const running = Boolean(status?.running);
  const knownBindIP = bindAddrDraft === ALL_INTERFACES || interfaces.some((i) => i.ip === bindAddrDraft);
  const showCustomInput = bindCustom || (bindAddrDraft !== '' && !knownBindIP);
  const selectValue = showCustomInput ? CUSTOM_BIND : bindAddrDraft;
  const selectedKind =
    bindAddrDraft === ALL_INTERFACES ? 'all'
      : (interfaces.find((i) => i.ip === bindAddrDraft)?.kind ?? '');

  return (
    <div className="grid gap-4 xl:grid-cols-2">
      <Card className="rounded-lg">
        <CardHeader><CardTitle>Web access</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            Serve the full Agent Workspace UI to a remote browser. Recommended over the
            Tailscale interface: HTTP rides Tailscale&apos;s WireGuard encryption and only
            tailnet peers are accepted. Log in with your vault password.
          </p>
          <div className="flex items-center gap-2 text-sm">
            <span
              className={`inline-block h-2 w-2 rounded-full ${running ? 'bg-emerald-500' : 'bg-muted-foreground/40'}`}
              aria-hidden="true"
            />
            <span data-awid="web-server-state">{running ? 'Running' : 'Stopped'}</span>
          </div>
          {status?.error && <p className="text-sm text-destructive">{status.error}</p>}
          {bindDraft === BIND_TAILSCALE && status && !status.tailscaleDetected && (
            <p className="text-sm text-amber-500">
              No Tailscale interface detected (no 100.64.0.0/10 address). The server cannot
              start in Tailscale mode until Tailscale is up, or switch to manual bind below.
            </p>
          )}
          <Field>
            <FieldLabel>URL</FieldLabel>
            <code className="min-w-0 truncate rounded-md border border-border bg-muted/40 px-2 py-1.5 text-sm">
              {status?.url || (() => {
                const scheme = status?.tlsEnabled ? 'https' : 'http';
                return status?.tailscaleIp ? `${scheme}://${status.tailscaleIp}:${portDraft}` : `${scheme}://<host>:${portDraft}`;
              })()}
            </code>
          </Field>
          <Button
            icon={running ? 'stop_circle' : 'play_circle'}
            variant={running ? 'destructive' : 'default'}
            disabled={busy || !status}
            onClick={toggleRunning}
            data-awid="web-server-toggle"
          >
            {running ? 'Stop now' : 'Start now'}
          </Button>
          <div className="flex items-center justify-between gap-3 rounded-md border border-border p-3">
            <div className="flex min-w-0 flex-col">
              <span className="text-sm">Enable HTTPS (TLS)</span>
              <span className="text-xs text-muted-foreground">Serves over the shared TLS certificate. A self-signed certificate still shows a browser warning until trusted.</span>
            </div>
            <Switch
              checked={Boolean(status?.tlsEnabled)}
              disabled={busy || !status}
              onCheckedChange={(checked) => void toggleTLS(checked)}
              aria-label="Enable HTTPS for web access"
              data-awid="web-server-tls"
            />
          </div>
          {tlsLinkVisible && (
            <button
              type="button"
              className="self-start text-sm text-primary underline-offset-2 hover:underline"
              onClick={() => openSettingsPage('tls')}
              data-awid="web-server-tls-link"
            >
              Open the TLS manager
            </button>
          )}
          {status?.tlsEnabled && status?.coverageWarning && (
            <p className="text-sm text-amber-500" data-awid="web-server-tls-coverage">{status.coverageWarning}</p>
          )}

          <Field>
            <FieldLabel>Port</FieldLabel>
            <Input
              type="number"
              min={1}
              max={65535}
              value={portDraft}
              onChange={(e) => setPortDraft(e.target.value)}
              className="w-32"
              aria-label="Web server port"
              data-awid="web-server-port"
            />
          </Field>
          <Field>
            <FieldLabel>Bind mode</FieldLabel>
            <ZoomSafeSelect
              value={bindDraft}
              onValueChange={setBindDraft}
              options={[
                { value: BIND_TAILSCALE, label: 'Tailscale (recommended)' },
                { value: BIND_MANUAL, label: 'Manual (advanced)' },
              ]}
              aria-label="Bind mode"
            />
          </Field>
          {manual && (
            <>
              <p className="text-sm text-amber-500">
                Manual bind serves plain HTTP with no Tailscale enforcement. Anyone who can
                reach this address can attempt to log in — protect it with a firewall/VPN.
              </p>
              <Field>
                <FieldLabel>
                  <span className="flex items-center justify-between gap-2">
                    Bind address
                    <button
                      type="button"
                      className="text-xs text-muted-foreground underline-offset-2 hover:underline"
                      onClick={() => void loadInterfaces()}
                      data-awid="web-server-iface-refresh"
                    >
                      Refresh interfaces
                    </button>
                  </span>
                </FieldLabel>
                <ZoomSafeSelect
                  value={selectValue}
                  onValueChange={onBindSelect}
                  options={[
                    { value: '', label: 'Select an interface…' },
                    ...interfaces.map((iface) => ({
                      value: iface.ip,
                      label: `${iface.iface} — ${iface.ip} · ${kindLabel(iface.kind)}`,
                    })),
                    { value: ALL_INTERFACES, label: 'All interfaces (0.0.0.0)' },
                    { value: CUSTOM_BIND, label: 'Custom address…' },
                  ]}
                  aria-label="Bind address"
                />
              </Field>
              {showCustomInput && (
                <Input
                  value={bindAddrDraft}
                  onChange={(e) => setBindAddrDraft(e.target.value)}
                  placeholder="127.0.0.1"
                  aria-label="Custom bind address"
                  data-awid="web-server-bindaddr"
                />
              )}
              {selectedKind === 'public' && (
                <p className="text-sm text-destructive">
                  {bindAddrDraft} is a publicly routable address. Serving plain HTTP here exposes
                  the UI to the open internet — strongly prefer Tailscale, or front it with a
                  reverse proxy that terminates TLS.
                </p>
              )}
              {selectedKind === 'all' && (
                <p className="text-sm text-destructive">
                  0.0.0.0 binds every interface, including any public one. Only do this behind a
                  firewall or VPN, and set Allowed CIDRs below to fence who can connect.
                </p>
              )}
              <Field>
                <FieldLabel>Allowed CIDRs (optional, comma-separated)</FieldLabel>
                <Input
                  value={cidrDraft}
                  onChange={(e) => setCidrDraft(e.target.value)}
                  placeholder="192.168.1.0/24, 10.0.0.0/8"
                  aria-label="Allowed CIDRs"
                  data-awid="web-server-cidrs"
                />
              </Field>
            </>
          )}
          <Field>
            <FieldLabel>Session timeout (minutes, 0 = until lock)</FieldLabel>
            <Input
              type="number"
              min={0}
              value={ttlDraft}
              onChange={(e) => setTtlDraft(e.target.value)}
              className="w-32"
              aria-label="Session timeout in minutes"
              data-awid="web-server-ttl"
            />
          </Field>
          <div className="flex items-center justify-between gap-3 rounded-md border border-border p-3">
            <div className="flex min-w-0 flex-col">
              <span className="text-sm">Auto-start web access when the app starts</span>
              <span className="text-xs text-muted-foreground">Starts automatically on launch and stays up across vault lock/unlock. Use Start now to run it without saving.</span>
            </div>
            <Switch checked={enabledDraft} disabled={!status} onCheckedChange={setEnabledDraft} aria-label="Auto-start web access when the app starts" data-awid="web-server-enabled" />
          </div>
          <SaveCancelActions
            onSave={save}
            onCancel={cancel}
            saving={busy}
            disabled={!status || !dirty}
            data-awid="web-server-save"
            />
        </CardContent>
      </Card>

      <Card className="rounded-lg">
        <CardHeader><CardTitle>Sessions</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            Sessions always end when the vault locks or auto-locks. Set a session timeout in
            minutes on the left (0 keeps a session alive until the vault locks). Rotating the
            signing key below signs out every browser immediately.
          </p>
          <Button variant="outline" icon="autorenew" disabled={busy} onClick={regenerate} data-awid="web-server-regen">
            Reset signing key &amp; sign out all
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
