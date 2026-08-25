import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { DataTable } from "@/components/common/DataTable"
import { Field } from "@/components/common/Field"
import { Panel } from "@/components/common/Panel"
import { Badge } from "@/components/ui/badge"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { TableCell, TableRow } from "@/components/ui/table"
import { api, asArray } from "@/lib/api"
import { formatDateTime } from "@/lib/format"
import type { DocumentSource, ParseRun } from "@/types"

export function RunsPage() {
  const [sources, setSources] = useState<DocumentSource[]>([])
  const [runs, setRuns] = useState<ParseRun[]>([])
  const [sourceId, setSourceId] = useState("all")

  const load = useCallback(async () => {
    try {
      const sourceRes = await api<{ sources: DocumentSource[] }>("/api/admin/sources")
      const params = new URLSearchParams()
      if (sourceId !== "all") params.set("source_id", sourceId)
      const runRes = await api<{ runs: ParseRun[] }>(`/api/admin/parse-runs?${params.toString()}`)
      setSources(asArray(sourceRes.sources))
      setRuns(asArray(runRes.runs))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载解析日志失败")
    }
  }, [sourceId])

  useEffect(() => {
    load()
  }, [load])

  return (
    <div className="grid gap-5">
      <Panel title="筛选">
        <div className="max-w-sm">
          <Field label="文档源">
            <Select value={sourceId} onValueChange={setSourceId}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部文档源</SelectItem>
                {sources.map((source) => (
                  <SelectItem key={source.id} value={String(source.id)}>{source.title || `来源 ${source.id}`}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        </div>
      </Panel>
      <Panel fullBleed title="解析日志">
        <DataTable headers={["状态", "来源", "发现", "新增", "时间", "错误"]}>
          {runs.map((run) => (
            <TableRow key={run.id}>
              <TableCell>
                <Badge variant={run.status === "success" ? "default" : "destructive"}>{run.status}</Badge>
              </TableCell>
              <TableCell>{run.source_id}</TableCell>
              <TableCell>{run.total_found}</TableCell>
              <TableCell>{run.inserted}</TableCell>
              <TableCell>{formatDateTime(run.started_at)}</TableCell>
              <TableCell className="max-w-md truncate text-destructive">{run.error || "-"}</TableCell>
            </TableRow>
          ))}
          {runs.length === 0 ? (
            <TableRow>
              <TableCell className="h-24 text-center text-muted-foreground" colSpan={6}>暂无解析日志</TableCell>
            </TableRow>
          ) : null}
        </DataTable>
      </Panel>
    </div>
  )
}
