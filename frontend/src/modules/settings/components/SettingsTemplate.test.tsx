import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { SettingsOptionButton } from './SettingsTemplate';

describe('SettingsTemplate', () => {
  it('highlights the active status badge with theme colors', () => {
    render(
      <SettingsOptionButton
        icon="route"
        title="OpenRouter"
        description="deepseek/deepseek-r1"
        meta="Active"
        metaTone="active"
        active
        onClick={vi.fn()}
      />,
    );

    const badge = screen.getByText('Active');
    expect(badge.classList.contains('bg-primary')).toBe(true);
    expect(badge.classList.contains('text-primary-foreground')).toBe(true);
    expect(badge.classList.contains('border-primary')).toBe(true);
  });
});
