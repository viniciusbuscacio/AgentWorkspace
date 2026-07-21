import { useEffect, useState } from 'react';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@ui/alert-dialog';
import { chatService } from '@services/chat.service';
import { onAwEvent, type ToolConfirmPayload } from '@services/events';

export function ToolConfirmDialog() {
  // A queue, not a single request: parallel tool calls (e.g. concurrent
  // subagents) can raise a second tool:confirm while the first is open. Holding
  // only one would overwrite the first, whose id then never resolves and hangs
  // that run. We show the head and advance as each is answered.
  const [queue, setQueue] = useState<ToolConfirmPayload[]>([]);
  const request = queue[0] ?? null;
  const safety = externalSafetyArgs(request?.args);
  const module = moduleArgs(request);

  useEffect(() => onAwEvent('tool:confirm', (payload) => {
    setQueue((current) => (current.some((item) => item.id === payload.id) ? current : [...current, payload]));
  }), []);

  async function resolve(approved: boolean) {
    if (!request) return;
    const { id } = request;
    // Advance to the next pending confirmation immediately; resolve the backend
    // for this id independently so a slow resolve does not block the queue.
    setQueue((current) => current.filter((item) => item.id !== id));
    await chatService.resolveToolConfirmation(id, approved);
  }

  return (
    <AlertDialog open={request !== null}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Approve tool action?</AlertDialogTitle>
          <AlertDialogDescription>
            {request?.summary || request?.tool}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {module && (
          <div className="rounded-md border border-border bg-muted/50 p-3 text-sm">
            <div className="font-semibold">Module</div>
            <div>{module.name || module.id}</div>
            {module.policy && <div className="text-muted-foreground">Policy: {module.policy}</div>}
          </div>
        )}
        {safety && (
          <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm text-amber-100">
            <div className="font-semibold">External content safety check</div>
            <div>Risk: {safety.riskLevel || 'unknown'}{safety.suspicious ? ' · suspicious content detected' : ''}</div>
            {safety.sourceType && <div>Source: {safety.sourceType}{safety.origin ? ` · ${safety.origin}` : ''}</div>}
          </div>
        )}
        {request?.args !== undefined && (
          <pre className="max-h-48 overflow-auto rounded-md bg-muted p-3 text-xs text-muted-foreground">
            {JSON.stringify(request.args, null, 2)}
          </pre>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel onClick={() => void resolve(false)}>Reject</AlertDialogCancel>
          <AlertDialogAction onClick={() => void resolve(true)}>Approve</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function externalSafetyArgs(args: unknown): { riskLevel?: string; suspicious?: boolean; sourceType?: string; origin?: string } | null {
  if (!args || typeof args !== 'object') return null;
  const record = args as Record<string, unknown>;
  if (!('external_notice' in record) && !('risk_level' in record) && !('suspicious' in record)) return null;
  return {
    riskLevel: typeof record.risk_level === 'string' ? record.risk_level : undefined,
    suspicious: record.suspicious === true,
    sourceType: typeof record.source_type === 'string' ? record.source_type : undefined,
    origin: typeof record.origin === 'string' ? record.origin : undefined,
  };
}

function moduleArgs(request: ToolConfirmPayload | null): { id?: string; name?: string; policy?: string } | null {
  if (!request) return null;
  const args = request.args && typeof request.args === 'object' ? (request.args as Record<string, unknown>) : {};
  const id = request.moduleId || (typeof args.module_id === 'string' ? args.module_id : undefined);
  const name = request.moduleName || (typeof args.module_name === 'string' ? args.module_name : undefined);
  const policy = request.modulePolicy || (typeof args.module_policy === 'string' ? args.module_policy : undefined);
  if (!id && !name && !policy) return null;
  return { id, name, policy };
}
