import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SettingsModule } from './SettingsModule';

vi.mock('./pages/ProvidersPage', () => ({ ProvidersPage: () => <div>Providers page</div> }));
vi.mock('./pages/ThemePage', () => ({ ThemePage: () => <div>Theme page</div> }));
vi.mock('./pages/VaultPage', () => ({ VaultPage: () => <div>Vault page</div> }));
vi.mock('./pages/SecurityPage', () => ({ SecurityPage: () => <div>Security page</div> }));
vi.mock('./pages/AboutPage', () => ({ AboutPage: () => <div>About page</div> }));

describe('SettingsModule', () => {
  afterEach(() => {
    try {
      sessionStorage.clear();
    } catch {
      /* ignore */
    }
  });

  it('opens a settings page and returns to the grid', async () => {
    render(<SettingsModule onLocked={vi.fn()} />);
    await userEvent.click(screen.getByText('LLM Providers'));
    expect(screen.getByText('Providers page')).toBeTruthy();
    const detailHeading = screen.getByRole('heading', { name: /Settings.*LLM Providers/ });
    expect(detailHeading).toBeTruthy();
    expect(detailHeading.classList.contains('home-title')).toBe(true);
    expect(screen.queryByRole('button', { name: 'Back to settings' })).toBeNull();
    await userEvent.click(screen.getByRole('button', { name: 'Settings' }));
    expect(screen.getByText('Security')).toBeTruthy();
  });

  it('uses the Apps template with search and resizable cards', async () => {
    render(<SettingsModule onLocked={vi.fn()} />);

    expect(screen.getByRole('textbox', { name: 'Search settings' })).toBeTruthy();
    expect(screen.getByRole('slider', { name: 'Settings card size' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Set minimum card size' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Set maximum card size' })).toBeTruthy();

    await userEvent.type(screen.getByRole('textbox', { name: 'Search settings' }), 'vault');
    expect(screen.getByRole('button', { name: 'Vault' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Security' })).toBeNull();
  });

  it('surfaces the Agent Instructions card and search finds it', async () => {
    render(<SettingsModule onLocked={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Agent Instructions' })).toBeTruthy();
    await userEvent.type(screen.getByRole('textbox', { name: 'Search settings' }), 'instruction');
    expect(screen.getByRole('button', { name: 'Agent Instructions' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Skills' })).toBeNull();
  });

  it('restores the last sub-page from session storage on a generic open', async () => {
    sessionStorage.setItem('settings-last-page', 'security');
    render(<SettingsModule onLocked={vi.fn()} />);
    // Returns straight to the detail page, not the settings grid.
    expect(screen.getByText('Security page')).toBeTruthy();
    expect(screen.queryByRole('textbox', { name: 'Search settings' })).toBeNull();
  });

  it('an explicit deep-link page wins over the stored sub-page', async () => {
    sessionStorage.setItem('settings-last-page', 'security');
    render(<SettingsModule onLocked={vi.fn()} initialPage="providers" />);
    expect(screen.getByText('Providers page')).toBeTruthy();
    expect(screen.queryByText('Security page')).toBeNull();
  });

  it('clickable breadcrumb buttons carry the breadcrumb-link affordance class', async () => {
    render(<SettingsModule onLocked={vi.fn()} />);
    await userEvent.click(screen.getByText('LLM Providers'));
    const settingsBtn = screen.getByRole('button', { name: 'Settings' });
    expect(settingsBtn.classList.contains('breadcrumb-link')).toBe(true);
  });
});
