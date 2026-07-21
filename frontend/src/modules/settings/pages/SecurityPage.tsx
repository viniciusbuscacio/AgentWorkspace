import { useCallback, useEffect, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@ui/card';
import { Field, FieldLabel } from '@ui/field';
import { ZoomSafeSelect } from '@ui/zoom-safe-select';
import { settingsService } from '@services/settings.service';
import { vaultService } from '@services/vault.service';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import type { dto } from '@wails/go/models';

const minuteOptions = [5, 10, 15, 30, 60, 120];

// Displayed as the first option; maps to 0 = disabled in the auto-lock policy.
const NEVER_VALUE = '0';

// Internal secrets back app config (provider/model selection, the MCP/REST
// server settings) and chat housekeeping. They are managed elsewhere and must
// not be hand-deletable here — removing one breaks config or a running server.
// They are namespaced with a leading underscore or the chat- housekeeping
// prefix; real user credentials (provider API keys, auth JSON) have neither.
function isUserManagedSecret(name: string): boolean {
  return !name.startsWith('_') && !name.startsWith('chat-');
}

export function SecurityPage() {
  const [savedMinutes, setSavedMinutes] = useState(NEVER_VALUE);
  const [minutes, setMinutes] = useState(NEVER_VALUE);
  const [secrets, setSecrets] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [touchIDAvailable, setTouchIDAvailable] = useState(false);
  const [touchIDEnabled, setTouchIDEnabled] = useState(false);
  // Signing-certificate trust: when the local signing cert is untrusted,
  // Keychain "Always Allow" grants never stick (macOS re-prompts on every
  // Touch ID unlock). Offer the one-click trust fix right here.
  const [codesignTrust, setCodesignTrust] = useState<dto.CodesignTrustResult | null>(null);
  const [trusting, setTrusting] = useState(false);

  const dirty = minutes !== savedMinutes;

  const refresh = useCallback(async () => {
    const loaded = String(await settingsService.getAutoLockMinutes());
    setSavedMinutes(loaded);
    setMinutes(loaded);
    setSecrets((await settingsService.listSecrets()).filter(isUserManagedSecret));
    try {
      const available = await vaultService.touchIDAvailable();
      setTouchIDAvailable(available);
      setTouchIDEnabled(available ? await vaultService.touchIDHasPassword() : false);
    } catch {
      // Touch ID bindings are web-denylisted (host hardware): over the web
      // bridge the calls reject — hide the card instead of dying mid-refresh.
      setTouchIDAvailable(false);
      setTouchIDEnabled(false);
    }
    try {
      setCodesignTrust(await vaultService.codesignTrustStatus());
    } catch {
      setCodesignTrust(null); // web-denylisted — hide the section
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function saveAutoLock() {
    setSaving(true);
    const result = await settingsService.setAutoLockMinutes(Number(minutes));
    setSaving(false);
    setMessage(result.error || 'Auto-lock updated.');
    if (!result.error) {
      setSavedMinutes(minutes);
    }
  }

  function cancelAutoLock() {
    setMinutes(savedMinutes);
    setMessage('');
  }

  async function deleteSecret(name: string) {
    const result = await settingsService.deleteSecret(name);
    setMessage(result.error || `Deleted ${name}.`);
    await refresh();
  }

  async function removeTouchID() {
    const result = await vaultService.touchIDRemove();
    setMessage(result.error || 'Touch ID removed for this vault.');
    await refresh();
  }

  async function repairTouchID() {
    const result = await vaultService.touchIDRepair();
    setMessage(result.error || 'Keychain entry repaired. macOS may ask one final time; after that the prompt is gone.');
    await refresh();
  }

  async function trustSigningCert() {
    setTrusting(true);
    try {
      const result = await vaultService.codesignTrustGrant();
      setMessage(result.error || 'Certificate trusted. Keychain permissions now persist across launches.');
    } finally {
      setTrusting(false);
      await refresh();
    }
  }

  return (
    <div className="grid gap-4 xl:grid-cols-2">
      <Card className="rounded-lg">
        <CardHeader><CardTitle>Auto-lock</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-3">
          <Field>
            <FieldLabel>Lock after inactivity</FieldLabel>
            <ZoomSafeSelect
              aria-label="Lock after inactivity"
              value={minutes}
              onValueChange={(v) => { setMinutes(v); setMessage(''); }}
              options={[
                { value: NEVER_VALUE, label: 'Never' },
                ...minuteOptions.map((item) => ({ value: String(item), label: `${item} minutes` })),
              ]}
            />
          </Field>
          {minutes === NEVER_VALUE && (
            <p className="text-xs text-muted-foreground">The vault stays unlocked until you lock it manually.</p>
          )}
          <SaveCancelActions
            onSave={() => void saveAutoLock()}
            onCancel={cancelAutoLock}
            saving={saving}
            disabled={!dirty}
            />
        </CardContent>
      </Card>
      <Card className="rounded-lg">
        <CardHeader><CardTitle>Touch ID</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-3">
          {!touchIDAvailable && <p className="text-sm text-muted-foreground">Touch ID is not available on this Mac.</p>}
          {touchIDAvailable && touchIDEnabled && (
            <>
              <p className="text-sm text-muted-foreground">Touch ID is enabled for this vault. Password unlock remains available.</p>
              <div className="flex flex-wrap gap-2">
                <Button variant="destructive" icon="fingerprint" onClick={() => void removeTouchID()}>Remove Touch ID</Button>
                <Button variant="outline" icon="build" title="Re-write the stored credential so the current app owns it" onClick={() => void repairTouchID()}>Repair Keychain permission</Button>
              </div>
              <p className="text-xs text-muted-foreground">If macOS keeps asking permission to access the vault key (e.g. after app updates), Repair re-writes the stored credential so the current app owns it — one final macOS prompt at most, then it stops.</p>
            </>
          )}
          {touchIDAvailable && !touchIDEnabled && (
            <p className="text-sm text-muted-foreground">Touch ID is off for this vault. Turn on &ldquo;Enable Touch ID&rdquo; when you unlock or create a vault to store its password in your Mac&apos;s Keychain.</p>
          )}
          {codesignTrust?.supported && codesignTrust.hasCertificate && !codesignTrust.trusted && (
            <div className="flex flex-col gap-2 rounded-md border border-amber-500/40 bg-amber-500/10 p-3">
              <p className="text-sm">
                macOS keeps asking for Keychain permission on every Touch ID unlock because the app&apos;s local
                signing certificate{codesignTrust.certificateName ? ` (${codesignTrust.certificateName})` : ''} is not
                trusted yet — &ldquo;Always Allow&rdquo; never sticks. Trust it once and the prompt goes away for good.
              </p>
              <Button className="self-start" icon="verified_user" disabled={trusting} onClick={() => void trustSigningCert()}>
                {trusting ? 'Waiting for macOS...' : 'Trust certificate'}
              </Button>
              <p className="text-xs text-muted-foreground">macOS will ask you to confirm with your Mac account password. This only trusts the app&apos;s own local certificate for code signing.</p>
            </div>
          )}
        </CardContent>
      </Card>
      <Card className="rounded-lg">
        <CardHeader><CardTitle>Vault secrets</CardTitle></CardHeader>
        <CardContent className="flex flex-col gap-2">
          {secrets.length === 0 && <p className="text-sm text-muted-foreground">No stored secrets.</p>}
          {secrets.map((secret) => (
            <div key={secret} className="flex items-center justify-between gap-3 rounded-md border border-border p-2">
              <span className="min-w-0 truncate text-sm">{secret}</span>
              <Button variant="destructive" size="sm" icon="delete" onClick={() => deleteSecret(secret)}>Delete</Button>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}
