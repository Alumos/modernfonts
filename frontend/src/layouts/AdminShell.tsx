import type { FormEvent } from "react"
import { useEffect, useState } from "react"
import { Activity, Database, FileText, House, KeyRound, LogOut, Menu, MonitorCheck, Settings, Users, X } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { api } from "@/lib/api"
import { FontsPage } from "@/pages/FontsPage"
import { RunsPage } from "@/pages/RunsPage"
import { SettingsPage } from "@/pages/SettingsPage"
import { SourcesPage } from "@/pages/SourcesPage"
import { UsersPage } from "@/pages/UsersPage"
import type { Account, Site } from "@/types"
import { NavButton } from "./NavButton"

const views = [
  { key: "sources", label: "文档源", icon: FileText },
  { key: "fonts", label: "字体库", icon: Database },
  { key: "runs", label: "解析日志", icon: Activity },
  { key: "users", label: "用户账号", icon: Users },
  { key: "settings", label: "站点设置", icon: Settings },
] as const

type View = (typeof views)[number]["key"]

function readView(): View {
  const route = window.location.hash.replace(/^#\/?/, "")
  return views.some((item) => item.key === route) ? route as View : "sources"
}

export function AdminShell({ account, site, refresh }: { account: Account; site?: Site; refresh: () => Promise<void> }) {
  const [view, setView] = useState<View>(readView)
  const [menuOpen, setMenuOpen] = useState(false)

  useEffect(() => {
    const sync = () => setView(readView())
    window.addEventListener("hashchange", sync)
    return () => window.removeEventListener("hashchange", sync)
  }, [])

  function navigate(next: View) {
    window.location.hash = `/${next}`
    setView(next)
    setMenuOpen(false)
  }

  async function logout() {
    await api("/api/auth/logout", { method: "POST" })
    await refresh()
    window.location.hash = "/"
  }

  const navigation = (
    <nav className="grid gap-1 p-3">
      <p className="px-2.5 pb-1 text-xs font-medium text-muted-foreground">解析中心</p>
      {views.map((item) => <NavButton active={view === item.key} icon={item.icon} key={item.key} label={item.label} onClick={() => navigate(item.key)} />)}
    </nav>
  )

  return (
    <div className="flex-1 bg-muted/30 text-foreground lg:grid lg:grid-cols-[15rem_minmax(0,1fr)]">
      <aside className="hidden border-r bg-background lg:block">
        <Brand site={site} />
        {navigation}
      </aside>
      {menuOpen ? <div className="fixed inset-0 z-40 bg-black/35 lg:hidden" onClick={() => setMenuOpen(false)} /> : null}
      <aside className={`fixed inset-y-0 left-0 z-50 w-72 border-r bg-background transition-transform lg:hidden ${menuOpen ? "translate-x-0" : "-translate-x-full"}`}>
        <div className="flex items-center justify-between border-b"><Brand site={site} /><Button aria-label="关闭导航" size="icon" variant="ghost" onClick={() => setMenuOpen(false)}><X /></Button></div>
        {navigation}
      </aside>
      <div className="min-w-0">
        <header className="sticky top-0 z-30 flex h-16 items-center gap-3 border-b bg-background/95 px-3 backdrop-blur sm:px-6">
          <Button aria-label="打开导航" className="lg:hidden" size="icon" variant="outline" onClick={() => setMenuOpen(true)}><Menu /></Button>
          <div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold">{views.find((item) => item.key === view)?.label}</p></div>
          <span className="hidden text-sm text-muted-foreground sm:inline">{account.username}</span>
          <Button asChild size="sm" variant="outline">
            <a aria-label="返回用户首页" href="#/" title="返回用户首页"><House /><span className="hidden sm:inline">用户首页</span></a>
          </Button>
          <Button size="sm" variant="outline" onClick={logout}><LogOut />退出</Button>
        </header>
        <main className="mx-auto w-full max-w-[1540px] p-3 sm:p-5 lg:p-7">
          {account.must_change_credentials ? <ForceChangeCredentials refresh={refresh} /> : (
            <>
              {view === "sources" ? <SourcesPage /> : null}
              {view === "fonts" ? <FontsPage /> : null}
              {view === "runs" ? <RunsPage /> : null}
              {view === "users" ? <UsersPage /> : null}
              {view === "settings" ? <SettingsPage refresh={refresh} /> : null}
            </>
          )}
        </main>
      </div>
    </div>
  )
}

function Brand({ site }: { site?: Site }) {
  return <div className="flex h-16 min-w-0 flex-1 items-center gap-3 px-4"><div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground">{site?.logo_path ? <img alt="" className="size-full rounded-md object-contain" src={site.logo_path} /> : <MonitorCheck className="size-5" />}</div><div className="min-w-0"><p className="truncate text-sm font-semibold">{site?.name || "腾讯文档字体解析"}</p>{site?.subtitle ? <p className="truncate text-xs text-muted-foreground">{site.subtitle}</p> : null}</div></div>
}

function ForceChangeCredentials({ refresh }: { refresh: () => Promise<void> }) {
  const [pending, setPending] = useState(false)
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    setPending(true)
    try {
      await api("/api/admin/auth/change-credentials", { method: "POST", body: JSON.stringify({ username: form.get("username"), password: form.get("password") }) })
      toast.success("账号和密码已更新")
      await refresh()
    } catch (err) { toast.error(err instanceof Error ? err.message : "修改失败") } finally { setPending(false) }
  }
  return <Dialog open><DialogContent onEscapeKeyDown={(event) => event.preventDefault()} onPointerDownOutside={(event) => event.preventDefault()}><DialogHeader><DialogTitle className="flex items-center gap-2"><KeyRound />首次登录必须修改账号和密码</DialogTitle><DialogDescription>完成修改前，系统会拒绝其它业务操作。</DialogDescription></DialogHeader><form className="grid gap-4" onSubmit={submit}><div className="grid gap-2"><Label htmlFor="new-username">新账号</Label><Input id="new-username" maxLength={80} minLength={3} name="username" required /></div><div className="grid gap-2"><Label htmlFor="new-password">新密码</Label><Input id="new-password" maxLength={72} minLength={6} name="password" required type="password" /></div><Button disabled={pending} type="submit">{pending ? "正在保存" : "保存并进入系统"}</Button></form></DialogContent></Dialog>
}
