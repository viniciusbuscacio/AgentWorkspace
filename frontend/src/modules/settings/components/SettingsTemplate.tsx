import type { ReactNode } from 'react';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@ui/card';
import { cn } from '@/lib/utils';

export function SettingsTemplate({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('grid gap-4 xl:grid-cols-[minmax(300px,0.9fr)_minmax(420px,1.1fr)]', className)}>
      {children}
    </div>
  );
}

export function SettingsPanel({
  icon,
  title,
  description,
  children,
  className,
  contentClassName,
}: {
  // A material icon name, or a ready-to-render node (e.g. a provider brand SVG).
  icon?: ReactNode;
  title: string;
  description?: string;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
}) {
  return (
    <Card className={cn('rounded-lg border-border/80 bg-card/95 shadow-sm', className)}>
      <CardHeader className="gap-1.5">
        <div className="flex items-start gap-3">
          {typeof icon === 'string' ? (
            <span className="material-symbols-outlined mt-[-1px] text-[22px] text-primary" aria-hidden="true">
              {icon}
            </span>
          ) : (
            icon
          )}
          <div className="min-w-0">
            <CardTitle className="text-base leading-6">{title}</CardTitle>
            {description && <CardDescription className="mt-1 leading-5">{description}</CardDescription>}
          </div>
        </div>
      </CardHeader>
      <CardContent className={cn('flex flex-col gap-3', contentClassName)}>{children}</CardContent>
    </Card>
  );
}

export function SettingsOptionButton({
  icon,
  title,
  description,
  meta,
  metaTone = 'default',
  active = false,
  onClick,
}: {
  icon: string;
  title: string;
  description?: string;
  meta?: string;
  metaTone?: 'default' | 'active';
  active?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'flex w-full items-center justify-between gap-3 rounded-lg border p-3 text-left transition-colors',
        active ? 'border-primary bg-primary/10 shadow-sm' : 'border-border bg-card hover:bg-accent',
      )}
      aria-pressed={active}
    >
      <span className="flex min-w-0 items-center gap-3">
        <span className="material-symbols-outlined text-[22px] text-primary" aria-hidden="true">{icon}</span>
        <span className="min-w-0">
          <span className="block truncate text-sm font-medium text-foreground">{title}</span>
          {description && <span className="block truncate text-xs text-muted-foreground">{description}</span>}
        </span>
      </span>
      {meta && (
        <span
          className={cn(
            'shrink-0 rounded-full border px-2 py-1 text-xs font-medium',
            metaTone === 'active'
              ? 'border-primary bg-primary text-primary-foreground shadow-sm'
              : 'border-border bg-background/50 text-muted-foreground',
          )}
        >
          {meta}
        </span>
      )}
    </button>
  );
}

export function SettingsNotice({
  children,
  tone = 'default',
}: {
  children: ReactNode;
  tone?: 'default' | 'warning' | 'error';
}) {
  return (
    <p
      className={cn(
        'rounded-md border p-3 text-sm',
        tone === 'error' && 'border-destructive/40 bg-destructive/10 text-destructive',
        tone === 'warning' && 'border-primary/35 bg-primary/10 text-foreground',
        tone === 'default' && 'border-border bg-muted text-muted-foreground',
      )}
    >
      {children}
    </p>
  );
}

export function SettingsActions({ children }: { children: ReactNode }) {
  return <div className="flex flex-wrap gap-2 pt-1">{children}</div>;
}
