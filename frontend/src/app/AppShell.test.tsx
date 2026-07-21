import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AppShell } from './AppShell';
import { sessionService } from '@services/session.service';

const listChats = vi.fn();
const createChat = vi.fn();
const deleteChat = vi.fn();
const getAppZoomPercent = vi.fn();
const setAppZoomPercent = vi.fn();
const listModules = vi.fn();
const addModule = vi.fn();
const removeModule = vi.fn();
const hideModule = vi.fn();
const showModule = vi.fn();
const moveModule = vi.fn();
const hideAllModules = vi.fn();

vi.mock('@services/chat.service', () => ({
  chatService: {
    listChats: (...args: unknown[]) => listChats(...args),
    createChat: (...args: unknown[]) => createChat(...args),
    deleteChat: (...args: unknown[]) => deleteChat(...args),
    setChatArchived: vi.fn(() => Promise.resolve({ success: true })),
  },
}));

vi.mock('@services/modules.service', () => ({
  modulesService: {
    list: (...args: unknown[]) => listModules(...args),
    add: (...args: unknown[]) => addModule(...args),
    remove: (...args: unknown[]) => removeModule(...args),
    hide: (...args: unknown[]) => hideModule(...args),
    hideAll: (...args: unknown[]) => hideAllModules(...args),
    show: (...args: unknown[]) => showModule(...args),
    move: (...args: unknown[]) => moveModule(...args),
  },
}));

vi.mock('@services/settings.service', () => ({
  settingsService: {
    getAppZoomPercent: (...args: unknown[]) => getAppZoomPercent(...args),
    setAppZoomPercent: (...args: unknown[]) => setAppZoomPercent(...args),
  },
}));

vi.mock('@services/events', () => ({
  onAwEvent: vi.fn(() => vi.fn()),
}));

// Stub out services whose Wails bindings do not exist in the test environment.
// Without these, ServiceCard and BrowserCardToggle fire rejected promises on
// every render; with them the cards start in the disabled-loading state, which
// is fine for AppShell-level tests that do not verify toggle behaviour.
vi.mock('@services/mcp.service', () => ({
  mcpService: { getStatus: vi.fn(() => Promise.resolve({ running: false })), start: vi.fn(), stop: vi.fn() },
}));
vi.mock('@services/rest.service', () => ({
  restService: { getStatus: vi.fn(() => Promise.resolve({ running: false })), start: vi.fn(), stop: vi.fn() },
}));
vi.mock('@services/webaccess.service', () => ({
  webaccessService: { getStatus: vi.fn(() => Promise.resolve({ running: false })), start: vi.fn(), stop: vi.fn() },
}));
vi.mock('@services/browser.service', () => ({
  browserService: { status: vi.fn(() => Promise.resolve({ success: true, status: { running: false } })), start: vi.fn(), stop: vi.fn() },
}));

vi.mock('@modules/settings/SettingsModule', () => ({
  SettingsModule: () => <div>Settings module</div>,
}));

vi.mock('@modules/chat/ChatModule', () => ({
  ChatModule: ({ chatId }: { chatId?: string }) => <div data-chat-id={chatId ?? ''}>Chat module</div>,
}));

vi.mock('@modules/notes/NotesModule', () => ({
  NotesModule: () => <div>Notes module</div>,
}));

vi.mock('@services/wallpaper.service', () => ({
  wallpaperService: {
    get: vi.fn(() => Promise.resolve('default')),
    getGlass: vi.fn(() => Promise.resolve(20)),
    set: vi.fn(() => Promise.resolve({ success: true })),
    setGlass: vi.fn(() => Promise.resolve({ success: true })),
    listCustom: vi.fn(() => Promise.resolve({ success: true, custom: [] })),
    pickAndUpload: vi.fn(() => Promise.resolve({ success: true, canceled: true, custom: [] })),
    getImage: vi.fn(() => Promise.resolve({ success: true, dataUri: '' })),
    deleteCustom: vi.fn(() => Promise.resolve({ success: true, custom: [] })),
  },
  DEFAULT_WALLPAPER_GLASS: 20,
}));

