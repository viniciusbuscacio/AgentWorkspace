import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import type { domain } from '@services/models';
import { Button } from '@ui/button';
import { Field, FieldLabel } from '@ui/field';
import { Input } from '@ui/input';
import { PasswordInput } from '@ui/password-input';
import { ZoomSafeSelect } from '@ui/zoom-safe-select';
import { Switch } from '@ui/switch';
import { providerService } from '@services/provider.service';
import { notify as setMessage } from '@/lib/notify';
import { onAwEvent } from '@services/events';
import { isWebMode } from '@/web/web-bindings';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@ui/alert-dialog';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import {
  ICON_SIZE_MAX,
  ICON_SIZE_MIN,
  ICON_SIZE_STEP,
  cardMetrics,
  cardStateClass,
  cardStyleVars,
} from '@modules/home/home-modules';
import {
  SettingsActions,
  SettingsNotice,
  SettingsPanel,
} from '../components/SettingsTemplate';
import {
  CUSTOM_PROVIDER_ICON_OPTIONS,
  ProviderIcon,
  providerIsIconCustomizable,
} from './provider-icon';

type ProviderOrderMode = 'configured' | 'default' | 'name';

const PROVIDER_ICON_SIZE_KEY = 'settings-providers-card-size';
const PROVIDER_ORDER_KEY = 'settings-providers-order-mode';
const PROVIDER_CONFIGURED_ORDER_KEY = 'settings-providers-configured-order';
const PROVIDER_CUSTOM_ICONS_KEY = 'settings-providers-custom-icons';
const PROVIDER_ICON_SIZE_DEFAULT = 220;
const BROWSER_AUTH_PROVIDER_IDS = new Set(['openai-codex', 'github-copilot']);
const PROVIDER_ORDER_LABELS: Record<ProviderOrderMode, string> = {
  configured: 'Configured first',
  default: 'Default',
  name: 'Name',
};


interface ProvidersPageProps {
  listRequest?: number;
  onProviderDetailTitleChange?: (title: string | null) => void;
}

