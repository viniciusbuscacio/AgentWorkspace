import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi, beforeEach } from 'vitest';
import { ProvidersPage } from './ProvidersPage';

// Notices now go through the app-wide toast (lib/notify -> sonner); module
// tests assert the notify text instead of inline DOM messages.
const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));

const getProviderStatus = vi.fn();
const saveProviderConfig = vi.fn();
const startProviderBrowserAuth = vi.fn();
const switchProvider = vi.fn();
const setProviderEnabled = vi.fn();
const deleteProviderCredential = vi.fn();
const deleteProviderCredentialConfirmed = vi.fn();
const testProviderConnection = vi.fn();
const getProviderBalance = vi.fn();
const listProviderModels = vi.fn();
const createCustomProvider = vi.fn();
const renameCustomProvider = vi.fn();
const deleteCustomProvider = vi.fn();
const deleteCustomProviderConfirmed = vi.fn();
const discardCustomProvider = vi.fn();
const getProviderFallbackOrder = vi.fn();
const setProviderFallbackOrder = vi.fn();

vi.mock('@services/provider.service', () => ({
  providerService: {
    getProviderStatus: () => getProviderStatus(),
    saveProviderConfig: (...args: unknown[]) => saveProviderConfig(...args),
    startProviderBrowserAuth: (...args: unknown[]) => startProviderBrowserAuth(...args),
    switchProvider: (...args: unknown[]) => switchProvider(...args),
    setProviderEnabled: (...args: unknown[]) => setProviderEnabled(...args),
    deleteProviderCredential: (...args: unknown[]) => deleteProviderCredential(...args),
    deleteProviderCredentialConfirmed: (...args: unknown[]) => deleteProviderCredentialConfirmed(...args),
    testProviderConnection: (...args: unknown[]) => testProviderConnection(...args),
    getProviderBalance: (...args: unknown[]) => getProviderBalance(...args),
    listProviderModels: (...args: unknown[]) => listProviderModels(...args),
    createCustomProvider: (...args: unknown[]) => createCustomProvider(...args),
    renameCustomProvider: (...args: unknown[]) => renameCustomProvider(...args),
    deleteCustomProvider: (...args: unknown[]) => deleteCustomProvider(...args),
    deleteCustomProviderConfirmed: (...args: unknown[]) => deleteCustomProviderConfirmed(...args),
    discardCustomProvider: (...args: unknown[]) => discardCustomProvider(...args),
    getProviderFallbackOrder: () => getProviderFallbackOrder(),
    setProviderFallbackOrder: (...args: unknown[]) => setProviderFallbackOrder(...args),
  },
}));

// isWebMode is mocked so a single test can flip into web mode and assert the
// React-modal + *Confirmed path (the desktop default keeps the native dialog).
const webModeControl = vi.hoisted(() => ({ enabled: false }));
vi.mock('@/web/web-bindings', () => ({
  isWebMode: () => webModeControl.enabled,
}));

const toastMock = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('sonner', () => ({ toast: toastMock }));

// Capture aw-event handlers so a test can fire 'provider:device-code'.
const eventBus = vi.hoisted(() => ({ handlers: {} as Record<string, (payload: { userCode: string; verificationUri: string }) => void> }));
vi.mock('@services/events', () => ({
  onAwEvent: (name: string, handler: (payload: { userCode: string; verificationUri: string }) => void) => {
    eventBus.handlers[name] = handler;
    return () => { delete eventBus.handlers[name]; };
  },
}));

