// @vitest-environment happy-dom
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiServerSettingsCards } from './ApiServerSettingsCards';
import type { ApiServerService, ApiServerStatus } from '@services/mcp.service';

const openSettingsPage = vi.fn();
vi.mock('@/lib/open-settings', () => ({
  openSettingsPage: (...a: unknown[]) => openSettingsPage(...a),
}));

function makeService(over: Partial<ApiServerService> = {}): ApiServerService {
  const base: ApiServerStatus = {
    success: true, autostart: false, running: false, port: 9300, tlsEnabled: false,
  } as ApiServerStatus;
  return {
    getStatus: vi.fn().mockResolvedValue(base),
    start: vi.fn().mockResolvedValue(base),
    stop: vi.fn().mockResolvedValue(base),
    setAutostart: vi.fn().mockResolvedValue(base),
    setPort: vi.fn().mockResolvedValue(base),
    setTLSEnabled: vi.fn().mockResolvedValue(base),
    getToken: vi.fn().mockResolvedValue({ success: true, value: 'tok', exists: true }),
    regenerateToken: vi.fn().mockResolvedValue({ success: true, value: 'tok', exists: true }),
    ...over,
  };
}

function renderCards(service: ApiServerService) {
  return render(
    <ApiServerSettingsCards
      service={service}
      serverTitle="MCP server"
      awid="mcp-server"
      endpointForPort={(p) => `http://127.0.0.1:${p}/mcp`}
      description="desc"
    />,
  );
}

describe('ApiServerSettingsCards — HTTPS toggle', () => {
  beforeEach(() => vi.clearAllMocks());

  it('defaults the HTTPS toggle to off', async () => {
    const { container } = renderCards(makeService());
    await waitFor(() => expect(container.querySelector('[data-awid="mcp-server-tls"]')).toBeTruthy());
    const toggle = container.querySelector('[data-awid="mcp-server-tls"]') as HTMLButtonElement;
    expect(toggle.getAttribute('aria-checked')).toBe('false');
  });

  it('shows an actionable link to the TLS manager when enabling without a certificate', async () => {
    const service = makeService({
      setTLSEnabled: vi.fn().mockResolvedValue({
        success: false, autostart: false, running: false, port: 9300, tlsEnabled: false,
        error: 'No TLS certificate yet — create one in the TLS manager',
      } as ApiServerStatus),
    });
    const { container } = renderCards(service);
    const toggle = await screen.findByRole('switch', { name: /Enable HTTPS for MCP server/i });
    await userEvent.click(toggle);
    await waitFor(() => expect(service.setTLSEnabled).toHaveBeenCalledWith(true));
    await waitFor(() => expect(container.textContent).toContain('No TLS certificate yet'));

    const link = await screen.findByText('Open the TLS manager');
    await userEvent.click(link);
    expect(openSettingsPage).toHaveBeenCalledWith('tls');
  });

  it('shows a coverage warning when TLS is on but the cert misses the bind', async () => {
    const service = makeService({
      getStatus: vi.fn().mockResolvedValue({
        success: true, autostart: false, running: true, port: 9300, tlsEnabled: true,
        coverageWarning: 'The TLS certificate does not cover 127.0.0.1',
      } as ApiServerStatus),
    });
    const { container } = renderCards(service);
    await waitFor(() => expect(container.querySelector('[data-awid="mcp-server-tls-coverage"]')).toBeTruthy());
    expect(screen.getByText(/does not cover 127.0.0.1/)).toBeTruthy();
  });
});
