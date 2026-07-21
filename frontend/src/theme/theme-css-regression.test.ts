import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

const globalsCss = readFileSync(join(process.cwd(), 'src/theme/globals.css'), 'utf8');
const awChatCss = readFileSync(join(process.cwd(), 'src/theme/aw-chat.css'), 'utf8');

describe('theme CSS regression guards', () => {
  it('keeps chat tokens tied to the active non-midnight app theme', () => {
    expect(globalsCss).toContain('[data-theme]:not([data-theme="midnight"])');
    expect(globalsCss).toContain('--chat-surface: var(--bg-primary);');
    expect(globalsCss).toContain('--chat-composer: var(--bg-input);');
    expect(globalsCss).toContain('--chat-user-bubble: var(--accent);');
    expect(globalsCss).toContain('--chat-assistant-bubble: var(--bg-elevated);');
    expect(globalsCss).toContain('--chat-code-surface: var(--bg-tertiary);');
  });

  it('keeps the message timeline spacing compact', () => {
    expect(awChatCss).toContain('.chat-timeline {');
    expect(awChatCss).toContain('gap: 2px;');
  });

  it('keeps message timestamps anchored to the chat right edge and readable in light theme', () => {
    expect(awChatCss).toContain('.message-row {');
    expect(awChatCss).toContain('display: flex;');
    expect(awChatCss).toContain('width: 100%;');
    expect(awChatCss).toContain('align-self: stretch;');
    expect(awChatCss).toContain('flex-direction: column;');
    expect(awChatCss).toContain('.message-row .bubble {');
    // 2026-06-12: the vanilla 860px bubble cap was intentionally dropped at
    // the owner's request — bubbles use the available width.
    expect(awChatCss).toContain('max-width: 95%;');
    expect(awChatCss).toContain('.message-time {');
    expect(awChatCss).toContain('align-self: flex-end;');
    expect(awChatCss).toContain('text-align: right;');
    expect(awChatCss).toContain('[data-theme="light"] .message-time');
    expect(awChatCss).toContain('color: var(--text-primary);');
    expect(awChatCss).toContain('opacity: 1;');
  });

  it('keeps sent user messages using the same base typography as assistant replies', () => {
    expect(awChatCss).toContain('.message-row .bubble {');
    expect(awChatCss).toContain('font-family: var(--font-family);');
    expect(awChatCss).toContain('font-size: var(--font-size);');
    expect(awChatCss).toContain('.user-content {');
    expect(awChatCss).toContain('font-family: inherit;');
    expect(awChatCss).toContain('font-size: inherit;');
    expect(awChatCss).toContain('line-height: inherit;');
  });
});
