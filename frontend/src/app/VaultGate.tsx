import { useEffect, useMemo, useState } from 'react';
import { AuthCard } from '@patterns/AuthCard';
import { Button } from '@ui/button';
import { Field, FieldLabel } from '@ui/field';
import { Input } from '@ui/input';
import { PasswordInput } from '@ui/password-input';
import { Switch } from '@ui/switch';
import { profileService, type ProfileInfo } from '@services/profile.service';
import { settingsService } from '@services/settings.service';
import { vaultService, type VaultStatusInfo } from '@services/vault.service';

type Mode = 'start' | 'create-vault' | 'unlock' | 'recover' | 'recovery-key';

interface VaultGateProps {
  status: VaultStatusInfo | null;
  onUnlocked: () => Promise<void> | void;
}

function ErrorMessage({ error }: { error: string }) {
  if (!error) return null;
  return (
    <div className="mb-4 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
      {error}
    </div>
  );
}

// Quick unlock is the same backend flow on both platforms; only the words and
// the guarantee differ. macOS: Keychain gated by Touch ID (the OS enforces the
// biometric). Windows: Credential Manager released on a plain click — the
// protection is the Windows login itself, and the stored password lapses
// after 7 days without a manual password unlock.
const IS_WINDOWS = typeof navigator !== 'undefined' && navigator.userAgent.includes('Windows');
const QUICK_UNLOCK = IS_WINDOWS
  ? {
      icon: 'bolt',
      enableTitle: 'Enable quick unlock',
      enableDescription: 'Store this vault’s password in the Windows Credential Manager to unlock with one click. Protected by your Windows login; expires after 7 days without a password unlock.',
      enableAria: 'Enable quick unlock for this vault',
      unlockButton: 'Quick unlock',
      failure: 'Quick unlock failed',
    }
  : {
      icon: 'fingerprint',
      enableTitle: 'Enable Touch ID',
      enableDescription: 'Store this vault’s password in your Mac’s Keychain to unlock with Touch ID.',
      enableAria: 'Enable Touch ID for this vault',
      unlockButton: 'Use Touch ID',
      failure: 'Touch ID unlock failed',
    };

// One auto quick-unlock attempt per app run: launching the app with a valid
// stored credential logs straight in — no click, no password. Module-scoped so
// it survives VaultGate remounts: after a manual "Lock vault" (or auto-lock)
// the gate does NOT silently reopen, otherwise locking would be meaningless;
// the one-click button remains for that case. Exported reset is test-only.
let autoQuickUnlockAttempted = false;
export function __resetAutoQuickUnlockForTests() {
  autoQuickUnlockAttempted = false;
}

function TouchIDOptIn({ checked, onChange, disabled }: { checked: boolean; onChange: (value: boolean) => void; disabled?: boolean }) {
  return (
    <label className="flex items-center gap-3 rounded-lg border border-border bg-background px-3 py-2.5 text-left">
      <span className="material-symbols-outlined text-[20px] text-primary" aria-hidden="true">{QUICK_UNLOCK.icon}</span>
      <span className="min-w-0 flex-1">
        <span className="block text-sm font-medium">{QUICK_UNLOCK.enableTitle}</span>
        <span className="block text-xs text-muted-foreground">{QUICK_UNLOCK.enableDescription}</span>
      </span>
      <Switch checked={checked} onCheckedChange={onChange} disabled={disabled} aria-label={QUICK_UNLOCK.enableAria} />
    </label>
  );
}

function joinDisplayPath(parent: string, child: string) {
  if (!parent) return child;
  const separator = parent.includes('\\') ? '\\' : '/';
  return `${parent.replace(/[\\/]+$/, '')}${separator}${child || 'AgentWorkspace'}`;
}

