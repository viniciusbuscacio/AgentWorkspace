import { useEffect, useMemo, useRef, useState } from 'react';
import type { LeaveGuard } from '@modules/module-contract';
import { readLastSettingsPage, writeLastSettingsPage } from './settings-nav-storage';
import {
  ICON_SIZE_MAX,
  ICON_SIZE_MIN,
  ICON_SIZE_STEP,
  cardMetrics,
  cardStateClass,
  cardStyleVars,
} from '@modules/home/home-modules';
import { ProvidersPage } from './pages/ProvidersPage';
import { ThemePage } from './pages/ThemePage';
import { FontPage } from './pages/FontPage';
import { VaultPage } from './pages/VaultPage';
import { SecurityPage } from './pages/SecurityPage';
import { ModuleHelpButton } from '@/components/patterns/ModuleHelpButton';
import { SystemAccessPage } from './pages/SystemAccessPage';
import { ServersPage } from './pages/ServersPage';
import { ServerTlsPage } from './pages/ServerTlsPage';
import { AgentFirewallPage } from './pages/AgentFirewallPage';
import { PermissionsPage } from './pages/PermissionsPage';
import { UserMemoryPage } from './pages/UserMemoryPage';
import { AgentInstructionsPage } from './pages/AgentInstructionsPage';
import { AboutPage } from './pages/AboutPage';
import { DebugPage } from './pages/DebugPage';
import { AgentPage } from './pages/AgentPage';
import { LogsModule } from '@modules/logs/LogsModule';
import { WallpaperModule } from '@modules/wallpaper/WallpaperModule';

// MCP Server and REST API pages moved to Apps (polish-wave-3-spec Decision 5).
// McpServerPage and RestApiPage are still imported there from settings/pages/.
export type SettingsPage = 'providers' | 'theme' | 'fonts' | 'wallpaper' | 'vault' | 'security' | 'servers' | 'tls' | 'firewall' | 'permissions' | 'system-access' | 'memory' | 'instructions' | 'agent' | 'logs' | 'debug' | 'about';

type SettingsModuleCard = { id: SettingsPage; title: string; description: string; icon: string };

const SETTINGS_ICON_SIZE_KEY = 'settings-icon-size';
const ICON_SIZE_DEFAULT = 220;

const modules: SettingsModuleCard[] = [
  { id: 'providers', title: 'LLM Providers', description: 'Models, credentials and active provider.', icon: 'hub' },
  { id: 'theme', title: 'Theme', description: 'Color theme for the workspace.', icon: 'palette' },
  { id: 'fonts', title: 'Fonts', description: 'Font family, base size and readability.', icon: 'format_size' },
  { id: 'wallpaper', title: 'Wallpaper', description: 'Choose the Home and Apps background wallpaper.', icon: 'wallpaper' },
  { id: 'vault', title: 'Vault', description: 'Vault location, recovery and password.', icon: 'database' },
  { id: 'security', title: 'Security', description: 'Auto-lock and local secrets.', icon: 'lock' },
  { id: 'servers', title: 'Servers', description: 'REST, MCP and Web listeners at a glance — the sidebar dot opens here.', icon: 'dns' },
  { id: 'tls', title: 'TLS', description: 'HTTPS certificate shared by the Web, MCP and REST servers.', icon: 'https' },
  { id: 'firewall', title: 'Agent Firewall', description: 'Network access rules for the Web, MCP and REST servers.', icon: 'security' },
  { id: 'permissions', title: 'Permissions', description: 'Shell access and filesystem paths visible to the agent.', icon: 'shield_lock' },
  { id: 'system-access', title: 'macOS Permissions', description: 'OS permissions the app needs outside its own window.', icon: 'desktop_windows' },
  { id: 'memory', title: 'Memory', description: 'What the agent remembers about you.', icon: 'psychology' },
  { id: 'instructions', title: 'Agent Instructions', description: 'Markdown instruction files that shape how the agent behaves.', icon: 'rule' },
  { id: 'agent', title: 'Subagents', description: 'Subagent delegation behavior.', icon: 'account_tree' },
  { id: 'logs', title: 'Logs', description: 'Inspect app and agent activity logs.', icon: 'clarify' },
  { id: 'debug', title: 'Debug Mode', description: 'Prompt payload visibility for each LLM turn.', icon: 'bug_report' },
  // MCP Server and REST API have moved to Apps (polish-wave-3-spec Decision 5).
  { id: 'about', title: 'About', description: 'Version and build information.', icon: 'info' },
];

// Settings pages with a workspace-guide help topic get the module "?" in the
// detail header; the value is the module name sent in the help question.
const SETTINGS_HELP: Partial<Record<SettingsPage, string>> = {
  memory: 'Memory',
};

