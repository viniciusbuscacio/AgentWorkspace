import { useCallback, useEffect, useState } from 'react';
import { Button } from '@ui/button';
import { mcpService } from '@services/mcp.service';
import { restService } from '@services/rest.service';
import { webaccessService } from '@services/webaccess.service';
import { onAwEvent } from '@services/events';
import { openModuleView } from '@/lib/open-view';

// Settings › Servers: the hub behind the sidebar exposure dot. One card per
// network server (REST, MCP, Web) with live status and start/stop; the full
// configuration lives in each server's own page (Apps modules), one click
// away. State refreshes on the same servers:state push the dot uses.

type ServerRow = {
  running: boolean;
  port: number;
  url: string;
  error: string;
};

type Rows = { rest: ServerRow; mcp: ServerRow; web: ServerRow };

const SERVER_CARDS = [
  {
    key: 'rest' as const,
    title: 'REST API',
    description: 'Drive aw actions from scripts and agents over HTTP (bearer token).',
    view: 'rest-server',
  },
  {
    key: 'mcp' as const,
    title: 'MCP Server',
    description: 'Expose the aw action registry to external MCP clients.',
    view: 'mcp-server',
  },
  {
    key: 'web' as const,
    title: 'Web Access',
    description: 'Remote browser access to the full UI (vault-password login).',
    view: 'web-server',
  },
];

function toRow(status: { running: boolean; port: number; url?: string; error?: string }): ServerRow {
  return { running: status.running, port: status.port, url: status.url ?? '', error: status.error ?? '' };
}

export function ServersPage() {
  const [rows, setRows] = useState<Rows | null>(null);
  const [busyKey, setBusyKey] = useState<string>('');

  const refresh = useCallback(async () => {
    const [rest, mcp, web] = await Promise.all([
      restService.getStatus(),
      mcpService.getStatus(),
      webaccessService.getStatus(),
    ]);
    setRows({ rest: toRow(rest), mcp: toRow(mcp), web: toRow(web) });
  }, []);

  useEffect(() => {
    void refresh();
    // The backend pushes servers:state after every start/stop, including ones
    // triggered elsewhere (agent actions, autostart, firewall retries).
    return onAwEvent('servers:state', () => void refresh());
  }, [refresh]);

  async function toggle(key: 'rest' | 'mcp' | 'web', running: boolean) {
    const service = key === 'rest' ? restService : key === 'mcp' ? mcpService : webaccessService;
    setBusyKey(key);
    try {
      if (running) await service.stop();
      else await service.start();
      await refresh();
    } finally {
      setBusyKey('');
    }
  }

  return (
    <div className="flex max-w-2xl flex-col gap-4">
      <p className="text-sm text-muted-foreground">
        The network listeners this app can open. The green dot in the sidebar is
        visible while at least one of them is running.
      </p>
      {SERVER_CARDS.map((card) => {
        const row = rows?.[card.key];
        return (
          <div key={card.key} className="rounded-lg border border-border bg-card p-6" data-awid={`servers-hub-${card.key}`}>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  {row?.running && <span className="server-dot" aria-hidden="true" />}
                  <h2 className="text-base font-semibold">{card.title}</h2>
                </div>
                <p className="mt-0.5 text-sm text-muted-foreground">{card.description}</p>
                <p className="mt-1 text-sm font-medium">
                  {row == null ? '…' : row.running ? `Running — ${row.url || `port ${row.port}`}` : 'Stopped'}
                </p>
                {row?.error ? <p className="mt-1 text-sm text-destructive">{row.error}</p> : null}
              </div>
              <div className="flex shrink-0 gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  icon={row?.running ? 'stop' : 'play_arrow'}
                  disabled={row == null || busyKey === card.key}
                  onClick={() => void toggle(card.key, Boolean(row?.running))}
                >
                  {row?.running ? 'Stop' : 'Start'}
                </Button>
                <Button variant="outline" size="sm" icon="open_in_new" onClick={() => openModuleView(card.view)}>
                  Open settings
                </Button>
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
