// @vitest-environment happy-dom
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { SAVE_CANCEL_BUTTON_CLASS, SaveCancelActions } from './SaveCancelActions';

describe('SaveCancelActions', () => {
  it('renders both Save and Cancel buttons', () => {
    render(<SaveCancelActions onSave={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.getByRole('button', { name: /save/i })).toBeTruthy();
    expect(screen.getByRole('button', { name: /cancel/i })).toBeTruthy();
  });

  it('calls onSave when Save is clicked', async () => {
    const onSave = vi.fn();
    const user = userEvent.setup();
    render(<SaveCancelActions onSave={onSave} onCancel={vi.fn()} />);
    await user.click(screen.getByRole('button', { name: /save/i }));
    expect(onSave).toHaveBeenCalledOnce();
  });

  it('calls onCancel when Cancel is clicked', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(<SaveCancelActions onSave={vi.fn()} onCancel={onCancel} />);
    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalledOnce();
  });

  it('disables Save and Cancel while saving', () => {
    render(<SaveCancelActions onSave={vi.fn()} onCancel={vi.fn()} saving />);
    expect((screen.getByRole('button', { name: /save/i }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole('button', { name: /cancel/i }) as HTMLButtonElement).disabled).toBe(true);
  });

  it('disables Save (but not Cancel) when disabled prop is set', () => {
    render(<SaveCancelActions onSave={vi.fn()} onCancel={vi.fn()} disabled />);
    expect((screen.getByRole('button', { name: /save/i }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole('button', { name: /cancel/i }) as HTMLButtonElement).disabled).toBe(false);
  });

  it('keeps the Save button visually stable while saving', () => {
    render(<SaveCancelActions onSave={vi.fn()} onCancel={vi.fn()} saving />);
    expect(screen.getByRole('button', { name: 'Save' })).toBeTruthy();
    expect(screen.getByText('Save')).toBeTruthy();
    expect(screen.queryByText('Saving…')).toBeNull();
  });

  it('renders Save and Cancel with the same fixed size class', () => {
    render(<SaveCancelActions onSave={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Save' }).classList.contains(SAVE_CANCEL_BUTTON_CLASS)).toBe(true);
    expect(screen.getByRole('button', { name: 'Cancel' }).classList.contains(SAVE_CANCEL_BUTTON_CLASS)).toBe(true);
  });

  it('always uses canonical Save and Cancel labels', () => {
    render(<SaveCancelActions onSave={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Save' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /apply|discard|save permissions/i })).toBeNull();
  });

  it('renders optional status node', () => {
    render(
      <SaveCancelActions onSave={vi.fn()} onCancel={vi.fn()} status={<span data-testid="status-msg">Unsaved changes</span>} />,
    );
    expect(screen.getByTestId('status-msg')).toBeTruthy();
  });
});