export function ProvidersPage({ listRequest = 0, onProviderDetailTitleChange }: ProvidersPageProps) {
  const [status, setStatus] = useState<domain.ProviderStatus | null>(null);
  const [editingProviderId, setEditingProviderId] = useState('');
  const [model, setModel] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [credential, setCredential] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [providerName, setProviderName] = useState('');
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  // Web-mode destructive-delete confirmation target (React modal replaces the
  // native dialog that the web bridge denies).
  const [pendingDelete, setPendingDelete] = useState<
    { provider: domain.ProviderInfo; kind: 'credential' | 'slot' } | null
  >(null);
  // Live model list fetched from the provider's own models endpoint when the
  // detail panel opens. Null until it arrives; the static catalog from the
  // provider definition is used meanwhile (and stays if the fetch fails).
  const [liveModels, setLiveModels] = useState<string[] | null>(null);
  const [balance, setBalance] = useState<domain.ProviderBalanceResult | null>(null);
  const [balanceLoading, setBalanceLoading] = useState(false);
  const [authBusyProviderId, setAuthBusyProviderId] = useState('');
  const [authSuccessProviderId, setAuthSuccessProviderId] = useState('');
  const [deviceCode, setDeviceCode] = useState<{ userCode: string; verificationUri: string } | null>(null);
  const [search, setSearch] = useState('');
  const [orderMode, setOrderMode] = useState<ProviderOrderMode>(() => getInitialProviderOrderMode());
  const [providerOrder, setProviderOrder] = useState<Record<string, number>>(() => getInitialConfiguredProviderOrder());
  // User-picked material icons for custom providers (cosmetic, applied
  // immediately like the card-size slider; persisted like the order map).
  const [customIcons, setCustomIcons] = useState<Record<string, string>>(() => getInitialCustomProviderIcons());
  const [iconSize, setIconSize] = useState<number>(() => getInitialProviderIconSize());
  const editingProviderIdRef = useRef('');
  // Id of a just-created custom provider that has not been saved yet. If the
  // user cancels (or leaves) without saving, it is discarded so "Add custom
  // provider" + Cancel does not leave an empty slot behind.
  const pendingNewProviderIdRef = useRef('');
  // Order baseline so the Order control joins Save/Cancel: changing it enables
  // Save and Cancel reverts it. savedProviderOrderRef keeps the full order map
  // for an exact revert; savedSelectedOrder drives the dirty check.
  const [savedSelectedOrder, setSavedSelectedOrder] = useState(1);
  const savedProviderOrderRef = useRef<Record<string, number>>({});
  const providers = useMemo(() => status?.providers ?? [], [status?.providers]);
  const selected = useMemo(() => providers.find((provider) => provider.id === editingProviderId), [editingProviderId, providers]);
  // Dirty tracking so Save/Cancel mirror the Wallpaper pattern: visible but
  // disabled until the form actually diverges from the saved provider config.
  const dirty = useMemo(() => {
    if (!selected) return false;
    const orderChanged = providerIsConfigured(selected)
      && providerConfiguredOrder(selected.id, providers, providerOrder) !== savedSelectedOrder;
    return (
      orderChanged
      || (selected.custom && providerName.trim() !== selected.name && providerName.trim() !== '')
      || model !== (selected.model || selected.defaultModel)
      || baseUrl !== (selected.baseUrl || '')
      || apiKey !== ''
      || credential !== ''
    );
  }, [selected, providers, providerOrder, savedSelectedOrder, model, baseUrl, apiKey, credential, providerName]);
  const metrics = useMemo(() => cardMetrics(iconSize), [iconSize]);
  const visibleProviders = useMemo(() => {
    const query = search.trim().toLowerCase();
    const ordered = orderProviders(providers, orderMode, providerOrder);
    if (!query) return ordered;
    return ordered.filter((provider) => (
      provider.name.toLowerCase().includes(query)
      || provider.id.toLowerCase().includes(query)
      || (provider.model || provider.defaultModel).toLowerCase().includes(query)
      || provider.status.toLowerCase().includes(query)
    ));
  }, [orderMode, providerOrder, providers, search]);

  const refresh = useCallback(async () => {
    const next = await providerService.getProviderStatus();
    setStatus(next);
    // The backend fallback order (config.json providerFallbackOrder) is the
    // real priority the chat chain uses; localStorage only caches it for
    // instant first paint. When no order was ever persisted, the effective
    // chain is the active provider first — number the cards from that
    // reality, never from a stale cached map.
    try {
      const order = await providerService.getProviderFallbackOrder();
      const map = order.length > 0
        ? Object.fromEntries(order.map((id, index) => [id, index + 1]))
        : deriveOrderFromStatus(next?.providers ?? []);
      setProviderOrder(map);
      persistConfiguredProviderOrder(map);
    } catch { /* keep the cached map */ }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (!selected) return;
    if (editingProviderIdRef.current === selected.id) {
      onProviderDetailTitleChange?.(selected.name);
      return;
    }
    editingProviderIdRef.current = selected.id;
    setModel(selected.model || selected.defaultModel);
    setBaseUrl(selected.baseUrl || '');
    setProviderName(selected.name);
    setApiKey('');
    setCredential('');
    setBalance(null);
    onProviderDetailTitleChange?.(selected.name);
  }, [onProviderDetailTitleChange, selected]);

  // Refresh the model list from the provider's own models endpoint whenever a
  // provider detail opens. Live and cached (last successful fetch) results
  // replace the static catalog; a static result is what we already show.
  useEffect(() => {
    setLiveModels(null);
    if (!selected?.id) return;
    const providerId = selected.id;
    void providerService.listProviderModels(providerId).then((result) => {
      if (editingProviderIdRef.current !== providerId) return; // stale response
      if (result.source !== 'static' && result.models?.length) setLiveModels(result.models);
    });
  }, [selected?.id]);

  useEffect(() => {
    editingProviderIdRef.current = '';
    setEditingProviderId('');
    setBalance(null);
    onProviderDetailTitleChange?.(null);
  }, [listRequest, onProviderDetailTitleChange]);

  useEffect(() => () => onProviderDetailTitleChange?.(null), [onProviderDetailTitleChange]);

  const [refreshingModels, setRefreshingModels] = useState(false);

  // Explicit "update this list" button: re-hits the provider's live models
  // endpoint (e.g. https://openrouter.ai/api/v1/models) and swaps the picker
  // list. The backend caches the successful fetch in the vault.
  async function refreshModels() {
    if (!selected) return;
    setRefreshingModels(true);
    try {
      const result = await providerService.listProviderModels(selected.id);
      if (result.source === 'live' && result.models?.length) {
        setLiveModels(result.models);
        setMessage(`Model list updated: ${result.models.length} models available.`);
      } else {
        setMessage(result.error || 'Could not fetch the live model list. Configure the provider first.');
      }
    } finally {
      setRefreshingModels(false);
    }
  }

  async function save() {
    if (!selected) return;
    setSaving(true);
    // Rename the custom provider first if its display name changed.
    if (selected.custom && providerName.trim() && providerName.trim() !== selected.name) {
      const renameResult = await providerService.renameCustomProvider(selected.id, providerName.trim());
      if (renameResult.error) {
        setSaving(false);
        setMessage(renameResult.error);
        return;
      }
    }
    const result = await providerService.saveProviderConfig({
      provider: selected.id,
      model,
      apiKey,
      credential,
      baseUrl,
      setActive: false,
    });
    setSaving(false);
    setMessage(result.error || result.warning || 'Configuration saved.');
    if (!result.error && !result.warning) {
      // The provider now has real config, so it is no longer a throwaway draft.
      pendingNewProviderIdRef.current = '';
      // Secrets are write-only: once saved they live in the vault, so clear the
      // inputs. This also returns the form to a clean (non-dirty) state.
      setApiKey('');
      setCredential('');
      // Commit the current order as the new baseline so Save disables again.
      savedProviderOrderRef.current = providerOrder;
      if (selected) setSavedSelectedOrder(providerConfiguredOrder(selected.id, providers, providerOrder));
    }
    await refresh();
  }

  function cancelEdit() {
    // Discard every unsaved change and go back to the providers list. The model
    // / key / base-URL inputs live only in local state (re-initialised on the
    // next open), but the Order persists live, so revert that map explicitly.
    // A freshly created, never-saved custom provider is removed entirely.
    const pendingId = pendingNewProviderIdRef.current;
    pendingNewProviderIdRef.current = '';
    setProviderOrder(savedProviderOrderRef.current);
    persistConfiguredProviderOrder(savedProviderOrderRef.current);
    pushFallbackOrder(savedProviderOrderRef.current);
    editingProviderIdRef.current = '';
    setEditingProviderId('');
    setBalance(null);
    onProviderDetailTitleChange?.(null);
    if (pendingId) {
      void providerService.discardCustomProvider(pendingId).then(() => refresh());
    }
  }

  // Pure per-provider on/off. Disabling the provider that is currently active
  // hands activity to the next enabled one by priority order (backend rule);
  // enabling only makes it eligible again.
  async function setEnabled(provider: domain.ProviderInfo, enabled: boolean) {
    const result = await providerService.setProviderEnabled(provider.id, enabled);
    setMessage(result.error || result.warning || (enabled ? 'Provider enabled.' : 'Provider disabled.'));
    await refresh();
  }

  async function deleteCredential(provider: domain.ProviderInfo) {
    // Desktop confirms with a native dialog (UI automation can't self-approve
    // it); web mode has no native dialog and the bridge denies the dialog
    // variant, so it confirms with a React modal and calls the *Confirmed
    // no-dialog variant instead.
    if (isWebMode()) {
      setPendingDelete({ provider, kind: 'credential' });
      return;
    }
    await afterDeleteCredential(await providerService.deleteProviderCredential(provider.id));
  }

  async function afterDeleteCredential(result: domain.ProviderOperationResult) {
    if (result.canceled) return; // user cancelled the native dialog
    setMessage(result.error || 'Credential removed.');
    await refresh();
  }

  async function addCustomProvider() {
    const result = await providerService.createCustomProvider('Custom OpenAI-compatible');
    if (result.error) {
      setMessage(result.error);
      return;
    }
    await refresh();
    if (result.providerId) {
      savedProviderOrderRef.current = providerOrder;
      pendingNewProviderIdRef.current = result.providerId;
      setEditingProviderId(result.providerId);
    }
  }

  function deleteCustomProviderSlot(provider: domain.ProviderInfo) {
    // Deleting a custom provider slot is low-stakes (and something the agent may
    // legitimately do), so it confirms with the themed in-app modal on every
    // platform instead of a native OS dialog. confirmDelete then calls the
    // no-dialog *Confirmed backend variant.
    setPendingDelete({ provider, kind: 'slot' });
  }

  async function afterDeleteSlot(result: domain.ProviderOperationResult, provider: domain.ProviderInfo) {
    if (result.canceled) return; // user cancelled the native dialog
    pendingNewProviderIdRef.current = '';
    if (result.error) {
      // Stay on the panel so the error is visible next to the failed action.
      setMessage(result.error);
      await refresh();
      return;
    }
    // Success: the provider is gone, so leave the detail panel for the list and
    // confirm with a toast (the in-panel notice would unmount on navigation).
    editingProviderIdRef.current = '';
    setEditingProviderId('');
    onProviderDetailTitleChange?.(null);
    await refresh();
    toast.success(`${provider.name} deleted`);
  }

  // confirmDelete runs after the themed React modal is accepted, calling the
  // no-dialog *Confirmed backend variant for the pending target. Used for the
  // provider-slot delete on all platforms, and for credential removal in web
  // mode (desktop credential removal keeps the native dialog).
  async function confirmDelete() {
    const pending = pendingDelete;
    setPendingDelete(null);
    if (!pending) return;
    if (pending.kind === 'credential') {
      await afterDeleteCredential(await providerService.deleteProviderCredentialConfirmed(pending.provider.id));
    } else {
      await afterDeleteSlot(await providerService.deleteCustomProviderConfirmed(pending.provider.id), pending.provider);
    }
  }

  async function testProvider(provider: domain.ProviderInfo) {
    setTesting(true);
    const result = await providerService.testProviderConnection(provider.id);
    if (result.success) {
      setMessage('Connection OK', `${result.latency}${result.model ? ` \u00b7 ${result.model}` : ''}`);
    } else {
      setMessage(result.error || 'Connection test failed.');
    }
    setTesting(false);
  }

  async function loadBalance(provider: domain.ProviderInfo) {
    setBalanceLoading(true);
    const result = await providerService.getProviderBalance(provider.id);
    setBalance(result);
    setBalanceLoading(false);
  }

  async function startBrowserAuth(provider: domain.ProviderInfo) {
    setAuthBusyProviderId(provider.id);
    setAuthSuccessProviderId('');
    setDeviceCode(null);
    setMessage('Waiting for browser authentication. Complete sign-in in the browser; aw will update this panel when the token arrives.');
    const poll = window.setInterval(() => { void refresh(); }, 1500);
    let offDeviceCode = () => {};
    try {
      offDeviceCode = onAwEvent('provider:device-code', (payload) => {
        setDeviceCode({ userCode: payload.userCode, verificationUri: payload.verificationUri });
      });
    } catch { /* event bus unavailable (e.g. tests); device code still shows via clipboard/notification */ }
    try {
      const result = await providerService.startProviderBrowserAuth(provider.id, model || provider.defaultModel);
      if (result.success) setAuthSuccessProviderId(provider.id);
      setMessage(result.error || result.warning || (result.activated ? 'Authenticated and activated.' : 'Authenticated.'));
    } finally {
      offDeviceCode();
      window.clearInterval(poll);
      setAuthBusyProviderId('');
      setDeviceCode(null);
      await refresh();
    }
  }

  function changeOrder(mode: ProviderOrderMode) {
    setOrderMode(mode);
    persistProviderOrderMode(mode);
  }

  function changeIconSize(size: number) {
    setIconSize(size);
    persistProviderIconSize(size);
  }

  function editProvider(provider: domain.ProviderInfo) {
    setEditingProviderId(provider.id);
    // Snapshot the order so Save/Cancel can track and revert changes to it.
    savedProviderOrderRef.current = providerOrder;
    setSavedSelectedOrder(providerConfiguredOrder(provider.id, providers, providerOrder));
    onProviderDetailTitleChange?.(provider.name);
  }

  // Persist the visible priority as the backend order (config.json). The
  // numbers ARE the chain: #1 becomes the active provider (the Agent
  // Workspace default), #2 the first fallback, and so on — so refresh after
  // the push to move the Active badge to the new #1.
  function pushFallbackOrder(order: Record<string, number>) {
    const ids = configuredProvidersInOrder(providers, order).map((provider) => provider.id);
    void providerService.setProviderFallbackOrder(ids)
      .then(() => refresh())
      .catch(() => { /* next refresh reconciles */ });
  }

  function changeProviderConfiguredOrder(providerID: string, nextOrder: number) {
    const next = reorderConfiguredProvider(providers, providerOrder, providerID, nextOrder);
    setProviderOrder(next);
    persistConfiguredProviderOrder(next);
    pushFallbackOrder(next);
  }

  function changeCustomProviderIcon(providerID: string, icon: string) {
    const next = { ...customIcons, [providerID]: icon };
    setCustomIcons(next);
    persistCustomProviderIcons(next);
  }

  const configuredProviders = providers.filter(providerIsConfigured);
  const selectedConfigured = selected ? providerIsConfigured(selected) : false;
  const selectedConfiguredOrder = selected ? providerConfiguredOrder(selected.id, providers, providerOrder) : 1;
  const selectedSupportsBrowserAuth = selected ? BROWSER_AUTH_PROVIDER_IDS.has(selected.id) : false;
  const selectedAuthBusy = selected ? authBusyProviderId === selected.id : false;
  const selectedAuthComplete = selectedSupportsBrowserAuth && Boolean(selected?.connected || (selected && authSuccessProviderId === selected.id));
  const selectedSupportsBalance = selected?.id === 'openrouter' && selectedConfigured;
  // Model options come from the live provider list when available, otherwise
  // the static catalog. A saved model missing from the live list stays
  // selectable (flagged) so the user can see what is configured and move off it.
  const selectedModelList = selected ? (liveModels ?? selected.models) : [];
  const modelSelectOptions = selectedModelList.map((item) => ({ value: item, label: item }));
  if (selected && model && !selectedModelList.includes(model)) {
    modelSelectOptions.unshift({ value: model, label: liveModels ? `${model} (not available)` : model });
  }

  // Auto-load balance when opening the OpenRouter detail panel.
  useEffect(() => {
    if (!selected || selected.id !== 'openrouter' || !providerIsConfigured(selected)) return;
    void loadBalance(selected);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- re-run per provider id only
  }, [selected?.id]);

  if (selected) {
    return (
      <>
      <SettingsPanel
        icon={<ProviderIcon providerId={selected.id} customIcon={customIcons[selected.id]} className="mt-[-1px] text-[22px] text-primary" />}
        title={selected.name}
        description={`Configure ${selected.name} model and credentials.`}
        contentClassName="gap-4"
      >
        {selectedSupportsBrowserAuth && (
          selectedAuthBusy && deviceCode ? (
            // Any device-code flow (GitHub Copilot today) gets a big, copyable
            // code box. Browser-redirect flows (OpenAI Subscription) have no code
            // to type, so they fall through to the notice below.
            <DeviceCodeBox code={deviceCode.userCode} verificationUri={deviceCode.verificationUri} />
          ) : (
            <SettingsNotice tone={selectedAuthComplete ? 'warning' : 'default'}>
              {selectedAuthBusy
                ? selected.id === 'github-copilot'
                  ? 'Starting GitHub device sign-in. A code will appear here and your browser will open at github.com/login/device.'
                  : 'Waiting for browser sign-in. When the provider redirects back to aw, this panel will update automatically.'
                : selectedAuthComplete
                  ? `Authenticated. Token received and stored in the encrypted vault. Status: ${providerStatusLabel(selected.status)}.`
                  : 'Not authenticated yet. Click Authenticate in browser to sign in and send the token back to aw.'}
            </SettingsNotice>
          )
        )}
        {selected.custom && (
          <Field>
            <FieldLabel>Provider name</FieldLabel>
            <Input
              value={providerName}
              onChange={(event) => setProviderName(event.target.value)}
              placeholder="Custom OpenAI-compatible"
            />
          </Field>
        )}
        {providerIsIconCustomizable(selected.id) && (
          <Field>
            <FieldLabel>Icon</FieldLabel>
            <div className="flex flex-wrap gap-1.5" role="radiogroup" aria-label="Provider icon">
              {CUSTOM_PROVIDER_ICON_OPTIONS.map((option) => {
                const active = (customIcons[selected.id] || CUSTOM_PROVIDER_ICON_OPTIONS[0]) === option;
                return (
                  <button
                    key={option}
                    type="button"
                    role="radio"
                    aria-checked={active}
                    aria-label={`Icon ${option}`}
                    title={option}
                    onClick={() => changeCustomProviderIcon(selected.id, option)}
                    className={`flex size-9 items-center justify-center rounded-md border transition-colors ${
                      active ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-card text-muted-foreground hover:bg-accent'
                    }`}
                  >
                    <span className="material-symbols-outlined text-[20px]" aria-hidden="true">{option}</span>
                  </button>
                );
              })}
            </div>
          </Field>
        )}
        {selectedConfigured && (
          <Field>
            <FieldLabel>Order</FieldLabel>
            <ZoomSafeSelect
              aria-label="Order"
              value={String(selectedConfiguredOrder)}
              onValueChange={(value) => changeProviderConfiguredOrder(selected.id, Number(value))}
              options={Array.from({ length: configuredProviders.length }, (_, index) => index + 1).map((order) => ({
                value: String(order),
                label: String(order),
              }))}
            />
            <p className="text-xs text-muted-foreground">
              Providers run in this order: #1 is the Agent Workspace default
              (it becomes the Active provider), #2 is the first fallback, and
              so on. Activating a provider moves it to #1.
            </p>
          </Field>
        )}
        <Field>
          <div className="flex items-center justify-between">
            <FieldLabel>Model</FieldLabel>
            <Button
              type="button"
              variant="outline"
              size="sm"
              icon="refresh"
              disabled={refreshingModels}
              onClick={() => void refreshModels()}
            >
              {refreshingModels ? 'Refreshing\u2026' : 'Refresh list'}
            </Button>
          </div>
          {selected.models.length > 1 && !selected.allowCustomModel ? (
            <ZoomSafeSelect
              aria-label="Model"
              placeholder="Model"
              value={model}
              onValueChange={setModel}
              options={modelSelectOptions}
            />
          ) : (
            <>
              <Input
                value={model}
                onChange={(event) => setModel(event.target.value)}
                placeholder={selected.defaultModel}
                list={selectedModelList.length > 0 ? `model-options-${selected.id}` : undefined}
              />
              {selectedModelList.length > 0 && (
                <datalist id={`model-options-${selected.id}`}>
                  {selectedModelList.map((item) => <option key={item} value={item} />)}
                </datalist>
              )}
            </>
          )}
        </Field>
        {selected.authType === 'api-key' && (
          <Field>
            <FieldLabel>API key</FieldLabel>
            <PasswordInput value={apiKey} onChange={(event) => setApiKey(event.target.value)} placeholder={selected.apiKeyPlaceholder || 'API key'} />
          </Field>
        )}
        {selected.authType !== 'api-key' && !selectedSupportsBrowserAuth && (
          <Field>
            <FieldLabel>Vault credential</FieldLabel>
            <PasswordInput value={credential} onChange={(event) => setCredential(event.target.value)} placeholder={selected.authDescription || 'Paste token JSON'} />
          </Field>
        )}
        {selected.requiresBaseUrl && (
          <Field>
            <FieldLabel>Base URL</FieldLabel>
            <Input value={baseUrl} onChange={(event) => setBaseUrl(event.target.value)} placeholder={selected.baseUrlPlaceholder || 'https://example.test/v1'} />
            <div className="mt-1.5 rounded-md border border-border bg-muted/40 px-3 py-2">
              <div className="text-xs text-muted-foreground">Requests will be sent to:</div>
              <div className="mt-0.5 break-all font-mono text-xs text-foreground">
                {effectiveChatEndpoint(baseUrl) || <span className="text-muted-foreground">(enter the Base URL above)</span>}
              </div>
            </div>
          </Field>
        )}
        {selectedSupportsBalance && (
          <Field>
            <FieldLabel>Balance</FieldLabel>
            <div className="flex items-center gap-2 text-sm">
              {balanceLoading && <span className="text-muted-foreground">Loading…</span>}
              {!balanceLoading && balance?.available && (
                <span>
                  {balance.used} used{balance.limit ? ` / ${balance.limit} limit` : ''}
                </span>
              )}
              {!balanceLoading && balance && !balance.available && null /* hide row on error */}
              {!balanceLoading && !balance && null /* not yet loaded */}
            </div>
          </Field>
        )}
        <SettingsActions>
          <SaveCancelActions
            onSave={() => void save()}
            onCancel={cancelEdit}
            saving={saving}
            disabled={!dirty}
            size="default"
          />
          <ProviderEnabledToggle
            enabled={selected.enabled}
            isConfigured={selectedConfigured}
            onToggle={(enabled) => void setEnabled(selected, enabled)}
          />
          {selectedConfigured && (
            <Button
              variant="outline"
              onClick={() => void testProvider(selected)}
              icon={testing ? 'hourglass_top' : 'network_check'}
              disabled={testing}
            >
              {testing ? 'Testing…' : 'Test'}
            </Button>
          )}
          {selectedSupportsBrowserAuth && (
            <Button
              variant="outline"
              onClick={() => startBrowserAuth(selected)}
              icon={selectedAuthBusy ? 'hourglass_top' : 'open_in_browser'}
              disabled={selectedAuthBusy}
            >
              {selectedAuthBusy ? 'Waiting for browser...' : selectedAuthComplete ? 'Re-authenticate in browser' : 'Authenticate in browser'}
            </Button>
          )}
          <Button variant="destructive" onClick={() => void deleteCredential(selected)} icon="delete">Remove credential</Button>
          {selected.deletable && (
            <Button variant="destructive" onClick={() => deleteCustomProviderSlot(selected)} icon="delete_forever">Delete provider</Button>
          )}
        </SettingsActions>
      </SettingsPanel>
      <AlertDialog open={pendingDelete !== null} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingDelete?.kind === 'slot' ? 'Delete provider?' : 'Remove credential?'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingDelete?.kind === 'slot'
                ? `Delete the custom provider "${pendingDelete?.provider.name}"? This removes its slot and deletes the saved key, base URL and model from the encrypted vault. This cannot be undone.`
                : `Remove the stored credential for ${pendingDelete?.provider.name}? This deletes the saved key from the encrypted vault. The provider will need to be reconfigured.`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setPendingDelete(null)}>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => void confirmDelete()}>
              {pendingDelete?.kind === 'slot' ? 'Delete' : 'Remove'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      </>
    );
  }

  return (
    <>
      <div className="home-toolbar">
        <div className="home-search">
          <span className="nav-icon home-search-icon material-symbols-outlined" aria-hidden="true">search</span>
          <input
            type="text"
            className="home-search-input"
            placeholder="Search providers..."
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            aria-label="Search providers"
          />
          {search && (
            <button
              type="button"
              className="chat-search-clear"
              onClick={() => setSearch('')}
              aria-label="Clear search"
            >
              <span className="material-symbols-outlined" aria-hidden="true">close</span>
            </button>
          )}
        </div>

        <div className="w-56">
          <ZoomSafeSelect
            value={orderMode}
            onValueChange={(value) => changeOrder(value as ProviderOrderMode)}
            options={(Object.keys(PROVIDER_ORDER_LABELS) as ProviderOrderMode[]).map((mode) => ({
              value: mode,
              label: `Order by: ${PROVIDER_ORDER_LABELS[mode]}`,
            }))}
            aria-label="Order providers"
          />
        </div>

        <div className="home-iconsize" title="Provider card size">
          <button
            type="button"
            className="home-iconsize-btn"
            onClick={() => changeIconSize(ICON_SIZE_MIN)}
            title="Set minimum card size"
            aria-label="Set minimum provider card size"
          >
            <span className="material-symbols-outlined" aria-hidden="true">apps</span>
          </button>
          <input
            type="range"
            className="home-size-range"
            min={ICON_SIZE_MIN}
            max={ICON_SIZE_MAX}
            step={ICON_SIZE_STEP}
            value={iconSize}
            onChange={(event) => changeIconSize(Number(event.target.value))}
            aria-label="Provider card size"
          />
          <button
            type="button"
            className="home-iconsize-btn"
            onClick={() => changeIconSize(ICON_SIZE_MAX)}
            title="Set maximum card size"
            aria-label="Set maximum provider card size"
          >
            <span className="material-symbols-outlined" aria-hidden="true">grid_view</span>
          </button>
        </div>

        <Button variant="outline" icon="add" onClick={() => void addCustomProvider()}>
          Add custom provider
        </Button>
      </div>

      {status?.error && <SettingsNotice tone="error">{status.error}</SettingsNotice>}
      {!status?.error && providers.length === 0 && <SettingsNotice>No providers available.</SettingsNotice>}
      {!status?.error && providers.length > 0 && visibleProviders.length === 0 && (
        <div className="home-empty">No providers found for &quot;{search}&quot;</div>
      )}
      {visibleProviders.length > 0 && (
        <div
          className="home-grid"
          style={{ gridTemplateColumns: `repeat(auto-fill, minmax(${metrics.gridMin}px, 1fr))` }}
        >
          {visibleProviders.map((provider) => (
            <ProviderCard
              key={provider.id}
              provider={provider}
              metrics={metrics}
              customIcon={customIcons[provider.id]}
              configuredOrder={providerIsConfigured(provider) ? providerConfiguredOrder(provider.id, providers, providerOrder) : undefined}
              onEdit={() => editProvider(provider)}
              onSetEnabled={(enabled) => void setEnabled(provider, enabled)}
            />
          ))}
        </div>
      )}

    </>
  );
}

