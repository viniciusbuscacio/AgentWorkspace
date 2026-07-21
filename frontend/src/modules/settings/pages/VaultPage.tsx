import { useCallback, useEffect, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { Field, FieldLabel } from '@ui/field';
import { Input } from '@ui/input';
import { PasswordInput } from '@ui/password-input';
import { settingsService } from '@services/settings.service';
import { vaultService, type VaultStatusInfo } from '@services/vault.service';

// ── Copy constants ────────────────────────────────────────────────────────────
// All user-visible strings live here (Decision 4). Refine wording here, never
// inline in JSX.
const COPY = {
  // Top: calm facts
  PROFILE_LABEL: 'Profile',
  PATH_LABEL: 'Vault location',
  STATUS_LABEL: 'Status',
  STATUS_UNLOCKED: 'Unlocked',
  STATUS_LOCKED: 'Locked',
  LOCK_BUTTON: 'Lock vault',

  // Danger zone header
  DANGER_ZONE_TITLE: 'Danger zone',

  // Change location
  CHANGE_LOCATION_TITLE: 'Change location',
  CHANGE_LOCATION_DESC:
    'This switches which vault the app opens. Your current vault stays where it is.',
  CHANGE_LOCATION_BUTTON: 'Change location\u2026',

  // Change password
  CHANGE_PASSWORD_TITLE: 'Change password',
  CHANGE_PASSWORD_DESC:
    'Replaces the encryption key. The vault becomes inaccessible with the old password.',
  CURRENT_PASSWORD_LABEL: 'Current password',
  NEW_PASSWORD_LABEL: 'New password',
  CHANGE_PASSWORD_BUTTON: 'Change password',

  // Recovery key
  RECOVERY_TITLE: 'Recovery key',
  RECOVERY_DESC:
    'Losing both your password and recovery key makes the vault permanently inaccessible.',
  RECOVERY_KEY_LABEL: 'Recovery key',
  RECOVERY_GENERATE: 'Generate',
  RECOVERY_VERIFY: 'Verify',
} as const;

interface VaultPageProps {
  onLocked: () => Promise<void> | void;
}

export function VaultPage({ onLocked }: VaultPageProps) {
  const [status, setStatus] = useState<VaultStatusInfo | null>(null);
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [recoveryKey, setRecoveryKey] = useState('');

  const refresh = useCallback(async () => {
    setStatus(await vaultService.status());
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // ── Actions ────────────────────────────────────────────────────────────────

  async function changeLocation() {
    // ChooseVaultDir in the Go backend shows a native confirmation dialog
    // (Decision 1 / Decision 3) before opening the folder picker.
    // We never use window.confirm here.
    const updated = await settingsService.chooseVaultDir();
    setStatus(updated);
    setMessage(`Vault location: ${updated.vaultDir}`);
  }

  async function changePassword() {
    const result = await settingsService.changePassword(currentPassword, newPassword);
    setMessage(result.error ?? 'Password changed. Save the new recovery key if shown.');
    setRecoveryKey(result.newRecoveryKey ?? result.recoveryKey ?? recoveryKey);
    setCurrentPassword('');
    setNewPassword('');
  }

  async function generateRecovery() {
    const result = await settingsService.generateRecoveryKey();
    setRecoveryKey(result.recoveryKey ?? result.newRecoveryKey ?? '');
    setMessage(result.error ?? 'Recovery key generated.');
  }

  async function verifyRecovery() {
    const result = await settingsService.verifyRecoveryKey(recoveryKey);
    setMessage(result.error ?? (result.valid ? 'Recovery key is valid.' : 'Recovery key is not valid.'));
  }

  async function lockVault() {
    await vaultService.lock();
    await onLocked();
  }

  // ── Render ─────────────────────────────────────────────────────────────────

  const profileName = status?.currentProfile?.name ?? '—';
  const vaultDir = status?.vaultDir ?? '—';
  const vaultUnlocked = status?.unlocked ?? false;

  return (
    <div className="flex max-w-2xl flex-col gap-6">

      {/* ── Top: calm facts ── */}
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-base font-semibold">Vault</h2>
          <Button variant="outline" icon="lock" size="sm" onClick={() => void lockVault()}>
            {COPY.LOCK_BUTTON}
          </Button>
        </div>
        <dl className="grid gap-3">
          <div className="grid grid-cols-[10rem_1fr] items-start gap-2">
            <dt className="text-sm text-muted-foreground">{COPY.PROFILE_LABEL}</dt>
            <dd className="text-sm font-medium">{profileName}</dd>
          </div>
          <div className="grid grid-cols-[10rem_1fr] items-start gap-2">
            <dt className="text-sm text-muted-foreground">{COPY.PATH_LABEL}</dt>
            <dd className="min-w-0 break-all text-sm font-medium">{vaultDir}</dd>
          </div>
          <div className="grid grid-cols-[10rem_1fr] items-start gap-2">
            <dt className="text-sm text-muted-foreground">{COPY.STATUS_LABEL}</dt>
            <dd className="text-sm font-medium">
              {vaultUnlocked ? COPY.STATUS_UNLOCKED : COPY.STATUS_LOCKED}
            </dd>
          </div>
        </dl>
      </div>

      {/* ── Bottom: Danger zone ── */}
      <section
        aria-label={COPY.DANGER_ZONE_TITLE}
        data-danger-zone
        className="rounded-lg border border-destructive/50"
      >
        {/* Header */}
        <div className="border-b border-destructive/50 px-6 py-3">
          <h2 className="text-sm font-semibold text-destructive">{COPY.DANGER_ZONE_TITLE}</h2>
        </div>

        {/* Divider list */}
        <div className="divide-y divide-destructive/20">

          {/* ── Change location ── */}
          <div className="flex flex-wrap items-center justify-between gap-4 px-6 py-4">
            <div className="min-w-0">
              <p className="text-sm font-medium">{COPY.CHANGE_LOCATION_TITLE}</p>
              <p className="mt-0.5 text-sm text-muted-foreground">{COPY.CHANGE_LOCATION_DESC}</p>
            </div>
            <Button
              variant="outline"
              icon="folder_open"
              onClick={() => void changeLocation()}
            >
              {COPY.CHANGE_LOCATION_BUTTON}
            </Button>
          </div>

          {/* ── Change password ── */}
          <div className="px-6 py-4">
            <div className="mb-3">
              <p className="text-sm font-medium">{COPY.CHANGE_PASSWORD_TITLE}</p>
              <p className="mt-0.5 text-sm text-muted-foreground">{COPY.CHANGE_PASSWORD_DESC}</p>
            </div>
            <div className="flex max-w-sm flex-col gap-3">
              <Field>
                <FieldLabel>{COPY.CURRENT_PASSWORD_LABEL}</FieldLabel>
                <PasswordInput
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel>{COPY.NEW_PASSWORD_LABEL}</FieldLabel>
                <PasswordInput
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                />
              </Field>
              <Button
                variant="outline"
                icon="key"
                disabled={!currentPassword || !newPassword}
                onClick={() => void changePassword()}
              >
                {COPY.CHANGE_PASSWORD_BUTTON}
              </Button>
            </div>
          </div>

          {/* ── Recovery key ── */}
          <div className="px-6 py-4">
            <div className="mb-3">
              <p className="text-sm font-medium">{COPY.RECOVERY_TITLE}</p>
              <p className="mt-0.5 text-sm text-muted-foreground">{COPY.RECOVERY_DESC}</p>
            </div>
            <div className="flex max-w-sm flex-col gap-3">
              <Field>
                <FieldLabel>{COPY.RECOVERY_KEY_LABEL}</FieldLabel>
                <Input
                  value={recoveryKey}
                  onChange={(e) => setRecoveryKey(e.target.value)}
                />
              </Field>
              <div className="flex flex-wrap gap-2">
                <Button icon="key" onClick={() => void generateRecovery()}>
                  {COPY.RECOVERY_GENERATE}
                </Button>
                <Button
                  variant="outline"
                  icon="verified"
                  disabled={!recoveryKey}
                  onClick={() => void verifyRecovery()}
                >
                  {COPY.RECOVERY_VERIFY}
                </Button>
              </div>
            </div>
          </div>

        </div>
      </section>

    </div>
  );
}
