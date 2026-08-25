import type { FormEvent } from "react"
import { useEffect, useState } from "react"
import { Play, RefreshCw } from "lucide-react"
import { toast } from "sonner"

import { DataTable } from "@/components/common/DataTable"
import { Field } from "@/components/common/Field"
import { Panel } from "@/components/common/Panel"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { TableCell, TableRow } from "@/components/ui/table"
import { api, asArray } from "@/lib/api"
import { formatDateTime } from "@/lib/format"
import type { DocumentSource } from "@/types"

export function SourcesPage() {
  const [sources, setSources] = useState<DocumentSource[]>([])
  const [loading, setLoading] = useState(true)
  const [busyId, setBusyId] = useState<number | null>(null)

  async function load() {
    setLoading(true)
    try {
      const res = await api<{ sources: DocumentSource[] }>("/api/admin/sources")
      setSources(asArray(res.sources))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载文档源失败")
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formElement = event.currentTarget
    const form = new FormData(formElement)
    try {
      const res = await api<{ source: DocumentSource }>("/api/admin/sources", {
        method: "POST",
        body: JSON.stringify({
          url: form.get("url"),
          refresh_interval_minutes: Number(form.get("refresh_interval_minutes") || 60),
        }),
      })
      toast.success("文档源已保存")
      setBusyId(res.source.id)
      const parsed = await api<{ result: { total: number; inserted: number } }>(`/api/admin/sources/${res.source.id}/parse`, { method: "POST" })
      toast.success(`首次解析完成，新增 ${parsed.result.inserted} 条`)
      formElement.reset()
      await load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "保存失败")
    } finally {
      setBusyId(null)
    }
  }

  async function parse(source: DocumentSource) {
    setBusyId(source.id)
    try {
      const res = await api<{ result: { total: number; inserted: number } }>(`/api/admin/sources/${source.id}/parse`, { method: "POST" })
      toast.success(`解析完成，发现 ${res.result.total} 条，新增 ${res.result.inserted} 条`)
      await load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "解析失败")
      await load()
    } finally {
      setBusyId(null)
    }
  }

  async function setEnabled(source: DocumentSource, enabled: boolean) {
    try {
      await api(`/api/admin/sources/${source.id}`, {
        method: "PATCH",
        body: JSON.stringify({
          refresh_interval_minutes: source.refresh_interval_minutes,
          enabled,
        }),
      })
      await load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "更新失败")
    }
  }

  return (
    <div className="grid gap-5">
      <Panel title="添加腾讯文档链接">
        <form className="grid gap-4 lg:grid-cols-[1fr_12rem_auto]" onSubmit={create}>
          <Field label="腾讯文档链接">
            <Input name="url" placeholder="https://docs.qq.com/doc/..." required />
          </Field>
          <Field label="自动刷新间隔">
            <Input defaultValue={60} min={1} name="refresh_interval_minutes" type="number" />
          </Field>
          <div className="flex items-end">
            <Button className="w-full lg:w-auto" type="submit">
              <Play className="size-4" />
              保存并解析
            </Button>
          </div>
        </form>
      </Panel>

      <Panel fullBleed title="文档源" action={<Button size="sm" variant="outline" onClick={load}><RefreshCw className="size-4" />刷新</Button>}>
        <DataTable headers={["标题", "刷新间隔", "状态", "最近解析", { label: "操作", className: "text-right" }]}>
          {sources.map((source) => (
            <TableRow key={source.id}>
              <TableCell>
                <div className="flex items-center gap-3">
                  <span className="font-medium">{source.title || "未解析标题"}</span>
                  <a className="text-xs text-primary underline-offset-4 hover:underline" href={source.url} rel="noreferrer" target="_blank">
                    {source.url}
                  </a>
                  {source.last_error ? <span className="max-w-80 truncate text-xs text-destructive" title={source.last_error}>{source.last_error}</span> : null}
                </div>
              </TableCell>
              <TableCell>{source.refresh_interval_minutes} 分钟</TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  <Checkbox checked={source.enabled} onCheckedChange={(value) => setEnabled(source, Boolean(value))} aria-label="启用自动刷新" />
                  <Badge variant={source.enabled ? "default" : "secondary"}>{source.enabled ? "启用" : "停用"}</Badge>
                </div>
              </TableCell>
              <TableCell>{formatDateTime(source.last_parsed_at)}</TableCell>
              <TableCell className="text-right">
                <Button disabled={busyId === source.id} size="sm" variant="outline" onClick={() => parse(source)}>
                  {busyId === source.id ? <RefreshCw className="size-4 animate-spin" /> : null}
                  立即解析
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {!loading && sources.length === 0 ? (
            <TableRow>
              <TableCell className="h-24 text-center text-muted-foreground" colSpan={5}>暂无文档源</TableCell>
            </TableRow>
          ) : null}
        </DataTable>
      </Panel>
    </div>
  )
}
