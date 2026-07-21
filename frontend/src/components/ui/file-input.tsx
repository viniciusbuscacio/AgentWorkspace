import * as React from "react"

import { Input } from "@/components/ui/input"

function FileInput(props: React.ComponentProps<typeof Input>) {
  return <Input type="file" data-slot="file-input" {...props} />
}

export { FileInput }
