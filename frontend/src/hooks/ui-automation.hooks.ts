import { useEffect } from 'react';
import { onAwEvent } from '@services/events';
import { uiService } from '@services/ui.service';
import { executeUiCommand } from '@/lib/ui-automation';

/**
 * Bridges the backend ui.* automation actions to the DOM: it listens for
 * ui:command round-trips, runs the command (snapshot, click, fill, screenshot)
 * and reports the JSON result — or the error — back via ResolveUICommand.
 */
export function useUiAutomationBridge() {
  useEffect(() => {
    return onAwEvent('ui:command', ({ id, command, params }) => {
      void (async () => {
        try {
          const result = await executeUiCommand(command, params || {});
          await uiService.resolveUiCommand(id, JSON.stringify(result ?? null), '');
        } catch (error) {
          const message = error instanceof Error ? error.message : String(error);
          await uiService.resolveUiCommand(id, '', message);
        }
      })();
    });
  }, []);
}