// DeviceCodeBox highlights an OAuth device-flow user code (GitHub Copilot) and
// makes it one click to copy. The panel still auto-updates once the user
// approves in the browser.
function DeviceCodeBox({ code, verificationUri }: { code: string; verificationUri: string }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  }
  return (
    <div className="flex flex-col gap-3 rounded-lg border border-primary/40 bg-primary/5 p-4">
      <p className="text-sm text-muted-foreground">
        Enter this code at{' '}
        <span className="font-medium text-foreground">{verificationUri}</span>{' '}
        to authorize. This panel updates automatically once you approve.
      </p>
      <div className="flex items-center gap-2">
        <div className="flex-1 select-all rounded-md border border-border bg-background px-4 py-3 text-center font-mono text-2xl font-bold tracking-[0.3em] text-foreground">
          {code}
        </div>
        <Button variant="outline" icon={copied ? 'check' : 'content_copy'} onClick={() => void copy()} aria-label="Copy code">
          {copied ? 'Copied' : 'Copy'}
        </Button>
      </div>
    </div>
  );
}

// ProviderEnabledToggle — the user's per-provider on/off switch. Disabled
// providers are never used (not even as fallback); which enabled provider is
// active follows the priority order. Used in both the provider list card and
// the provider detail panel.
function ProviderEnabledToggle({
  enabled,
  isConfigured,
  onToggle,
  size = 'default',
}: {
  enabled: boolean;
  isConfigured: boolean;
  onToggle: (enabled: boolean) => void;
  size?: 'sm' | 'default';
}) {
  return (
    <div
      className="flex items-center gap-2"
      title={!isConfigured ? 'Configure first' : undefined}
    >
      <Switch
        checked={enabled}
        onCheckedChange={onToggle}
        disabled={!isConfigured}
        size={size}
        aria-label="Enabled"
      />
      <span className="text-sm text-muted-foreground">Enabled</span>
    </div>
  );
}