vi.mock('@services/session.service', () => ({
  sessionService: {
    getLastView: vi.fn(() => Promise.resolve('home')),
    getLastChatID: vi.fn(() => Promise.resolve('')),
    saveLastSession: vi.fn(() => Promise.resolve()),
  },
}));

// Exposed so individual tests can override the return value.

vi.mock('@services/ui.service', () => ({
  uiService: {
    reportUiState: vi.fn(() => Promise.resolve()),
  },
}));

const chatModule = { id: 'chat', name: 'Chat', icon: 'chat', description: 'Chat with the agent.', core: true, comingSoon: false, added: true, hidden: false, sidebarPosition: -1 };
const notesModule = { id: 'notes', name: 'Notes', icon: 'note', description: 'Personal notes.', core: false, comingSoon: false, added: true, hidden: false, sidebarPosition: 0 };

beforeEach(() => {
  localStorage.clear();
  listChats.mockResolvedValue([{ id: 'chat-1', title: 'Existing', archived: false }]);
  createChat.mockResolvedValue({ id: 'chat-2', title: 'New Chat', archived: false });
  deleteChat.mockClear();
  deleteChat.mockResolvedValue({ success: true });
  getAppZoomPercent.mockResolvedValue(100);
  setAppZoomPercent.mockResolvedValue({ success: true });
  listModules.mockResolvedValue({ success: true, modules: [chatModule] });
  addModule.mockResolvedValue({ success: true, modules: [chatModule] });
  removeModule.mockResolvedValue({ success: true, modules: [chatModule] });
  hideModule.mockResolvedValue({ success: true, modules: [chatModule] });
  hideAllModules.mockResolvedValue({ success: true, modules: [chatModule] });
  showModule.mockResolvedValue({ success: true, modules: [chatModule] });
  moveModule.mockResolvedValue({ success: true, modules: [chatModule] });
});

