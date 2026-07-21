import * as React from "react"

import { Input } from "@/components/ui/input"

// Masked input with an eye toggle to reveal what was typed (vault passwords,
// API keys, tokens). Reveal is view-only local state; the value is unchanged.
// While revealed, data-sensitive keeps ui.snapshot masking the value so the
// reveal is for the human's eyes only — never the agent's.
function PasswordInput({ className, containerClassName, ...props }: React.ComponentProps<typeof Input> & { containerClassName?: string }) {
  const [revealed, setRevealed] = React.useState(false)

  return (
    <div className={`relative ${containerClassName ?? ""}`}>
      <Input
        type={revealed ? "text" : "password"}
        data-slot="password-input"
        data-sensitive={revealed ? true : undefined}
        className={`pr-10 ${className ?? ""}`}
        {...props}
      />
      <button
        type="button"
        tabIndex={-1}
        aria-label={revealed ? "Hide password" : "Show password"}
        title={revealed ? "Hide password" : "Show password"}
        onClick={() => setRevealed((value) => !value)}
        className="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-muted-foreground hover:text-foreground disabled:pointer-events-none"
        disabled={props.disabled}
      >
        <span className="material-symbols-outlined text-[18px]" aria-hidden="true">
          {revealed ? "visibility_off" : "visibility"}
        </span>
      </button>
    </div>
  )
}

export { PasswordInput }
