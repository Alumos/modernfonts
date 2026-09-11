import { useCallback, useEffect, useMemo, useState } from "react"
import { ArrowRight, Download, ExternalLink, FileArchive, Loader2, LogOut, RefreshCw, Search, SearchX, SlidersHorizontal, Type, X } from "lucide-react"
import { toast } from "sonner"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { fontSeriesOptions, fontStyleOptions, fontWeightOptions, getFontProfile, matchesFontProfile, type FontFilters } from "@/lib/font-tags"
import { api, asArray } from "@/lib/api"
import { formatDateTime } from "@/lib/format"
import type { Account, ArchiveFile, LanzouDownloadFile, PublicFont, Site } from "@/types"

const pageSize = 36
const catalogLimit = 1000

export function HomePage({ site, account, refresh }: { site?: Site; account?: Account | null; refresh: () => Promise<void> }) {
  const [catalog, setCatalog] = useState<PublicFont[]>([])
  const [catalogTotal, setCatalogTotal] = useState(0)
  const [visibleCount, setVisibleCount] = useState(pageSize)
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(true)
  const [downloadFont, setDownloadFont] = useState<PublicFont | null>(null)
  const [downloadFiles, setDownloadFiles] = useState<LanzouDownloadFile[]>([])
  const [downloadLoading, setDownloadLoading] = useState(false)
  const [downloadError, setDownloadError] = useState("")
  const [archiveFiles, setArchiveFiles] = useState<ArchiveFile[]>([])
  const [archiveId, setArchiveId] = useState("")
  const [archiveLoading, setArchiveLoading] = useState(false)
  const [logoutPending, setLogoutPending] = useState(false)
  const [filters, setFilters] = useState<FontFilters>({ style: "all", series: "all", weight: "all" })

  const load = useCallback(async (searchQuery = "") => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ limit: String(catalogLimit), offset: "0" })
      if (searchQuery.trim()) params.set("q", searchQuery.trim())
      const result = await api<{ fonts: PublicFont[]; total: number }>(`/api/fonts?${params.toString()}`)
      setCatalog(asArray(result.fonts))
      setCatalogTotal(result.total)
      setVisibleCount(pageSize)
      return result.total
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载字体列表失败")
      return null
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    const timer = window.setTimeout(() => { void load(query) }, 250)
    return () => window.clearTimeout(timer)
  }, [load, query])

  async function refreshFonts() {
    const refreshedTotal = await load(query)
    if (refreshedTotal !== null) toast.success(`字体列表已刷新，共 ${refreshedTotal} 条记录`)
  }

  function setFilter<Key extends keyof FontFilters>(key: Key, value: FontFilters[Key]) {
    setFilters((current) => ({ ...current, [key]: value }))
    setVisibleCount(pageSize)
  }

  function clearFilters() {
    setFilters({ style: "all", series: "all", weight: "all" })
    setVisibleCount(pageSize)
  }

  async function resolveDownloads(font: PublicFont) {
    if (!account) {
      toast.info("请先登录后解析下载链接")
      return
    }
    setDownloadFont(font)
    setDownloadFiles([])
    setArchiveFiles([]); setArchiveId("")
    setDownloadError("")
    setDownloadLoading(true)
    try {
      const result = await api<{ files: LanzouDownloadFile[] }>(`/api/fonts/${font.id}/downloads`, { method: "POST" })
      setDownloadFiles(asArray(result.files))
    } catch (err) {
      setDownloadError(err instanceof Error ? err.message : "解析下载链接失败")
    } finally {
      setDownloadLoading(false)
    }
  }

  async function previewArchive(file: LanzouDownloadFile) {
    if (!file.url) return
    setArchiveLoading(true); setDownloadError("")
    try { const result = await api<{session_id:string; files:ArchiveFile[]}>("/api/archives", { method:"POST", body: JSON.stringify({url:file.url, name:file.name}) }); setArchiveId(result.session_id); setArchiveFiles(result.files) }
    catch (err) { setDownloadError(err instanceof Error ? err.message : "读取压缩包失败") } finally { setArchiveLoading(false) }
  }

  async function logout() {
    setLogoutPending(true)
    try {
      await api("/api/auth/logout", { method: "POST" })
      toast.success("已退出登录")
      await refresh()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "退出登录失败")
    } finally {
      setLogoutPending(false)
    }
  }

  const catalogProfiles = useMemo(() => catalog.map((font) => ({ font, profile: getFontProfile(font.font_name) })), [catalog])
  const filteredFonts = useMemo(
    () => catalogProfiles.filter(({ profile }) => matchesFontProfile(profile, filters)),
    [catalogProfiles, filters],
  )
  const fonts = filteredFonts.slice(0, visibleCount)
  const total = filteredFonts.length
  const activeFilterCount = Object.values(filters).filter((value) => value !== "all").length
  const hasMore = fonts.length < total
  const hasFilter = activeFilterCount > 0 || query.trim() !== ""
  const title = site?.name || "字体首页"
  const subtitle = total === 0
    ? hasFilter ? "换个关键词或筛选条件试试。" : "解析腾讯文档后，字体会显示在这里。"
    : hasFilter ? `筛选出 ${total} 条字体，已显示 ${fonts.length} 条` : `共 ${catalogTotal} 条字体记录，已显示 ${fonts.length} 条`

  return (
    <div className="flex-1 bg-muted/25">
      <header className="border-b bg-background">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4 px-4 py-3 sm:px-6 lg:px-8">
          <div className="flex min-w-0 items-center gap-3">
            <Avatar className="size-10 rounded-lg border bg-muted">
              {site?.logo_path ? <img alt="" className="size-full object-contain" src={site.logo_path} /> : <AvatarFallback className="rounded-lg bg-primary text-primary-foreground"><Type className="size-5" /></AvatarFallback>}
            </Avatar>
            <div className="min-w-0">
              <h1 className="truncate text-base font-semibold sm:text-lg">{title}</h1>
              {site?.subtitle ? <p className="truncate text-sm text-muted-foreground">{site.subtitle}</p> : null}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button aria-label="刷新字体列表" disabled={loading} size="icon" title="刷新字体列表" variant="outline" onClick={refreshFonts}><RefreshCw className={loading ? "size-4 animate-spin" : "size-4"} /></Button>
            {!account ? <Button asChild size="sm"><a href="#/login"><ArrowRight className="size-4" />登录</a></Button> : null}
            {account?.role === "admin" ? <Button asChild size="sm"><a href="#/sources"><ArrowRight className="size-4" />进入后台</a></Button> : null}
            {account && account.role !== "admin" ? <><Badge className="max-w-40 truncate px-3" title={account.username} variant="secondary">{account.username}</Badge><Button aria-label="退出登录" disabled={logoutPending} size="sm" title="退出登录" variant="outline" onClick={logout}>{logoutPending ? <Loader2 className="size-4 animate-spin" /> : <LogOut className="size-4" />}退出</Button></> : null}
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 sm:py-8 lg:px-8">
        <section className="grid gap-4 sm:gap-5">
          <div className="flex flex-wrap items-end justify-between gap-3">
            <div className="grid gap-1">
              <p className="text-sm font-medium text-primary">字体资料库</p>
              <h2 className="text-2xl font-semibold sm:text-3xl">浏览并获取字体</h2>
              <p className="text-sm text-muted-foreground">{subtitle}</p>
            </div>
            <Badge className="h-7 px-3 text-sm" variant="secondary">{total} 条记录</Badge>
          </div>
          <Card className="gap-0 overflow-hidden border-border/70 py-0 shadow-sm">
            <CardContent className="grid gap-3 p-3 sm:p-4">
              <div className="flex flex-col gap-2 sm:flex-row">
                <div className="relative min-w-0 flex-1">
                  <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input aria-label="搜索字体名称" className="h-10 bg-background pl-10 text-base" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索字体名称，如：黑、丸、iOS" />
                </div>
                {activeFilterCount > 0 ? <Button className="shrink-0" size="sm" variant="ghost" onClick={clearFilters}><X className="size-4" />清除筛选</Button> : null}
              </div>
              <div className="flex items-center gap-2 text-xs text-muted-foreground"><SlidersHorizontal className="size-3.5" />按字形、系列和字重筛选，点击卡片即可解析下载</div>
              <div className="grid gap-2 sm:grid-cols-3">
                <Select value={filters.style} onValueChange={(value) => setFilter("style", value as FontFilters["style"])}>
                  <SelectTrigger aria-label="按字形筛选" className="h-9 w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>{fontStyleOptions.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent>
                </Select>
                <Select value={filters.series} onValueChange={(value) => setFilter("series", value as FontFilters["series"])}>
                  <SelectTrigger aria-label="按系列筛选" className="h-9 w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>{fontSeriesOptions.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent>
                </Select>
                <Select value={filters.weight} onValueChange={(value) => setFilter("weight", value as FontFilters["weight"])}>
                  <SelectTrigger aria-label="按字重筛选" className="h-9 w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>{fontWeightOptions.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent>
                </Select>
              </div>
            </CardContent>
          </Card>
        </section>

        <section className="mt-6 grid gap-4 sm:mt-8 sm:gap-5">
          <div className="flex items-center justify-between gap-4"><h2 className="text-base font-semibold">字体列表</h2>{hasFilter ? <span className="text-sm text-muted-foreground">{total} 条匹配</span> : null}</div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4 lg:grid-cols-3 xl:grid-cols-4">
            {fonts.map(({ font, profile }) => <Card key={font.id} className="group h-full min-w-0 overflow-hidden border-border/70 py-0 transition-all hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-lg">
              <CardHeader className="flex-1 gap-4 px-4 py-4 sm:px-5 sm:py-5">
                <div className="flex items-center justify-between gap-3"><Badge className="font-normal" variant="secondary">字体</Badge><span className="text-xs text-muted-foreground">{formatDateTime(font.first_seen_at)}</span></div>
                <CardTitle className="min-h-10 break-words text-base leading-5">{font.font_name}</CardTitle>
                <div aria-label="字体标签" className="flex min-h-6 flex-wrap content-start gap-1.5">
                  {profile.tags.map((tag) => <Badge className="px-2 py-0.5 text-[11px] font-normal" key={`${font.id}-${tag}`} variant="outline">{tag}</Badge>)}
                </div>
              </CardHeader>
              <CardFooter className="mt-auto border-t bg-muted/20 px-4 py-3 sm:px-5"><Button className="w-full" disabled={downloadLoading} size="sm" onClick={() => void resolveDownloads(font)}>{downloadLoading && downloadFont?.id === font.id ? <Loader2 className="size-4 animate-spin" /> : <Download className="size-4" />}解析下载</Button></CardFooter>
            </Card>)}
            {!loading && fonts.length === 0 ? <Card className="col-span-full items-center gap-3 border-dashed py-12 text-center shadow-none"><SearchX className="size-8 text-muted-foreground" /><CardHeader className="gap-1 px-6 text-center"><CardTitle className="text-base">{hasFilter ? "没有匹配的字体" : "暂无字体记录"}</CardTitle><CardDescription>{hasFilter ? "调整搜索词或筛选条件再试试。" : "新的字体记录将在解析完成后显示。"}</CardDescription></CardHeader></Card> : null}
          </div>
        </section>
        {hasMore ? <div className="mt-6 flex justify-center"><Button variant="outline" onClick={() => setVisibleCount((current) => current + pageSize)}>加载更多</Button></div> : null}
      </main>

      <Dialog open={downloadFont !== null} onOpenChange={(open) => { if (!open) setDownloadFont(null) }}>
        <DialogContent className="max-h-[92vh] max-w-3xl overflow-hidden p-0">
          <DialogHeader className="border-b px-5 py-4 pr-12"><DialogTitle className="break-words text-base">{downloadFont?.font_name}</DialogTitle><DialogDescription>{downloadLoading ? "正在解析蓝奏云文件" : downloadFiles.length > 0 ? `共解析到 ${downloadFiles.length} 个文件` : "蓝奏云文件列表"}</DialogDescription></DialogHeader>
          <div className="max-h-[calc(92vh-5.5rem)] overflow-y-auto bg-muted/30 p-3 sm:p-5">
            {downloadLoading ? <div className="flex h-40 items-center justify-center"><Loader2 className="size-6 animate-spin text-muted-foreground" /></div> : null}
            {!downloadLoading && downloadError ? <div className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive">{downloadError}</div> : null}
            {!downloadLoading && !downloadError && downloadFiles.length === 0 ? <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">没有找到可下载文件</div> : null}
            <div className="grid gap-2" data-testid="download-file-list">
              {downloadFiles.map((file, index) => <div className="grid min-w-0 gap-3 rounded-md border bg-background p-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center" key={`${file.url || file.name}-${index}`}>
                <div className="flex min-w-0 items-start gap-3"><div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground"><FileArchive className="size-4" /></div><div className="min-w-0 flex-1"><div className="flex min-w-0 flex-wrap items-center gap-2"><div className="min-w-0 break-all text-sm font-medium">{file.name || `文件 ${index + 1}`}</div>{file.size ? <Badge className="shrink-0 font-normal" variant="secondary">{file.size}</Badge> : null}</div>{file.path ? <div className="mt-1 truncate text-xs text-muted-foreground">{file.path}</div> : null}{file.url ? <a className="mt-1 flex min-w-0 items-center gap-1 text-xs text-muted-foreground hover:text-foreground" href={file.url} rel="noreferrer" target="_blank" title={file.url}><span className="truncate">{file.url}</span><ExternalLink className="size-3 shrink-0" /></a> : null}{file.error ? <div className="mt-1 text-xs text-destructive">{file.error}</div> : null}</div></div>
                {file.url ? <div className="flex w-full gap-2 sm:w-auto"><Button asChild size="sm"><a href={file.url} rel="noreferrer" target="_blank"><Download className="size-4" />压缩包</a></Button>{/\.zip$/i.test(file.name) ? <Button size="sm" variant="outline" onClick={() => void previewArchive(file)}>浏览</Button> : null}</div> : <Button className="w-full sm:w-auto" disabled size="sm"><Download className="size-4" />不可下载</Button>}
              </div>)}
            </div>
            {archiveLoading ? <div className="mt-4 text-sm text-muted-foreground">正在读取压缩包目录…</div> : null}
            {archiveFiles.length > 0 ? <div className="mt-4 grid gap-2 rounded-md border bg-background p-3"><div className="text-sm font-medium">压缩包内容</div>{archiveFiles.map((entry) => <div className="flex items-center justify-between gap-3 text-sm" key={entry.path}><span className="min-w-0 truncate">{entry.path}</span><Button asChild size="sm" variant="outline"><a href={`/api/archives/${archiveId}/file?path=${encodeURIComponent(entry.path)}`}>下载</a></Button></div>)}</div> : null}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
