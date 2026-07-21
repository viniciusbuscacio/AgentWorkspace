// @vitest-environment happy-dom
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ServerTlsPage } from './ServerTlsPage';

const getStatus = vi.fn();
const createSelfSigned = vi.fn();
const regenerateSelfSigned = vi.fn();
const installCustom = vi.fn();

vi.mock('@services/servertls.service', () => ({
  serverTlsService: {
    getStatus: (...a: unknown[]) => getStatus(...a),
    createSelfSigned: (...a: unknown[]) => createSelfSigned(...a),
    regenerateSelfSigned: (...a: unknown[]) => regenerateSelfSigned(...a),
    installCustom: (...a: unknown[]) => installCustom(...a),
  },
}));

const NO_CERT = { success: true, mode: 'self_signed', hasCertificate: false, ready: false, selfSigned: false, expiringSoon: false };
const HAS_CERT = {
  success: true, mode: 'self_signed', hasCertificate: true, ready: true, selfSigned: true,
  subject: 'CN=Agent Workspace Local', issuer: 'CN=Agent Workspace Local',
  notBefore: '2026-01-01T00:00:00Z', notAfter: '2027-01-01T00:00:00Z',
  fingerprintSha256: 'AB:CD', dnsNames: ['localhost'], ipAddresses: ['127.0.0.1'], expiringSoon: false,
};

describe('ServerTlsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getStatus.mockResolvedValue(NO_CERT);
    createSelfSigned.mockResolvedValue(HAS_CERT);
    regenerateSelfSigned.mockResolvedValue(HAS_CERT);
    installCustom.mockResolvedValue(HAS_CERT);
  });

  it('shows a create button and no certificate notice when none exists', async () => {
    render(<ServerTlsPage />);
    expect(await screen.findByText(/No certificate yet/i)).toBeTruthy();
    expect(screen.getByText('Create self-signed certificate')).toBeTruthy();
  });

  it('creates a self-signed certificate', async () => {
    render(<ServerTlsPage />);
    const btn = await screen.findByText('Create self-signed certificate');
    await userEvent.click(btn);
    await waitFor(() => expect(createSelfSigned).toHaveBeenCalledTimes(1));
    expect((await screen.findAllByText(/CN=Agent Workspace Local/)).length).toBeGreaterThan(0);
  });

  it('install is disabled until both files are provided and only sends PEM contents', async () => {
    getStatus.mockResolvedValue(HAS_CERT);
    const { container } = render(<ServerTlsPage />);
    const install = (await screen.findByText('Validate and install')).closest('button')!;
    expect(install.disabled).toBe(true);

    const certFile = new File(['-----BEGIN CERTIFICATE-----\nx\n-----END CERTIFICATE-----'], 'chain.pem', { type: 'application/x-pem-file' });
    const keyFile = new File(['-----BEGIN PRIVATE KEY-----\ny\n-----END PRIVATE KEY-----'], 'key.pem', { type: 'application/x-pem-file' });
    await userEvent.upload(container.querySelector('[data-awid="server-tls-cert-file"]') as HTMLInputElement, certFile);
    await userEvent.upload(container.querySelector('[data-awid="server-tls-key-file"]') as HTMLInputElement, keyFile);
    await waitFor(() => expect(install.disabled).toBe(false));

    await userEvent.click(install);
    await waitFor(() => expect(installCustom).toHaveBeenCalledTimes(1));
    // Only the PEM strings are passed; no file objects or paths leak through.
    expect(installCustom.mock.calls[0][0]).toContain('BEGIN CERTIFICATE');
    expect(installCustom.mock.calls[0][1]).toContain('BEGIN PRIVATE KEY');
  });

  it('keeps the current certificate and shows a sanitized error on a failed install', async () => {
    getStatus.mockResolvedValue(HAS_CERT);
    installCustom.mockResolvedValue({ ...HAS_CERT, error: 'certificate and key do not match' });
    const { container } = render(<ServerTlsPage />);
    const certFile = new File(['-----BEGIN CERTIFICATE-----\nx\n-----END CERTIFICATE-----'], 'chain.pem');
    const keyFile = new File(['-----BEGIN PRIVATE KEY-----\ny\n-----END PRIVATE KEY-----'], 'key.pem');
    await userEvent.upload(container.querySelector('[data-awid="server-tls-cert-file"]') as HTMLInputElement, certFile);
    await userEvent.upload(container.querySelector('[data-awid="server-tls-key-file"]') as HTMLInputElement, keyFile);
    await userEvent.click((await screen.findByText('Validate and install')).closest('button')!);
    expect((await screen.findAllByText(/do not match/)).length).toBeGreaterThan(0);
    // Current (self-signed) certificate is still shown.
    expect(screen.getAllByText(/CN=Agent Workspace Local/).length).toBeGreaterThan(0);
  });
});

describe('ServerTlsPage data-testid helpers', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getStatus.mockResolvedValue(NO_CERT);
  });
  it('exposes file inputs by data-awid', async () => {
    const { container } = render(<ServerTlsPage />);
    await screen.findByText('Create self-signed certificate');
    expect(container.querySelector('[data-awid="server-tls-cert-file"]')).toBeTruthy();
    expect(container.querySelector('[data-awid="server-tls-key-file"]')).toBeTruthy();
  });
});
