import { useCallback, useEffect, useMemo, useState } from "react"
import { Database, RefreshCw, Search, Trash2 } from "lucide-react"
import { toast } from "sonner"

import { DataTable } from "@/components/common/DataTable"
import { Field } from "@/components/common/Field"
import { PaginationBar } from "@/components/common/PaginationBar"
import { Panel } from "@/components/common/Panel"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { TableCell, TableRow } from "@/components/ui/table"
import { api, asArray } from "@/lib/api"
import { formatDateTime } from "@/lib/format"
import type { DocumentSource, FontItem } from "@/types"

const pageSize = 20

export function FontsPage() {
  const [sources, setSources] = useState<DocumentSource[]>([])
  const [fonts, setFonts] = useState<FontItem[]>([])
  const [total, setTotal] = useState(0)
  const [sourceId, setSourceId] = useState("all")
  const [query, setQuery] = useState("")
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [clearing, setClearing] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ limit: String(pageSize), offset: String((page - 1) * pageSize) })
      if (sourceId !== "all") params.set("source_id", sourceId)
      if (query.trim()) params.set("q", query.trim())
      const [sourceRes, fontRes] = await Promise.all([
        api<{ sources: DocumentSource[] }>("/api/admin/sources"),
        api<{ fonts: FontItem[]; total: number }>(`/api/admin/fonts?${params.toString()}`),
      ])
      setSources(asArray(sourceRes.sources))
      setFonts(asArray(fontRes.fonts))
      setTotal(fontRes.total)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载字体失败")
    } finally {
      setLoading(false)
    }
  }, [page, query, sourceId])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    setPage(1)
  }, [query, sourceId])

  async function clearAllFonts() {
    if (!window.confirm("确定要删除所有字体数据吗？")) return
    setClearing(true)
    try {
      await api("/api/admin/fonts", { method: "DELETE" })
      toast.success("字体数据已清空")
      setPage(1)
      await load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "清空失败")
    } finally {
      setClearing(false)
    }
  }

  const sourceMap = useMemo(() => new Map(sources.map((source) => [source.id, source.title || source.url])), [sources])
  const pageCount = Math.max(1, Math.ceil(total / pageSize))
  const currentPage = Math.min(page, pageCount)

  return (
    <div className="grid gap-5">
      <div className="flex min-h-24 items-center gap-4 rounded-lg border bg-card px-5 py-4">
        <div className="grid size-10 shrink-0 place-items-center rounded-md bg-muted text-muted-foreground">
          <Database className="size-5" aria-hidden="true" />
        </div>
        <div>
          <p className="text-sm text-muted-foreground">字体数量</p>
          <p className="text-2xl font-semibold tabular-nums">{total}</p>
        </div>
      </div>

      <Panel title="筛选">
        <div className="grid gap-4 lg:grid-cols-[1fr_18rem]">
          <Field label="搜索">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input className="pl-9" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="字体名或下载链接" />
            </div>
          </Field>
          <Field label="文档源">
            <Select value={sourceId} onValueChange={setSourceId}>
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部文档源</SelectItem>
                {sources.map((source) => <SelectItem key={source.id} value={String(source.id)}>{source.title || `来源 ${source.id}`}</SelectItem>)}
              </SelectContent>
            </Select>
          </Field>
        </div>
      </Panel>

      <Panel
        fullBleed
        title={`字体记录（${total}）`}
        action={<Button disabled={clearing} size="sm" variant="destructive" onClick={clearAllFonts}><Trash2 className="size-4" />删除所有字体数据</Button>}
      >
        <DataTable headers={["字体名称", "下载链接", "文档访问码", "来源", "最近发现"]}>
          {fonts.map((font) => (
            <TableRow key={font.id}>
              <TableCell className="min-w-48 font-medium">{font.font_name}</TableCell>
              <TableCell className="max-w-[28rem]">
                <a className="block truncate text-primary underline-offset-4 hover:underline" href={font.download_url} rel="noreferrer" target="_blank" title={font.download_url}>{font.download_url}</a>
              </TableCell>
              <TableCell title="腾讯文档随下载链接提取的 dkey，用于记录该链接的访问信息">{font.access_code ? <Badge variant="secondary">{font.access_code}</Badge> : "-"}</TableCell>
              <TableCell>{sourceMap.get(font.source_id) || font.source_id}</TableCell>
              <TableCell>{formatDateTime(font.last_seen_at || font.first_seen_at)}</TableCell>
            </TableRow>
          ))}
          {!loading && fonts.length === 0 ? <TableRow><TableCell className="h-24 text-center text-muted-foreground" colSpan={5}>暂无字体记录</TableCell></TableRow> : null}
          {loading ? <TableRow><TableCell className="h-24 text-center text-muted-foreground" colSpan={5}><RefreshCw className="mr-2 inline-block size-4 animate-spin" />正在加载</TableCell></TableRow> : null}
        </DataTable>
        <PaginationBar>
          <span>第 {currentPage} / {pageCount} 页</span>
          <Button size="sm" variant="outline" disabled={currentPage <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))}>上一页</Button>
          <Button size="sm" variant="outline" disabled={currentPage >= pageCount} onClick={() => setPage((value) => Math.min(pageCount, value + 1))}>下一页</Button>
        </PaginationBar>
      </Panel>
    </div>
  )
}
