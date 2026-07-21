import { useCallback, useEffect, useRef, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@ui/card';
import { Field, FieldLabel } from '@ui/field';
import { FileInput } from '@ui/file-input';
import { serverTlsService, type ServerTLSStatus } from '@services/servertls.service';

// ServerTlsPage is the shared TLS manager (Settings/Security). The user manages
// ONE certificate here — app-managed self-signed, or a custom PEM chain + key —
// and each server (Web/MCP/REST) turns HTTPS on/off independently against it.
// A self-signed certificate encrypts the connection but does not remove the
// browser warning: to clear it, trust the certificate locally or use your own
// trusted certificate and connect via a name it covers.
export function ServerTlsPage() {
  const [status, setStatus] = useState<ServerTLSStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const certInputRef = useRef<HTMLInputElement>(null);
  const keyInputRef = useRef<HTMLInputElement>(null);
  const [certName, setCertName] = useState('');
  const [keyName, setKeyName] = useState('');
  const [certPEM, setCertPEM] = useState('');
  const [keyPEM, setKeyPEM] = useState('');

  const refresh = useCallback(async () => {
    setStatus(await serverTlsService.getStatus());
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  function clearUpload() {
    setCertName(''); setKeyName(''); setCertPEM(''); setKeyPEM('');
    if (certInputRef.current) certInputRef.current.value = '';
    if (keyInputRef.current) keyInputRef.current.value = '';
  }

  async function createSelfSigned() {
    setBusy(true);
    try {
      const next = await serverTlsService.createSelfSigned();
      setStatus(next);
      setMessage(next.error || 'Self-signed certificate created. Servers running with TLS were restarted.');
    } finally { setBusy(false); }
  }

  async function regenerate() {
    setBusy(true);
    try {
      const next = await serverTlsService.regenerateSelfSigned();
      setStatus(next);
      setMessage(next.error || 'Certificate regenerated — browsers may prompt to trust it again.');
    } finally { setBusy(false); }
  }

  // File contents are read in the frontend (works over remote Web Access too,
  // with no native dialog). Only the file names are shown — never the contents.
  async function onPickCert(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) { setCertName(''); setCertPEM(''); return; }
    setCertName(file.name);
    setCertPEM(await file.text());
  }
  async function onPickKey(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) { setKeyName(''); setKeyPEM(''); return; }
    setKeyName(file.name);
    setKeyPEM(await file.text());
  }

  async function installCustom() {
    if (!certPEM || !keyPEM) return;
    setBusy(true);
    try {
      const next = await serverTlsService.installCustom(certPEM, keyPEM);
      setStatus(next);
      if (next.error) {
        setMessage(next.error); // sanitized; the current certificate is kept
      } else {
        setMessage('Custom certificate installed. Servers running with TLS were restarted.');
        clearUpload();
      }
    } finally { setBusy(false); }
  }

  const hasCert = Boolean(status?.hasCertificate);

  return (
    <div className="grid gap-4 xl:grid-cols-2">
      <Card className="rounded-lg xl:col-span-2">
        <CardHeader><CardTitle>Certificate</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            One certificate is shared by the Web, MCP and REST servers. Turn HTTPS on per
            server from each server&apos;s settings. TLS is defense in depth — it encrypts the
            connection but does not replace the vault password, peer filtering or bearer tokens.
          </p>
          {!hasCert && (
            <p className="text-sm text-amber-500" data-awid="server-tls-none">
              No certificate yet. Create a self-signed certificate below, or upload your own,
              before enabling HTTPS on any server.
            </p>
          )}
          {hasCert && status && (
            <div className="flex flex-col gap-1 rounded-md border border-border p-3 text-sm" data-awid="server-tls-info">
              <Row label="Mode" value={status.mode === 'custom' ? 'Custom (uploaded)' : 'Self-signed (app-managed)'} />
              <Row label="Subject" value={status.subject || '—'} />
              <Row label="Issuer" value={status.issuer || '—'} />
              <Row label="Valid from" value={formatDate(status.notBefore)} />
              <Row label="Valid until" value={formatDate(status.notAfter)} />
              <Row label="DNS names" value={(status.dnsNames || []).join(', ') || '—'} />
              <Row label="IP addresses" value={(status.ipAddresses || []).join(', ') || '—'} />
              <Row label="SHA-256" value={status.fingerprintSha256 || '—'} mono />
              {status.expiringSoon && (
                <p className="mt-1 text-amber-500">This certificate expires soon — regenerate or replace it.</p>
              )}
            </div>
          )}
          {status?.error && <p className="text-sm text-destructive" data-awid="server-tls-error">{status.error}</p>}
        </CardContent>
      </Card>

      <Card className="rounded-lg">
        <CardHeader><CardTitle>Self-signed</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            App-managed certificate covering this machine&apos;s local names and addresses
            (loopback, hostname, Tailscale IP). Self-signed certificates are untrusted by
            default: the browser will warn until you trust it locally.
          </p>
          {!hasCert ? (
            <Button icon="add_moderator" disabled={busy} onClick={() => void createSelfSigned()} data-awid="server-tls-create">
              Create self-signed certificate
            </Button>
          ) : (
            <Button variant="outline" icon="autorenew" disabled={busy} onClick={() => void regenerate()} data-awid="server-tls-regenerate">
              Regenerate
            </Button>
          )}
        </CardContent>
      </Card>

      <Card className="rounded-lg">
        <CardHeader><CardTitle>Custom certificate</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            Upload your own PEM certificate chain (leaf first) and an unencrypted PEM private
            key. The pair is validated before it replaces the current certificate.
          </p>
          <Field>
            <FieldLabel>Certificate chain (.pem / .crt)</FieldLabel>
            <FileInput
              ref={certInputRef}
              accept=".pem,.crt,.cer,application/x-pem-file"
              onChange={(e) => void onPickCert(e)}
              data-awid="server-tls-cert-file"
            />
            {certName && <span className="text-xs text-muted-foreground">{certName}</span>}
          </Field>
          <Field>
            <FieldLabel>Private key (.pem / .key)</FieldLabel>
            <FileInput
              ref={keyInputRef}
              accept=".pem,.key"
              onChange={(e) => void onPickKey(e)}
              data-awid="server-tls-key-file"
            />
            {keyName && <span className="text-xs text-muted-foreground">{keyName}</span>}
          </Field>
          <Button
            icon="upload_file"
            disabled={busy || !certPEM || !keyPEM}
            onClick={() => void installCustom()}
            data-awid="server-tls-install"
          >
            Validate and install
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex gap-2">
      <span className="w-28 shrink-0 text-muted-foreground">{label}</span>
      <span className={`min-w-0 break-all ${mono ? 'font-mono text-xs' : ''}`}>{value}</span>
    </div>
  );
}

function formatDate(iso?: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
}
