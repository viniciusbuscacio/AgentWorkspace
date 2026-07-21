import React from 'react';
import { cn } from '../../lib/utils';

interface CloseButtonProps extends Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, 'onClick'> {
  onClick: (e: React.MouseEvent) => void;
  className?: string;
}

export function CloseButton({ onClick, className = '', ...props }: CloseButtonProps) {
  return (
    <button
      {...props}
      onClick={(e) => {
        e.stopPropagation();
        onClick(e);
      }}
      className={cn(
        'flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded-[6px] border-0 bg-[var(--bg-tertiary)] p-0 text-[var(--text-primary)]',
        'opacity-0 pointer-events-none transition-[background-color,color,opacity] hover:bg-[var(--text-faint)] hover:text-[var(--text-primary)]',
        'focus-visible:pointer-events-auto focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50 group-hover:pointer-events-auto group-hover:opacity-100',
        className,
      )}
      aria-label={props['aria-label'] || 'Close'}
    >
      <span className="-translate-y-px text-[16px] leading-none">×</span>
    </button>
  );
}
