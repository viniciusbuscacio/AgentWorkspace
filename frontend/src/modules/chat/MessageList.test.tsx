import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { domain } from '@services/models';
import { MessageList } from './MessageList';
import type { SpawnBlock } from './subagent-tasks';

function assistantMessage(id: string, content: string): domain.Message {
  return new domain.Message({ id, sessionId: 'chat-1', role: 'assistant', content, createdAt: '2026-07-01T10:00:00Z' });
}

function spawnBlock(overrides: Partial<SpawnBlock> = {}): SpawnBlock {
  return {
    runId: 'run-1',
    messageId: 'm1',
    tasks: [{ id: 't1', task: 'hello-1', status: 'success', output: 'hello world' }],
    ...overrides,
  };
}

describe('MessageList spawn cards', () => {
  it('splits the message at the ::spawn marker: bubble -> card -> bubble, no timeline duplicate', () => {
    const message = assistantMessage('m1', 'Vou disparar os agentes:\n\n::spawn{run="run-1"}\n\nPronto, o resultado foi X.');
    const { container } = render(<MessageList messages={[message]} spawnBlocks={[spawnBlock()]} />);

    const cards = screen.getAllByText('1 subagent completed');
    expect(cards).toHaveLength(1);
    // The card sits at timeline level, between two separate bubbles.
    expect(cards[0].closest('.bubble')).toBeNull();
    const before = screen.getByText('Vou disparar os agentes:');
    const after = screen.getByText('Pronto, o resultado foi X.');
    expect(before.closest('.bubble')).not.toBeNull();
    expect(after.closest('.bubble')).not.toBeNull();
    expect(before.closest('.bubble')).not.toBe(after.closest('.bubble'));
    const text = container.textContent || '';
    expect(text.indexOf('Vou disparar os agentes:')).toBeLessThan(text.indexOf('1 subagent completed'));
    expect(text.indexOf('1 subagent completed')).toBeLessThan(text.indexOf('Pronto, o resultado foi X.'));
    // The marker line itself never renders as text.
    expect(screen.queryByText(/::spawn/)).toBeNull();
  });

  it('keeps the after-message anchoring for content without a marker', () => {
    const message = assistantMessage('m1', 'Resposta sem marcador.');
    render(<MessageList messages={[message]} spawnBlocks={[spawnBlock()]} />);

    const cards = screen.getAllByText('1 subagent completed');
    expect(cards).toHaveLength(1);
    expect(cards[0].closest('.bubble')).toBeNull();
  });

  it('does not trail a live block whose marker is already in the streaming text', () => {
    const streaming = assistantMessage('streaming-chat-1', 'Disparando:\n\n::spawn{run="run-1"}\n\n');
    render(<MessageList messages={[streaming]} spawnBlocks={[spawnBlock({ messageId: undefined, tasks: [{ id: 't1', task: 'hello-1', status: 'running' }] })]} />);

    expect(screen.getAllByText('Running 1 subagent... (0/1)')).toHaveLength(1);
  });

  it('renders the card from a persisted results block after reload (no in-memory blocks)', () => {
    const block = '::spawn{run="run-1"}\n{"tasks":[{"id":"t1","task":"subagent 1","status":"success","output":"subagent 1 ok"}]}\n::end-spawn';
    const message = assistantMessage('m1', `Vou disparar:\n\n${block}\n\nPronto.`);
    render(<MessageList messages={[message]} spawnBlocks={[]} />);

    const card = screen.getByText('1 subagent completed');
    expect(card.closest('.bubble')).toBeNull();
    expect(screen.getByText('Vou disparar:')).toBeTruthy();
    expect(screen.getByText('Pronto.')).toBeTruthy();
    expect(screen.queryByText(/::spawn|::end-spawn|"tasks"/)).toBeNull();
  });

  it('prefers the live block over the persisted results while both exist', () => {
    const block = '::spawn{run="run-1"}\n{"tasks":[{"id":"t1","task":"subagent 1","status":"running"}]}\n::end-spawn';
    const message = assistantMessage('m1', `Vou disparar:\n\n${block}\n\nPronto.`);
    render(<MessageList messages={[message]} spawnBlocks={[spawnBlock()]} />);

    // The live block says success -> completed label, not the persisted running.
    expect(screen.getByText('1 subagent completed')).toBeTruthy();
  });

  it('pins an unanchored live card BEFORE the forming reply bubble, not below it', () => {
    const done = assistantMessage('m1', 'qual a sua dificuldade?');
    // The "..." bubble: a pending assistant reply still being composed.
    const pending = Object.assign(assistantMessage('streaming-chat-1', ''), { pending: true });
    const { container } = render(
      <MessageList
        messages={[done, pending]}
        spawnBlocks={[spawnBlock({ messageId: undefined })]}
      />,
    );

    const text = container.textContent || '';
    expect(text.indexOf('qual a sua dificuldade?')).toBeLessThan(text.indexOf('1 subagent completed'));
    // The card sits before the pending bubble in the DOM.
    const card = screen.getByText('1 subagent completed');
    const pendingRow = container.querySelector('[data-message-id="streaming-chat-1"]');
    expect(pendingRow).not.toBeNull();
    expect(card.compareDocumentPosition(pendingRow!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('hides an orphan marker (no live block) and still splits the text into bubbles', () => {
    const message = assistantMessage('m1', 'Antes.\n\n::spawn{run="run-gone"}\n\nDepois.');
    render(<MessageList messages={[message]} spawnBlocks={[]} />);

    expect(screen.queryByText(/::spawn/)).toBeNull();
    expect(screen.getByText('Antes.')).toBeTruthy();
    expect(screen.getByText('Depois.')).toBeTruthy();
  });
});