interface SettingsModuleProps {
  onLocked: () => Promise<void> | void;
  /** When set, the module opens directly to this page instead of the settings grid. */
  initialPage?: SettingsPage;
  /** Receives the active child unsaved-changes guard (e.g. the Skills editor) so
   *  the shell can confirm before any navigation discards it. */
  onRegisterLeaveGuard?: (guard: LeaveGuard | null) => void;
}

function getInitialSettingsIconSize(): number {
  const saved = Number(localStorage.getItem(SETTINGS_ICON_SIZE_KEY));
  if (!Number.isNaN(saved) && saved >= ICON_SIZE_MIN && saved <= ICON_SIZE_MAX) return saved;
  return ICON_SIZE_DEFAULT;
}

// getInitialPage resolves the page to open on mount: an explicit deep link
// (Account menu → a specific page) always wins; otherwise restore the last
// sub-page the user was on this session so returning from a chat lands there
// instead of the grid. An unknown stored value falls back to the grid.
function getInitialPage(initialPage?: SettingsPage): SettingsPage | null {
  if (initialPage) return initialPage;
  const stored = readLastSettingsPage();
  return modules.some((m) => m.id === stored) ? (stored as SettingsPage) : null;
}

function persistSettingsIconSize(size: number): void {
  try { localStorage.setItem(SETTINGS_ICON_SIZE_KEY, String(size)); } catch { /* ignore */ }
}

export function SettingsModule({ onLocked, initialPage, onRegisterLeaveGuard }: SettingsModuleProps) {
  const [page, setPageState] = useState<SettingsPage | null>(() => getInitialPage(initialPage));
  // Persist the current sub-page for the session so a remount (after visiting a
  // chat) restores it. Empty string clears it back to the grid.
  useEffect(() => {
    writeLastSettingsPage(page);
  }, [page]);
  // The active child's unsaved-changes guard (the Skills editor). It bubbles up
  // to the shell and also gates this module's own page switches.
  const childGuardRef = useRef<LeaveGuard | null>(null);
  const registerChildGuard = (guard: LeaveGuard | null) => {
    childGuardRef.current = guard;
    onRegisterLeaveGuard?.(guard);
  };
  const requestLeave = (proceed: () => void) => {
    const guard = childGuardRef.current;
    if (guard) guard(proceed);
    else proceed();
  };
  const [providerDetailTitle, setProviderDetailTitle] = useState<string | null>(null);
  const [themeDetailTitle, setThemeDetailTitle] = useState<string | null>(null);
  const [instructionsDetailTitle, setInstructionsDetailTitle] = useState<string | null>(null);
  const [providersListRequest, setProvidersListRequest] = useState(0);
  const [instructionsListRequest, setInstructionsListRequest] = useState(0);
  const [search, setSearch] = useState('');
  const [iconSize, setIconSize] = useState<number>(() => getInitialSettingsIconSize());
  const selected = modules.find((item) => item.id === page);
  const metrics = useMemo(() => cardMetrics(iconSize), [iconSize]);

  const filteredModules = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) return modules;
    return modules.filter(
      (item) => item.title.toLowerCase().includes(query) || item.description.toLowerCase().includes(query),
    );
  }, [search]);

  function setPage(next: SettingsPage | null) {
    setProviderDetailTitle(null);
    setThemeDetailTitle(null);
    setInstructionsDetailTitle(null);
    setPageState(next);
  }

  function openProvidersList() {
    setProviderDetailTitle(null);
    setProvidersListRequest((current) => current + 1);
  }

  function changeIconSize(size: number) {
    setIconSize(size);
    persistSettingsIconSize(size);
  }

  if (!page) {
    return (
      <section className="home-screen" data-awid="settings-home-screen">
        <div className="home-header">
          <h1 className="home-title">Settings</h1>
          <p className="home-subtitle">Choose a section to configure.</p>
        </div>

        <div className="home-toolbar">
          <div className="home-search">
            <span className="nav-icon home-search-icon material-symbols-outlined" aria-hidden="true">search</span>
            <input
              type="text"
              className="home-search-input"
              placeholder="Search settings..."
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              aria-label="Search settings"
            />
            {search && (
              <button
                type="button"
                className="chat-search-clear"
                onClick={() => setSearch('')}
                aria-label="Clear search"
              >
                <span className="material-symbols-outlined" aria-hidden="true">close</span>
              </button>
            )}
          </div>

          <div className="home-iconsize" title="Settings card size">
            <button
              type="button"
              className="home-iconsize-btn"
              onClick={() => changeIconSize(ICON_SIZE_MIN)}
              title="Set minimum card size"
              aria-label="Set minimum card size"
            >
              <span className="material-symbols-outlined" aria-hidden="true">apps</span>
            </button>
            <input
              type="range"
              className="home-size-range"
              min={ICON_SIZE_MIN}
              max={ICON_SIZE_MAX}
              step={ICON_SIZE_STEP}
              value={iconSize}
              onChange={(event) => changeIconSize(Number(event.target.value))}
              aria-label="Settings card size"
            />
            <button
              type="button"
              className="home-iconsize-btn"
              onClick={() => changeIconSize(ICON_SIZE_MAX)}
              title="Set maximum card size"
              aria-label="Set maximum card size"
            >
              <span className="material-symbols-outlined" aria-hidden="true">grid_view</span>
            </button>
          </div>
        </div>

        {filteredModules.length > 0 ? (
          <div
            className="home-grid"
            style={{ gridTemplateColumns: `repeat(auto-fill, minmax(${metrics.gridMin}px, 1fr))` }}
          >
            {filteredModules.map((item) => (
              <SettingsCard key={item.id} item={item} metrics={metrics} onOpen={() => requestLeave(() => setPage(item.id))} />
            ))}
          </div>
        ) : (
          <div className="home-empty">No settings found for &quot;{search}&quot;</div>
        )}
      </section>
    );
  }

  return (
    <section className="home-screen" data-awid="settings-detail-screen">
      <div className="home-header">
        <div className="flex items-start justify-between gap-3">
          <h1 className="home-title">
          <SettingsDetailTitle
            current={selected ? selected.title : 'Settings'}
            child={
              page === 'providers'
                ? providerDetailTitle
                : page === 'theme'
                  ? themeDetailTitle
                  : page === 'instructions'
                    ? instructionsDetailTitle
                    : null
            }
            onSettingsClick={() => requestLeave(() => setPage(null))}
            onCurrentClick={
              page === 'providers' && providerDetailTitle
                ? openProvidersList
                : page === 'instructions' && instructionsDetailTitle
                  ? () => setInstructionsListRequest((current) => current + 1)
                  : undefined
            }
          />
          </h1>
          {selected && SETTINGS_HELP[selected.id] && (
            <ModuleHelpButton module={SETTINGS_HELP[selected.id]!} />
          )}
        </div>
        <p className="home-subtitle">{selected ? selected.description : 'Choose a section to configure.'}</p>
      </div>

      {page === 'providers' && (
        <ProvidersPage
          listRequest={providersListRequest}
          onProviderDetailTitleChange={setProviderDetailTitle}
        />
      )}
      {page === 'theme' && <ThemePage onDetailTitleChange={setThemeDetailTitle} onBack={() => setPage(null)} />}
      {page === 'fonts' && <FontPage />}
      {page === 'wallpaper' && <WallpaperModule />}
      {page === 'vault' && <VaultPage onLocked={onLocked} />}
      {page === 'security' && <SecurityPage />}
      {page === 'system-access' && <SystemAccessPage />}
      {page === 'servers' && <ServersPage />}
      {page === 'tls' && <ServerTlsPage />}
      {page === 'firewall' && <AgentFirewallPage />}
      {page === 'permissions' && <PermissionsPage />}
      {page === 'memory' && <UserMemoryPage />}
      {page === 'instructions' && (
        <AgentInstructionsPage
          onDetailTitleChange={setInstructionsDetailTitle}
          listRequest={instructionsListRequest}
          onRegisterLeaveGuard={registerChildGuard}
        />
      )}
      {page === 'agent' && <AgentPage onBack={() => setPage(null)} />}
      {page === 'logs' && <LogsModule />}
      {page === 'debug' && <DebugPage />}
      {page === 'about' && <AboutPage />}
    </section>
  );
}

