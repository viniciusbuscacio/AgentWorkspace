import type { ReactNode } from 'react';

interface PageHeaderProps {
  /** Big page title (h1). */
  title: ReactNode;
  /** Optional one-line description under the title. */
  subtitle?: ReactNode;
  /** Optional small node rendered above the title (e.g. a breadcrumb). */
  breadcrumb?: ReactNode;
  /** Optional leading node before the title (e.g. a back button). */
  leading?: ReactNode;
  /** Optional actions rendered on the right (buttons, toggles). */
  actions?: ReactNode;
}

/**
 * PageHeader is the single, fixed-position page title used by every module via
 * PageShell. Keeping the typography and spacing here guarantees that every
 * page's header lands at the exact same vertical position. Ported from AW2.
 */
export function PageHeader({ title, subtitle, breadcrumb, leading, actions }: PageHeaderProps) {
  return (
    <div className="flex shrink-0 items-start justify-between gap-4">
      <div className="flex min-w-0 items-center gap-3">
        {leading}
        <div className="flex min-w-0 flex-col">
          {breadcrumb && (
            <div className="mb-1 truncate text-xs font-medium text-muted-foreground">{breadcrumb}</div>
          )}
          <h1 className="m-0 truncate text-2xl font-semibold leading-tight text-foreground">{title}</h1>
          {subtitle && <p className="mt-1 truncate text-sm text-muted-foreground">{subtitle}</p>}
        </div>
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  );
}
