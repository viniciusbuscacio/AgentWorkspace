import {
  GetServerTLSStatus,
  CreateSelfSignedCertificate,
  RegenerateSelfSignedCertificate,
  InstallCustomCertificate,
} from '@wails/go/main/App';
import type { dto } from '@wails/go/models';

// serverTlsService wraps the App bindings for the shared TLS manager. One
// certificate (self-signed and app-managed, or a custom PEM the user uploads)
// serves the Web/MCP/REST servers; each server enables HTTPS independently. The
// private key never crosses this boundary — status is sanitized metadata only.
export type ServerTLSStatus = dto.ServerTLSStatus;

export const serverTlsService = {
  getStatus(): Promise<ServerTLSStatus> {
    return GetServerTLSStatus();
  },
  createSelfSigned(): Promise<ServerTLSStatus> {
    return CreateSelfSignedCertificate();
  },
  regenerateSelfSigned(): Promise<ServerTLSStatus> {
    return RegenerateSelfSignedCertificate();
  },
  installCustom(certPEM: string, keyPEM: string): Promise<ServerTLSStatus> {
    return InstallCustomCertificate(certPEM, keyPEM);
  },
};
