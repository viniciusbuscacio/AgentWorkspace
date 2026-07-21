import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PageShell } from './PageShell';

describe('PageShell', () => {
  it('renders the fixed header (title, subtitle, breadcrumb, actions) and content', () => {
    render(
      <PageShell
        title="LLM Providers"
        subtitle="Models, credentials and active provider."
        breadcrumb={<span>Settings</span>}
        actions={<button type="button">Save</button>}
      >
        <div>Body content</div>
      </PageShell>,
    );
    expect(screen.getByRole('heading', { level: 1, name: 'LLM Providers' })).toBeTruthy();
    expect(screen.getByText('Models, credentials and active provider.')).toBeTruthy();
    expect(screen.getByText('Settings')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Save' })).toBeTruthy();
    expect(screen.getByText('Body content')).toBeTruthy();
  });

  it('applies the fixed inset for the standard variant and drops it for fullscreen', () => {
    const { container, rerender } = render(
      <PageShell title="Standard"><div>x</div></PageShell>,
    );
    const standardFrame = container.firstElementChild as HTMLElement;
    expect(standardFrame.className).toContain('px-7');

    rerender(<PageShell title="Full" variant="fullscreen"><div>x</div></PageShell>);
    const fullscreenFrame = container.firstElementChild as HTMLElement;
    expect(fullscreenFrame.className).not.toContain('px-7');
  });
});