function ProviderCard({
  provider,
  metrics,
  customIcon,
  configuredOrder,
  onEdit,
  onSetEnabled,
}: {
  provider: domain.ProviderInfo;
  metrics: ReturnType<typeof cardMetrics>;
  customIcon?: string;
  configuredOrder?: number;
  onEdit: () => void;
  onSetEnabled: (enabled: boolean) => void;
}) {
  const stateClass = cardStateClass(metrics);
  const status = providerStatusLabel(provider.status);
  const active = provider.status === 'active';
  const configured = !active && (provider.status === 'configured' || provider.connected);
  return (
    <div
      className={`home-card${stateClass ? ` ${stateClass}` : ''}`}
      style={cardStyleVars(metrics)}
      title={`${provider.name} - ${provider.model || provider.defaultModel}`}
      role="button"
      aria-label={provider.name}
      tabIndex={0}
      onClick={onEdit}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onEdit();
        }
      }}
    >
      <div className="home-card-head">
        <ProviderIcon providerId={provider.id} customIcon={customIcon} className="home-card-icon" />
        <div className="home-card-title">{provider.name}</div>
      </div>
      <div className="home-card-desc">{provider.model || provider.defaultModel}</div>
      <div className="mt-auto flex flex-col gap-1.5">
        <div className="flex items-center justify-between gap-2 text-xs">
          <span className="min-w-0 truncate text-muted-foreground">{provider.authType}</span>
          <span className="flex shrink-0 items-center gap-1">
            {configuredOrder && (
              <span className="rounded-full border border-primary bg-primary px-2 py-1 font-medium text-primary-foreground shadow-sm">
                #{configuredOrder}
              </span>
            )}
            <span className={active ? 'rounded-full border border-primary bg-primary px-2 py-1 font-medium text-primary-foreground shadow-sm' : configured ? 'rounded-full border border-primary bg-primary px-2 py-1 font-medium text-primary-foreground shadow-sm opacity-70' : 'rounded-full border border-border bg-background/50 px-2 py-1 text-muted-foreground'}>
              {status}
            </span>
          </span>
        </div>
        {/* The whole card opens the editor (like Apps cards); the toggle row
            stops propagation so flipping enabled never also navigates. */}
        <div
          className="flex items-center justify-between gap-2"
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <ProviderEnabledToggle
            enabled={provider.enabled}
            isConfigured={active || configured}
            onToggle={onSetEnabled}
            size="sm"
          />
          <Button variant="outline" size="sm" icon="edit" onClick={onEdit}>Edit</Button>
        </div>
      </div>
    </div>
  );
}

