import { GetServerIndicator } from '@wails/go/main/App';
import type { ServersStatePayload } from '@services/events';

// The aggregate network-server state (REST, MCP, Web) behind the sidebar
// exposure dot. Initial pull only — live updates arrive via the
// servers:state event, never by polling.

export type ServerIndicator = ServersStatePayload;

export const serversService = {
  // async so a missing Wails bridge (tests, web mode) surfaces as a rejected
  // promise the caller can catch instead of a synchronous TypeError.
  async getIndicator(): Promise<ServerIndicator> {
    return GetServerIndicator();
  },
};
