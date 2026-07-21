import {
  GetWebServerStatus,
  ListWebServerInterfaces,
  SetWebServerEnabled,
  SetWebServerPort,
  SetWebServerBindMode,
  SetWebServerSessionTTL,
  SetWebServerTLSEnabled,
  StartWebServer,
  StopWebServer,
  RegenerateWebServerSessionKey,
} from '@wails/go/main/App';
import type { dto } from '@wails/go/models';

// webaccessService wraps the App bindings for the Web Access server (remote
// browser access to the full UI over the Tailscale interface). Unlike MCP/REST
// it authenticates with the vault password (no per-request bearer token), so the
// settings UI exposes bind mode, session TTL and a signing-key reset instead.
export type WebServerStatus = dto.WebServerStatus;
export type WebBindCandidate = dto.WebBindCandidate;

export const webaccessService = {
  getStatus(): Promise<WebServerStatus> {
    return GetWebServerStatus();
  },
  async listInterfaces(): Promise<WebBindCandidate[]> {
    const result = await ListWebServerInterfaces();
    return result.success ? (result.candidates ?? []) : [];
  },
  start(): Promise<WebServerStatus> {
    return StartWebServer();
  },
  stop(): Promise<WebServerStatus> {
    return StopWebServer();
  },
  setEnabled(enabled: boolean): Promise<WebServerStatus> {
    return SetWebServerEnabled(enabled);
  },
  setPort(port: number): Promise<WebServerStatus> {
    return SetWebServerPort(port);
  },
  setBindMode(mode: string, bindAddr: string, allowedCIDRs: string[]): Promise<WebServerStatus> {
    return SetWebServerBindMode(mode, bindAddr, allowedCIDRs);
  },
  setSessionTTL(minutes: number): Promise<WebServerStatus> {
    return SetWebServerSessionTTL(minutes);
  },
  setTLSEnabled(enabled: boolean): Promise<WebServerStatus> {
    return SetWebServerTLSEnabled(enabled);
  },
  regenerateSessionKey(): Promise<WebServerStatus> {
    return RegenerateWebServerSessionKey();
  },
};