// effectiveChatEndpoint mirrors the backend base-URL normalization
// (normalizeOpenAICompatibleBaseURL) and appends the fixed OpenAI-compatible
// path, so the UI can show the user exactly which URL requests will hit.
function effectiveChatEndpoint(rawBaseUrl: string): string {
  let url = rawBaseUrl.trim();
  if (url === '') return '';
  url = url.replace(/\/+$/, '');
  if (/\/chat\/completions$/i.test(url)) {
    url = url.replace(/\/chat\/completions$/i, '').replace(/\/+$/, '');
  }
  return `${url}/chat/completions`;
}

function getInitialProviderIconSize(): number {
  const saved = Number(localStorage.getItem(PROVIDER_ICON_SIZE_KEY));
  if (!Number.isNaN(saved) && saved >= ICON_SIZE_MIN && saved <= ICON_SIZE_MAX) return saved;
  return PROVIDER_ICON_SIZE_DEFAULT;
}

function persistProviderIconSize(size: number): void {
  try { localStorage.setItem(PROVIDER_ICON_SIZE_KEY, String(size)); } catch { /* ignore */ }
}

function getInitialProviderOrderMode(): ProviderOrderMode {
  const saved = localStorage.getItem(PROVIDER_ORDER_KEY);
  return saved === 'configured' || saved === 'default' || saved === 'name' ? saved : 'configured';
}

