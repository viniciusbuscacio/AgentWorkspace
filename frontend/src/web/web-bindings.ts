/* eslint-disable @typescript-eslint/no-explicit-any */
// Web-mode Wails bindings shim. A remote browser has no Wails runtime and no
// direct access to the unlocked vault, so window.go / window.runtime are backed
// by the host app's HTTP bridge (POST /bridge) and SSE event stream
// (GET /events). This is the generic sibling of pip/pip-bindings.ts: instead of
// a hand-written method map it proxies *every* App method by name, so the same
// React app runs unchanged in the desktop window and in a remote tab.
//
// Installed by main.tsx before React renders. Auth is a vault-password login
// that mints a session token (kept in sessionStorage, cleared when the tab
// closes); a 401 from the bridge means the session died (lock/autolock/regen)
// and we fall back to the login screen.

export interface WebConfig {
  api: string;
  token: string;
}

const TOKEN_KEY = 'aw_web_token';

type Listener = (data: any) => void;

export function readWebConfig(): WebConfig | null {
  const config = (window as any).aw_WEB;
  if (!config || typeof config.api !== 'string') return null;
  return config as WebConfig;
}

// isWebMode reports whether the app is running in a remote browser (web mode),
// as opposed to the desktop Wails window or a PiP process. Components branch on
// it when a native-dialog method is denied over the bridge and a React
// alternative must be used instead (e.g. destructive provider deletes).
export function isWebMode(): boolean {
  return readWebConfig() !== null;
}

function storedToken(): string {
  return sessionStorage.getItem(TOKEN_KEY) || '';
}

// isErrorEnvelope detects the bridge's "the Go method returned a non-nil error"
// response — exactly {"error": "..."} — which must reject the promise, mirroring
// how Wails rejects on a Go error return. Real result DTOs carry more fields
// (e.g. {success, error, ...}) so they resolve normally.
function isErrorEnvelope(body: any): boolean {
  return (
    body && typeof body === 'object' && !Array.isArray(body) &&
    Object.keys(body).length === 1 && typeof body.error === 'string'
  );
}

/**
 * bootWebMode is the entry point main.tsx calls. If a session token is present
 * it installs the bindings and renders the app; otherwise it shows the login
 * screen, then installs and renders once authenticated.
 */
export async function bootWebMode(config: WebConfig, renderApp: () => void): Promise<void> {
  const onAuthLost = () => {
    sessionStorage.removeItem(TOKEN_KEY);
    // A full reload re-reads index.html (token empty) and returns to login,
    // tearing down any stale React/event state cleanly.
    window.location.reload();
  };

  if (storedToken()) {
    installWebBindings(config, onAuthLost);
    renderApp();
    return;
  }
  await renderLogin(config, () => {
    installWebBindings(config, onAuthLost);
    renderApp();
  });
}

