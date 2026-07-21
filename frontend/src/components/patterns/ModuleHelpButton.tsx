import { useState } from 'react';
import { openModuleHelp } from '@/lib/module-help';

// The "?" in a module header (top-right). Clicking it opens the PiP chat
// window on a fresh "Help — <module>" chat with the what-does-this-module-do
// question already sent (see lib/module-help.ts).
export function ModuleHelpButton({ module }: { module: string }) {
  const [busy, setBusy] = useState(false);
  return (
    <button
      type="button"
      className="icon-button shrink-0"
      title={`What does the ${module} module do?`}
      aria-label={`Help: ${module}`}
      disabled={busy}
      onClick={() => {
        setBusy(true);
        void openModuleHelp(module).finally(() => setBusy(false));
      }}
    >
      <span className="material-symbols-outlined" aria-hidden="true">help</span>
    </button>
  );
}
