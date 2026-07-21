import type { ReactNode } from 'react';
import { McpServerPage } from '@modules/settings/pages/McpServerPage';
import { RestApiPage } from '@modules/settings/pages/RestApiPage';
import { WebAccessPage } from '@modules/settings/pages/WebAccessPage';
import { ModuleHelpButton } from '@/components/patterns/ModuleHelpButton';

// The three server surfaces are ordinary workspace modules (Apps card ->
// sidebar item + view), thin shells around the existing server pages.
function ServerModuleShell({ title, subtitle, children }: { title: string; subtitle: string; children: ReactNode }) {
  return (
    <section className="home-screen">
      <div className="home-header">
        <div className="flex items-start justify-between gap-3">
          <h1 className="home-title">{title}</h1>
          <ModuleHelpButton module={title} />
        </div>
        <p className="home-subtitle">{subtitle}</p>
      </div>
      {children}
    </section>
  );
}

export function McpServerModule() {
  return (
    <ServerModuleShell title="MCP Server" subtitle="Expose this app to external agents over MCP.">
      <McpServerPage />
    </ServerModuleShell>
  );
}

export function RestServerModule() {
  return (
    <ServerModuleShell title="REST API Server" subtitle="Expose this app to external agents over REST API.">
      <RestApiPage />
    </ServerModuleShell>
  );
}

export function WebServerModule() {
  return (
    <ServerModuleShell title="Web Access" subtitle="Open the full app in a remote browser over Tailscale.">
      <WebAccessPage />
    </ServerModuleShell>
  );
}