const providers = [
  {
    id: 'openai-codex',
    name: 'OpenAI Subscription',
    authType: 'oauth-browser',
    status: 'not-configured',
    model: '',
    connected: false,
    baseUrl: '',
    apiKeyPlaceholder: '',
    baseUrlPlaceholder: '',
    defaultModel: 'gpt-5.5',
    models: ['gpt-5.5'],
    authDescription: 'Paste OpenAI Codex OAuth/token JSON to keep it inside the encrypted vault.',
    allowCustomModel: false,
    requiresBaseUrl: false,
    enabled: true,
  },
  {
    id: 'openai',
    name: 'OpenAI API Key',
    authType: 'api-key',
    status: 'not-configured',
    model: '',
    connected: false,
    baseUrl: 'https://api.openai.com/v1',
    apiKeyPlaceholder: 'sk-...',
    baseUrlPlaceholder: '',
    defaultModel: 'gpt-5.5',
    models: ['gpt-5.5'],
    authDescription: '',
    allowCustomModel: false,
    requiresBaseUrl: false,
    enabled: true,
  },
  {
    id: 'openrouter',
    name: 'OpenRouter',
    authType: 'api-key',
    status: 'active',
    model: 'deepseek/deepseek-r1',
    connected: true,
    baseUrl: 'https://openrouter.ai/api/v1',
    apiKeyPlaceholder: 'sk-or-...',
    baseUrlPlaceholder: '',
    defaultModel: 'deepseek/deepseek-r1',
    models: ['deepseek/deepseek-r1'],
    authDescription: '',
    allowCustomModel: false,
    requiresBaseUrl: false,
    enabled: true,
  },
  {
    id: 'custom-openai',
    name: 'Custom OpenAI-compatible',
    authType: 'api-key',
    status: 'configured',
    model: 'model-id',
    connected: true,
    baseUrl: 'https://example.test/v1',
    apiKeyPlaceholder: 'API key',
    baseUrlPlaceholder: 'https://your-provider.example/v1',
    defaultModel: 'model-id',
    models: ['model-id'],
    authDescription: '',
    allowCustomModel: true,
    requiresBaseUrl: true,
    enabled: true,
  },
];

