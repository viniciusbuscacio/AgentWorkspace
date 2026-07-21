import {
  AddMcpConnection,
  GetMcpConnection,
  ListMcpConnections,
  ListMcpTools,
  RemoveMcpConnection,
  SetMcpConnectionEnabled,
  TestMcpConnection,
  UpdateMcpConnection,
} from '@wails/go/main/App';
import type { domain, dto } from '@wails/go/models';

export type McpConnection = domain.McpConnection;
export type McpConnectionsResult = dto.McpConnectionsResult;
export type McpConnectionResult = dto.McpConnectionResult;
export type McpConnectionTestResult = dto.McpConnectionTestResult;
export type McpToolsResult = dto.McpToolsResult;
export type McpToolView = dto.McpToolView;

export interface McpConnectionDraft {
  name: string;
  url: string;
  transport?: string;
  authType?: string;
  token?: string;
  enabled?: boolean;
}

export interface McpConnectionUpdate {
  id: string;
  name: string;
  url: string;
  authType: string;
  enabled: boolean;
  token?: string;
  clearToken?: boolean;
}

// mcpConnectionsService is the only frontend import point for the generated MCP
// Connections Wails bindings, mirroring skills.service.ts / instructions.service.ts.
export const mcpConnectionsService = {
  list(): Promise<McpConnectionsResult> {
    return ListMcpConnections();
  },
  get(id: string): Promise<McpConnectionResult> {
    return GetMcpConnection(id);
  },
  add(input: McpConnectionDraft): Promise<McpConnectionResult> {
    return AddMcpConnection(
      input.name,
      input.url,
      input.transport ?? 'streamable_http',
      input.authType ?? 'none',
      input.token ?? '',
      input.enabled ?? true,
    );
  },
  update(input: McpConnectionUpdate): Promise<McpConnectionResult> {
    return UpdateMcpConnection(
      input.id,
      input.name,
      input.url,
      input.authType,
      input.enabled,
      input.token ?? '',
      input.clearToken ?? false,
    );
  },
  remove(id: string): Promise<McpConnectionResult> {
    return RemoveMcpConnection(id);
  },
  setEnabled(id: string, enabled: boolean): Promise<McpConnectionResult> {
    return SetMcpConnectionEnabled(id, enabled);
  },
  test(id: string): Promise<McpConnectionTestResult> {
    return TestMcpConnection(id);
  },
  listTools(id: string): Promise<McpToolsResult> {
    return ListMcpTools(id);
  },
};
