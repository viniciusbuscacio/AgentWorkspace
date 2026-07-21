import { useCallback, useEffect, useMemo, useState } from 'react';
import { Switch } from '@ui/switch';
import { AwIcon } from '@ui/aw-icon';
import { browserService } from '@services/browser.service';
import { mcpService } from '@services/mcp.service';
import { restService } from '@services/rest.service';
import { webaccessService } from '@services/webaccess.service';
import {
  type AppModule,
  type OrderMode,
  cardMetrics,
  cardStateClass,
  cardStyleVars,
  getInitialIconSize,
  getInitialOrderMode,
  orderModules,
  persistIconSize,
  persistOrderMode,
  recordModuleUsage,
} from './home-modules';
import { AppGridToolbar } from '@patterns/AppGridToolbar';

// Apps/Home screen — faithful React port of the AW2 HomeModule (Open Module):
// search + order-by + card-size toolbar, and a grid of module launcher cards.
// The grid is the workspace catalog: clicking a card adds the module to the
// workspace (if needed) and opens it.

interface HomeModuleProps {
  modules: AppModule[];
  onOpenModule: (type: string) => void;
}

export function HomeModule({ modules: catalog, onOpenModule }: HomeModuleProps) {
  const [search, setSearch] = useState('');
  const [orderMode, setOrderMode] = useState<OrderMode>(() => getInitialOrderMode());
  const [iconSize, setIconSize] = useState<number>(() => getInitialIconSize());

  const metrics = useMemo(() => cardMetrics(iconSize), [iconSize]);

  const modules = useMemo(() => {
    const query = search.trim().toLowerCase();
    const ordered = orderModules(catalog, orderMode);
    if (!query) return ordered;
    return ordered.filter(
      (m) => m.name.toLowerCase().includes(query) || m.description.toLowerCase().includes(query),
    );
  }, [catalog, search, orderMode]);

  function changeOrder(mode: OrderMode) {
    setOrderMode(mode);
    persistOrderMode(mode);
  }

  function changeIconSize(size: number) {
    setIconSize(size);
    persistIconSize(size);
  }

  function openModule(type: string) {
    recordModuleUsage(type);
    onOpenModule(type);
  }

  return (
    <section className="home-screen">
      <div className="home-header">
        <h1 className="home-title">Open Apps and Modules</h1>
        <p className="home-subtitle">Choose a module type to open in your workspace.</p>
      </div>

      <AppGridToolbar
        search={search}
        onSearchChange={setSearch}
        searchPlaceholder="Search modules..."
        orderMode={orderMode}
        onOrderChange={changeOrder}
        iconSize={iconSize}
        onIconSizeChange={changeIconSize}
        sizeLabel="Module card size"
      />

      {modules.length > 0 ? (
        <div
          className="home-grid"
          style={{ gridTemplateColumns: `repeat(auto-fill, minmax(${metrics.gridMin}px, 1fr))` }}
        >
          {modules.map((mod) => (
            <ModuleCard key={mod.type} module={mod} metrics={metrics} onOpen={() => openModule(mod.type)} />
          ))}
        </div>
      ) : (
        <div className="home-empty">No modules found for &quot;{search}&quot;</div>
      )}
    </section>
  );
}

function isBrowserModule(type: string): boolean {
  return type === 'browser-chrome' || type === 'browser-edge';
}

// BrowserCardToggle polls the browser CDP-endpoint state every 5 s (same
// interval as BrowserModule) and surfaces a Switch that starts/stops the
// browser. It never exposes the personal-profile restart flow — that lives
// in the module view where the full warning UX is present.
function BrowserCardToggle({ browserId }: { browserId: string }) {
  const [running, setRunning] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    const result = await browserService.status(browserId);
    if (result.success && result.status) {
      setRunning(result.status.running);
    }
  }, [browserId]);

  useEffect(() => {
    void refresh();
    const timer = setInterval(() => void refresh(), 5000);
    return () => clearInterval(timer);
  }, [refresh]);

  async function toggle(checked: boolean) {
    if (busy || running === null) return;
    setBusy(true);
    try {
      if (checked) {
        await browserService.start(browserId);
      } else {
        await browserService.stop(browserId);
      }
    } finally {
      setBusy(false);
      await refresh();
    }
  }

  return (
    <Switch
      size="sm"
      checked={running === true}
      disabled={busy || running === null}
      onCheckedChange={(checked) => void toggle(checked)}
      aria-label={`Toggle ${browserId}`}
    />
  );
}

