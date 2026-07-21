import {
  GetMcpServerStatus,
  GetMcpServerToken,
  RegenerateMcpServerToken,
  SetMcpServerAutostart,
  SetMcpServerPort,
  SetMcpServerTLSEnabled,
  StartMcpServer,
  StopMcpServer,
} from '@wails/go/main/App';
import type { dto } from '@wails/go/models';

export type ApiServerStatus = dto.APIServerStatus;
export type SecretResult = dto.SecretResult;

// Shared shape of the MCP/REST server services, consumed by the
// ApiServerSettingsCards component.
export interface ApiServerService {
  getStatus(): Promise<ApiServerStatus>;
  start(): Promise<ApiServerStatus>;
  stop(): Promise<ApiServerStatus>;
  setAutostart(enabled: boolean): Promise<ApiServerStatus>;
  setPort(port: number): Promise<ApiServerStatus>;
  setTLSEnabled(enabled: boolean): Promise<ApiServerStatus>;
  getToken(): Promise<SecretResult>;
  regenerateToken(): Promise<SecretResult>;
}

export const mcpService: ApiServerService = {
  getStatus(): Promise<ApiServerStatus> {
    return GetMcpServerStatus();
  },
  start(): Promise<ApiServerStatus> {
    return StartMcpServer();
  },
  stop(): Promise<ApiServerStatus> {
    return StopMcpServer();
  },
  setAutostart(enabled: boolean): Promise<ApiServerStatus> {
    return SetMcpServerAutostart(enabled);
  },
  setPort(port: number): Promise<ApiServerStatus> {
    return SetMcpServerPort(port);
  },
  setTLSEnabled(enabled: boolean): Promise<ApiServerStatus> {
    return SetMcpServerTLSEnabled(enabled);
  },
  getToken(): Promise<SecretResult> {
    return GetMcpServerToken();
  },
  regenerateToken(): Promise<SecretResult> {
    return RegenerateMcpServerToken();
  },
};
