import { beforeEach, describe, expect, it, vi } from 'vitest';

const listChats = vi.fn();
const createChatWithTitle = vi.fn();
const streamChatMessage = vi.fn();
const openPipWindow = vi.fn();
const getDoc = vi.fn();

vi.mock('@services/chat.service', () => ({
  chatService: {
    listChats: (...a: unknown[]) => listChats(...a),
    createChatWithTitle: (...a: unknown[]) => createChatWithTitle(...a),
    streamChatMessage: (...a: unknown[]) => streamChatMessage(...a),
    openPipWindow: (...a: unknown[]) => openPipWindow(...a),
  },
}));
vi.mock('@services/user-memory.service', () => ({
  userMemoryService: { getDoc: (...a: unknown[]) => getDoc(...a) },
}));
vi.mock('@/lib/notify', () => ({ notify: vi.fn() }));

import { openModuleHelp, preferredLanguageFrom } from './module-help';

describe('preferredLanguageFrom', () => {
  it('reads the preferred-language line from the memory doc', () => {
    const doc = '- editor: Neovim\n- preferred-language: Português\n- os: macOS';
    expect(preferredLanguageFrom(doc)).toBe('Português');
  });

  it('tolerates case, spacing and separator variants', () => {
    expect(preferredLanguageFrom('Preferred Language = Spanish')).toBe('Spanish');
    expect(preferredLanguageFrom('preferred_language: French.')).toBe('French');
  });

  it('defaults to English when the line is absent or empty', () => {
    expect(preferredLanguageFrom('- editor: Neovim')).toBe('English');
    expect(preferredLanguageFrom('')).toBe('English');
    expect(preferredLanguageFrom('preferred-language:   ')).toBe('English');
  });
});

describe('openModuleHelp', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getDoc.mockResolvedValue({ success: true, doc: { content: 'preferred-language: Português' } });
    listChats.mockResolvedValue([]);
    createChatWithTitle.mockResolvedValue({ id: 'chat-new' });
    streamChatMessage.mockResolvedValue({ success: true });
    openPipWindow.mockResolvedValue({ success: true });
  });

  it('creates the help chat, sends the question and opens the PiP', async () => {
    await openModuleHelp('Skills');

    expect(createChatWithTitle).toHaveBeenCalledWith('Help — Skills');
    expect(streamChatMessage).toHaveBeenCalledWith(
      'chat-new',
      "What does the Skills module do? Explain in the user's preferred language: Português",
      [],
    );
    expect(openPipWindow).toHaveBeenCalledWith('chat-new');
  });

  it('reuses an active help chat without re-sending the question', async () => {
    listChats.mockResolvedValue([
      { id: 'chat-old', title: 'Help — Skills', archived: false },
    ]);

    await openModuleHelp('Skills');

    expect(openPipWindow).toHaveBeenCalledWith('chat-old');
    expect(createChatWithTitle).not.toHaveBeenCalled();
    expect(streamChatMessage).not.toHaveBeenCalled();
  });

  it('starts fresh when the previous help chat was archived', async () => {
    listChats.mockResolvedValue([
      { id: 'chat-old', title: 'Help — Skills', archived: true },
    ]);

    await openModuleHelp('Skills');

    expect(createChatWithTitle).toHaveBeenCalledWith('Help — Skills');
    expect(openPipWindow).toHaveBeenCalledWith('chat-new');
  });

  it('ignores other modules’ help chats', async () => {
    listChats.mockResolvedValue([
      { id: 'chat-notes', title: 'Help — Notes', archived: false },
    ]);

    await openModuleHelp('Skills');

    expect(createChatWithTitle).toHaveBeenCalledWith('Help — Skills');
  });
});