export function VaultGate({ status, onUnlocked }: VaultGateProps) {
  const [mode, setMode] = useState<Mode>('start');
  const [profiles, setProfiles] = useState<ProfileInfo[]>([]);
  const [selectedProfile, setSelectedProfile] = useState<ProfileInfo | null>(status?.currentProfile ?? null);
  const [selectedParentDir, setSelectedParentDir] = useState('');
  const [folderName, setFolderName] = useState('AgentWorkspace');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [recoveryKey, setRecoveryKey] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [touchIDAvailable, setTouchIDAvailable] = useState(false);
  const [touchIDHasPassword, setTouchIDHasPassword] = useState(false);
  // Touch ID enrollment is opt-in: the password is only written to the macOS
  // Keychain when the user explicitly turns this on at unlock/create time.
  const [enableTouchID, setEnableTouchID] = useState(false);

  const recentProfiles = useMemo(
    () => profiles.filter((profile) => profile.hasVault).slice(0, 5),
    [profiles],
  );
  const selectedLabel = selectedProfile?.name || folderName || 'AgentWorkspace';
  const targetVaultPath = useMemo(
    () => joinDisplayPath(selectedParentDir, folderName.trim() || 'AgentWorkspace'),
    [folderName, selectedParentDir],
  );

  useEffect(() => {
    setSelectedProfile(status?.currentProfile ?? null);
  }, [status?.currentProfile]);

  useEffect(() => {
    void refreshProfiles();
  }, []);

  // App launch with a stored quick-unlock credential for the current profile:
  // skip the start screen and go straight to unlock, where the one-shot auto
  // attempt (refreshTouchID) logs in without a click. Runs once per app run,
  // so a manual "Lock vault" lands back on the start screen and STAYS locked.
  useEffect(() => {
    if (autoQuickUnlockAttempted || mode !== 'start') return;
    if (!status?.exists || status.unlocked) return;
    void (async () => {
      try {
        if (!(await vaultService.touchIDAvailable())) return;
        if (!(await vaultService.touchIDHasPassword())) return;
        setMode('unlock');
      } catch {
        // availability probe failed (e.g. web bridge): stay on the start screen
      }
    })();
  }, [mode, status?.exists, status?.unlocked]);

  useEffect(() => {
    if (mode === 'unlock' || mode === 'create-vault') void refreshTouchID();
  }, [mode, selectedProfile?.id]);

  async function refreshProfiles() {
    const list = await profileService.listProfiles();
    setProfiles(list);
    return list;
  }

  async function refreshTouchID() {
    try {
      const available = await vaultService.touchIDAvailable();
      const hasPassword = available ? await vaultService.touchIDHasPassword() : false;
      setTouchIDAvailable(available);
      setTouchIDHasPassword(hasPassword);
      // App launch with a stored credential: log straight in. A failure (e.g.
      // the 7-day expiry) falls back to the password form with the reason
      // shown — and is not retried this run.
      if (available && hasPassword && mode === 'unlock' && !autoQuickUnlockAttempted) {
        autoQuickUnlockAttempted = true;
        void unlockWithTouchID();
      }
    } catch {
      setTouchIDAvailable(false);
      setTouchIDHasPassword(false);
    }
  }

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError('');
    try {
      await action();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  function startCreateVault() {
    setSelectedProfile(null);
    setSelectedParentDir('');
    setFolderName('AgentWorkspace');
    setPassword('');
    setConfirmPassword('');
    setRecoveryKey('');
    setError('');
    setMode('create-vault');
  }

  async function chooseParentDir() {
    await run(async () => {
      const folder = await settingsService.selectFolder();
      if (folder.canceled || !folder.path) return;
      setSelectedParentDir(folder.path);
    });
  }

  async function selectProfile(profile: ProfileInfo) {
    await run(async () => {
      const result = await profileService.selectProfile(profile.id);
      if (!result.success) throw new Error(result.error || 'Failed to select vault');
      setSelectedProfile(result.profile ?? profile);
      setPassword('');
      setConfirmPassword('');
      setRecoveryKey('');
      setMode('unlock');
    });
  }

  async function clearRecentVaults() {
    await run(async () => {
      const result = await profileService.clearRecentProfiles();
      if (!result.success) throw new Error(result.error || 'Failed to clear recent vaults');
      await refreshProfiles();
    });
  }

  async function importVault() {
    await run(async () => {
      const folder = await settingsService.selectFolder();
      if (folder.canceled || !folder.path) return;
      const result = await profileService.importVaultProfile(folder.path);
      if (!result.success || !result.profile) throw new Error(result.error || 'Failed to open vault');
      setSelectedProfile(result.profile);
      await refreshProfiles();
      setPassword('');
      setConfirmPassword('');
      setRecoveryKey('');
      setMode('unlock');
    });
  }

  async function unlockVault() {
    await run(async () => {
      const currentPassword = password;
      const result = await vaultService.unlock(currentPassword);
      if (!result.success) throw new Error(result.error || 'Unlock failed');
      // Opt-in only: store the password in the Keychain solely when the user
      // turned on the Touch ID switch. Password unlock stays the canonical path.
      if (enableTouchID) {
        void vaultService.touchIDEnroll(currentPassword).then(() => refreshTouchID()).catch(() => {});
      }
      setPassword('');
      await onUnlocked();
    });
  }

  async function unlockWithTouchID() {
    await run(async () => {
      const result = await vaultService.touchIDUnlock();
      if (!result.success) throw new Error(result.error || QUICK_UNLOCK.failure);
      await onUnlocked();
    });
  }

  async function createVault() {
    await run(async () => {
      if (!selectedParentDir && !selectedProfile) throw new Error('Choose where to store the vault');
      if (password.length < 4) throw new Error('Use a password with at least 4 characters');
      if (password !== confirmPassword) throw new Error('Passwords do not match');

      let profile = selectedProfile;
      if (!profile) {
        const result = await profileService.createProfileAtLocation(
          selectedParentDir,
          folderName.trim() || 'AgentWorkspace',
        );
        if (!result.success || !result.profile) throw new Error(result.error || 'Failed to create vault folder');
        profile = result.profile;
        setSelectedProfile(profile);
      }

      const currentPassword = password;
      const result = await vaultService.create(currentPassword);
      if (!result.success || !result.recoveryKey) throw new Error(result.error || 'Vault creation failed');
      // Opt-in only — never silently enroll a freshly created vault.
      if (enableTouchID) {
        void vaultService.touchIDEnroll(currentPassword).then(() => refreshTouchID()).catch(() => {});
      }
      await refreshProfiles();
      setPassword('');
      setConfirmPassword('');
      setRecoveryKey(result.recoveryKey);
      setMode('recovery-key');
    });
  }

  async function recoverVault() {
    await run(async () => {
      if (!recoveryKey.trim()) throw new Error('Recovery key is required');
      if (password.length < 4) throw new Error('Use a new password with at least 4 characters');
      const result = await vaultService.recover(recoveryKey.trim(), password);
      if (!result.success || !result.newRecoveryKey) throw new Error(result.error || 'Recovery failed');
      setPassword('');
      setRecoveryKey(result.newRecoveryKey);
      setMode('recovery-key');
    });
  }

  if (mode === 'start') {
    return (
      <AuthCard title="aw" subtitle="Create a new encrypted vault or open an existing one.">
        <ErrorMessage error={error} />

        <div className="grid gap-3">
          <Button onClick={startCreateVault} disabled={busy} className="h-auto justify-start px-4 py-3 text-left">
            <span className="material-symbols-outlined text-[18px]" aria-hidden="true">add_circle</span>
            <span className="flex min-w-0 flex-col items-start">
              <span>Create new vault</span>
              <span className="text-xs font-normal text-primary-foreground/80">
                Choose a location and create an AgentWorkspace folder.
              </span>
            </span>
          </Button>

          <Button variant="outline" onClick={importVault} disabled={busy} className="h-auto justify-start px-4 py-3 text-left">
            <span className="material-symbols-outlined text-[18px]" aria-hidden="true">folder_open</span>
            <span className="flex min-w-0 flex-col items-start">
              <span>Open existing vault</span>
              <span className="text-xs font-normal text-muted-foreground">
                Select an existing folder with vault.db, vault.salt and vault.recovery.
              </span>
            </span>
          </Button>
        </div>

        {recentProfiles.length > 0 && (
          <div className="mt-6">
            <div className="mb-2 flex items-center justify-between">
              <div className="text-xs font-medium uppercase text-muted-foreground">Recent vaults</div>
              <Button
                variant="ghost"
                size="xs"
                onClick={() => void clearRecentVaults()}
                disabled={busy}
                className="text-muted-foreground hover:text-foreground"
                title="Clear this list. Vault folders on disk are not deleted — reopen any vault with Open existing vault."
              >
                <span className="material-symbols-outlined" aria-hidden="true">delete_sweep</span>
                Clear
              </Button>
            </div>
            <div className="flex max-h-[260px] flex-col gap-2 overflow-y-auto">
              {recentProfiles.map((profile) => (
                <Button
                  key={profile.id}
                  variant="ghost"
                  onClick={() => selectProfile(profile)}
                  disabled={busy}
                  className="h-auto justify-start whitespace-normal rounded-lg border border-border bg-background px-4 py-3 text-left hover:bg-accent"
                >
                  <span className="material-symbols-outlined text-[20px] text-primary" aria-hidden="true">database</span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium text-foreground">{profile.name}</span>
                    <span className="block truncate text-xs font-normal text-muted-foreground">{profile.vaultDir}</span>
                  </span>
                </Button>
              ))}
            </div>
          </div>
        )}
      </AuthCard>
    );
  }

  if (mode === 'unlock') {
    return (
      <AuthCard title={`Unlock ${selectedLabel}`} subtitle={selectedProfile?.vaultDir}>
        <ErrorMessage error={error} />
        <Field>
          <FieldLabel htmlFor="vault-password">Password</FieldLabel>
          <PasswordInput
            id="vault-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') void unlockVault();
            }}
            autoFocus
          />
        </Field>
        <div className="mt-4 flex flex-col gap-2">
          {touchIDAvailable && touchIDHasPassword && (
            <Button variant="outline" onClick={() => void unlockWithTouchID()} disabled={busy}>
              <span className="material-symbols-outlined text-[18px]" aria-hidden="true">{QUICK_UNLOCK.icon}</span>
              <span>{QUICK_UNLOCK.unlockButton}</span>
            </Button>
          )}
          {touchIDAvailable && !touchIDHasPassword && (
            <TouchIDOptIn checked={enableTouchID} onChange={setEnableTouchID} disabled={busy} />
          )}
          <Button onClick={unlockVault} disabled={busy || !password}>Unlock with password</Button>
          <Button variant="outline" onClick={() => setMode('recover')} disabled={busy}>Recover with key</Button>
          <Button variant="ghost" onClick={() => setMode('start')} disabled={busy}>Back</Button>
        </div>
      </AuthCard>
    );
  }

  if (mode === 'create-vault') {
    return (
      <AuthCard title="Create new vault" subtitle="Select a parent folder. aw will create an AgentWorkspace folder inside it by default.">
        <ErrorMessage error={error} />

        <div className="grid gap-4">
          <Field>
            <FieldLabel htmlFor="vault-folder-name">Folder name</FieldLabel>
            <Input
              id="vault-folder-name"
              value={folderName}
              onChange={(event) => setFolderName(event.target.value)}
              placeholder="AgentWorkspace"
            />
          </Field>

          <Field>
            <FieldLabel>Location</FieldLabel>
            <Button variant="outline" onClick={chooseParentDir} disabled={busy} className="justify-start overflow-hidden text-left">
              <span className="material-symbols-outlined text-[18px]" aria-hidden="true">folder_open</span>
              <span className="truncate">{selectedParentDir || 'Choose location...'}</span>
            </Button>
            {selectedParentDir && (
              <div className="rounded-md border border-border bg-background px-3 py-2 text-xs leading-relaxed text-muted-foreground [overflow-wrap:anywhere]">
                Vault will be created at:<br />
                <span className="text-foreground">{targetVaultPath}</span>
              </div>
            )}
          </Field>

          <Field>
            <FieldLabel htmlFor="vault-create-password">Password</FieldLabel>
            <PasswordInput
              id="vault-create-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoFocus
            />
          </Field>

          <Field>
            <FieldLabel htmlFor="vault-confirm-password">Confirm password</FieldLabel>
            <PasswordInput
              id="vault-confirm-password"
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void createVault();
              }}
            />
          </Field>
        </div>

        <div className="mt-5 flex flex-col gap-2">
          {touchIDAvailable && (
            <TouchIDOptIn checked={enableTouchID} onChange={setEnableTouchID} disabled={busy} />
          )}
          <Button onClick={createVault} disabled={busy || !selectedParentDir || !password || !confirmPassword}>
            Create vault
          </Button>
          <Button variant="ghost" onClick={() => setMode('start')} disabled={busy}>Back</Button>
        </div>
      </AuthCard>
    );
  }

  if (mode === 'recover') {
    return (
      <AuthCard title="Recover vault" subtitle="Enter your recovery key and choose a new password.">
        <ErrorMessage error={error} />
        <div className="grid gap-4">
          <Field>
            <FieldLabel htmlFor="vault-recovery-key">Recovery key</FieldLabel>
            <Input
              id="vault-recovery-key"
              value={recoveryKey}
              onChange={(event) => setRecoveryKey(event.target.value)}
              placeholder="xxxx-xxxx-xxxx-xxxx-xxxx-xxxx"
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="vault-new-password">New password</FieldLabel>
            <PasswordInput
              id="vault-new-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </Field>
        </div>
        <div className="mt-4 flex flex-col gap-2">
          <Button onClick={recoverVault} disabled={busy || !recoveryKey || !password}>Recover</Button>
          <Button variant="ghost" onClick={() => setMode('unlock')} disabled={busy}>Back</Button>
        </div>
      </AuthCard>
    );
  }

  return (
    <AuthCard title="Save your recovery key" subtitle="You need this key if you forget the vault password. Store it somewhere safe.">
      <div className="mb-4 select-all break-all rounded-lg border border-border bg-background p-4 text-center font-mono text-base leading-relaxed text-foreground">
        {recoveryKey}
      </div>
      <div className="mb-4 rounded-lg border border-[#ffaa0059] bg-[#ffaa001a] px-3 py-2 text-sm leading-relaxed text-[#ffa657]">
        This key is shown only now. Copy it before continuing.
      </div>
      <Button onClick={() => void onUnlocked()} className="w-full">I saved the recovery key</Button>
    </AuthCard>
  );
}