describe('AppShell', () => {
  it('sidebar close button hides the module without removing it from the workspace', async () => {
    listModules.mockResolvedValue({ success: true, modules: [chatModule, notesModule] });
    // Close = hide: the module stays added (actions untouched), only the
    // sidebar item goes away. Remove is a different, context-menu-only path.
    hideModule.mockResolvedValue({ success: true, modules: [chatModule, { ...notesModule, hidden: true }] });
    render(<AppShell onLocked={vi.fn()} />);

    // "Notes" renders on the Home grid card AND in the sidebar block.
    const labels = await screen.findAllByText('Notes');
    expect(labels.some((el) => el.closest('.module-nav-item'))).toBe(true);

    await userEvent.click(screen.getByRole('button', { name: 'Close Notes' }));

    expect(hideModule).toHaveBeenCalledWith('notes');
    expect(removeModule).not.toHaveBeenCalled();
    // Gone from the sidebar, still on the Home grid (added, just hidden).
    await waitFor(() => {
      const remaining = screen.queryAllByText('Notes');
      expect(remaining.some((el) => el.closest('.module-nav-item'))).toBe(false);
      expect(remaining.length).toBeGreaterThan(0);
    });
  });

  it('reopens a hidden module when its view is opened from the Apps grid', async () => {
    listModules.mockResolvedValue({ success: true, modules: [chatModule, { ...notesModule, hidden: true }] });
    showModule.mockResolvedValue({ success: true, modules: [chatModule, notesModule] });
    render(<AppShell onLocked={vi.fn()} />);

    // Hidden module: no sidebar item, but the Apps card is there.
    const cards = await screen.findAllByText('Notes');
    expect(cards.some((el) => el.closest('.module-nav-item'))).toBe(false);
    await userEvent.click(cards[0]!.closest('button') ?? cards[0]!);

    await waitFor(() => expect(showModule).toHaveBeenCalledWith('notes'));
    await waitFor(() => {
      const labels = screen.queryAllByText('Notes');
      expect(labels.some((el) => el.closest('.module-nav-item'))).toBe(true);
    });
  });

  it('navigates between home, settings and chat', async () => {
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));
    await userEvent.click(screen.getByRole('button', { name: 'Settings' }));
    expect(screen.getByText('Settings module')).toBeTruthy();
    const existingChat = screen.getAllByText('Existing')[0]?.closest('.chat-nav-item');
    expect(existingChat).toBeTruthy();
    await userEvent.click(existingChat!);
    expect(screen.getByText('Chat module')).toBeTruthy();
  });

  it('creates a new chat when opening Chat from the Open Module screen', async () => {
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));

    await userEvent.click(screen.getByRole('button', { name: 'Chat' }));

    await waitFor(() => expect(createChat).toHaveBeenCalledTimes(1));
    expect(screen.getByText('Chat module')).toBeTruthy();
  });

  it('sidebar hide button lives in the bottom status bar, not the top', async () => {
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));

    expect(document.querySelector('.collapse-button')).toBeNull();
    const hide = document.querySelector('[data-awid="sidebar-hide"]');
    expect(hide).toBeTruthy();
    expect(hide?.closest('[data-awid="sidebar-status"]')).toBeTruthy();
    // Row order: hide, lock, archived toggle (server dot only while listening).
    const ids = Array.from(document.querySelectorAll('[data-awid="sidebar-status"] [data-awid]'))
      .map((el) => el.getAttribute('data-awid'));
    expect(ids).toEqual(['sidebar-hide', 'lock-now', 'archived-toggle']);

    // Instant tooltip: appears on hover, goes away on leave.
    const lock = document.querySelector('[data-awid="lock-now"]')!;
    fireEvent.mouseEnter(lock);
    expect(screen.getByRole('tooltip').textContent).toBe('Lock');
    fireEvent.mouseLeave(lock);
    expect(screen.queryByRole('tooltip')).toBeNull();
  });

  it('icon mode shows an instant tooltip with the chat title on hover', async () => {
    localStorage.setItem('aw-sidebar-width', '84');
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));

    const item = screen.getAllByText('Existing')[0]?.closest('.chat-nav-item');
    expect(item).toBeTruthy();
    fireEvent.mouseEnter(item!);
    expect(screen.getByRole('tooltip').textContent).toBe('Existing');
    fireEvent.mouseLeave(item!);
    expect(screen.queryByRole('tooltip')).toBeNull();
  });

  it('the archived-chat trash asks for confirmation before deleting — delete is irreversible', async () => {
    listChats.mockResolvedValue([
      { id: 'chat-1', title: 'Existing', archived: false },
      { id: 'archived-1', title: 'Old one', archived: true },
    ]);
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));

    await userEvent.click(screen.getByRole('button', { name: 'Archived Chats' }));
    await userEvent.click(screen.getByRole('button', { name: 'Delete Old one' }));

    // The modal is up and nothing was deleted yet.
    expect(screen.getByText(/cannot be undone/i)).toBeTruthy();
    expect(deleteChat).not.toHaveBeenCalled();

    // Cancel keeps the chat.
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(deleteChat).not.toHaveBeenCalled();

    // Confirming deletes it.
    await userEvent.click(screen.getByRole('button', { name: 'Delete Old one' }));
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }));
    await waitFor(() => expect(deleteChat).toHaveBeenCalledWith('archived-1'));
  });

  it('closing (archiving) the displayed chat clears the pane and lands on the desktop', async () => {
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));

    // Open the chat, then close it from the sidebar X (archive).
    const chatItem = screen.getAllByText('Existing')[0]?.closest('.chat-nav-item');
    await userEvent.click(chatItem!);
    expect(screen.getByText('Chat module')).toBeTruthy();

    await userEvent.click(screen.getByRole('button', { name: 'Archive Existing' }));
    await waitFor(() => expect(screen.queryByText('Chat module')).toBeNull());
  });

  it('an archived lastChatID never restores — closed chats stay closed across restarts', async () => {
    listChats.mockResolvedValue([
      { id: 'archived-1', title: 'Closed One', archived: true },
      { id: 'chat-1', title: 'Existing', archived: false },
    ]);
    vi.mocked(sessionService.getLastChatID).mockResolvedValue('archived-1');
    vi.mocked(sessionService.getLastView).mockResolvedValue('chat');
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));

    // Whatever ends up displayed, it is NEVER the archived chat.
    await waitFor(() => {
      const pane = document.querySelector('[data-chat-id]');
      if (pane) expect(pane.getAttribute('data-chat-id')).not.toBe('archived-1');
    });
  });

  it('the chat view never restores when every chat is archived — desktop instead', async () => {
    listChats.mockResolvedValue([
      { id: 'archived-1', title: 'Closed One', archived: true },
    ]);
    vi.mocked(sessionService.getLastChatID).mockResolvedValue('archived-1');
    vi.mocked(sessionService.getLastView).mockResolvedValue('chat');
    render(<AppShell onLocked={vi.fn()} />);

    // Home (the desktop) renders; the empty chat pane never does.
    await waitFor(() => expect(screen.getByText('Open Apps and Modules')).toBeTruthy());
    expect(screen.queryByText('Chat module')).toBeNull();
  });

  it('selecting a different chat sticks — boot restore does not yank it back', async () => {
    listChats.mockResolvedValue([
      { id: 'chat-1', title: 'First', archived: false },
      { id: 'chat-2', title: 'Second', archived: false },
    ]);
    // The persisted session points at chat-1: selecting chat-2 must NOT bounce
    // back to chat-1 (the boot-restore effect must run once, not per switch).
    vi.mocked(sessionService.getLastChatID).mockResolvedValue('chat-1');
    vi.mocked(sessionService.getLastView).mockResolvedValue('chat');
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Second').length).toBeGreaterThan(0));

    await userEvent.click(screen.getAllByText('Second')[0]!.closest('.chat-nav-item')!);
    await waitFor(() => expect(document.querySelector('[data-chat-id]')?.getAttribute('data-chat-id')).toBe('chat-2'));
    // Give any stray re-run a chance to misfire, then confirm it stuck.
    await new Promise((r) => setTimeout(r, 50));
    expect(document.querySelector('[data-chat-id]')?.getAttribute('data-chat-id')).toBe('chat-2');
  });

  it('opening a second chat shows the new chat, never the first one', async () => {
    listChats.mockResolvedValue([
      { id: 'chat-1', title: 'First', archived: false },
      { id: 'chat-2', title: 'Second', archived: false },
    ]);
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('First').length).toBeGreaterThan(0));

    await userEvent.click(screen.getAllByText('First')[0]!.closest('.chat-nav-item')!);
    await waitFor(() => expect(document.querySelector('[data-chat-id]')?.getAttribute('data-chat-id')).toBe('chat-1'));

    await userEvent.click(screen.getAllByText('Second')[0]!.closest('.chat-nav-item')!);
    await waitFor(() => expect(document.querySelector('[data-chat-id]')?.getAttribute('data-chat-id')).toBe('chat-2'));
  });

  // chat-identity-spec Decision 4: a stale lastChatID (deleted chat) must
  // fall back gracefully — never crash, never resurrect a ghost selection.
  it('survives a stale lastChatID from session restore', async () => {
    vi.mocked(sessionService.getLastChatID).mockResolvedValue('ghost-chat-deleted-long-ago');
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));

    // The real chat opens normally; the ghost id never became the selection.
    const existingChat = screen.getAllByText('Existing')[0]?.closest('.chat-nav-item');
    expect(existingChat).toBeTruthy();
    await userEvent.click(existingChat!);
    expect(screen.getByText('Chat module')).toBeTruthy();
  });

  it('keeps the AW2-style sidebar context menu but exposes only left/right positions for now', async () => {
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));

    const sidebar = document.querySelector('aside.sidebar');
    expect(sidebar).toBeTruthy();
    fireEvent.contextMenu(sidebar!, { clientX: 80, clientY: 260 });

    expect(screen.getByText('Sidebar position')).toBeTruthy();
    expect(screen.getByText('Icon size')).toBeTruthy();
    expect(screen.queryByText('Show icon names')).toBeNull();
    expect(screen.queryByText('Auto-collapse to icons')).toBeNull();
    await userEvent.click(screen.getByText('Sidebar position'));

    expect(screen.getByText('Left')).toBeTruthy();
    expect(screen.getByText('Right')).toBeTruthy();
    expect(screen.queryByText('Top')).toBeNull();
    expect(screen.queryByText('Bottom')).toBeNull();
  });

  it('shows the icon-name toggle only when the sidebar is collapsed to icons', async () => {
    localStorage.setItem('aw-sidebar-width', '84');
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));

    const sidebar = document.querySelector('aside.sidebar');
    expect(sidebar).toBeTruthy();
    fireEvent.contextMenu(sidebar!, { clientX: 80, clientY: 260 });

    expect(screen.getByText('Show icon names')).toBeTruthy();
  });

  // Decision 2 (polish-wave-3-spec): the Show-desktop sidebar button was removed
  // at user request. HideAllModules plumbing and the agent action are kept.
  it('Show desktop sidebar button is absent (removed per user veto)', async () => {
    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));
    expect(screen.queryByRole('button', { name: /show desktop/i })).toBeNull();
  });

  it('hideAllModules plumbing still works when called programmatically', async () => {
    listModules.mockResolvedValue({ success: true, modules: [chatModule, notesModule] });
    hideAllModules.mockResolvedValue({ success: true, modules: [chatModule, { ...notesModule, hidden: true }] });

    render(<AppShell onLocked={vi.fn()} />);
    await waitFor(() => expect(screen.getAllByText('Notes').length).toBeGreaterThan(0));

    // No Show-desktop button in the sidebar.
    expect(screen.queryByRole('button', { name: /show desktop/i })).toBeNull();
  });

  // Regression guard for polish-wave-3-spec Decision 4: clicking a sidebar chat
  // from any module view must navigate to that chat.
  it('navigates from a module view to chat when a ChatNavItem is clicked', async () => {
    const wallpaperModule = {
      id: 'wallpaper', name: 'Wallpaper', icon: 'wallpaper', description: 'Wallpaper.',
      core: false, comingSoon: false, added: true, hidden: false, sidebarPosition: 1,
    };
    listModules.mockResolvedValue({ success: true, modules: [chatModule, wallpaperModule] });
    vi.mocked(sessionService.getLastView).mockResolvedValue('wallpaper');

    render(<AppShell onLocked={vi.fn()} />);

    // Wait for session restore to land on the wallpaper view.
    await waitFor(() => expect(screen.queryByText('Chat module')).toBeNull());

    // Click the sidebar chat item — must navigate to chat.
    const chatNavItem = (await screen.findAllByText('Existing'))[0]?.closest('.chat-nav-item');
    expect(chatNavItem).toBeTruthy();
    await userEvent.click(chatNavItem!);

    expect(screen.getByText('Chat module')).toBeTruthy();
  });

  // Regression guard for the session-restore race condition:
  // if the user navigates while modules are still loading, the deferred restore
  // must NOT override that navigation.
  it('session-restore does not override navigation that happened before modules loaded', async () => {
    vi.mocked(sessionService.getLastView).mockResolvedValue('wallpaper');

    // Hold modules list until we explicitly resolve it.
    let resolveModules!: (v: { success: boolean; modules: typeof chatModule[] }) => void;
    listModules.mockReturnValue(
      new Promise<{ success: boolean; modules: typeof chatModule[] }>((res) => { resolveModules = res; }),
    );

    render(<AppShell onLocked={vi.fn()} />);

    // App is on 'home' (modules not yet loaded). Session-restore view is set
    // but pending. Chat list is already populated from refreshChats().
    await waitFor(() => expect(screen.getAllByText('Existing').length).toBeGreaterThan(0));

    // User navigates to chat before modules finish loading.
    const chatNavItem = screen.getAllByText('Existing')[0]?.closest('.chat-nav-item');
    expect(chatNavItem).toBeTruthy();
    await userEvent.click(chatNavItem!);
    expect(screen.getByText('Chat module')).toBeTruthy();

    // Now modules load — the session restore must not redirect back to wallpaper.
    resolveModules({ success: true, modules: [chatModule] });
    await waitFor(() => expect(listModules).toHaveBeenCalled());

    // Still on chat.
    expect(screen.getByText('Chat module')).toBeTruthy();
  });
});
