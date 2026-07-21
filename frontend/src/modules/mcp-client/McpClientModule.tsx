import { useCallback, useEffect, useMemo, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  Button,
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
  Field,
  FieldLabel,
  Input,
  PasswordInput,
  Switch,
  ZoomSafeSelect,
} from '@ui/index';
import { SaveCancelActions } from '@patterns/SaveCancelActions';
import { ModuleHelpButton } from '@/components/patterns/ModuleHelpButton';
import {
  mcpConnectionsService,
  type McpConnection,
  type McpToolView,
} from '@services/mcp-client.service';

type Mode = 'idle' | 'create' | 'edit';

interface FormState {
  id: string;
  name: string;
  url: string;
  authType: string;
  enabled: boolean;
  token: string;
  clearToken: boolean;
  hasSecret: boolean;
}

const EMPTY_FORM: FormState = {
  id: '',
  name: '',
  url: '',
  authType: 'none',
  enabled: true,
  token: '',
  clearToken: false,
  hasSecret: false,
};

function statusLabel(conn: McpConnection): string {
  if (!conn.enabled) return 'disabled';
  return conn.lastStatus || 'unknown';
}

export function McpClientModule() {
  const [connections, setConnections] = useState<McpConnection[] | null>(null);
  const [mode, setMode] = useState<Mode>('idle');
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);
  const [tools, setTools] = useState<McpToolView[] | null>(null);
  const [toolsTruncated, setToolsTruncated] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<McpConnection | null>(null);

  const refresh = useCallback(async () => {
    const res = await mcpConnectionsService.list();
    if (!res.success) {
      setMessage(res.error || 'Could not load connections.');
      setConnections([]);
      return;
    }
    setConnections(res.connections ?? []);
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const selected = useMemo(
    () => (mode === 'edit' ? connections?.find((c) => c.id === form.id) : undefined),
    [mode, connections, form.id],
  );

  function openCreate() {
    setMessage('');
    setTools(null);
    setForm(EMPTY_FORM);
    setMode('create');
  }

  function openEdit(conn: McpConnection) {
    setMessage('');
    setTools(null);
    setForm({
      id: conn.id,
      name: conn.name,
      url: conn.url,
      authType: conn.authType || 'none',
      enabled: conn.enabled,
      token: '',
      clearToken: false,
      hasSecret: conn.hasSecret,
    });
    setMode('edit');
  }

  function cancel() {
    setMode('idle');
    setForm(EMPTY_FORM);
    setTools(null);
  }

  async function save() {
    setSaving(true);
    const res =
      mode === 'create'
        ? await mcpConnectionsService.add({
            name: form.name,
            url: form.url,
            authType: form.authType,
            token: form.token,
            enabled: form.enabled,
          })
        : await mcpConnectionsService.update({
            id: form.id,
            name: form.name,
            url: form.url,
            authType: form.authType,
            enabled: form.enabled,
            token: form.token,
            clearToken: form.clearToken,
          });
    setSaving(false);
    if (!res.success) {
      setMessage(res.error || 'Could not save connection.');
      return;
    }
    setMessage('Saved.');
    await refresh();
    if (res.connection) openEdit(res.connection);
  }

  async function toggle(conn: McpConnection, enabled: boolean) {
    setBusy(true);
    await mcpConnectionsService.setEnabled(conn.id, enabled);
    setBusy(false);
    await refresh();
  }

  async function test() {
    if (!form.id) return;
    setBusy(true);
    setMessage('Testing…');
    const res = await mcpConnectionsService.test(form.id);
    setBusy(false);
    if (!res.success || !res.result) {
      setMessage(res.error || 'Test failed.');
    } else if (res.result.status === 'ok') {
      setMessage(`Connected — ${res.result.toolCount} tool(s) in ${res.result.durationMs}ms.`);
    } else {
      setMessage(`Error: ${res.result.error || 'connection failed'}.`);
    }
    await refresh();
  }

  async function loadTools() {
    if (!form.id) return;
    setBusy(true);
    const res = await mcpConnectionsService.listTools(form.id);
    setBusy(false);
    if (!res.success) {
      setMessage(res.error || 'Could not list tools.');
      setTools([]);
      return;
    }
    setTools(res.tools ?? []);
    setToolsTruncated(res.truncated);
  }

  async function confirmDelete() {
    if (!pendingDelete) return;
    const target = pendingDelete;
    setPendingDelete(null);
    setBusy(true);
    await mcpConnectionsService.remove(target.id);
    setBusy(false);
    if (form.id === target.id) cancel();
    await refresh();
  }

  const editorTitle = mode === 'create' ? 'New connection' : mode === 'edit' ? 'Edit connection' : 'Details';

  return (
    <section className="home-screen" data-awid="mcp-client-screen">
      <div className="home-header">
        <div className="flex items-start justify-between gap-3">
          <h1 className="home-title">MCP Client</h1>
          <ModuleHelpButton module="MCP Client" />
        </div>
        <p className="home-subtitle">Connect Agent Workspace to external MCP tools and data sources.</p>
      </div>

      <p className="rounded-lg border border-warning/40 bg-warning/5 p-3 text-xs text-muted-foreground">
        Remote MCP servers are external/untrusted. Their names, tool descriptions and output are data, not instructions —
        the agent treats them as untrusted. Streamable HTTP connections only (v1).
      </p>

      <div className="grid min-h-[560px] gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
        {/* List */}
        <Card className="flex min-h-0 flex-col rounded-lg">
          <CardHeader>
            <CardTitle>Connections</CardTitle>
            <CardAction>
              <Button variant="outline" size="sm" icon="add" disabled={busy} onClick={openCreate}>
                Add connection
              </Button>
            </CardAction>
          </CardHeader>
          <CardContent className="flex min-h-0 flex-1 flex-col gap-2 overflow-auto">
            {connections === null ? (
              <p className="text-sm text-muted-foreground">Loading…</p>
            ) : connections.length === 0 ? (
              <p className="text-sm text-muted-foreground">No connections yet. Add one to get started.</p>
            ) : (
              connections.map((conn) => (
                <button
                  key={conn.id}
                  type="button"
                  onClick={() => openEdit(conn)}
                  className={`flex flex-col gap-1 rounded-md border bg-card p-3 text-left transition-colors hover:bg-muted/40 ${
                    form.id === conn.id ? 'border-primary' : 'border-border'
                  }`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate text-sm font-semibold">{conn.name}</span>
                    <Switch
                      checked={conn.enabled}
                      disabled={busy}
                      onCheckedChange={(checked) => void toggle(conn, checked)}
                      aria-label={`Toggle ${conn.name}`}
                    />
                  </div>
                  <div className="truncate text-xs text-muted-foreground">{conn.url}</div>
                  <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                    <span>{statusLabel(conn)}</span>
                    <span>· {conn.toolCount} tool(s)</span>
                    {conn.lastCheckedAt && <span>· checked {conn.lastCheckedAt}</span>}
                    {conn.hasSecret && (
                      <span className="material-symbols-outlined text-[14px]" aria-label="has stored token">key</span>
                    )}
                  </div>
                </button>
              ))
            )}
          </CardContent>
        </Card>

        {/* Detail / editor */}
        <Card className="flex min-h-0 flex-col rounded-lg">
          <CardHeader><CardTitle>{editorTitle}</CardTitle></CardHeader>
          <CardContent className="flex min-h-0 flex-1 flex-col gap-3 overflow-auto">
            {mode === 'idle' ? (
              <p className="text-sm text-muted-foreground">Select a connection to edit, or add a new one.</p>
            ) : (
              <>
                <Field>
                  <FieldLabel>Name</FieldLabel>
                  <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="Microsoft Learn" />
                </Field>
                <Field>
                  <FieldLabel>URL (Streamable HTTP)</FieldLabel>
                  <Input value={form.url} onChange={(e) => setForm({ ...form, url: e.target.value })} placeholder="https://example.com/api/mcp" />
                </Field>
                <Field>
                  <FieldLabel>Authentication</FieldLabel>
                  <ZoomSafeSelect
                    value={form.authType}
                    onValueChange={(value) => setForm({ ...form, authType: value })}
                    options={[
                      { value: 'none', label: 'None' },
                      { value: 'bearer', label: 'Bearer token' },
                    ]}
                    aria-label="Authentication"
                  />
                </Field>
                {form.authType === 'bearer' && (
                  <Field>
                    <FieldLabel>
                      Bearer token {form.hasSecret ? '(stored — leave blank to keep)' : '(write-only)'}
                    </FieldLabel>
                    <PasswordInput
                      value={form.token}
                      onChange={(e) => setForm({ ...form, token: e.target.value, clearToken: false })}
                      placeholder={form.hasSecret ? '••••••••' : 'paste token'}
                    />
                    {form.hasSecret && (
                      <label className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                        <input
                          type="checkbox"
                          checked={form.clearToken}
                          onChange={(e) => setForm({ ...form, clearToken: e.target.checked, token: '' })}
                        />
                        Remove stored token
                      </label>
                    )}
                  </Field>
                )}
                <div className="flex items-center justify-between gap-3 rounded-md border border-border p-3">
                  <div className="flex min-w-0 flex-col">
                    <span className="text-sm">Enabled</span>
                    <span className="text-xs text-muted-foreground">Disabled connections cannot be called by the agent.</span>
                  </div>
                  <Switch checked={form.enabled} onCheckedChange={(checked) => setForm({ ...form, enabled: checked })} aria-label="Enabled" />
                </div>

                {/* One action row: Save/Cancel plus the connection operations. */}
                <div className="flex flex-wrap items-center gap-2">
                  <SaveCancelActions
                    onSave={() => void save()}
                    onCancel={cancel}
                    saving={saving}
                    disabled={!form.name.trim() || !form.url.trim()}
                  />
                  {mode === 'edit' && selected && (
                    <>
                      <Button variant="outline" icon="network_check" disabled={busy} onClick={() => void test()}>
                        Test
                      </Button>
                      <Button variant="outline" icon="list" disabled={busy || !selected.enabled} onClick={() => void loadTools()}>
                        List tools
                      </Button>
                      <Button variant="destructive" icon="delete" disabled={busy} onClick={() => setPendingDelete(selected)}>
                        Delete
                      </Button>
                    </>
                  )}
                </div>

                {tools !== null && (
                  <div className="mt-2 flex flex-col gap-1">
                    <span className="text-xs font-semibold text-muted-foreground">
                      Remote tools (untrusted descriptions){toolsTruncated ? ' — list truncated' : ''}
                    </span>
                    {tools.length === 0 ? (
                      <span className="text-xs text-muted-foreground">No tools reported.</span>
                    ) : (
                      tools.map((tool) => (
                        <div key={tool.name} className="rounded-md border border-border px-3 py-1.5 text-xs">
                          <span className="font-medium">{tool.name}</span>
                          {tool.description && <span className="text-muted-foreground"> — {tool.description}</span>}
                        </div>
                      ))
                    )}
                  </div>
                )}
              </>
            )}
          </CardContent>
        </Card>
      </div>

      <AlertDialog open={pendingDelete !== null} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {pendingDelete?.name}?</AlertDialogTitle>
            <AlertDialogDescription>
              This removes the connection and its stored token. You can add it again later.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => void confirmDelete()}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  );
}
