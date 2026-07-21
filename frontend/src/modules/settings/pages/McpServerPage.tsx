import { ApiServerSettingsCards } from '../components/ApiServerSettingsCards';
import { mcpService } from '@services/mcp.service';

export function McpServerPage() {
  return (
    <ApiServerSettingsCards
      service={mcpService}
      serverTitle="MCP server"
      awid="mcp-server"
      endpointForPort={(port) => `http://127.0.0.1:${port}/mcp`}
      description={
        <>
          External agents (Claude Code, other MCP clients) drive this app through the single{' '}
          <code>aw</code> tool. Access requires the bearer token below, passes the Agent
          Firewall, and works only while the vault is unlocked.
          {' '}
          <strong>To connect:</strong> point the client at the endpoint with{' '}
          <code>Authorization: Bearer &lt;token&gt;</code>; <code>aw.actions</code> lists every
          capability.
        </>
      }
    />
  );
}
