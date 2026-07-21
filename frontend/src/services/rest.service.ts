import {
  GetRestServerStatus,
  GetRestServerToken,
  RegenerateRestServerToken,
  SetRestServerAutostart,
  SetRestServerPort,
  SetRestServerTLSEnabled,
  StartRestServer,
  StopRestServer,
} from '@wails/go/main/App';
import type { ApiServerService, ApiServerStatus, SecretResult } from '@services/mcp.service';

export const restService: ApiServerService = {
  getStatus(): Promise<ApiServerStatus> {
    return GetRestServerStatus();
  },
  start(): Promise<ApiServerStatus> {
    return StartRestServer();
  },
  stop(): Promise<ApiServerStatus> {
    return StopRestServer();
  },
  setAutostart(enabled: boolean): Promise<ApiServerStatus> {
    return SetRestServerAutostart(enabled);
  },
  setPort(port: number): Promise<ApiServerStatus> {
    return SetRestServerPort(port);
  },
  setTLSEnabled(enabled: boolean): Promise<ApiServerStatus> {
    return SetRestServerTLSEnabled(enabled);
  },
  getToken(): Promise<SecretResult> {
    return GetRestServerToken();
  },
  regenerateToken(): Promise<SecretResult> {
    return RegenerateRestServerToken();
  },
};
