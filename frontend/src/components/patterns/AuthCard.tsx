import type { ReactNode } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@ui/card';

interface AuthCardProps {
  title: string;
  subtitle?: ReactNode;
  icon?: string;
  children?: ReactNode;
}

function currentAppZoomFactor(): number {
  if (typeof window === 'undefined') return 1;
  const root = document.documentElement;
  const raw = getComputedStyle(root).getPropertyValue('--aw-app-zoom') || root.style.getPropertyValue('--aw-app-zoom');
  const parsed = Number(raw.trim());
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1;
}

export function AuthCard({ title, subtitle, icon = 'database', children }: AuthCardProps) {
  const zoom = currentAppZoomFactor();

  return (
    <div
      className="flex items-center justify-center overflow-y-auto bg-background p-6 text-foreground"
      data-testid="auth-card-root"
      style={{
        width: `${100 / zoom}vw`,
        minHeight: `${100 / zoom}vh`,
      }}
    >
      <Card className="w-full max-w-[520px] shadow-2xl">
        <CardHeader className="text-center">
          <div className="mx-auto mb-3 flex h-11 w-11 items-center justify-center rounded-xl bg-muted text-primary">
            <span className="material-symbols-outlined text-[24px]" aria-hidden="true">{icon}</span>
          </div>
          <CardTitle className="text-xl leading-tight">{title}</CardTitle>
          {subtitle && (
            <CardDescription className="mx-auto mt-0 max-w-full [overflow-wrap:anywhere]">
              {subtitle}
            </CardDescription>
          )}
        </CardHeader>
        {children && <CardContent>{children}</CardContent>}
      </Card>
    </div>
  );
}
