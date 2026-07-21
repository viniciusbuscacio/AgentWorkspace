import { useCallback, useEffect, useState } from 'react';
import { Toaster } from '@ui/index';
import { vaultService, type VaultStatusInfo } from '@services/vault.service';
import { onAwEvent } from '@services/events';
import { readPipConfig } from '@/pip/pip-bindings';
import { PipChatWindow } from '@/pip/PipChatWindow';
import { useUiControlBridge } from '@/hooks/ui-control.hooks';
import { useUiAutomationBridge } from '@/hooks/ui-automation.hooks';
import { useExternalLinks } from './external-links';
import { VaultGate } from './VaultGate';
import { AppShell } from './AppShell';

type Phase = 'loading' | 'gate' | 'ready';

export function App() {
  // Route external link clicks to the OS browser (desktop) / a new tab (web)
  // instead of letting the webview navigate top-level. Installed at the
  // always-mounted root so it covers the main window and the PiP window alike.
  useExternalLinks();
  const pipConfig = readPipConfig();
  if (pipConfig) {
    // PiP window: the host app already holds the unlocked vault; no gate.
    return <PipChatWindow chatId={pipConfig.chatId} />;
  }
  // Mount the toast surface once, as a sibling of the whole UI (loading, lock
  // gate, and main shell). Sonner renders through its own portal on document.body,
  // so it does not need to wrap the tree. Note: the app applies a CSS zoom
  // transform inside AppShell; since the Toaster is mounted outside that container
  // it keeps a fixed size regardless of zoom.
  return (
    <>
      <Toaster />
      <MainApp />
    </>
  );
}

function MainApp() {
  const [phase, setPhase] = useState<Phase>('loading');
  const [status, setStatus] = useState<VaultStatusInfo | null>(null);
  useUiControlBridge();
  useUiAutomationBridge();

  const refresh = useCallback(async () => {
    const next = await vaultService.status();
    setStatus(next);
    setPhase(next.unlocked ? 'ready' : 'gate');
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Dev binary only: the backend may auto-unlock at startup (build tag
  // `devunlock`). If that completes after this view mounted, advance the gate.
  useEffect(() => onAwEvent('vault:dev-unlocked', () => void refresh()), [refresh]);

  if (phase === 'loading') {
    return (
      <main className="flex h-screen items-center justify-center bg-background text-foreground">
        <p className="text-sm text-muted-foreground">Loading…</p>
      </main>
    );
  }

  if (phase === 'gate' || !status?.unlocked) {
    return <VaultGate status={status} onUnlocked={refresh} />;
  }

  return <AppShell onLocked={refresh} />;
}
