// @vitest-environment happy-dom
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { APP_FONT_FAMILY_STORAGE_KEY, APP_FONT_SIZE_STORAGE_KEY } from '@/lib/app-font';
import { APP_ZOOM_EVENT } from '@/lib/app-zoom';
import { FontPage } from './FontPage';

vi.mock('@services/settings.service', () => ({
  settingsService: {
    getAppZoomPercent: () => Promise.resolve(100),
    setAppZoomPercent: () => Promise.resolve({ success: true }),
  },
}));

describe('FontPage', () => {
  afterEach(() => {
    localStorage.clear();
    document.documentElement.style.removeProperty('--font-family');
    document.documentElement.style.removeProperty('--font-size');
  });

  it('keeps font changes as draft until Save is clicked', async () => {
    const user = userEvent.setup();
    render(<FontPage />);

    await user.click(screen.getByRole('combobox', { name: 'Font family' }));
    await user.click(screen.getByRole('option', { name: 'JetBrains Mono' }));
    fireEvent.change(screen.getByRole('slider', { name: 'Font size' }), { target: { value: '18' } });

    const preview = screen.getByTestId('font-preview');
    expect(preview.style.fontSize).toBe('18px');
    expect(preview.style.fontFamily).toContain('monospace');
    expect(screen.getByText('Unsaved changes')).toBeTruthy();
    expect(localStorage.getItem(APP_FONT_FAMILY_STORAGE_KEY)).toBeNull();
    expect(localStorage.getItem(APP_FONT_SIZE_STORAGE_KEY)).toBeNull();
    expect(document.documentElement.style.getPropertyValue('--font-family')).toBe('');
    expect(document.documentElement.style.getPropertyValue('--font-size')).toBe('');

    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(localStorage.getItem(APP_FONT_FAMILY_STORAGE_KEY)).toBe('jetbrains-mono');
    expect(localStorage.getItem(APP_FONT_SIZE_STORAGE_KEY)).toBe('18');
    expect(document.documentElement.style.getPropertyValue('--font-family')).toContain('monospace');
    expect(document.documentElement.style.getPropertyValue('--font-size')).toBe('18px');
    expect(screen.queryByText('Unsaved changes')).toBeNull();
  });

  it('cancels draft font changes without saving them', async () => {
    const user = userEvent.setup();
    render(<FontPage />);

    const initialSize = (screen.getByRole('slider', { name: 'Font size' }) as HTMLInputElement).value;

    await user.click(screen.getByRole('combobox', { name: 'Font family' }));
    await user.click(screen.getByRole('option', { name: 'Lora' }));
    fireEvent.change(screen.getByRole('slider', { name: 'Font size' }), { target: { value: '20' } });
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    // ZoomSafeSelect trigger shows the selected label as text (no .value).
    expect(screen.getByRole('combobox', { name: 'Font family' }).textContent).toContain('System');
    expect((screen.getByRole('slider', { name: 'Font size' }) as HTMLInputElement).value).toBe(initialSize);
    expect(document.documentElement.style.getPropertyValue('--font-size')).toBe(`${initialSize}px`);
  });

  it('requests app zoom changes through the shell event, applied immediately', async () => {
    const user = userEvent.setup();
    const events: number[] = [];
    const onZoom = (event: Event) => events.push((event as CustomEvent<number>).detail);
    window.addEventListener(APP_ZOOM_EVENT, onZoom);
    try {
      render(<FontPage />);
      await waitFor(() => expect(screen.getByRole('button', { name: 'Reset zoom to 100%' }).textContent).toBe('100%'));

      await user.click(screen.getByRole('button', { name: 'Zoom in' }));
      expect(screen.getByRole('button', { name: 'Reset zoom to 100%' }).textContent).toBe('110%');

      await user.click(screen.getByRole('button', { name: 'Reset zoom to 100%' }));
      expect(events).toEqual([110, 100]);
    } finally {
      window.removeEventListener(APP_ZOOM_EVENT, onZoom);
    }
  });
});
