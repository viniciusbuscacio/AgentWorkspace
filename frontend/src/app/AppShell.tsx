import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react';
import type { domain } from '@services/models';
import { HomeModule } from '@modules/home/HomeModule';
import { type SettingsPage } from '@modules/settings/SettingsModule';
import { OPEN_SETTINGS_EVENT, type OpenSettingsDetail } from '@/lib/open-settings';
import { OPEN_VIEW_EVENT, type OpenViewDetail } from '@/lib/open-view';
import type { LeaveGuard } from '@modules/module-contract';
import { AwIcon } from '@ui/aw-icon';
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
import { ChatModule } from '@modules/chat/ChatModule';
import { initChatSendState } from '@modules/chat/chat-send-state';
import { initSubagentTasks } from '@modules/chat/subagent-tasks';
import { addInlineImageMessage, clearInlineImageMessages } from '@modules/chat/inline-image-messages';
import { clearModelChangeMessages } from '@modules/chat/model-change-messages';
import { clearPromptDebugMessages } from '@modules/chat/prompt-debug-messages';
import { hasModuleView, moduleView } from '@modules/module-views';
import { moduleMenuExtras } from '@modules/module-contract';
import { chatService } from '@services/chat.service';
import { settingsService } from '@services/settings.service';
import { modulesService, type ModuleInfo } from '@services/modules.service';
import { serversService, type ServerIndicator } from '@services/servers.service';
import { vaultService } from '@services/vault.service';
import { onAwEvent } from '@services/events';
import { notify } from '@/lib/notify';
import { uiService } from '@services/ui.service';
import { wallpaperService } from '@services/wallpaper.service';
import { sessionService } from '@services/session.service';
import { useSidebar, useSidebarPrefs, SB_ICON_SIZES, SB_MENU_POSITIONS, type SidebarIconSizeKey, type SidebarPosition, type UseSidebarPrefs } from '@/hooks/sidebar.hooks';
import { APP_ZOOM_EVENT, APP_ZOOM_STEP, applyAppZoom, clampAppZoom, measureFixedScale } from '@/lib/app-zoom';
import { getWallpaperBackgroundImage, applyWallpaperGlassVars, wallpaperGlassWrapperClass, isCustomWallpaperId } from '@modules/wallpaper/wallpaper-data';

// A view is a fixed surface (home/settings), the core chat module, or any
// added workspace-module id rendered through MODULE_VIEWS.
type View = string;

interface AppShellProps {
  onLocked: () => Promise<void> | void;
}

interface ChatMenuState {
  chatId: string;
  archived: boolean;
  x: number;
  y: number;
}

interface ModuleMenuState {
  moduleId: string;
  name: string;
  fixed: boolean;
  x: number;
  y: number;
}

