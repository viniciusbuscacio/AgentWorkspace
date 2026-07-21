import { useCallback, useEffect, useState } from 'react';
import { notify } from '@/lib/notify';
import { ModuleHelpButton } from '@/components/patterns/ModuleHelpButton';
import { Button } from '@ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@ui/card';
import { Switch } from '@ui/switch';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { obsidianService, type ObsidianSettings } from '@services/obsidian.service';

// ObsidianModule configures the agent's access to the user's Obsidian vault:
// the vault folder (the jail every obsidian.* action lives in), the Write
// toggle (default OFF) and the always-read notes injected into the agent
// prompt. Settings persist immediately — there is no draft state, matching
// the module toggles pattern (adding the module is already the master fence).
export function ObsidianModule() {
  // Draft-edit pattern: changes stay local until Save; Cancel reverts to the
  // last saved snapshot.
  const [settings, setSettings] = useState<ObsidianSettings | null>(null);
  const [saved, setSaved] = useState<ObsidianSettings | null>(null);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const dirty = settings !== null && saved !== null && JSON.stringify(settings) !== JSON.stringify(saved);

  const refresh = useCallback(async () => {
    const loaded = await obsidianService.getSettings();
    setSettings(loaded);
    setSaved(loaded);
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function save() {
    if (!settings) return;
    setSaving(true);
    const result = await obsidianService.setSettings(settings);
    setSaving(false);
    if (!result.success) {
      notify(result.error || 'Could not save the Obsidian settings.');
      return;
    }
    setSaved(settings);
    notify('Obsidian settings saved.');
  }

  function cancel() {
    if (saved) setSettings(saved);
  }

  async function pickVault() {
    try {
      const picked = await obsidianService.chooseVaultDir();
      if (!picked.canceled && picked.path && settings) {
        setSettings({ ...settings, vaultDir: picked.path });
      }
    } catch {
      notify('Could not open the folder picker.');
    }
  }

  async function addAlwaysRead() {
    const picked = await obsidianService.chooseAlwaysReadFile(settings?.vaultDir ?? '');
    if (picked.error) {
      notify(picked.error);
      return;
    }
    if (picked.canceled || !picked.path || !settings) return;
    if (settings.alwaysRead?.includes(picked.path)) return;
    setSettings({ ...settings, alwaysRead: [...(settings.alwaysRead ?? []), picked.path] });
  }

  function removeAlwaysRead(path: string) {
    if (!settings) return;
    setSettings({ ...settings, alwaysRead: (settings.alwaysRead ?? []).filter((f) => f !== path) });
  }

  async function testAccess() {
    setTesting(true);
    try {
      const result = await obsidianService.testAccess();
      notify(result.success ? 'Access OK' : result.error || 'Test failed.', result.success ? result.message : undefined);
    } finally {
      setTesting(false);
    }
  }

  return (
    <section className="home-screen" data-awid="obsidian-screen">
      <div className="home-header">
        <div className="flex items-start justify-between gap-3">
          <h1 className="home-title">Obsidian</h1>
          <ModuleHelpButton module="Obsidian" />
        </div>
        <p className="home-subtitle">The agent reads — and, if you allow, updates — your Obsidian vault.</p>
      </div>

      {settings && (
        <div className="flex flex-col gap-4">
          <Card className="rounded-lg">
            <CardHeader>
              <CardTitle>Obsidian access</CardTitle>
            </CardHeader>
            <CardContent className="flex items-center justify-between gap-3">
              <p className="text-sm text-muted-foreground">
                Enable/Disable Obsidian access.
              </p>
              <Switch
                checked={settings.enabled}
                onCheckedChange={(enabled) => setSettings({ ...settings, enabled })}
                aria-label="Enable Obsidian access"
              />
            </CardContent>
          </Card>

          <Card className="rounded-lg">
            <CardHeader>
              <CardTitle>Vault folder</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <p className="text-sm text-muted-foreground">
                The folder where your Obsidian vault is located.
              </p>
              <div className="flex items-center justify-between gap-3 rounded-md border border-border p-3">
                <span className="min-w-0 break-all text-sm font-medium">
                  {settings.vaultDir || 'No folder selected yet'}
                </span>
                <Button variant="outline" size="sm" icon="folder_open" onClick={() => void pickVault()}>
                  {settings.vaultDir ? 'Change…' : 'Select…'}
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card className="rounded-lg">
            <CardHeader>
              <CardTitle>Always read these notes:</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <p className="text-sm text-muted-foreground">
                Notes the agent always keeps in mind, loaded in full on every message.
              </p>
              {settings.alwaysReadChars > 15000 && (
                <p className="text-sm text-destructive">
                  ⚠️ These notes total {settings.alwaysReadChars.toLocaleString()} characters — that weight rides on
                  every single message. Consider trimming them; the agent can read full notes on demand.
                </p>
              )}
              {(settings.alwaysRead ?? []).map((path) => (
                <div key={path} className="flex items-center justify-between gap-3 rounded-md border border-border p-2">
                  <span className="min-w-0 truncate text-sm">{path}</span>
                  <Button
                    variant="destructive"
                    size="sm"
                    icon="delete"
                    onClick={() => void removeAlwaysRead(path)}
                  >
                    Remove
                  </Button>
                </div>
              ))}
              <div>
                <Button
                  variant="outline"
                  size="sm"
                  icon="add"
                  disabled={!settings.vaultDir}
                  onClick={() => void addAlwaysRead()}
                >
                  Add note
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card className="rounded-lg">
            <CardHeader>
              <CardTitle>Write access</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <div className="flex items-center justify-between gap-3 rounded-md border border-border p-3">
                <p className="text-sm">Create/Update Notes/Folders</p>
                <Switch
                  checked={settings.writeEnabled}
                  disabled={!settings.vaultDir}
                  onCheckedChange={(enabled) => setSettings({ ...settings, writeEnabled: enabled })}
                  aria-label="Create/Update Notes/Folders"
                />
              </div>
              <div className="flex items-center justify-between gap-3 rounded-md border border-border p-3">
                <p className="text-sm">Delete Notes/Folders</p>
                <Switch
                  checked={settings.deleteEnabled}
                  disabled={!settings.vaultDir}
                  onCheckedChange={(enabled) => setSettings({ ...settings, deleteEnabled: enabled })}
                  aria-label="Delete Notes/Folders"
                />
              </div>
            </CardContent>
          </Card>


          <div className="flex items-center gap-2">
            <SaveCancelActions
              onSave={() => void save()}
              onCancel={cancel}
              disabled={!dirty}
              saving={saving}
            />
            <Button
              variant="outline"
              icon={testing ? 'hourglass_top' : 'network_check'}
              disabled={testing || !settings.vaultDir || dirty}
              title={dirty ? 'Save first — the test runs against the saved settings' : undefined}
              onClick={() => void testAccess()}
            >
              {testing ? 'Testing…' : 'Test access'}
            </Button>
          </div>
        </div>
      )}
    </section>
  );
}
