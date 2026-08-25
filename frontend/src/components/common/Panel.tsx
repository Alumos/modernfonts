import type { ReactNode } from "react"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { cn } from "@/lib/utils"

export function Panel({
  title,
  action,
  children,
  fullBleed = false,
}: {
  title: string
  action?: ReactNode
  children: ReactNode
  fullBleed?: boolean
}) {
  return (
    <Card className={cn("min-w-0", fullBleed && "pb-0")}>
      <CardHeader className="gap-3 sm:flex-row sm:items-center sm:justify-between">
        <CardTitle className="text-base">{title}</CardTitle>
        {action}
      </CardHeader>
      <CardContent className={fullBleed ? "min-w-0 p-0" : "grid min-w-0 gap-4"}>{children}</CardContent>
    </Card>
  )
}
