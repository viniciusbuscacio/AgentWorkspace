// @vitest-environment happy-dom
import { render, act, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import React from 'react';
import { toast } from 'sonner';
import { Toaster } from './sonner';

// Sonner mounts through a portal on document.body and keeps a module-level queue,
// so we scope every assertion to document.body and dismiss all toasts between
// tests to avoid cross-test bleed.
afterEach(() => {
  act(() => {
    toast.dismiss();
  });
});

describe('Toaster', () => {
  it('renders a toast into the document body', async () => {
    render(<Toaster />);
    act(() => {
      toast('Hello world');
    });
    await waitFor(() => {
      expect(document.body.textContent).toContain('Hello world');
    });
  });

  it('tags error toasts with data-type="error"', async () => {
    render(<Toaster />);
    act(() => {
      toast.error('Something failed');
    });
    await waitFor(() => {
      const el = document.body.querySelector('[data-type="error"]');
      expect(el).not.toBeNull();
      expect(el?.textContent).toContain('Something failed');
    });
  });

  it('exposes a close button using the material close glyph', async () => {
    render(<Toaster />);
    act(() => {
      toast('Closable');
    });
    await waitFor(() => {
      const close = document.body.querySelector('[data-close-button]') as HTMLElement | null;
      expect(close).not.toBeNull();
      // Uses the material-symbols close glyph, not Sonner's default SVG. Its
      // placement (top-right), hover-only reveal, and sizing live in
      // theme/aw-toast.css (CSS overrides, not testable via className here).
      expect(close?.querySelector('.material-symbols-outlined')?.textContent).toBe('close');
    });
  });

  it('points Sonner CSS vars at the repo theme tokens', async () => {
    render(<Toaster />);
    // Sonner spreads the `style` prop onto the list element (`[data-sonner-toaster]`),
    // which only mounts once there is a toast to show.
    act(() => {
      toast('Themed');
    });
    await waitFor(() => {
      const list = document.body.querySelector('[data-sonner-toaster]') as HTMLElement | null;
      expect(list).not.toBeNull();
      expect(list?.style.getPropertyValue('--normal-bg')).toBe('var(--bg-elevated)');
      expect(list?.style.getPropertyValue('--normal-text')).toBe('var(--text-primary)');
      expect(list?.style.getPropertyValue('--normal-border')).toBe('var(--border)');
    });
  });
});
