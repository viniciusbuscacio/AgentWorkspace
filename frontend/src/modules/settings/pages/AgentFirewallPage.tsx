import { useEffect, useMemo, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@ui/card';
import { Table, TableBody, TableCell, TableFooter, TableHead, TableHeader, TableRow } from '@ui/table';
import { ZoomSafeSelect } from '@ui/zoom-safe-select';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import {
  agentfwService,
  type AgentFirewallState,
  type FirewallRuleInput,
} from '@services/agentfw.service';

const ACTIONS = ['PERMIT', 'DENY'];
const INTERFACES = ['loopback', 'tailscale', 'lan', 'public', 'all'];

const SELECT_CLASS =
  'h-9 rounded-md border border-input bg-input px-2 text-sm text-foreground outline-none transition-colors hover:bg-muted focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50';

function newRule(services: string[]): FirewallRuleInput {
  return { action: 'PERMIT', service: services[0] ?? 'rest', interface: 'loopback', origin: '127.0.0.1/32' };
}

function sameRules(a: FirewallRuleInput[], b: FirewallRuleInput[]): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

export function AgentFirewallPage() {
  const [saved, setSaved] = useState<FirewallRuleInput[]>([]);
  const [rules, setRules] = useState<FirewallRuleInput[]>([]);
  const [services, setServices] = useState<string[]>([]);
  // Rules render as plain text (same style as the implicit DENY row); only
  // the row being edited becomes a form.
  const [editing, setEditing] = useState<number | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  function apply(state: AgentFirewallState) {
    const next = (state.rules ?? []).map((r) => ({
      action: r.action,
      service: r.service,
      interface: r.interface,
      origin: r.origin,
    }));
    setSaved(next);
    setRules(next);
    setServices(state.services ?? []);
    setError(state.success ? '' : state.error || 'Failed to load firewall.');
  }

  useEffect(() => {
    void agentfwService.getState().then(apply);
  }, []);

  const dirty = useMemo(() => !sameRules(rules, saved), [rules, saved]);
  const serviceOptions = useMemo(() => ['all', ...services], [services]);

  function patch(index: number, field: keyof FirewallRuleInput, value: string) {
    setRules((prev) => prev.map((r, i) => (i === index ? { ...r, [field]: value } : r)));
  }
  function move(index: number, delta: number) {
    setRules((prev) => {
      const next = [...prev];
      const target = index + delta;
      if (target < 0 || target >= next.length) return prev;
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });
    // The editing form follows its row across the swap.
    setEditing((cur) => {
      if (cur === index) return index + delta;
      if (cur === index + delta) return index;
      return cur;
    });
  }
  function remove(index: number) {
    setRules((prev) => prev.filter((_, i) => i !== index));
    setEditing((cur) => {
      if (cur === null) return null;
      if (cur === index) return null;
      return cur > index ? cur - 1 : cur;
    });
  }
  function add() {
    setRules((prev) => {
      setEditing(prev.length); // a fresh rule opens ready to edit
      return [...prev, newRule(serviceOptions)];
    });
  }

  async function save() {
    setBusy(true);
    setMessage('');
    setError('');
    try {
      const state = await agentfwService.setRules(rules);
      if (state.success) {
        apply(state);
        setMessage('Firewall rules saved and applied.');
      } else {
        // A failed save returns the previously persisted rules — applying them
        // would wipe the user's in-progress edits. Keep the local list so the
        // rejected rule can be corrected in place.
        setError(state.error || 'Failed to save rules.');
      }
    } finally {
      setBusy(false);
    }
  }
  function cancel() {
    setRules(saved);
    setEditing(null);
    setMessage('');
    setError('');
  }

  return (
    <div className="grid gap-4">
      <Card className="rounded-lg">
        <CardHeader>
          <CardTitle>Agent Firewall</CardTitle>
          <CardDescription>
            One ordered access list for every local server (MCP, REST, Web). Rules are read top-down;
            the first match wins. Anything not permitted is blocked by the final <b>DENY ALL ALL</b>{' '}
            rule — loopback included. A PERMIT with origin <b>any</b> on a non-loopback interface opens
            that server to every source that can reach it — your call, use with care.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          {error && (
            <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          )}

          {/* Rule table: ordered ACL, first match wins, implicit DENY as the footer. */}
          <div className="rounded-md border border-border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="w-10 text-center">#</TableHead>
                  <TableHead className="w-36">Action</TableHead>
                  <TableHead className="w-36">Service</TableHead>
                  <TableHead className="w-40">Interface</TableHead>
                  <TableHead>Origin (CIDR / IP / any)</TableHead>
                  <TableHead className="w-28" aria-label="Row actions" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {rules.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={6} className="py-4 text-center text-muted-foreground">
                      No rules — every server is blocked (deny-all). Add a PERMIT rule to open one.
                    </TableCell>
                  </TableRow>
                )}
                {rules.map((rule, i) => (
                  <TableRow key={i}>
                    <TableCell className="text-center tabular-nums text-muted-foreground">{i + 1}</TableCell>
                    {editing === i ? (
                      <>
                        <TableCell>
                          <ZoomSafeSelect value={rule.action} onValueChange={(value) => patch(i, 'action', value)}
                            options={ACTIONS.map((a) => ({ value: a, label: a }))} aria-label="Action"
                            className={`h-8 font-semibold ${rule.action === 'PERMIT' ? 'text-emerald-500' : 'text-destructive'}`} />
                        </TableCell>
                        <TableCell>
                          <ZoomSafeSelect value={rule.service} onValueChange={(value) => patch(i, 'service', value)}
                            options={serviceOptions.map((s) => ({ value: s, label: s.toUpperCase() }))} aria-label="Service"
                            className="h-8" />
                        </TableCell>
                        <TableCell>
                          <ZoomSafeSelect value={rule.interface} onValueChange={(value) => patch(i, 'interface', value)}
                            options={INTERFACES.map((f) => ({ value: f, label: f }))} aria-label="Interface"
                            className="h-8" />
                        </TableCell>
                        <TableCell>
                          <input className={`${SELECT_CLASS} h-8 w-full min-w-[160px] font-mono text-xs`} value={rule.origin}
                            placeholder="CIDR / IP / any" aria-label="Origin"
                            onChange={(e) => patch(i, 'origin', e.target.value)} />
                        </TableCell>
                      </>
                    ) : (
                      <>
                        <TableCell className={`font-semibold ${rule.action === 'PERMIT' ? 'text-emerald-500' : 'text-destructive'}`}>
                          {rule.action}
                        </TableCell>
                        <TableCell>{rule.service.toUpperCase()}</TableCell>
                        <TableCell>{rule.interface}</TableCell>
                        <TableCell className="font-mono text-xs">{rule.origin || 'any'}</TableCell>
                      </>
                    )}
                    <TableCell>
                      <div className="flex items-center justify-end">
                        <Button type="button" variant="ghost" size="icon-sm"
                          icon={editing === i ? 'check' : 'edit'}
                          aria-label={editing === i ? 'Done editing' : 'Edit rule'}
                          onClick={() => setEditing(editing === i ? null : i)} />
                        <Button type="button" variant="ghost" size="icon-sm" icon="keyboard_arrow_up"
                          aria-label="Move up" disabled={i === 0} onClick={() => move(i, -1)} />
                        <Button type="button" variant="ghost" size="icon-sm" icon="keyboard_arrow_down"
                          aria-label="Move down" disabled={i === rules.length - 1} onClick={() => move(i, 1)} />
                        <Button type="button" variant="ghost" size="icon-sm" icon="delete"
                          aria-label="Delete rule" onClick={() => remove(i)} />
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
              <TableFooter>
                <TableRow className="text-muted-foreground hover:bg-transparent">
                  <TableCell className="text-center">
                    <span className="material-symbols-outlined align-middle text-[16px]" aria-hidden="true">block</span>
                  </TableCell>
                  <TableCell className="font-semibold text-destructive/70">DENY</TableCell>
                  <TableCell>ALL</TableCell>
                  <TableCell>ALL</TableCell>
                  <TableCell className="font-mono text-xs">any</TableCell>
                  <TableCell className="text-right text-xs font-normal italic">implicit · always last</TableCell>
                </TableRow>
              </TableFooter>
            </Table>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" variant="outline" icon="add" onClick={add}>Add rule</Button>
            <SaveCancelActions
              onSave={save}
              onCancel={cancel}
              disabled={!dirty || busy}
              status={dirty ? <span className="text-xs text-muted-foreground">Unsaved changes</span> : null}
            />
          </div>
        </CardContent>
      </Card>

    </div>
  );
}
