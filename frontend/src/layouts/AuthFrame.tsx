import type { ReactNode } from "react"
import { MonitorCheck } from "lucide-react"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

export function AuthFrame({
  title,
  description,
  children,
}: {
  title: string
  description: string
  children: ReactNode
}) {
  return (
    <main className="flex-1 bg-muted/30 p-3 text-foreground sm:grid sm:place-items-center sm:p-6">
      <Card className="mx-auto w-full max-w-3xl">
        <CardHeader className="gap-3">
          <div className="flex size-11 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <MonitorCheck className="size-5" />
          </div>
          <div className="min-w-0 space-y-1">
            <CardTitle className="break-words text-xl sm:text-2xl">{title}</CardTitle>
            <CardDescription>{description}</CardDescription>
          </div>
        </CardHeader>
        <CardContent>{children}</CardContent>
      </Card>
    </main>
  )
}