export function installWebBindings(config: WebConfig, onAuthLost: () => void): void {
  const bus = new Map<string, Set<Listener>>();

  function emit(name: string, payload: any) {
    bus.get(name)?.forEach((listener) => listener(payload));
  }
  function on(name: string, listener: Listener): () => void {
    if (!bus.has(name)) bus.set(name, new Set());
    bus.get(name)!.add(listener);
    return () => bus.get(name)?.delete(listener);
  }

  async function bridge(method: string, args: any[]): Promise<any> {
    const response = await fetch(`${config.api}/bridge`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${storedToken()}`,
      },
      body: JSON.stringify({ method, args }),
    });
    if (response.status === 401) {
      onAuthLost();
      throw new Error('session expired');
    }
    const body = await response.json().catch(() => null);
    if (!response.ok) {
      throw new Error((body && body.error) || `web bridge error ${response.status}`);
    }
    if (isErrorEnvelope(body)) {
      throw new Error(body.error);
    }
    return body;
  }

  const appProxy = new Proxy(
    {},
    {
      get(_target, prop) {
        if (typeof prop !== 'string') return undefined;
        return (...args: any[]) => bridge(prop, args);
      },
    },
  );

  (window as any).go = { main: { App: appProxy } };
  (window as any).runtime = {
    EventsOn: on,
    EventsOnMultiple: (name: string, listener: Listener) => on(name, listener),
    EventsOnce: (name: string, listener: Listener) => {
      const off = on(name, (data) => { off(); listener(data); });
      return off;
    },
    EventsOff: (name: string) => bus.delete(name),
    EventsEmit: emit,
    LogInfo: console.log,
    LogError: console.error,
    LogDebug: console.debug,
    LogWarning: console.warn,
    LogTrace: console.trace,
    LogPrint: console.log,
    LogFatal: console.error,
    Environment: () => Promise.resolve({ buildType: 'production', platform: 'web', arch: 'web' }),
    BrowserOpenURL: (url: string) => { window.open(url, '_blank', 'noopener'); },
    ClipboardSetText: async (text: string) => {
      try { await navigator.clipboard?.writeText(text); return true; } catch { return false; }
    },
    ClipboardGetText: async () => {
      try { return (await navigator.clipboard?.readText()) || ''; } catch { return ''; }
    },
    Quit: () => {},
    WindowReload: () => window.location.reload(),
    WindowReloadApp: () => window.location.reload(),
  };

  if (typeof EventSource !== 'undefined') {
    connectEvents(config, emit, onAuthLost);
  }
}

function connectEvents(config: WebConfig, emit: (name: string, payload: any) => void, onAuthLost: () => void) {
  let consecutiveErrors = 0;
  const connect = () => {
    const url = new URL(`${config.api || window.location.origin}/events`, window.location.origin);
    url.searchParams.set('token', storedToken());
    const events = new EventSource(url.toString());
    events.onopen = () => { consecutiveErrors = 0; };
    events.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data || '{}');
        if (data?.name) emit(data.name, data.payload ?? {});
      } catch {
        // Ignore malformed frames / heartbeat comments.
      }
    };
    events.onerror = () => {
      // EventSource auto-reconnects, but a session that died (lock/autolock)
      // never recovers — after several failures, drop to the login screen.
      consecutiveErrors += 1;
      if (consecutiveErrors >= 4) {
        events.close();
        onAuthLost();
      }
    };
  };
  connect();
}

// renderLogin draws a minimal vault-password login (and first-run setup) screen
// directly into #root before React boots. On success it stores the session
// token and calls onAuthed. Kept as plain DOM so it works before — and
// independent of — the React bundle.
async function renderLogin(config: WebConfig, onAuthed: () => void): Promise<void> {
  const root = document.getElementById('root');
  if (!root) throw new Error('root container #root not found');

  let setup = { vaultExists: true, unlocked: false, bindMode: 'tailscale', bindWarning: '' };
  try {
    const res = await fetch(`${config.api}/setup/status`);
    if (res.ok) setup = { ...setup, ...(await res.json()) };
  } catch {
    // Fall back to login-only when setup status is unavailable.
  }

  return new Promise<void>((resolve) => {
    const isSetup = !setup.vaultExists;
    root.innerHTML = '';
    const wrap = document.createElement('div');
    wrap.setAttribute('style',
      'min-height:100vh;display:flex;align-items:center;justify-content:center;' +
      'background:#171717;color:#e5e5e5;font-family:system-ui,sans-serif;');
    const card = document.createElement('form');
    card.setAttribute('style',
      'width:320px;display:flex;flex-direction:column;gap:12px;padding:28px;' +
      'background:#222;border:1px solid #333;border-radius:12px;');
    card.innerHTML =
      `<h1 style="margin:0 0 4px;font-size:18px;">Agent Workspace</h1>` +
      `<p style="margin:0;font-size:13px;color:#9ca3af;">${
        isSetup ? 'Create a vault password to set up this server.' : 'Unlock with your vault password.'
      }</p>` +
      (setup.bindWarning
        ? `<p style="margin:0;font-size:12px;color:#fbbf24;">${setup.bindWarning}</p>`
        : '');

    const password = document.createElement('input');
    password.type = 'password';
    password.placeholder = 'Vault password';
    password.autofocus = true;
    password.setAttribute('style', inputStyle());
    card.appendChild(password);

    const error = document.createElement('div');
    error.setAttribute('style', 'min-height:16px;font-size:12px;color:#f87171;');
    const submit = document.createElement('button');
    submit.type = 'submit';
    submit.textContent = isSetup ? 'Create vault' : 'Unlock';
    submit.setAttribute('style',
      'padding:9px;border:0;border-radius:8px;background:#6366f1;color:#fff;' +
      'font-weight:600;cursor:pointer;');

    card.appendChild(error);
    card.appendChild(submit);
    wrap.appendChild(card);
    root.appendChild(wrap);

    card.addEventListener('submit', async (e) => {
      e.preventDefault();
      error.textContent = '';
      submit.disabled = true;
      submit.textContent = isSetup ? 'Creating…' : 'Unlocking…';
      try {
        const path = isSetup ? '/setup/create' : '/login';
        const res = await fetch(`${config.api}${path}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ password: password.value }),
        });
        const body = await res.json().catch(() => ({}));
        if (!res.ok || !body.token) {
          throw new Error(body.error || (isSetup ? 'Could not create vault' : 'Invalid password'));
        }
        sessionStorage.setItem(TOKEN_KEY, body.token);
        root.innerHTML = '';
        resolve();
        onAuthed();
      } catch (err: any) {
        error.textContent = err?.message || 'Login failed';
        submit.disabled = false;
        submit.textContent = isSetup ? 'Create vault' : 'Unlock';
      }
    });
  });
}

function inputStyle(): string {
  return 'padding:9px;border:1px solid #3f3f46;border-radius:8px;background:#18181b;' +
    'color:#e5e5e5;font-size:14px;outline:none;';
}
