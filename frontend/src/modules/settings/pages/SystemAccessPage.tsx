import { useCallback, useEffect, useState } from 'react';
import { Button } from '@ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@ui/card';
import { settingsService } from '@services/settings.service';
import type { dto } from '@wails/go/models';
import { notify } from '@/lib/notify';
import {
  MACOS_PERM_CARD_TITLE,
  MACOS_PERM_CARD_DESC,
  MACOS_PERM_STATUS_UNKNOWN,
  MACOS_PERM_STATUS_GRANTED,
  MACOS_PERM_STATUS_DENIED,
  MACOS_PERM_STATUS_INCONCLUSIVE,
  MACOS_PERM_NO_PROBE,
  MACOS_PERM_BTN_OPEN,
  MACOS_PERM_BTN_TEST,
  MACOS_PERM_BTN_TESTING,
} from './macos-permissions.constants';

// Settings › macOS Permissions: what the OS lets this app do outside its own
// window. Today that is the macOS TCC inventory (moved out of Security);
// a Windows inventory can join later — the backend already returns a
// per-platform list (empty off-macOS). Desktop-control capabilities and their
// aw-side fences are a planned future module, not this page.

// ProbeStatus is the per-row probe result cache keyed by permission id.
// 'inconclusive' is a ran-but-undecided probe — distinct from the untested
// 'unknown' so pressing Test always visibly changes something.
type ProbeStatus = 'unknown' | 'granted' | 'denied' | 'testing' | 'inconclusive';

function probeStatusLabel(s: ProbeStatus): string {
  switch (s) {
    case 'granted':      return MACOS_PERM_STATUS_GRANTED;
    case 'denied':       return MACOS_PERM_STATUS_DENIED;
    case 'testing':      return MACOS_PERM_BTN_TESTING;
    case 'inconclusive': return MACOS_PERM_STATUS_INCONCLUSIVE;
    default:             return MACOS_PERM_STATUS_UNKNOWN;
  }
}

function probeStatusColor(s: ProbeStatus): string {
  switch (s) {
    case 'granted': return 'text-green-600 dark:text-green-400';
    case 'denied':  return 'text-red-600 dark:text-red-400';
    default:        return 'text-muted-foreground';
  }
}

function MacosPermissionsCard({ items }: { items: dto.MacosPermission[] }) {
  const [probeStatus, setProbeStatus] = useState<Record<string, ProbeStatus>>({});

  async function runProbe(id: string) {
    setProbeStatus((prev) => ({ ...prev, [id]: 'testing' }));
    try {
      const res = await settingsService.probeMacosPermission(id);
      const s = res.status as ProbeStatus;
      if (s === 'granted' || s === 'denied') {
        setProbeStatus((prev) => ({ ...prev, [id]: s }));
        return;
      }
      // Ran but undecided: say why instead of silently keeping the label.
      setProbeStatus((prev) => ({ ...prev, [id]: 'inconclusive' }));
      notify(res.detail || 'Test inconclusive — could not determine the permission status.');
    } catch {
      setProbeStatus((prev) => ({ ...prev, [id]: 'inconclusive' }));
      notify('Test inconclusive — could not determine the permission status.');
    }
  }

  async function openPane(deepLink: string) {
    try {
      await settingsService.openSystemSettingsPane(deepLink);
    } catch {
      // Web-denylisted (opens System Settings on the HOST): the bridge rejects
      // over a remote browser. Nothing useful to do remotely.
    }
  }

  return (
    <Card className="rounded-lg">
      <CardHeader>
        <CardTitle>{MACOS_PERM_CARD_TITLE}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <p className="text-sm text-muted-foreground">{MACOS_PERM_CARD_DESC}</p>
        <div className="flex flex-col gap-2">
          {items.map((p) => {
            const status = probeStatus[p.id] ?? 'unknown';
            return (
              <div
                key={p.id}
                className="flex flex-col gap-1 rounded-md border border-border p-3 sm:flex-row sm:items-center sm:justify-between sm:gap-3"
              >
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{p.name}</p>
                  <p className="text-xs text-muted-foreground">{p.why}</p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  {p.hasProbe ? (
                    <span className={`text-xs ${probeStatusColor(status)}`}>
                      {probeStatusLabel(status)}
                    </span>
                  ) : (
                    <span className="text-xs text-muted-foreground">{MACOS_PERM_NO_PROBE}</span>
                  )}
                  {p.hasProbe && (
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={status === 'testing'}
                      onClick={() => void runProbe(p.id)}
                    >
                      {status === 'testing' ? MACOS_PERM_BTN_TESTING : MACOS_PERM_BTN_TEST}
                    </Button>
                  )}
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void openPane(p.deepLink)}
                  >
                    {MACOS_PERM_BTN_OPEN}
                  </Button>
                </div>
              </div>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}

export function SystemAccessPage() {
  const [perms, setPerms] = useState<dto.MacosPermission[] | null>(null);

  const refresh = useCallback(async () => {
    const result = await settingsService.getMacosPermissions();
    setPerms(result.items ?? []);
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  if (perms === null) return null; // first load
  return (
    <div className="flex max-w-3xl flex-col gap-4">
      {perms.length > 0 ? (
        <MacosPermissionsCard items={perms} />
      ) : (
        <p className="text-sm text-muted-foreground">
          Nothing to manage on this platform — system permission inventories exist for macOS only (Windows support is planned).
        </p>
      )}
    </div>
  );
}