describe('ProvidersPage', () => {
  beforeEach(() => {
    localStorage.clear();
    getProviderFallbackOrder.mockReset();
    getProviderFallbackOrder.mockResolvedValue([]);
    setProviderFallbackOrder.mockReset();
    setProviderFallbackOrder.mockResolvedValue({ success: true });
    startProviderBrowserAuth.mockReset();
    startProviderBrowserAuth.mockResolvedValue({ success: true, saved: true, activated: true });
    switchProvider.mockReset();
    switchProvider.mockResolvedValue({ success: true, activated: true });
    setProviderEnabled.mockReset();
    setProviderEnabled.mockResolvedValue({ success: true });
    deleteProviderCredential.mockReset();
    deleteProviderCredential.mockResolvedValue({ success: true });
    deleteProviderCredentialConfirmed.mockReset();
    deleteProviderCredentialConfirmed.mockResolvedValue({ success: true });
    webModeControl.enabled = false;
    testProviderConnection.mockReset();
    testProviderConnection.mockResolvedValue({ success: true, latency: '123ms', model: 'deepseek/deepseek-r1' });
    getProviderBalance.mockReset();
    getProviderBalance.mockResolvedValue({ available: true, used: '$0.0500', limit: '$10.00' });
    listProviderModels.mockReset();
    listProviderModels.mockResolvedValue({ models: [], source: 'static' });
    createCustomProvider.mockReset();
    createCustomProvider.mockResolvedValue({ success: true, saved: true, providerId: 'custom-abc123' });
    renameCustomProvider.mockReset();
    renameCustomProvider.mockResolvedValue({ success: true, saved: true });
    deleteCustomProvider.mockReset();
    deleteCustomProvider.mockResolvedValue({ success: true });
    deleteCustomProviderConfirmed.mockReset();
    deleteCustomProviderConfirmed.mockResolvedValue({ success: true });
    discardCustomProvider.mockReset();
    discardCustomProvider.mockResolvedValue({ success: true });
    toastMock.success.mockReset();
    toastMock.error.mockReset();
    getProviderStatus.mockResolvedValue({ active: 'openrouter', providers });
  });

  it('numbers cards from the active-first chain when no backend order exists', async () => {
    // A stale page-local map (the pre-fix cosmetic order) must be overridden
    // by reality: with no persisted backend order the chain starts at the
    // active provider, so its card is #1.
    localStorage.setItem('settings-providers-configured-order', JSON.stringify({ 'custom-openai': 1, openrouter: 2 }));
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card').length).toBe(4));
    const cardFor = (name: string) => Array.from(document.querySelectorAll('.home-card'))
      .find((card) => card.querySelector('.home-card-title')?.textContent === name) as HTMLElement;
    await waitFor(() => expect(cardFor('OpenRouter').textContent).toContain('#1'));
    expect(cardFor('Custom OpenAI-compatible').textContent).toContain('#2');
  });

  it('uses the Apps-style provider grid with configured providers first', async () => {
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    expect(screen.getByRole('textbox', { name: 'Search providers' })).toBeTruthy();
    // ZoomSafeSelect trigger shows the selected label as text (no .value).
    expect(screen.getByRole('combobox', { name: 'Order providers' }).textContent).toContain('Configured first');
    expect(screen.getByRole('slider', { name: 'Provider card size' })).toBeTruthy();

    const cardTitles = [...document.querySelectorAll('.home-card .home-card-title')].map((item) => item.textContent);
    expect(cardTitles).toEqual(['OpenRouter', 'Custom OpenAI-compatible', 'OpenAI Subscription', 'OpenAI API Key']);

    const activeTexts = screen.getAllByText('Active');
    const activeBadge = activeTexts.find((el) => el.classList.contains('bg-primary') && el.classList.contains('text-primary-foreground'));
    expect(activeBadge).toBeTruthy();
    expect(activeBadge!.classList.contains('bg-primary')).toBe(true);
    expect(activeBadge!.classList.contains('text-primary-foreground')).toBe(true);
    expect(screen.queryByText('API key')).toBeNull();
    expect(screen.getAllByRole('button', { name: /Edit/i })).toHaveLength(4);
  });

  it('opens provider editing as a nested settings detail page', async () => {
    const onProviderDetailTitleChange = vi.fn();
    render(<ProvidersPage onProviderDetailTitleChange={onProviderDetailTitleChange} />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getAllByRole('button', { name: /Edit/i })[0]!);

    expect(onProviderDetailTitleChange).toHaveBeenCalledWith('OpenRouter');
    expect(screen.getByText('OpenRouter')).toBeTruthy();
    expect(screen.getByText('Order')).toBeTruthy();
    expect(screen.getByText('API key')).toBeTruthy();
    expect(screen.queryByRole('textbox', { name: 'Search providers' })).toBeNull();
  });

  it('live model list from the provider replaces the static catalog', async () => {
    listProviderModels.mockResolvedValue({ models: ['live-1', 'live-2'], source: 'live' });
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getAllByRole('button', { name: /Edit/i })[0]!); // OpenRouter
    await waitFor(() => expect(listProviderModels).toHaveBeenCalledWith('openrouter'));
    // The datalist under the Model input now carries the live ids.
    await waitFor(() => {
      const options = Array.from(document.querySelectorAll('#model-options-openrouter option'));
      expect(options.map((option) => option.getAttribute('value'))).toEqual(['live-1', 'live-2']);
    });
  });

  it('cached model list (endpoint down) also replaces the static catalog', async () => {
    listProviderModels.mockResolvedValue({ models: ['cached-1'], source: 'cache', error: 'endpoint down' });
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getAllByRole('button', { name: /Edit/i })[0]!); // OpenRouter
    await waitFor(() => {
      const options = Array.from(document.querySelectorAll('#model-options-openrouter option'));
      expect(options.map((option) => option.getAttribute('value'))).toEqual(['cached-1']);
    });
  });

  it('brand providers render their SVG mark; custom providers get an icon picker', async () => {
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    // OpenRouter / OpenAI cards carry the bundled brand SVG.
    expect(document.querySelector('[data-provider-brand-icon="openrouter"] svg')).toBeTruthy();
    // The custom provider card falls back to a material icon.
    expect(document.querySelector('[data-provider-brand-icon="custom-openai"]')).toBeNull();

    // Open the custom provider: the icon picker appears and a choice persists.
    await userEvent.click(screen.getAllByRole('button', { name: /Edit/i })[1]!); // Custom OpenAI-compatible (2nd configured)
    const option = await screen.findByRole('radio', { name: 'Icon bolt' });
    await userEvent.click(option);
    expect(option.getAttribute('aria-checked')).toBe('true');
    expect(localStorage.getItem('settings-providers-custom-icons')).toContain('"custom-openai":"bolt"');
  });

  it('Cancel discards changes and returns to the providers list', async () => {
    const onProviderDetailTitleChange = vi.fn();
    render(<ProvidersPage onProviderDetailTitleChange={onProviderDetailTitleChange} />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getAllByRole('button', { name: /Edit/i })[0]!);
    expect(screen.getByText('Order')).toBeTruthy();

    await userEvent.click(screen.getByRole('button', { name: /cancel/i }));

    // Back on the list: the search box returns and the breadcrumb title clears.
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Search providers' })).toBeTruthy());
    expect(onProviderDetailTitleChange).toHaveBeenLastCalledWith(null);
    expect(saveProviderConfig).not.toHaveBeenCalled();
  });

  it('starts browser auth and imports the selected provider token into aw', async () => {
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.type(screen.getByRole('textbox', { name: 'Search providers' }), 'subscription');
    await userEvent.click(screen.getByRole('button', { name: /Edit/i }));
    await userEvent.click(screen.getByRole('button', { name: 'Authenticate in browser' }));

    expect(startProviderBrowserAuth).toHaveBeenCalledWith('openai-codex', 'gpt-5.5');
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith('Authenticated and activated.'));
  });

  it('shows a copyable code box when a device code arrives', async () => {
    let resolveAuth: (value: { success: boolean }) => void = () => {};
    startProviderBrowserAuth.mockReturnValue(new Promise((resolve) => { resolveAuth = resolve; }));
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(globalThis.navigator, 'clipboard', { value: { writeText }, configurable: true });

    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.type(screen.getByRole('textbox', { name: 'Search providers' }), 'subscription');
    await userEvent.click(screen.getByRole('button', { name: /Edit/i }));
    await userEvent.click(screen.getByRole('button', { name: 'Authenticate in browser' }));

    // Backend surfaces the device code via the aw event bus.
    act(() => eventBus.handlers['provider:device-code']?.({ userCode: '38F8-FC3E', verificationUri: 'https://github.com/login/device' }));

    expect(await screen.findByText('38F8-FC3E')).toBeTruthy();
    await userEvent.click(screen.getByRole('button', { name: 'Copy code' }));
    expect(writeText).toHaveBeenCalledWith('38F8-FC3E');

    resolveAuth({ success: true });
  });

  it('enabled toggle in list disables a provider (pure on/off, no switchProvider)', async () => {
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    // Toggles follow card order: [openrouter(active), custom-openai, openai-codex, openai].
    const toggles = screen.getAllByRole('switch', { name: 'Enabled' });
    const customOpenAIToggle = toggles[1]!;
    expect(customOpenAIToggle.getAttribute('data-state')).toBe('checked');

    await userEvent.click(customOpenAIToggle);

    expect(setProviderEnabled).toHaveBeenCalledWith('custom-openai', false);
    expect(switchProvider).not.toHaveBeenCalled();
  });

  it('disabling the active provider is allowed (backend hands activity over)', async () => {
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    const toggles = screen.getAllByRole('switch', { name: 'Enabled' });
    const openrouterToggle = toggles[0]!;
    expect(openrouterToggle.getAttribute('data-state')).toBe('checked');

    await userEvent.click(openrouterToggle);

    expect(setProviderEnabled).toHaveBeenCalledWith('openrouter', false);
  });

  it('re-enables a disabled provider from its card toggle', async () => {
    const withDisabled = providers.map((provider) =>
      provider.id === 'custom-openai' ? { ...provider, enabled: false } : provider,
    );
    getProviderStatus.mockResolvedValue({ active: 'openrouter', providers: withDisabled });

    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    const toggles = screen.getAllByRole('switch', { name: 'Enabled' });
    const customOpenAIToggle = toggles[1]!;
    expect(customOpenAIToggle.getAttribute('data-state')).toBe('unchecked');

    await userEvent.click(customOpenAIToggle);

    expect(setProviderEnabled).toHaveBeenCalledWith('custom-openai', true);
  });

  it('enabled toggle in detail panel flips the provider off', async () => {
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    // Open custom-openai detail
    await userEvent.type(screen.getByRole('textbox', { name: 'Search providers' }), 'custom');
    await userEvent.click(screen.getByRole('button', { name: /Edit/i }));

    const detailToggle = screen.getByRole('switch', { name: 'Enabled' });
    expect(detailToggle.getAttribute('data-state')).toBe('checked');

    await userEvent.click(detailToggle);

    expect(setProviderEnabled).toHaveBeenCalledWith('custom-openai', false);
  });

  it('unconfigured provider toggle is disabled with Configure first title', async () => {
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    // openai and openai-codex are not-configured; their toggles should be disabled
    const toggles = screen.getAllByRole('switch', { name: 'Enabled' });
    // Toggles: [openrouter(active), custom-openai(configured), openai-codex(not-configured), openai(not-configured)]
    expect((toggles[2] as HTMLButtonElement).disabled).toBe(true);
    expect((toggles[3] as HTMLButtonElement).disabled).toBe(true);
  });

  it('Remove credential button invokes the binding (native dialog on backend)', async () => {
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getAllByRole('button', { name: /Edit/i })[0]!); // openrouter
    const removeBtn = screen.getByRole('button', { name: /Remove credential/i });
    await userEvent.click(removeBtn);

    expect(deleteProviderCredential).toHaveBeenCalledWith('openrouter');
  });

  it('web mode: Remove credential confirms with a React modal and calls the no-dialog *Confirmed variant', async () => {
    webModeControl.enabled = true;
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getAllByRole('button', { name: /Edit/i })[0]!); // openrouter
    await userEvent.click(screen.getByRole('button', { name: /Remove credential/i }));

    // The native-dialog binding (denied by the web bridge) must NOT be called;
    // instead a React confirmation modal appears.
    expect(deleteProviderCredential).not.toHaveBeenCalled();
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog.textContent).toMatch(/Remove credential\?/i);

    // Confirming routes through the no-dialog *Confirmed variant.
    await userEvent.click(screen.getByRole('button', { name: /^Remove$/i }));
    expect(deleteProviderCredentialConfirmed).toHaveBeenCalledWith('openrouter');
    expect(deleteProviderCredential).not.toHaveBeenCalled();
  });

  it('Test button calls testProviderConnection with the provider id and toasts the result', async () => {
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getAllByRole('button', { name: /Edit/i })[0]!); // openrouter (configured)
    const testBtn = screen.getByRole('button', { name: /^Test$/i });
    await userEvent.click(testBtn);

    expect(testProviderConnection).toHaveBeenCalledWith('openrouter');
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith('Connection OK', expect.stringContaining('123ms')));
  });

  it('Test button shows provider error verbatim on failure', async () => {
    testProviderConnection.mockResolvedValue({ success: false, error: '401: Invalid API key' });

    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getAllByRole('button', { name: /Edit/i })[0]!);
    await userEvent.click(screen.getByRole('button', { name: /^Test$/i }));

    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith('401: Invalid API key'));
  });

  it('balance row appears on OpenRouter detail and hides on error response', async () => {
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getAllByRole('button', { name: /Edit/i })[0]!); // openrouter

    // Balance should load automatically
    expect(await screen.findByText(/\$0\.0500 used/)).toBeTruthy();
    expect(screen.getByText(/Balance/)).toBeTruthy();
  });

  it('balance row is hidden when getProviderBalance returns available=false', async () => {
    getProviderBalance.mockResolvedValue({ available: false });

    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getAllByRole('button', { name: /Edit/i })[0]!); // openrouter
    await waitFor(() => expect(getProviderBalance).toHaveBeenCalled());

    // Balance label should still be visible (it's the FieldLabel), but no dollar amount
    expect(screen.queryByText(/\$0/)).toBeNull();
  });

  it('Test button does not appear for unconfigured providers', async () => {
    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    // Open openai (not configured)
    await userEvent.type(screen.getByRole('textbox', { name: 'Search providers' }), 'OpenAI API');
    await userEvent.click(screen.getByRole('button', { name: /Edit/i }));

    expect(screen.queryByRole('button', { name: /^Test$/i })).toBeNull();
  });

  it('Add custom provider creates a new slot and opens it for editing', async () => {
    const newProvider = {
      id: 'custom-abc123',
      name: 'Custom OpenAI-compatible',
      authType: 'api-key',
      status: 'not-configured',
      model: 'model-id',
      connected: false,
      baseUrl: '',
      apiKeyPlaceholder: 'API key',
      baseUrlPlaceholder: 'https://your-provider.example/v1',
      defaultModel: 'model-id',
      models: ['model-id'],
      authDescription: '',
      allowCustomModel: true,
      requiresBaseUrl: true,
      custom: true,
      deletable: true,
    };
    // First load: base providers. After create + refresh: include the new one.
    getProviderStatus
      .mockResolvedValueOnce({ active: 'openrouter', providers })
      .mockResolvedValue({ active: 'openrouter', providers: [...providers, newProvider] });

    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getByRole('button', { name: /Add custom provider/i }));

    expect(createCustomProvider).toHaveBeenCalledWith('Custom OpenAI-compatible');
    // The new provider's detail panel opens: it has the editable name field.
    expect(await screen.findByText('Provider name')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Delete provider/i })).toBeTruthy();
  });

  it('Add custom provider then Cancel discards the unsaved slot', async () => {
    const newProvider = {
      id: 'custom-abc123',
      name: 'Custom OpenAI-compatible',
      authType: 'api-key',
      status: 'not-configured',
      model: 'model-id',
      connected: false,
      baseUrl: '',
      apiKeyPlaceholder: 'API key',
      baseUrlPlaceholder: 'https://your-provider.example/v1',
      defaultModel: 'model-id',
      models: ['model-id'],
      authDescription: '',
      allowCustomModel: true,
      requiresBaseUrl: true,
      custom: true,
      deletable: true,
    };
    getProviderStatus
      .mockResolvedValueOnce({ active: 'openrouter', providers })
      .mockResolvedValue({ active: 'openrouter', providers: [...providers, newProvider] });

    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(4));

    await userEvent.click(screen.getByRole('button', { name: /Add custom provider/i }));
    expect(await screen.findByText('Provider name')).toBeTruthy();

    await userEvent.click(screen.getByRole('button', { name: /cancel/i }));

    // The freshly created slot must be discarded (no native dialog).
    expect(discardCustomProvider).toHaveBeenCalledWith('custom-abc123');
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Search providers' })).toBeTruthy());
  });

  it('Delete provider confirms with the themed modal (no native dialog) and returns to list', async () => {
    const customProvider = {
      id: 'custom-abc123',
      name: 'Maritaca',
      authType: 'api-key',
      status: 'configured',
      model: 'sabia-4',
      connected: true,
      baseUrl: 'https://chat.maritaca.ai/api',
      apiKeyPlaceholder: 'API key',
      baseUrlPlaceholder: 'https://your-provider.example/v1',
      defaultModel: 'model-id',
      models: ['model-id'],
      authDescription: '',
      allowCustomModel: true,
      requiresBaseUrl: true,
      custom: true,
      deletable: true,
    };
    getProviderStatus.mockResolvedValue({ active: 'openrouter', providers: [...providers, customProvider] });

    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(5));

    await userEvent.type(screen.getByRole('textbox', { name: 'Search providers' }), 'Maritaca');
    await userEvent.click(screen.getByRole('button', { name: /Edit/i }));

    // Custom provider detail shows the rename field.
    expect(screen.getByText('Provider name')).toBeTruthy();
    await userEvent.click(screen.getByRole('button', { name: /Delete provider/i }));

    // Desktop now uses the themed in-app modal (not the native dialog), so the
    // native-variant binding must NOT be called; confirming routes through the
    // no-dialog *Confirmed variant.
    expect(deleteCustomProvider).not.toHaveBeenCalled();
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog.textContent).toMatch(/Delete provider\?/i);
    await userEvent.click(screen.getByRole('button', { name: /^Delete$/i }));
    expect(deleteCustomProviderConfirmed).toHaveBeenCalledWith('custom-abc123');
    expect(deleteCustomProvider).not.toHaveBeenCalled();

    // On success it returns to the list (detail panel unmounts) and confirms
    // with a toast carrying the provider name.
    await waitFor(() => expect(toastMock.success).toHaveBeenCalledWith('Maritaca deleted'));
    expect(screen.queryByText('Provider name')).toBeNull();
  });

  it('shows the effective /chat/completions endpoint preview under Base URL', async () => {
    const customProvider = {
      id: 'custom-abc123',
      name: 'Maritaca',
      authType: 'api-key',
      status: 'configured',
      model: 'sabia-4',
      connected: true,
      baseUrl: 'https://chat.maritaca.ai/api',
      apiKeyPlaceholder: 'API key',
      baseUrlPlaceholder: 'https://your-provider.example/v1',
      defaultModel: 'model-id',
      models: ['model-id'],
      authDescription: '',
      allowCustomModel: true,
      requiresBaseUrl: true,
      custom: true,
      deletable: true,
    };
    getProviderStatus.mockResolvedValue({ active: 'openrouter', providers: [...providers, customProvider] });

    render(<ProvidersPage />);
    await waitFor(() => expect(document.querySelectorAll('.home-card .home-card-title').length).toBe(5));

    await userEvent.type(screen.getByRole('textbox', { name: 'Search providers' }), 'Maritaca');
    await userEvent.click(screen.getByRole('button', { name: /Edit/i }));

    // Preview reflects the stored base URL + the fixed path.
    expect(screen.getByText('https://chat.maritaca.ai/api/chat/completions')).toBeTruthy();

    // Pasting the full endpoint still previews the correct (non-doubled) URL.
    const baseInput = screen.getByPlaceholderText('https://your-provider.example/v1');
    await userEvent.clear(baseInput);
    await userEvent.type(baseInput, 'https://chat.maritaca.ai/api/chat/completions');
    expect(screen.getByText('https://chat.maritaca.ai/api/chat/completions')).toBeTruthy();
  });
});