// SERVER_RUNTIMES maps the server modules to their start/stop services so
// their Apps cards carry the same runtime toggle the browser cards have.
const SERVER_RUNTIMES: Record<string, {
  getStatus: () => Promise<{ running: boolean; error?: string }>;
  start: () => Promise<{ running: boolean; error?: string }>;
  stop: () => Promise<{ running: boolean; error?: string }>;
}> = {
  'mcp-server': {
    getStatus: async () => { const s = await mcpService.getStatus(); return { running: s.running, error: s.error }; },
    start: async () => { const s = await mcpService.start(); return { running: s.running, error: s.error }; },
    stop: async () => { const s = await mcpService.stop(); return { running: s.running, error: s.error }; },
  },
  'rest-server': {
    getStatus: async () => { const s = await restService.getStatus(); return { running: s.running, error: s.error }; },
    start: async () => { const s = await restService.start(); return { running: s.running, error: s.error }; },
    stop: async () => { const s = await restService.stop(); return { running: s.running, error: s.error }; },
  },
  'web-server': {
    getStatus: async () => { const s = await webaccessService.getStatus(); return { running: s.running, error: s.error }; },
    start: async () => { const s = await webaccessService.start(); return { running: s.running, error: s.error }; },
    stop: async () => { const s = await webaccessService.stop(); return { running: s.running, error: s.error }; },
  },
};

// ServerCardToggle mirrors BrowserCardToggle for the server modules: it polls
// the service state and starts/stops it without opening the module view.
function ServerCardToggle({ moduleId }: { moduleId: string }) {
  const runtime = SERVER_RUNTIMES[moduleId];
  const [running, setRunning] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    const result = await runtime.getStatus();
    setRunning(result.running);
  }, [runtime]);

  useEffect(() => {
    void refresh();
    const timer = setInterval(() => void refresh(), 5000);
    return () => clearInterval(timer);
  }, [refresh]);

  async function toggle(checked: boolean) {
    if (busy || running === null) return;
    setBusy(true);
    try {
      const result = checked ? await runtime.start() : await runtime.stop();
      setRunning(result.running);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Switch
      size="sm"
      checked={running === true}
      disabled={busy || running === null}
      onCheckedChange={(checked) => void toggle(checked)}
      aria-label={`Toggle ${moduleId}`}
    />
  );
}

function ModuleCard({
  module: mod,
  metrics,
  onOpen,
}: {
  module: AppModule;
  metrics: ReturnType<typeof cardMetrics>;
  onOpen: () => void;
}) {
  const stateClass = cardStateClass(metrics);
  return (
    <div
      className={`home-card${stateClass ? ` ${stateClass}` : ''}`}
      role="button"
      tabIndex={0}
      style={cardStyleVars(metrics)}
      title={`${mod.name} - ${mod.description}`}
      aria-label={mod.name}
      onClick={onOpen}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen();
        }
      }}
    >
      <div className="home-card-head">
        <AwIcon icon={mod.icon} className="home-card-icon" />
        <div className="home-card-title">{mod.name}</div>
        {/* Browser and server modules get a runtime toggle on the card. */}
        {isBrowserModule(mod.type) && (
          <div
            className="home-card-toggle"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <BrowserCardToggle browserId={mod.type} />
          </div>
        )}
        {mod.type in SERVER_RUNTIMES && (
          <div
            className="home-card-toggle"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <ServerCardToggle moduleId={mod.type} />
          </div>
        )}
      </div>
      <div className="home-card-desc">{mod.description}</div>
    </div>
  );
}
