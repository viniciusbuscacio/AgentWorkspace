import * as React from "react"

import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

function SearchInput({ className, type = "search", ...props }: React.ComponentProps<typeof Input>) {
  return (
    <Input
      type={type}
      data-slot="search-input"
      className={cn("rounded-full px-4", className)}
      {...props}
    />
  )
}

export { SearchInput }
