// @vitest-environment happy-dom
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import React from 'react';
import { PasswordInput } from './password-input';

describe('PasswordInput', () => {
  it('masks by default and reveals via the eye toggle', async () => {
    const user = userEvent.setup();
    render(<PasswordInput aria-label="Password" defaultValue="hunter2" />);

    const input = screen.getByLabelText('Password') as HTMLInputElement;
    expect(input.type).toBe('password');

    await user.click(screen.getByRole('button', { name: 'Show password' }));
    expect(input.type).toBe('text');
    expect(input.value).toBe('hunter2');
    // While revealed, ui.snapshot must keep masking: the reveal is for the
    // human's eyes only, never the agent's.
    expect(input.hasAttribute('data-sensitive')).toBe(true);

    await user.click(screen.getByRole('button', { name: 'Hide password' }));
    expect(input.type).toBe('password');
    expect(input.hasAttribute('data-sensitive')).toBe(false);
  });
});
