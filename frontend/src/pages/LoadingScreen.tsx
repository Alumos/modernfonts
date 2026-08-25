import { MonitorCheck } from "lucide-react"

export function LoadingScreen() {
  return (
    <main className="grid flex-1 place-items-center bg-background p-4 text-foreground">
      <div className="flex items-center gap-3 text-sm text-muted-foreground">
        <MonitorCheck className="size-5 animate-pulse" />
        正在加载系统状态
      </div>
    </main>
  )
}
