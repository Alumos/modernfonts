import type { ReactNode } from "react"

import { cn } from "@/lib/utils"

export function PaginationBar({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <div
      className={cn("flex flex-wrap items-center justify-end gap-2 border-t px-4 py-2 text-sm text-muted-foreground sm:px-6", className)}
      data-testid="pagination-bar"
    >
      {children}
    </div>
  )
}
