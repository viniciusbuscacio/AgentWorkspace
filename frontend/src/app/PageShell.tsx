import type { ReactNode } from 'react';
import { PageHeader } from '@/components/patterns/PageHeader';

type PageShellVariant = 'standard' | 'fullscreen';

interface PageShellProps {
  title: ReactNode;
  subtitle?: ReactNode;
  breadcrumb?: ReactNode;
  leading?: ReactNode;
  actions?: ReactNode;
  /**
   * 'standard' (default) applies the fixed outer inset shared by every page so
   * headers always line up. 'fullscreen' drops the inset for tool surfaces such
   * as Chat that manage their own internal layout.
   */
  variant?: PageShellVariant;
  /** Extra classes for the scrollable content region. */
  contentClassName?: string;
  children: ReactNode;
}

// Fixed inset shared by every standard page. This single source of truth is why
// the page header always lands at the same vertical position (ported from AW2's
// ModuleTemplateFrame: px-7 pt-4 pb-8).
const STANDARD_INSET = 'px-7 pt-5 pb-8';

/**
 * PageShell is the canonical page template. Every module renders through it so
 * the header position and content padding are identical across the app.
 */
export function PageShell({
  title,
  subtitle,
  breadcrumb,
  leading,
  actions,
  variant = 'standard',
  contentClassName = '',
  children,
}: PageShellProps) {
  const inset = variant === 'fullscreen' ? '' : STANDARD_INSET;
  return (
    <div className={`flex h-full min-h-0 min-w-0 flex-col ${inset}`}>
      <PageHeader title={title} subtitle={subtitle} breadcrumb={breadcrumb} leading={leading} actions={actions} />
      <div className={`mt-5 min-h-0 flex-1 overflow-y-auto overflow-x-hidden ${contentClassName}`}>
        {children}
      </div>
    </div>
  );
}