function persistProviderOrderMode(mode: ProviderOrderMode): void {
  try { localStorage.setItem(PROVIDER_ORDER_KEY, mode); } catch { /* ignore */ }
}

function getInitialConfiguredProviderOrder(): Record<string, number> {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(PROVIDER_CONFIGURED_ORDER_KEY) || '{}');
    if (!parsed || typeof parsed !== 'object') return {};
    return Object.fromEntries(
      Object.entries(parsed as Record<string, unknown>)
        .filter(([, value]) => typeof value === 'number' && Number.isFinite(value) && value > 0)
        .map(([key, value]) => [key, Math.round(value as number)]),
    );
  } catch {
    return {};
  }
}

function persistConfiguredProviderOrder(order: Record<string, number>): void {
  try { localStorage.setItem(PROVIDER_CONFIGURED_ORDER_KEY, JSON.stringify(order)); } catch { /* ignore */ }
}

function orderProviders(
  providers: domain.ProviderInfo[],
  mode: ProviderOrderMode,
  configuredOrder: Record<string, number>,
): domain.ProviderInfo[] {
  const indexed = providers.map((provider, index) => ({ provider, index }));
  if (mode === 'configured') {
    return indexed
      .sort((a, b) => providerConfiguredRank(a.provider, providers, configuredOrder) - providerConfiguredRank(b.provider, providers, configuredOrder) || a.index - b.index)
      .map((item) => item.provider);
  }
  if (mode === 'name') {
    return indexed
      .sort((a, b) => a.provider.name.localeCompare(b.provider.name) || a.index - b.index)
      .map((item) => item.provider);
  }
  return indexed.map((item) => item.provider);
}