// Sidebar + layout ported 1:1 from the vanilla AW2/aw frontend (render()).
// The DOM is rendered the same in every mode; aw-sidebar.css hides labels/search
// per sidebar-mode-* on the #app container. Only the #app class + --sb-* vars
// change with the resize.
export function AppShell({ onLocked }: AppShellProps) {
  const [view, setView] = useState<View>('home');
  const [chats, setChats] = useState<domain.Chat[]>([]);
  const [selectedChatId, setSelectedChatId] = useState<string | undefined>();
  const [archivedOpen, setArchivedOpen] = useState(false);
  const [chatFilter, setChatFilter] = useState('');
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [chatMenu, setChatMenu] = useState<ChatMenuState | null>(null);
  // Permanent chat delete is confirmed via modal — it is irreversible.
  const [pendingDeleteChat, setPendingDeleteChat] = useState<{ id: string; title: string } | null>(null);
  const [moduleMenu, setModuleMenu] = useState<ModuleMenuState | null>(null);
  const [sidebarMenu, setSidebarMenu] = useState<{ x: number; y: number } | null>(null);
  // Which settings page to open + a nonce to force a fresh mount each time the
  // user navigates to a specific page (so it always lands on the right page).
  const [settingsNav, setSettingsNav] = useState<{ page: SettingsPage | undefined; nonce: number }>({ page: undefined, nonce: 0 });
  const [modules, setModules] = useState<ModuleInfo[]>([]);
  // Live network-server state behind the sidebar exposure dot (initial pull +
  // servers:state pushes — no polling).
  const [servers, setServers] = useState<ServerIndicator | null>(null);
  const [wallpaperId, setWallpaperId] = useState('default');
  // Resolved CSS background-image for the content area. For bundled wallpapers
  // it is a url(/wallpapers/..) or the default gradient; for user uploads it is
  // a base64 data URI fetched from the backend (uploads are not served assets).
  const [wallpaperImage, setWallpaperImage] = useState(() => getWallpaperBackgroundImage('default'));
  const [wallpaperGlass, setWallpaperGlass] = useState(20);
  // Session restore: stored view from last session, applied once modules load.
  const [sessionRestoreView, setSessionRestoreView] = useState<string | null>(null);
  const zoomRef = useRef(100);
const sidebar = useSidebar();
  const sidebarPrefs = useSidebarPrefs();

  // Unsaved-changes guard. The active Settings child (the Skills editor)
  // registers a LeaveGuard here; requestLeave routes any navigation that would
  // discard Settings through it, so a dirty editor confirms before leaving.
  // Reads via refs so it stays stable for use inside event handlers/effects.
  const viewRef = useRef<View>('home');
  const leaveGuardRef = useRef<LeaveGuard | null>(null);
  const registerLeaveGuard = useCallback((guard: LeaveGuard | null) => {
    leaveGuardRef.current = guard;
  }, []);
  const requestLeave = useCallback((proceed: () => void) => {
    const guard = viewRef.current === 'settings' ? leaveGuardRef.current : null;
    if (guard) guard(proceed);
    else proceed();
  }, []);
  useEffect(() => {
    viewRef.current = view;
  }, [view]);

  // A settings deep-link (e.g. the Account menu's "LLM Providers") is one-shot:
  // once the user leaves the Settings view, a later generic return (sidebar
  // item) must restore their last open page, not replay the stale link.
  useEffect(() => {
    if (view !== 'settings') {
      setSettingsNav((s) => (s.page === undefined ? s : { page: undefined, nonce: s.nonce }));
    }
  }, [view]);

  // Open/focus the singleton Settings module at an optional deep-link page,
  // guarded so a dirty editor confirms first. The settings-add effect ensures
  // the sidebar item exists; the nonce forces a fresh page on each open.
  const goToSettings = useCallback((page?: SettingsPage) => {
    requestLeave(() => {
      setSettingsNav((s) => ({ page, nonce: s.nonce + 1 }));
      setView('settings');
    });
  }, [requestLeave]);

  // In-app deep links to a Settings page from outside the Settings module (e.g.
  // a server card linking to the TLS manager) arrive as a window event.
  useEffect(() => {
    const handler = (event: Event) => {
      const detail = (event as CustomEvent<OpenSettingsDetail>).detail;
      if (detail?.page) goToSettings(detail.page);
    };
    window.addEventListener(OPEN_SETTINGS_EVENT, handler);
    return () => window.removeEventListener(OPEN_SETTINGS_EVENT, handler);
  }, [goToSettings]);

  // In-app deep links to a module view (e.g. Settings › Servers hub cards
  // opening the full REST/MCP/Web pages) — the frontend twin of ui:navigate.
  useEffect(() => {
    const handler = (event: Event) => {
      const detail = (event as CustomEvent<OpenViewDetail>).detail;
      if (detail?.view) requestLeave(() => setView(detail.view));
    };
    window.addEventListener(OPEN_VIEW_EVENT, handler);
    return () => window.removeEventListener(OPEN_VIEW_EVENT, handler);
  }, [requestLeave]);

  // Sidebar exposure dot: mirror the network servers' live state.
  useEffect(() => {
    serversService.getIndicator().then(setServers).catch(() => setServers(null));
    return onAwEvent('servers:state', setServers);
  }, []);

  // Optional chaining throughout: a bridge without the binding (web mode,
  // tests, dev mock) resolves to {} and must not crash the shell.
  const anyServerRunning = Boolean(servers?.rest?.running || servers?.mcp?.running || servers?.web?.running);
  const serverDotTitle = useMemo(() => {
    if (!servers) return '';
    const on = [
      servers.rest?.running ? `REST :${servers.rest.port}` : null,
      servers.mcp?.running ? `MCP :${servers.mcp.port}` : null,
      servers.web?.running ? `Web :${servers.web.port}` : null,
    ].filter(Boolean);
    const label = on.length === 1 ? '1 server listening' : `${on.length} servers listening`;
    return `${label} — ${on.join(', ')} — click to manage`;
  }, [servers]);

  // One-click lock: flip to the gate optimistically (the backend lock also
  // snapshots the working copy to the master before closing the DB).
  const lockNow = useCallback(async () => {
    try {
      await vaultService.lock();
    } finally {
      await onLocked();
    }
  }, [onLocked]);

  const activeChats = useMemo(() => chats.filter((chat) => !chat.archived), [chats]);
  const archivedChats = useMemo(() => {
    const query = chatFilter.trim().toLowerCase();
    return chats
      .filter((chat) => chat.archived)
      .filter((chat) => !query || (chat.title || 'New Chat').toLowerCase().includes(query));
  }, [chatFilter, chats]);

  // Auto-select the newest chat ONLY on the first load (app open / session
  // restore fallback). Later refreshes never re-select: after the user
  // deletes the displayed chat, "nothing selected" must stay that way.
  const autoSelectDoneRef = useRef(false);
  const refreshChats = useCallback(async () => {
    const next = await chatService.listChats();
    setChats(next);
    if (!autoSelectDoneRef.current) {
      autoSelectDoneRef.current = true;
      // Newest ACTIVE chat only — an archived chat never self-selects.
      const firstActive = next.find((chat) => !chat.archived);
      if (!selectedChatId && firstActive) setSelectedChatId(firstActive.id);
    }
    return next;
  }, [selectedChatId]);

  // Chat run/queue bookkeeping for the whole app lifetime: queued messages
  // flush on run completion even while the chat view is unmounted.
  useEffect(() => initChatSendState(), []);

  // Live subagent (spawn) cards: keep the per-chat store fed by chat:subagent
  // events for the whole app lifetime, independent of which view is mounted.
  useEffect(() => initSubagentTasks(), []);

  // Boot ONCE. This reads the persisted lastChatId and restores it — so it
  // MUST NOT re-run on every chat switch. It used to depend on [refreshChats],
  // which changes identity whenever selectedChatId changes, re-running this
  // block and yanking the selection back to the saved chat (clicking chat2
  // bounced to chat1). The ref makes it fire exactly once.
  const bootedRef = useRef(false);
  useEffect(() => {
    if (bootedRef.current) return;
    bootedRef.current = true;
    void (async () => {
      const [chats, lastView, lastChatID] = await Promise.all([
        refreshChats(),
        sessionService.getLastView(),
        sessionService.getLastChatID(),
      ]);
      // Restore last session. A flash of Home is acceptable per the spec;
      // a crash on a deleted chat is not — validate both before applying.
      // lastView is validated against the module list loaded after chats.
      // We defer the view restore to a separate tick so modules list is ready.
      if (lastChatID) {
        // An archived chat is CLOSED: restoring it would resurrect the pane
        // the user dismissed ("chat travado"). Only live chats restore.
        const found = chats.find((c) => c.id === lastChatID && !c.archived);
        if (found) {
          setSelectedChatId(lastChatID);
        }
        // If not found, selectedChatId stays as the first chat (set by refreshChats).
      }
      if (lastView && lastView !== 'home') {
        // The chat view only restores when there is a live chat to show —
        // with everything archived/deleted it would render an empty chat
        // pane instead of the desktop (screenshot 2026-06-12 16:57).
        const hasLiveChat = chats.some((c) => !c.archived);
        if (lastView !== 'chat' || hasLiveChat) {
          // Modules list may not be loaded yet; schedule restore after modules load.
          setSessionRestoreView(lastView);
        }
      }
    })();
    void wallpaperService.get().then(setWallpaperId);
    void wallpaperService.getGlass().then((glass) => {
      setWallpaperGlass(glass);
      applyWallpaperGlassVars(glass);
    });
    void settingsService.getAppZoomPercent().then((next) => {
      zoomRef.current = clampAppZoom(next);
      applyAppZoom(zoomRef.current);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps -- boot once; see bootedRef
  }, []);

  // Workspace modules: catalog + added state from the backend registry,
  // kept in sync with agent-driven changes via modules:changed.
  useEffect(() => {
    void modulesService.list().then((result) => setModules(result.modules ?? []));
    return onAwEvent('modules:changed', (payload) => {
      setModules((payload.modules ?? []) as ModuleInfo[]);
    });
  }, []);

  // Session restore: once modules load, validate the saved view against the
  // navigable set (fixed surfaces + added modules) and jump there. Falls back
  // to 'home' for any module that was removed since the last close.
  //
  // Race-condition guard: if the user navigated away from 'home' before
  // modules finished loading (session restore was pending but modules came
  // last), the user's explicit navigation wins — we discard the restore.
  // Without this guard, setView(sessionRestoreView) would override the chat
  // the user just clicked, making module-to-chat navigation appear broken.
  useEffect(() => {
    if (!sessionRestoreView || modules.length === 0) return;
    if (view !== 'home') {
      // User navigated before modules loaded — honour that choice.
      setSessionRestoreView(null);
      return;
    }
    const navigable = new Set(['home', 'settings', 'chat',
      ...modules.filter((m) => m.added).map((m) => m.id)]);
    setView(navigable.has(sessionRestoreView) ? sessionRestoreView : 'home');
    setSessionRestoreView(null); // apply once
  }, [sessionRestoreView, modules, view]);

  // Visible sidebar items in the user's order. Hidden modules stay added
  // (their actions are untouched) — they just have no sidebar item until
  // reopened from Apps, module.add or app.navigate.
  // Settings is excluded: it has a permanent nav button below Apps.
  const sidebarModules = useMemo(
    () => modules
      .filter((mod) => mod.added && !mod.core && !mod.hidden && mod.id !== 'settings' && hasModuleView(mod.id))
      .sort((a, b) => a.sidebarPosition - b.sidebarPosition),
    [modules],
  );

  // Single source of truth for app zoom: persists and applies.
  const changeZoom = useCallback((percent: number) => {
    const next = clampAppZoom(percent);
    if (next === zoomRef.current) return;
    zoomRef.current = next;
    applyAppZoom(next);
    void settingsService.setAppZoomPercent(next);
  }, []);

  // Ctrl/Cmd + scroll and Ctrl/Cmd +/-/0 zoom the whole app (vanilla parity), persisted.
  useEffect(() => {
    const setZoom = changeZoom;
    const onWheel = (event: WheelEvent) => {
      if (!(event.ctrlKey || event.metaKey)) return;
      event.preventDefault();
      setZoom(zoomRef.current + (event.deltaY < 0 ? APP_ZOOM_STEP : -APP_ZOOM_STEP));
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (!(event.ctrlKey || event.metaKey) || event.altKey) return;
      const key = String(event.key || '').toLowerCase();
      const code = String(event.code || '').toLowerCase();
      const isZoomIn = key === '+' || key === '=' || key === 'plus' || code === 'equal' || code === 'numpadadd';
      const isZoomOut = key === '-' || key === '_' || key === 'minus' || code === 'minus' || code === 'numpadsubtract';
      const isZoomReset = key === '0' || code === 'digit0' || code === 'numpad0';
      if (isZoomIn) {
        event.preventDefault();
        setZoom(zoomRef.current + APP_ZOOM_STEP);
      } else if (isZoomOut) {
        event.preventDefault();
        setZoom(zoomRef.current - APP_ZOOM_STEP);
      } else if (isZoomReset) {
        event.preventDefault();
        setZoom(100);
      }
    };
    // Views outside the shell (Settings → Fonts) request zoom changes through
    // this window event — same deep-link pattern as OPEN_SETTINGS_EVENT.
    const onZoomEvent = (event: Event) => setZoom((event as CustomEvent<number>).detail);
    document.addEventListener('wheel', onWheel, { passive: false });
    document.addEventListener('keydown', onKeyDown);
    window.addEventListener(APP_ZOOM_EVENT, onZoomEvent);
    return () => {
      document.removeEventListener('wheel', onWheel);
      document.removeEventListener('keydown', onKeyDown);
      window.removeEventListener(APP_ZOOM_EVENT, onZoomEvent);
    };
  }, [changeZoom]);

  useEffect(() => {
    const offLocked = onAwEvent('vault:auto-locked', () => { void onLocked(); });
    const offRenamed = onAwEvent('chat:renamed', (payload) => {
      setChats((current) => current.map((chat) => chat.id === payload.chatId ? { ...chat, title: payload.title } : chat));
    });
    const offRefresh = onAwEvent('chat:refresh', () => { void refreshChats(); });
    // Notify when a chat reply arrives while the window is not focused.
    // In-app toast only (user decision 2026-07-09) — it is still visible when
    // the user returns to the window via the toast history Sonner keeps
    // during the session.
    const offDone = onAwEvent('chat:done', () => {
      if (document.hidden) notify('Chat reply ready.');
    });
    // Backend-originated notifications (agent app.notify, provider fallback,
    // sign-in prompts) arrive here and use the same toast as local notices.
    const offNotify = onAwEvent('app:notify', (payload) => {
      const title = (payload.title || '').trim();
      const body = (payload.body || '').trim();
      // Title + body render as the two Alert lines; with only one of them,
      // that one becomes the single title line.
      if (title && body) notify(title, body);
      else notify(title || body);
    });
    // Persist agent self-screenshots here, at the always-mounted shell, so the
    // image is captured even when the user has navigated away from the chat
    // (e.g. the agent clicked "Apps", screenshotted, then returned). The active
    // ChatModule renders it live; on return, the chat loads it from storage.
    const offInlineImage = onAwEvent('chat:inline-image', (payload) => {
      addInlineImageMessage(payload);
    });
    return () => { offLocked(); offRenamed(); offRefresh(); offDone(); offNotify(); offInlineImage(); };
  }, [onLocked, refreshChats]);

  // Backend-driven navigation/zoom (aw actions app.navigate, chat.open,
  // chat.create and app.zoom.set). Zoom is already persisted by the backend
  // before the event arrives, so it is only applied here.
  useEffect(() => {
    const offNavigate = onAwEvent('ui:navigate', (payload) => {
      if (payload.view === 'chat') {
        requestLeave(() => {
          if (payload.chatId) setSelectedChatId(payload.chatId);
          setView('chat');
          void refreshChats();
        });
        return;
      }
      if (payload.view === 'settings') {
        // Accept settings even when it is closed/hidden/not added — the
        // settings-add effect makes the singleton sidebar item appear.
        requestLeave(() => {
          setSettingsNav((s) => ({ page: undefined, nonce: s.nonce + 1 }));
          setView('settings');
        });
        return;
      }
      requestLeave(() => setView(payload.view));
    });
    const offZoom = onAwEvent('ui:set-zoom', ({ percent }) => {
      const next = clampAppZoom(percent);
      zoomRef.current = next;
      applyAppZoom(next);
    });
    const offWallpaper = onAwEvent('ui:set-wallpaper', ({ id }) => setWallpaperId(id));
    const offGlass = onAwEvent('ui:set-wallpaper-glass', ({ opacity }) => {
      const v = Number(opacity);
      setWallpaperGlass(v);
      applyWallpaperGlassVars(v);
    });
    return () => { offNavigate(); offZoom(); offWallpaper(); offGlass(); };
  }, [refreshChats, requestLeave]);

  // Resolve the content-area background whenever the wallpaper changes. Bundled
  // wallpapers map to a url()/gradient synchronously; user uploads need a data
  // URI fetched from the backend.
  useEffect(() => {
    if (!isCustomWallpaperId(wallpaperId)) {
      setWallpaperImage(getWallpaperBackgroundImage(wallpaperId));
      return;
    }
    let active = true;
    void wallpaperService.getImage(wallpaperId).then((result) => {
      if (!active) return;
      setWallpaperImage(
        result.success && result.dataUri
          ? `url("${result.dataUri}")`
          : getWallpaperBackgroundImage('default'),
      );
    });
    return () => { active = false; };
  }, [wallpaperId]);

  // Mirror the active view/chat to the backend for the aw app.state action.
  useEffect(() => {
    void uiService.reportUiState({ view, chatId: selectedChatId ?? '' });
  }, [view, selectedChatId]);

  // Persist last session (debounced) so it survives restarts.
  const sessionSaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (sessionSaveTimerRef.current) clearTimeout(sessionSaveTimerRef.current);
    sessionSaveTimerRef.current = setTimeout(() => {
      void sessionService.saveLastSession(view, selectedChatId ?? '');
    }, 500);
    return () => {
      if (sessionSaveTimerRef.current) clearTimeout(sessionSaveTimerRef.current);
    };
  }, [view, selectedChatId]);

  async function createChat() {
    const chat = await chatService.createChat();
    setChats((current) => [chat, ...current.filter((item) => item.id !== chat.id)]);
    setSelectedChatId(chat.id);
    setView('chat');
  }

  async function createSpecChat(title: string, body: string) {
    const trimmedTitle = title.trim() || 'Task spec';
    const base = `${trimmedTitle}${body.trim() ? `\n\n${body.trim()}` : ''}`;
    const message = `Create a technical spec for this task (tasks item):\n\n${base}\n\nWrite the spec in the body of the tasks item using tasks.update. Include: problem description, solution approach, files to modify/create, dependencies, and effort estimate.`;
    const chat = await chatService.createChatWithTitle(`Spec: ${trimmedTitle}`);
    setChats((current) => [chat, ...current.filter((item) => item.id !== chat.id)]);
    setSelectedChatId(chat.id);
    setView('chat');
    window.setTimeout(() => {
      void chatService.streamChatMessage(chat.id, message, []);
    }, 250);
  }

  function openChat(chatId?: string) {
    if (chatId) {
      setSelectedChatId(chatId);
      setView('chat');
      return;
    }
    void createChat();
  }

  // Opening a chat from the Archived list restores it to a normal, active chat
  // in the sidebar (user request 2026-06-13). Show it on the right immediately,
  // then unarchive in the background so it reappears under the active list.
  async function openArchivedChat(chatId: string) {
    setSelectedChatId(chatId);
    setView('chat');
    await chatService.setChatArchived(chatId, false);
    await refreshChats();
  }

  // Home-card click: chat opens a new chat; any other module is added to the
  // workspace (idempotent) and opened.
  async function openModule(type: string) {
    if (type === 'chat') {
      await createChat();
      return;
    }
    const mod = modules.find((item) => item.id === type);
    if (!mod || mod.comingSoon || !hasModuleView(type)) return;
    if (!mod.added) {
      const result = await modulesService.add(type);
      if (!result.success) return;
      setModules(result.modules ?? []);
    }
    setView(type);
  }

  async function removeModule(moduleId: string) {
    const result = await modulesService.remove(moduleId);
    if (result.success) setModules(result.modules ?? []);
    if (view === moduleId) setView('home');
  }

  // Close (the hover ×): hide from the sidebar, keep the module added.
  async function hideModule(moduleId: string) {
    const result = await modulesService.hide(moduleId);
    if (result.success) setModules(result.modules ?? []);
    if (view === moduleId) setView('home');
  }

async function moveModule(moduleId: string, up: boolean) {
    const result = await modulesService.move(moduleId, up);
    if (result.success) setModules(result.modules ?? []);
  }

  // Every path that lands on a hidden module's view (Apps card, agent
  // app.navigate, deep links) reopens it — the agent cannot see the hidden
  // state and must never get a view without a sidebar item.
  useEffect(() => {
    const mod = modules.find((entry) => entry.id === view);
    if (mod?.added && mod.hidden) {
      void modulesService.show(view).then((result) => {
        if (result.success) setModules(result.modules ?? []);
      });
    }
  }, [view, modules]);

  // Settings is a fixed built-in module: navigating to it (nav button,
  // ui:navigate, session restore) keeps its added state in sync in the module
  // registry. AddModule is idempotent and also unhides, so this never duplicates.
  useEffect(() => {
    if (view !== 'settings') return;
    const mod = modules.find((entry) => entry.id === 'settings');
    if (mod && !mod.added) {
      void modulesService.add('settings').then((result) => {
        if (result.success) setModules(result.modules ?? []);
      });
    }
  }, [view, modules]);

  function openModuleMenu(event: React.MouseEvent, mod: ModuleInfo) {
    event.preventDefault();
    event.stopPropagation();
    const scale = measureFixedScale();
    setModuleMenu({ moduleId: mod.id, name: mod.name, fixed: Boolean(mod.fixed), x: event.clientX / scale, y: event.clientY / scale });
    setChatMenu(null);
    setSidebarMenu(null);
  }

  function startRename(chat: domain.Chat) {
    setChatMenu(null);
    setRenamingId(chat.id);
    setRenameValue(chat.title || '');
  }

  async function commitRename(chatId: string) {
    const title = renameValue.trim();
    setRenamingId(null);
    if (!title) return;
    await chatService.renameChat(chatId, title);
    await refreshChats();
  }

  async function setArchived(chatId: string, archived: boolean) {
    // Closing (archiving) the DISPLAYED chat closes its pane too — land on
    // the desktop BEFORE the round-trip, so a slow or failed backend call
    // never leaves the closed chat rendered on the right.
    if (archived && selectedChatId === chatId) {
      setSelectedChatId(undefined);
      setView('home');
    }
    await chatService.setChatArchived(chatId, archived);
    await refreshChats();
  }

  async function removeChat(chatId: string) {
    await chatService.deleteChat(chatId);
    // The chat's client-side system bubbles (model changes, inline images,
    // prompt debug) live in web storage keyed by this id; drop them with it.
    clearModelChangeMessages(chatId);
    clearInlineImageMessages(chatId);
    clearPromptDebugMessages(chatId);
    if (selectedChatId === chatId) {
      setSelectedChatId(undefined);
      setView('home');
    }
    await refreshChats();
  }

  function toggleArchived() {
    setArchivedOpen((current) => {
      if (current) setChatFilter('');
      return !current;
    });
  }

  function openChatMenu(event: React.MouseEvent, chat: domain.Chat) {
    event.preventDefault();
    event.stopPropagation();
    const scale = measureFixedScale();
    setChatMenu({ chatId: chat.id, archived: Boolean(chat.archived), x: event.clientX / scale, y: event.clientY / scale });
    setSidebarMenu(null);
  }

  // Right-click on the empty sidebar area opens the sidebar config menu
  // (vanilla parity). Coordinates and viewport bounds are converted to the
  // fixed-element coordinate space so the menu stays visible under app zoom.
  function openSidebarMenu(event: React.MouseEvent) {
    event.preventDefault();
    const scale = measureFixedScale();
    const viewportW = window.innerWidth / scale;
    const viewportH = window.innerHeight / scale;
    const x = event.clientX / scale;
    const y = event.clientY / scale;
    const menuHeight = collapsed ? SIDEBAR_MENU_H_WITH_ICON_NAMES : SIDEBAR_MENU_H;
    const clampedX = Math.max(8, Math.min(x, viewportW - SIDEBAR_MENU_W - 8));
    const clampedY = Math.max(8, Math.min(y, viewportH - menuHeight - 8));
    setSidebarMenu({ x: clampedX, y: clampedY });
    setChatMenu(null);
  }

  const collapsed = sidebar.mode === 'icon' || sidebar.mode === 'peek';
  const isHorizontal = sidebarPrefs.position === 'top' || sidebarPrefs.position === 'bottom';

  // Instant tooltip for the mini status bar. position:fixed via React state —
  // a CSS ::after inside the sidebar gets clipped by its overflow:hidden when
  // the sidebar is narrow (icon mode). Same fixed-coordinate technique as the
  // context menus (rect ÷ measureFixedScale, see app-zoom.ts).
  const [statusTip, setStatusTip] = useState<{ text: string; x: number; y: number; side: 'top' | 'bottom' | 'left' | 'right' } | null>(null);
  function showStatusTip(event: ReactMouseEvent<HTMLElement>, text: string) {
    const rect = event.currentTarget.getBoundingClientRect();
    const scale = measureFixedScale();
    if (sidebarPrefs.position === 'top') {
      setStatusTip({ text, side: 'bottom', x: (rect.left + rect.width / 2) / scale, y: rect.bottom / scale + 6 });
    } else if (collapsed && !isHorizontal) {
      // Vertical stack in the narrow sidebar: float the tip beside the button.
      if (sidebarPrefs.position === 'right') {
        setStatusTip({ text, side: 'left', x: rect.left / scale - 8, y: (rect.top + rect.height / 2) / scale });
      } else {
        setStatusTip({ text, side: 'right', x: rect.right / scale + 8, y: (rect.top + rect.height / 2) / scale });
      }
    } else {
      setStatusTip({ text, side: 'top', x: (rect.left + rect.width / 2) / scale, y: rect.top / scale - 6 });
    }
  }
  const hideStatusTip = () => setStatusTip(null);
  const sidebarMode = isHorizontal ? 'horizontal' : sidebar.mode;
  const showIconNames = sidebarMode === 'icon' && sidebarPrefs.showIconNames;
  // Nav labels are hidden only in plain icon mode — that's when the top nav
  // items (New Chat / Apps / Settings) need the instant tooltip too.
  const showNavTips = sidebarMode === 'icon' && !showIconNames;
  const appClassName = [
    `sidebar-pos-${sidebarPrefs.position}`,
    `sidebar-mode-${sidebarMode}`,
    showIconNames ? 'sidebar-icon-names' : '',
  ].filter(Boolean).join(' ');

  return (
    <div
      id="app"
      className={appClassName}
      style={{
        ['--sb-w' as string]: `${sidebar.width}px`,
        ['--sb-icon' as string]: `${SB_ICON_SIZES[sidebarPrefs.iconSizeKey]}px`,
      }}
    >
      <aside className="sidebar" onContextMenu={openSidebarMenu}>
        <div
          className="sidebar-resize"
          title="Drag to resize · double-click to toggle"
          onPointerDown={sidebar.beginResize}
          onDoubleClick={sidebar.toggleCollapse}
        />

        <div className="nav-block">
          <button
            className="nav-item primary"
            type="button"
            onClick={() => requestLeave(() => void createChat())}
            aria-label="New Chat"
            onMouseEnter={showNavTips ? (event) => showStatusTip(event, 'New Chat') : undefined}
            onMouseLeave={showNavTips ? hideStatusTip : undefined}
          >
            <span className="nav-icon"><span className="material-symbols-outlined">add</span></span>
            <span className="nav-label">New Chat</span>
          </button>
          <NavItem
            icon="apps"
            label="Apps"
            active={view === 'home'}
            onClick={() => requestLeave(() => setView('home'))}
            onTipEnter={showNavTips ? (event) => showStatusTip(event, 'Apps') : undefined}
            onTipLeave={showNavTips ? hideStatusTip : undefined}
          />
          <NavItem
            icon="settings"
            label="Settings"
            active={view === 'settings'}
            onClick={() => requestLeave(() => setView('settings'))}
            onTipEnter={showNavTips ? (event) => showStatusTip(event, 'Settings') : undefined}
            onTipLeave={showNavTips ? hideStatusTip : undefined}
          />
          <div className="sidebar-divider" role="separator" aria-hidden="true" />
        </div>

        <div className="sidebar-scroll">
          {sidebarModules.length > 0 && (
            <div className="nav-block">
              {sidebarModules.map((mod) => (
                <ModuleNavItem
                  key={mod.id}
                  name={mod.name}
                  icon={mod.icon}
                  active={view === mod.id}
                  onOpen={() => requestLeave(() => setView(mod.id))}
                  onClose={() => void hideModule(mod.id)}
                  onContextMenu={(event) => openModuleMenu(event, mod)}
                />
              ))}
            </div>
          )}
          <div className="nav-block">
            {activeChats.slice(0, 200).map((chat) => (
              <ChatNavItem
                key={chat.id}
                chat={chat}
                active={view === 'chat' && selectedChatId === chat.id}
                renaming={renamingId === chat.id}
                renameValue={renameValue}
                onRenameValueChange={setRenameValue}
                onOpen={() => requestLeave(() => openChat(chat.id))}
                onCommitRename={() => void commitRename(chat.id)}
                onCancelRename={() => setRenamingId(null)}
                onClose={() => void setArchived(chat.id, true)}
                onContextMenu={(event) => openChatMenu(event, chat)}
                onTipEnter={showNavTips ? (event) => showStatusTip(event, chat.title || 'New Chat') : undefined}
                onTipLeave={showNavTips ? hideStatusTip : undefined}
              />
            ))}
          </div>
        </div>

        <div className={`nav-block archived-block ${archivedOpen ? 'open' : ''}`}>
          <div className={`archived-list ${archivedOpen ? 'open' : ''}`}>
            {archivedOpen && (
              <div className="chat-search">
                <span className="nav-icon chat-search-icon"><span className="material-symbols-outlined">search</span></span>
                <input
                  type="text"
                  className="chat-search-input"
                  placeholder="Search archived..."
                  value={chatFilter}
                  onChange={(event) => setChatFilter(event.target.value)}
                  aria-label="Search archived chats"
                />
                {chatFilter && (
                  <button type="button" className="chat-search-clear" onClick={() => setChatFilter('')} aria-label="Clear search">
                    <span className="material-symbols-outlined">close</span>
                  </button>
                )}
              </div>
            )}
            {archivedOpen && archivedChats.length === 0 && chatFilter.trim()
              ? <p className="chat-list-empty">No archived chats match your search.</p>
              : archivedChats.slice(0, 200).map((chat) => (
                <ChatNavItem
                  key={chat.id}
                  chat={chat}
                  active={view === 'chat' && selectedChatId === chat.id}
                  renaming={renamingId === chat.id}
                  renameValue={renameValue}
                  onRenameValueChange={setRenameValue}
                  onOpen={() => requestLeave(() => void openArchivedChat(chat.id))}
                  onCommitRename={() => void commitRename(chat.id)}
                  onCancelRename={() => setRenamingId(null)}
                  onClose={() => setPendingDeleteChat({ id: chat.id, title: chat.title })}
                  onContextMenu={(event) => openChatMenu(event, chat)}
                  onTipEnter={showNavTips ? (event) => showStatusTip(event, chat.title || 'New Chat') : undefined}
                  onTipLeave={showNavTips ? hideStatusTip : undefined}
                />
              ))}
          </div>
        </div>

        {/* Mini status bar, left to right: hide sidebar, lock vault, archived
            chats, server exposure dot. The dot exists only while a network
            server is actually listening (servers:state); the archived toggle
            opens the list upward in the archived-block above. In icon mode the
            bar stacks vertically with its own order (archived, servers, lock,
            hide) — see aw-sidebar.css. */}
        <div className="sidebar-status" data-awid="sidebar-status">
          <button
            type="button"
            className="sidebar-status-btn"
            aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            data-awid="sidebar-hide"
            onMouseEnter={(event) => showStatusTip(event, collapsed ? 'Show' : 'Hide')}
            onMouseLeave={hideStatusTip}
            onClick={() => { hideStatusTip(); sidebar.toggleCollapse(); }}
          >
            <span className="material-symbols-outlined sidebar-status-icon">{collapsed ? 'left_panel_open' : 'left_panel_close'}</span>
          </button>
          <button
            type="button"
            className="sidebar-status-btn"
            aria-label="Lock vault"
            data-awid="lock-now"
            onMouseEnter={(event) => showStatusTip(event, 'Lock')}
            onMouseLeave={hideStatusTip}
            onClick={() => { hideStatusTip(); void lockNow(); }}
          >
            <span className="material-symbols-outlined sidebar-status-icon">lock</span>
          </button>
          <button
            type="button"
            className={`sidebar-status-btn ${archivedOpen ? 'open' : ''}`}
            aria-label="Archived Chats"
            data-awid="archived-toggle"
            onMouseEnter={(event) => showStatusTip(event, 'Archived Chats')}
            onMouseLeave={hideStatusTip}
            onClick={toggleArchived}
          >
            <span className="material-symbols-outlined sidebar-status-icon">archive</span>
          </button>
          {anyServerRunning && (
            <button
              type="button"
              className="sidebar-status-btn"
              aria-label={serverDotTitle}
              data-awid="server-dot"
              onMouseEnter={(event) => showStatusTip(event, 'Servers')}
              onMouseLeave={hideStatusTip}
              onClick={() => { hideStatusTip(); goToSettings('servers'); }}
            >
              <span className="server-dot" aria-hidden="true" />
            </button>
          )}
        </div>

      </aside>

      <main className="content" style={{ ['--aw-wallpaper-image' as string]: wallpaperImage }}>
        <div className="content-body">
          {/* ONE wallpaper treatment for EVERY view (Home included) so the
             glass slider applies uniformly across the whole app. Home used to
             bypass this with aw-wallpaper-surface and looked different from the
             modules at the same setting. wallpaperGlassWrapperClass is the
             single source of truth: solid only at glass 0, frosted veil
             (wallpaper visible) above it. --aw-wallpaper-glass-pct is set on
             :root by AppShell (load + events). */}
          <div className={`${wallpaperGlassWrapperClass(wallpaperGlass)} min-h-full`}>
            {view === 'home' && (
              <HomeModule
                modules={modules
                  // Settings has its own permanent sidebar button — no Apps card.
                  .filter((mod) => !mod.comingSoon && mod.id !== 'settings')
                  .map((mod) => ({
                    type: mod.id,
                    name: mod.name,
                    icon: mod.icon,
                    description: mod.description,
                    added: mod.added,
                    core: mod.core,
                  }))}
                onOpenModule={(type) => void openModule(type)}
              />
            )}
            {view === 'chat' && (
              // Key by chat id: each chat gets a fresh ChatModule mount, so
              // one chat's messages/state never bleed into another (opening
              // a second chat used to show the first). Safe to remount —
              // in-flight + queue state lives in chat-send-state, outside
              // the component.
              <ChatModule
                key={selectedChatId ?? 'no-chat'}
                chatId={selectedChatId}
                onChatsChanged={refreshChats}
                onSelectChat={(chatId) => setSelectedChatId(chatId)}
                onOpenProvidersSettings={() => goToSettings('providers')}
              />
            )}
            {hasModuleView(view) && (
              <ModuleView
                key={view === 'settings' ? `settings-${settingsNav.nonce}` : view}
                id={view}
                createSpecChat={createSpecChat}
                onLocked={onLocked}
                settingsInitialPage={view === 'settings' ? settingsNav.page : undefined}
                registerLeaveGuard={registerLeaveGuard}
              />
            )}
          </div>
        </div>
      </main>

      {moduleMenu && (
        <ModuleContextMenu
          state={moduleMenu}
          moduleIndex={sidebarModules.findIndex((mod) => mod.id === moduleMenu.moduleId)}
          moduleCount={sidebarModules.length}
          onClose={() => setModuleMenu(null)}
          onCloseModule={() => {
            const id = moduleMenu.moduleId;
            if (id === 'settings') requestLeave(() => void hideModule(id));
            else void hideModule(id);
            setModuleMenu(null);
          }}
          onMoveUp={() => { void moveModule(moduleMenu.moduleId, true); setModuleMenu(null); }}
          onMoveDown={() => { void moveModule(moduleMenu.moduleId, false); setModuleMenu(null); }}
          onRemove={() => { void removeModule(moduleMenu.moduleId); setModuleMenu(null); }}
        />
      )}
      {chatMenu && (
        <ChatContextMenu
          state={chatMenu}
          onClose={() => setChatMenu(null)}
          onRename={() => {
            const chat = chats.find((item) => item.id === chatMenu.chatId);
            if (chat) startRename(chat);
          }}
          onArchive={() => { void setArchived(chatMenu.chatId, !chatMenu.archived); setChatMenu(null); }}
          onDelete={() => {
            const chat = chats.find((item) => item.id === chatMenu.chatId);
            setPendingDeleteChat({ id: chatMenu.chatId, title: chat?.title ?? '' });
            setChatMenu(null);
          }}
        />
      )}
      {sidebarMenu && (
        <SidebarContextMenu
          state={sidebarMenu}
          prefs={sidebarPrefs}
          showIconNamesOption={collapsed}
          onClose={() => setSidebarMenu(null)}
        />
      )}
      {statusTip && (
        <div className={`sidebar-tip sidebar-tip-${statusTip.side}`} style={{ left: statusTip.x, top: statusTip.y }} role="tooltip">
          {statusTip.text}
        </div>
      )}
      <AlertDialog open={pendingDeleteChat !== null} onOpenChange={(open) => !open && setPendingDeleteChat(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Delete {pendingDeleteChat?.title ? `“${pendingDeleteChat.title}”` : 'this chat'}?
            </AlertDialogTitle>
            <AlertDialogDescription>
              The chat and all of its messages will be permanently deleted. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-white hover:bg-destructive/90"
              onClick={() => {
                const id = pendingDeleteChat?.id;
                setPendingDeleteChat(null);
                if (id) void removeChat(id);
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

// ModuleView renders a registered workspace-module view by id. Settings-only
// runtime props (onLocked, settingsInitialPage, registerLeaveGuard) are passed
// through generically; non-settings views ignore them.
function ModuleView({
  id,
  createSpecChat,
  onLocked,
  settingsInitialPage,
  registerLeaveGuard,
}: {
  id: string;
  createSpecChat: (title: string, body: string) => Promise<void>;
  onLocked: () => Promise<void> | void;
  settingsInitialPage?: SettingsPage;
  registerLeaveGuard: (guard: LeaveGuard | null) => void;
}) {
  const def = moduleView(id);
  if (!def) return null;
  const Component = def.view;
  return (
    <Component
      createSpecChat={createSpecChat}
      onLocked={onLocked}
      settingsInitialPage={settingsInitialPage}
      registerLeaveGuard={registerLeaveGuard}
    />
  );
}

// ModuleNavItem is THE sidebar entry for a workspace module — the single
// implementation of the sidebar rules (hover × close button, right-click
// menu). Every handler prop is required, so a module item without close or
// menu behavior cannot be rendered; the registry side of the same contract is
// defineModuleView in module-contract.ts.
interface ModuleNavItemProps {
  name: string;
  icon: string;
  active: boolean;
  onOpen: () => void;
  /** The hover × button — removes the module from the workspace. */
  onClose: () => void;
  /** The right-click context menu trigger. */
  onContextMenu: (event: React.MouseEvent) => void;
}

function ModuleNavItem({ name, icon, active, onOpen, onClose, onContextMenu }: ModuleNavItemProps) {
  return (
    <div
      className={`nav-item module-nav-item ${active ? 'active' : ''}`}
      role="button"
      tabIndex={0}
      aria-label={name}
      onClick={onOpen}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen();
        }
      }}
      onContextMenu={onContextMenu}
    >
      <span className="nav-icon"><AwIcon icon={icon} /></span>
      <span className="nav-label">{name}</span>
      <button
        className="nav-chat-close"
        type="button"
        onClick={(event) => { event.stopPropagation(); onClose(); }}
        aria-label={`Close ${name}`}
        title="Close (reopen from Apps)"
      >
        <span className="material-symbols-outlined">close</span>
      </button>
    </div>
  );
}

// ModuleContextMenu renders the module's registered extras (from
// defineModuleView) above the standard destructive Remove action.
function ModuleContextMenu({
  state,
  moduleIndex,
  moduleCount,
  onClose,
  onCloseModule,
  onMoveUp,
  onMoveDown,
  onRemove,
}: {
  state: ModuleMenuState;
  moduleIndex: number;
  moduleCount: number;
  onClose: () => void;
  onCloseModule: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onRemove: () => void;
}) {
  const scale = useMemo(() => measureFixedScale(), []);
  const def = moduleView(state.moduleId);
  const extras = def ? moduleMenuExtras(def) : [];
  const viewportW = window.innerWidth / scale;
  const viewportH = window.innerHeight / scale;
  const x = Math.min(state.x, viewportW - 220);
  const y = Math.min(state.y, viewportH - 196 - extras.length * 36);
  return (
    <>
      <div className="context-menu-overlay" onClick={onClose} onContextMenu={(event) => { event.preventDefault(); onClose(); }} />
      <div className="context-menu" role="menu" style={{ left: x, top: y }}>
        <button className="context-menu-item" type="button" role="menuitem" onClick={onCloseModule}>
          <span className="context-menu-icon material-symbols-outlined">close</span>
          <span>Close</span>
        </button>
        <button className="context-menu-item" type="button" role="menuitem" disabled={moduleIndex <= 0} onClick={onMoveUp}>
          <span className="context-menu-icon material-symbols-outlined">arrow_upward</span>
          <span>Move up</span>
        </button>
        <button
          className="context-menu-item"
          type="button"
          role="menuitem"
          disabled={moduleIndex < 0 || moduleIndex >= moduleCount - 1}
          onClick={onMoveDown}
        >
          <span className="context-menu-icon material-symbols-outlined">arrow_downward</span>
          <span>Move down</span>
        </button>
        {extras.map((extra) => (
          <button
            key={extra.id}
            className="context-menu-item"
            type="button"
            role="menuitem"
            onClick={() => { void extra.onSelect(); onClose(); }}
          >
            <span className="context-menu-icon material-symbols-outlined">{extra.icon}</span>
            <span>{extra.label}</span>
          </button>
        ))}
        {!state.fixed && (
          <>
            <div className="context-menu-separator" />
            <button className="context-menu-item destructive" type="button" role="menuitem" onClick={onRemove}>
              <span className="context-menu-icon material-symbols-outlined">remove_circle_outline</span>
              <span>Remove {state.name} from workspace</span>
            </button>
          </>
        )}
      </div>
    </>
  );
}

function NavItem({
  icon,
  label,
  active = false,
  extraClass = '',
  onClick,
  onTipEnter,
  onTipLeave,
}: {
  icon: string;
  label: string;
  active?: boolean;
  extraClass?: string;
  onClick: () => void;
  onTipEnter?: (event: ReactMouseEvent<HTMLElement>) => void;
  onTipLeave?: () => void;
}) {
  return (
    <button
      className={`nav-item ${extraClass} ${active ? 'active' : ''}`}
      type="button"
      onClick={onClick}
      aria-label={label}
      onMouseEnter={onTipEnter}
      onMouseLeave={onTipLeave}
    >
      <span className="nav-icon"><span className="material-symbols-outlined">{icon}</span></span>
      <span className="nav-label">{label}</span>
    </button>
  );
}

interface ChatNavItemProps {
  chat: domain.Chat;
  active: boolean;
  renaming: boolean;
  renameValue: string;
  onRenameValueChange: (value: string) => void;
  onOpen: () => void;
  onCommitRename: () => void;
  onCancelRename: () => void;
  /** Archive (active chat) or delete (archived chat) — the × button on hover. */
  onClose: () => void;
  onContextMenu: (event: React.MouseEvent) => void;
  /** Instant tooltip with the chat title while icon mode hides the label. */
  onTipEnter?: (event: ReactMouseEvent<HTMLElement>) => void;
  onTipLeave?: () => void;
}

function ChatNavItem({
  chat,
  active,
  renaming,
  renameValue,
  onRenameValueChange,
  onOpen,
  onCommitRename,
  onCancelRename,
  onClose,
  onContextMenu,
  onTipEnter,
  onTipLeave,
}: ChatNavItemProps) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const label = chat.title || 'New Chat';
  const archived = Boolean(chat.archived);

  useEffect(() => {
    if (renaming) inputRef.current?.focus();
  }, [renaming]);

  return (
    <div
      className={`nav-item chat-nav-item ${active ? 'active' : ''}`}
      role="button"
      tabIndex={0}
      aria-label={label}
      onClick={() => { if (!renaming) onOpen(); }}
      onKeyDown={(event) => { if (!renaming && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); onOpen(); } }}
      onContextMenu={onContextMenu}
      onMouseEnter={onTipEnter}
      onMouseLeave={onTipLeave}
    >
      <span className="nav-icon"><span className="material-symbols-outlined">chat</span></span>
      {renaming ? (
        <input
          ref={inputRef}
          className="nav-rename-input"
          type="text"
          value={renameValue}
          aria-label="Rename chat"
          onClick={(event) => event.stopPropagation()}
          onChange={(event) => onRenameValueChange(event.target.value)}
          onBlur={onCommitRename}
          onKeyDown={(event) => {
            if (event.key === 'Enter') onCommitRename();
            if (event.key === 'Escape') onCancelRename();
          }}
        />
      ) : (
        <>
          <span className="nav-label">{label}</span>
          <button
            className="nav-chat-close"
            type="button"
            onClick={(event) => { event.stopPropagation(); onClose(); }}
            aria-label={archived ? `Delete ${label}` : `Archive ${label}`}
            title={archived ? 'Delete chat permanently' : 'Archive chat'}
          >
            <span className="material-symbols-outlined">{archived ? 'delete' : 'close'}</span>
          </button>
        </>
      )}
    </div>
  );
}

function ChatContextMenu({
  state,
  onClose,
  onRename,
  onArchive,
  onDelete,
}: {
  state: ChatMenuState;
  onClose: () => void;
  onRename: () => void;
  onArchive: () => void;
  onDelete: () => void;
}) {
  const scale = useMemo(() => measureFixedScale(), []);
  const viewportW = window.innerWidth / scale;
  const viewportH = window.innerHeight / scale;
  const x = Math.min(state.x, viewportW - 200);
  const y = Math.min(state.y, viewportH - 160);
  return (
    <>
      <div className="context-menu-overlay" onClick={onClose} onContextMenu={(event) => { event.preventDefault(); onClose(); }} />
      <div className="context-menu" role="menu" style={{ left: x, top: y }}>
        <button className="context-menu-item" type="button" role="menuitem" onClick={onRename}>
          <span className="context-menu-icon material-symbols-outlined">edit</span>
          <span>Rename</span>
        </button>
        <button className="context-menu-item" type="button" role="menuitem" onClick={onArchive}>
          <span className="context-menu-icon material-symbols-outlined">{state.archived ? 'unarchive' : 'archive'}</span>
          <span>{state.archived ? 'Unarchive' : 'Archive'}</span>
        </button>
        <div className="context-menu-separator" />
        <button className="context-menu-item destructive" type="button" role="menuitem" onClick={onDelete}>
          <span className="context-menu-icon material-symbols-outlined">delete</span>
          <span>Delete</span>
        </button>
      </div>
    </>
  );
}

const SB_POSITION_META: Record<SidebarPosition, { label: string; icon: string }> = {
  left: { label: 'Left', icon: 'dock_to_left' },
  right: { label: 'Right', icon: 'dock_to_right' },
  top: { label: 'Top', icon: 'vertical_align_top' },
  bottom: { label: 'Bottom', icon: 'vertical_align_bottom' },
};

const SB_SIZE_OPTIONS: Array<[SidebarIconSizeKey, string]> = [
  ['small', 'Small'],
  ['medium', 'Medium'],
  ['large', 'Large'],
  ['extraLarge', 'Extra Large'],
];

type SidebarSubmenu = 'position' | 'iconSize';

const SIDEBAR_MENU_W = 220;
const SIDEBAR_MENU_H = 96;
const SIDEBAR_MENU_H_WITH_ICON_NAMES = 136;
const SIDEBAR_SUBMENU_W = 180;
const SIDEBAR_SUBMENU_H = 148;
const SIDEBAR_SUBMENU_GAP = 6;

// Sidebar config menu — visual parity with the AW2 Radix context menu (main
// menu + submenus), rendered manually so we keep the zoom-safe positioning.
function SidebarContextMenu({
  state,
  prefs,
  showIconNamesOption,
  onClose,
}: {
  state: { x: number; y: number };
  prefs: UseSidebarPrefs;
  showIconNamesOption: boolean;
  onClose: () => void;
}) {
  const [openSubmenu, setOpenSubmenu] = useState<SidebarSubmenu | null>(null);
  const scale = useMemo(() => measureFixedScale(), []);
  const viewportW = window.innerWidth / scale;
  const viewportH = window.innerHeight / scale;
  const canOpenRight = state.x + SIDEBAR_MENU_W + SIDEBAR_SUBMENU_GAP + SIDEBAR_SUBMENU_W <= viewportW - 8;
  const submenuLeft = canOpenRight
    ? state.x + SIDEBAR_MENU_W + SIDEBAR_SUBMENU_GAP
    : Math.max(8, state.x - SIDEBAR_SUBMENU_W - SIDEBAR_SUBMENU_GAP);
  const submenuBaseTop = openSubmenu === 'iconSize' ? state.y + 37 : state.y;
  const submenuTop = Math.max(8, Math.min(submenuBaseTop, viewportH - SIDEBAR_SUBMENU_H - 8));

  return (
    <>
      <div className="context-menu-overlay" onClick={onClose} onContextMenu={(event) => { event.preventDefault(); onClose(); }} />
      <div
        className="context-menu sidebar-context-menu"
        role="menu"
        style={{ left: state.x, top: state.y, width: SIDEBAR_MENU_W }}
      >
        <button
          className={`context-menu-item is-subtrigger ${openSubmenu === 'position' ? 'is-open' : ''}`}
          type="button"
          role="menuitem"
          aria-haspopup="menu"
          aria-expanded={openSubmenu === 'position'}
          onMouseEnter={() => setOpenSubmenu('position')}
          onFocus={() => setOpenSubmenu('position')}
          onClick={() => setOpenSubmenu('position')}
        >
          <span className="context-menu-item-label">Sidebar position</span>
          <span className="context-menu-arrow" aria-hidden="true">›</span>
        </button>
        <button
          className={`context-menu-item is-subtrigger ${openSubmenu === 'iconSize' ? 'is-open' : ''}`}
          type="button"
          role="menuitem"
          aria-haspopup="menu"
          aria-expanded={openSubmenu === 'iconSize'}
          onMouseEnter={() => setOpenSubmenu('iconSize')}
          onFocus={() => setOpenSubmenu('iconSize')}
          onClick={() => setOpenSubmenu('iconSize')}
        >
          <span className="context-menu-item-label">Icon size</span>
          <span className="context-menu-arrow" aria-hidden="true">›</span>
        </button>
        <div className="context-menu-separator" />
        {showIconNamesOption && (
          <button
            className="context-menu-item"
            type="button"
            role="menuitemcheckbox"
            aria-checked={prefs.showIconNames}
            onMouseEnter={() => setOpenSubmenu(null)}
            onFocus={() => setOpenSubmenu(null)}
            onClick={() => { prefs.toggleShowIconNames(); onClose(); }}
          >
            <span className="context-menu-item-label">Show icon names</span>
            {prefs.showIconNames && <span className="context-menu-check material-symbols-outlined">check</span>}
          </button>
        )}
      </div>
      {openSubmenu === 'position' && (
        <div
          className="context-menu sidebar-context-submenu"
          role="menu"
          style={{ left: submenuLeft, top: submenuTop, width: SIDEBAR_SUBMENU_W }}
          onMouseEnter={() => setOpenSubmenu('position')}
        >
          {SB_MENU_POSITIONS.map((pos) => (
            <button
              key={pos}
              className="context-menu-item"
              type="button"
              role="menuitemradio"
              aria-checked={prefs.position === pos}
              onClick={() => { prefs.setPosition(pos); onClose(); }}
            >
              <span className="context-menu-icon material-symbols-outlined">{SB_POSITION_META[pos].icon}</span>
              <span className="context-menu-item-label">{SB_POSITION_META[pos].label}</span>
              {prefs.position === pos && <span className="context-menu-check material-symbols-outlined">check</span>}
            </button>
          ))}
        </div>
      )}
      {openSubmenu === 'iconSize' && (
        <div
          className="context-menu sidebar-context-submenu"
          role="menu"
          style={{ left: submenuLeft, top: submenuTop, width: SIDEBAR_SUBMENU_W }}
          onMouseEnter={() => setOpenSubmenu('iconSize')}
        >
          {SB_SIZE_OPTIONS.map(([key, label]) => (
            <button
              key={key}
              className="context-menu-item"
              type="button"
              role="menuitemradio"
              aria-checked={prefs.iconSizeKey === key}
              onClick={() => { prefs.setIconSizeKey(key); onClose(); }}
            >
              <span className="context-menu-icon material-symbols-outlined">format_size</span>
              <span className="context-menu-item-label">{label}</span>
              {prefs.iconSizeKey === key && <span className="context-menu-check material-symbols-outlined">check</span>}
            </button>
          ))}
        </div>
      )}
    </>
  );
}