function SettingsDetailTitle({
  current,
  child,
  onSettingsClick,
  onCurrentClick,
}: {
  current: string;
  child?: string | null;
  onSettingsClick: () => void;
  onCurrentClick?: () => void;
}) {
  const textButtonClass = 'breadcrumb-link m-0 min-w-0 truncate border-0 bg-transparent p-0 text-left font-inherit text-current focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50';
  return (
    <span className="inline-flex min-w-0 items-baseline gap-2">
      <button type="button" className={textButtonClass} onClick={onSettingsClick}>
        Settings
      </button>
      <span className="shrink-0 text-muted-foreground" aria-hidden="true">›</span>
      {onCurrentClick ? (
        <button type="button" className={textButtonClass} onClick={onCurrentClick}>
          {current}
        </button>
      ) : (
        <span className="min-w-0 truncate">{current}</span>
      )}
      {child && (
        <>
          <span className="shrink-0 text-muted-foreground" aria-hidden="true">›</span>
          <span className="min-w-0 truncate">{child}</span>
        </>
      )}
    </span>
  );
}

function SettingsCard({
  item,
  metrics,
  onOpen,
}: {
  item: SettingsModuleCard;
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
      title={`${item.title} - ${item.description}`}
      aria-label={item.title}
      onClick={onOpen}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen();
        }
      }}
    >
      <div className="home-card-head">
        <span className="home-card-icon material-symbols-outlined" aria-hidden="true">{item.icon}</span>
        <div className="home-card-title">{item.title}</div>
      </div>
      <div className="home-card-desc">{item.description}</div>
    </div>
  );
}
