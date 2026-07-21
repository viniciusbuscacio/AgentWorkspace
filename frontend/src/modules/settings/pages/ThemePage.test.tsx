import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ThemePage } from './ThemePage';

vi.mock('@services/events', () => ({ onAwEvent: vi.fn(() => vi.fn()) }));
vi.mock('@services/ui.service', () => ({ uiService: { reportUiState: vi.fn() } }));
vi.mock('@services/theme.service', () => ({
  themeService: {
    getActiveTheme: vi.fn(() => Promise.resolve('midnight')),
    getCustomThemes: vi.fn(() => Promise.resolve({
      'custom:alpha': { name: 'Alpha', background: '#111', surface: '#222', accent: '#f00', text: '#fff' },
      'custom:beta': { name: 'Beta', background: '#333', surface: '#444', accent: '#0f0', text: '#fff' },
    })),
    saveActiveTheme: vi.fn(() => Promise.resolve()),
  },
}));
vi.mock('@/lib/app-theme', () => ({
  getSavedAppTheme: vi.fn(() => 'midnight'),
  applyAppTheme: vi.fn(),
}));
vi.mock('@/theme/custom-theme', () => ({
  applyCustomThemeTokens: vi.fn(),
  clearCustomThemeProperties: vi.fn(),
  isCustomThemeId: (id: string) => String(id).startsWith('custom:'),
  normalizeCustomThemeTokens: (t: unknown) => t,
}));

beforeEach(() => {
  localStorage.clear();
  vi.clearAllMocks();
});

describe('ThemePage Apps-screen controls (Decision 3 — polish-wave-3-spec)', () => {
  it('renders the search bar, order-by select and card-size slider', async () => {
    render(<ThemePage />);

    // Search bar
    expect(screen.getByPlaceholderText('Search themes...')).toBeTruthy();

    // Order-by select
    const select = screen.getByRole('combobox', { name: /order by/i });
    expect(select).toBeTruthy();

    // Card-size range input
    const sizeSlider = screen.getByRole('slider', { name: /theme card size/i });
    expect(sizeSlider).toBeTruthy();
  });

  it('search bar filters built-in theme cards', async () => {
    const user = userEvent.setup();
    render(<ThemePage />);

    const searchInput = screen.getByPlaceholderText('Search themes...');

    // Before filtering: built-in themes visible (e.g. "Midnight")
    await waitFor(() => expect(screen.getByText('Midnight')).toBeTruthy());

    // Type a query that only matches one built-in theme
    await user.type(searchInput, 'ocean');

    expect(screen.queryByText('Midnight')).toBeNull();
    expect(screen.getByText('Ocean')).toBeTruthy();
  });

  it('card-size slider changes the grid column style', async () => {
    const user = userEvent.setup();
    const { container } = render(<ThemePage />);

    await waitFor(() => expect(screen.getByText('Midnight')).toBeTruthy());

    const slider = screen.getByRole('slider', { name: /theme card size/i }) as HTMLInputElement;
    const initialStyle = container.querySelector('[data-testid="theme-builtin-section"] .grid')?.getAttribute('style') ?? '';

    // Change slider value and fire the change event
    await user.pointer([
      { target: slider },
    ]);
    Object.defineProperty(slider, 'value', { writable: true, value: '80' });
    slider.dispatchEvent(new Event('input', { bubbles: true }));
    slider.dispatchEvent(new Event('change', { bubbles: true }));

    // The grid column template changes when iconSize changes
    await waitFor(() => {
      const updatedStyle = container.querySelector('[data-testid="theme-builtin-section"] .grid')?.getAttribute('style') ?? '';
      expect(updatedStyle).not.toBe(initialStyle);
    });
  });
});