function providerConfiguredRank(
  provider: domain.ProviderInfo,
  providers: domain.ProviderInfo[],
  configuredOrder: Record<string, number>,
): number {
  if (providerIsConfigured(provider)) return providerConfiguredOrder(provider.id, providers, configuredOrder) - 1;
  return 1000;
}

function providerIsConfigured(provider: domain.ProviderInfo): boolean {
  return provider.status === 'active' || provider.status === 'configured' || provider.connected;
}

// With no persisted order, the backend chain is the active provider first,
// then the remaining configured providers — derive the card numbers from that
// so #1 always names the real Agent Workspace default.
function deriveOrderFromStatus(providers: domain.ProviderInfo[]): Record<string, number> {
  const configured = providers.filter(providerIsConfigured);
  configured.sort((a, b) => Number(b.status === 'active') - Number(a.status === 'active'));
  return Object.fromEntries(configured.map((provider, index) => [provider.id, index + 1]));
}

function providerConfiguredOrder(
  providerID: string,
  providers: domain.ProviderInfo[],
  configuredOrder: Record<string, number>,
): number {
  const ordered = configuredProvidersInOrder(providers, configuredOrder);
  const index = ordered.findIndex((provider) => provider.id === providerID);
  return index >= 0 ? index + 1 : Math.max(1, ordered.length + 1);
}

