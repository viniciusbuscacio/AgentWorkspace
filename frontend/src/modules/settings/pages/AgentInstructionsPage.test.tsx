// @vitest-environment happy-dom
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentInstructionsPage } from './AgentInstructionsPage';

const list = vi.fn();
const read = vi.fn();
const save = vi.fn();
const reset = vi.fn();
const effective = vi.fn();
const sources = vi.fn();

vi.mock('@services/instructions.service', async (importOriginal) => {
  const original = await importOriginal<typeof import('@services/instructions.service')>();
  return {
    ...original,
    instructionsService: {
      list: (...a: unknown[]) => list(...a),
      read: (...a: unknown[]) => read(...a),
      save: (...a: unknown[]) => save(...a),
      reset: (...a: unknown[]) => reset(...a),
      effective: (...a: unknown[]) => effective(...a),
      sources: (...a: unknown[]) => sources(...a),
    },
  };
});

const agentsDoc = {
  id: 'AGENTS.md',
  title: 'AGENTS.md',
  description: 'Runtime/project rules for the agent.',
  origin: 'builtin',
  status: ['active', 'built-in'],
  editable: true,
  resettable: true,
  enabled: true,
};
const userDoc = {
  id: 'USER.md',
  title: 'USER.md',
  description: 'User preferences and stable behavior rules.',
  origin: 'user',
  status: ['empty'],
  editable: true,
  resettable: false,
  enabled: true,
};

beforeEach(() => {
  vi.clearAllMocks();
  list.mockResolvedValue({ success: true, documents: [agentsDoc, userDoc] });
  sources.mockResolvedValue({
    success: true,
    sources: [{ kind: 'builtin', id: 'AGENTS.md', active: true }],
  });
  read.mockResolvedValue({
    success: true,
    document: { id: 'AGENTS.md', content: 'SEED RULES', origin: 'builtin', editable: true, resettable: true, status: ['active'] },
  });
  effective.mockResolvedValue({
    success: true,
    generatedAt: '2026-06-24T00:00:00Z',
    effective: {
      content: 'COMPOSED BLOCK',
      sources: [{ id: 'AGENTS.md', origin: 'builtin', chars: 10 }],
      metadata: { charCount: 14, devOverrideActive: false },
      redacted: true,
    },
  });
  save.mockResolvedValue({ success: true, devOverrideActive: false });
  reset.mockResolvedValue({ success: true });
});

describe('AgentInstructionsPage', () => {
  it('renders the Effective Instructions entry and the two instruction files', async () => {
    render(<AgentInstructionsPage />);
    await screen.findByText('Effective Instructions');
    expect(screen.getByText('AGENTS.md')).toBeTruthy();
    expect(screen.getByText('USER.md')).toBeTruthy();
    // The simplified list has no search box.
    expect(screen.queryByRole('textbox')).toBeNull();
  });

  it('opens the editor and saves, returning to the list', async () => {
    const user = userEvent.setup();
    render(<AgentInstructionsPage />);
    await screen.findByText('Effective Instructions');

    // AGENTS.md is editable → the row shows an Edit button.
    const editButtons = screen.getAllByRole('button', { name: 'Edit' });
    await user.click(editButtons[0]);
    const textarea = (await screen.findByRole('textbox')) as HTMLTextAreaElement;
    expect(textarea.value).toBe('SEED RULES');

    await user.clear(textarea);
    await user.type(textarea, 'NEW RULES');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(save).toHaveBeenCalledWith('AGENTS.md', 'NEW RULES'));
  });

  it('warns before leaving a dirty editor', async () => {
    const user = userEvent.setup();
    render(<AgentInstructionsPage />);
    await screen.findByText('Effective Instructions');
    await user.click(screen.getAllByRole('button', { name: 'Edit' })[0]);
    const textarea = await screen.findByRole('textbox');
    await user.type(textarea, ' extra');
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.getByText('Discard unsaved changes?')).toBeTruthy();
  });

  it('requires confirmation to reset AGENTS.md', async () => {
    const user = userEvent.setup();
    render(<AgentInstructionsPage />);
    await screen.findByText('Effective Instructions');
    // The AGENTS.md row Reset button (resettable) opens a confirmation first.
    await user.click(screen.getByRole('button', { name: 'Reset' }));
    expect(screen.getByText(/Reset AGENTS.md to the default settings/)).toBeTruthy();
    // Confirmation is required: nothing is reset until the dialog is confirmed.
    expect(reset).not.toHaveBeenCalled();
  });

  it('shows the effective instructions view with source breakdown', async () => {
    const user = userEvent.setup();
    render(<AgentInstructionsPage />);
    await screen.findByText('Effective Instructions');
    await user.click(screen.getByRole('button', { name: 'View' }));
    await waitFor(() => expect(effective).toHaveBeenCalled());
    expect(await screen.findByText('COMPOSED BLOCK')).toBeTruthy();
    expect(screen.getByText(/14 chars/)).toBeTruthy();
  });
});
