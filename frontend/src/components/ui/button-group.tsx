import * as React from 'react';
import { cn } from '@/lib/utils';
import { Button, type ButtonProps } from './button';

function ButtonGroup({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="button-group"
      role="group"
      className={cn('inline-flex items-center rounded-md shadow-xs [&>*:not(:first-child)]:-ml-px [&>*:not(:first-child)]:rounded-l-none [&>*:not(:last-child)]:rounded-r-none', className)}
      {...props}
    />
  );
}

function ButtonGroupButton({ className, variant = 'outline', ...props }: ButtonProps) {
  return <Button variant={variant} className={cn('relative focus:z-10', className)} {...props} />;
}

export { ButtonGroup, ButtonGroupButton };