function reorderConfiguredProvider(
  providers: domain.ProviderInfo[],
  configuredOrder: Record<string, number>,
  providerID: string,
  nextOrder: number,
): Record<string, number> {
  const orderedIDs = configuredProvidersInOrder(providers, configuredOrder).map((provider) => provider.id);
  const currentIndex = orderedIDs.indexOf(providerID);
  if (currentIndex < 0) return configuredOrder;
  orderedIDs.splice(currentIndex, 1);
  orderedIDs.splice(Math.max(0, Math.min(nextOrder - 1, orderedIDs.length)), 0, providerID);
  return Object.fromEntries(orderedIDs.map((id, index) => [id, index + 1]));
}

function configuredProvidersInOrder(
  providers: domain.ProviderInfo[],
  configuredOrder: Record<string, number>,
): domain.ProviderInfo[] {
  return providers
    .filter(providerIsConfigured)
    .map((provider, index) => ({ provider, index }))
    .sort((a, b) => (configuredOrder[a.provider.id] || 1000 + a.index) - (configuredOrder[b.provider.id] || 1000 + b.index) || a.index - b.index)
    .map((item) => item.provider);
}

function providerStatusLabel(status: string) {
  return status.split('-').map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ');
}

function getInitialCustomProviderIcons(): Record<string, string> {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(PROVIDER_CUSTOM_ICONS_KEY) || '{}');
    if (!parsed || typeof parsed !== 'object') return {};
    return Object.fromEntries(
      Object.entries(parsed as Record<string, unknown>).filter(([, value]) => typeof value === 'string'),
    ) as Record<string, string>;
  } catch {
    return {};
  }
}

function persistCustomProviderIcons(icons: Record<string, string>): void {
  try { localStorage.setItem(PROVIDER_CUSTOM_ICONS_KEY, JSON.stringify(icons)); } catch { /* ignore */ }
}
