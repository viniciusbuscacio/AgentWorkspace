import * as React from "react"
import type { VariantProps } from "class-variance-authority"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"
import { buttonVariants, type ButtonSize, type ButtonVariant } from "./button-variants"

type ButtonProps = React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
    icon?: string
  }

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  icon,
  children,
  ...props
}: ButtonProps) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    >
      {icon && <span className="material-symbols-outlined text-[16px]" aria-hidden="true">{icon}</span>}
      {children}
    </Comp>
  )
}

export { Button }
export type { ButtonProps, ButtonSize, ButtonVariant }
