import { useEffect, useMemo, useState } from "react"
import { SiteFooter } from "@/components/common/SiteFooter"
import { Toaster } from "@/components/ui/sonner"
import { useBoot } from "@/hooks/useBoot"
import { AdminShell } from "@/layouts/AdminShell"
import { HomePage } from "@/pages/HomePage"
import { LoadingScreen } from "@/pages/LoadingScreen"
import { LoginPage } from "@/pages/LoginPage"

const adminViews = new Set(["sources", "fonts", "runs", "users", "settings"])
function readRoute() { const route = window.location.hash.replace(/^#\/?/, ""); return route === "login" || route === "auth" ? "login" : adminViews.has(route) ? "admin" : "home" }

export default function App() {
  const boot = useBoot()
  const site = useMemo(() => boot.status?.site, [boot.status])
  const isAdmin = boot.account?.role === "admin"
  const [route, setRoute] = useState(readRoute)
  useEffect(() => { const sync = () => setRoute(readRoute()); window.addEventListener("hashchange", sync); return () => window.removeEventListener("hashchange", sync) }, [])
  useEffect(() => { document.title = site?.name || "腾讯文档字体解析" }, [site])

  let page
  if (boot.loading || !boot.status) {
    page = <LoadingScreen />
  } else if (route === "login" || (route === "admin" && !boot.account)) {
    page = <LoginPage site={site} onDone={async (account) => { await boot.refresh(); window.location.hash = account.role === "admin" ? "/sources" : "/" }} />
  } else if (route === "admin" && isAdmin && boot.account) {
    page = <AdminShell account={boot.account} refresh={boot.refresh} site={site} />
  } else {
    page = <HomePage account={boot.account} refresh={boot.refresh} site={site} />
  }

  return (
    <>
      <div className="flex min-h-svh flex-col">
        <div className="flex flex-1 flex-col">{page}</div>
        <SiteFooter />
      </div>
      <Toaster richColors position="top-right" />
    </>
  )
}
