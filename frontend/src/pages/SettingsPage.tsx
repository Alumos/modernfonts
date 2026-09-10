import type { FormEvent } from "react"
import { useEffect, useState } from "react"
import { ImageIcon, KeyRound, Save } from "lucide-react"
import { toast } from "sonner"

import { Field } from "@/components/common/Field"
import { Panel } from "@/components/common/Panel"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { api } from "@/lib/api"
import { formatDateTime } from "@/lib/format"
import type { Site, TencentDocsSettings } from "@/types"

export function SettingsPage({ refresh }: { refresh: () => Promise<void> }) {
  const [site, setSite] = useState<Site | null>(null)
  const [tencentDocs, setTencentDocs] = useState<TencentDocsSettings>({ configured: false })

  async function load() {
    const [nextSite, nextTencentDocs] = await Promise.all([
      api<Site>("/api/admin/settings/site"),
      api<TencentDocsSettings>("/api/admin/settings/tencent-docs"),
    ])
    setSite(nextSite)
    setTencentDocs(nextTencentDocs)
  }

  useEffect(() => {
    load()
  }, [])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    try {
      await api("/api/admin/settings/site", { method: "PUT", body: new FormData(event.currentTarget) })
      toast.success("站点和下载设置已保存")
      await load()
      await refresh()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "保存失败")
    }
  }

  async function submitTencentDocs(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formElement = event.currentTarget
    const form = new FormData(formElement)
    try {
      await api("/api/admin/settings/tencent-docs", {
        method: "PUT",
        body: JSON.stringify({
          client_id: form.get("client_id"),
          access_token: form.get("access_token"),
          open_id: form.get("open_id"),
        }),
      })
      toast.success("腾讯文档 OpenAPI 凭据已保存")
      formElement.reset()
      await load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "保存腾讯文档凭据失败")
    }
  }

  return (
    <div className="grid gap-5">
      <Panel title="站点设置">
        <form className="grid w-full max-w-2xl gap-4" key={`${site?.id || "site"}-${site?.name || ""}-${site?.subtitle || ""}-${site?.logo_path || ""}-${site?.lanzou_password || ""}`} onSubmit={submit}>
          <div className="flex items-center gap-3 rounded-lg border p-3">
            {site?.logo_path ? (
              <img alt="" className="size-12 rounded-md object-contain" src={site.logo_path} />
            ) : (
              <div className="flex size-12 items-center justify-center rounded-md bg-muted text-muted-foreground">
                <ImageIcon className="size-5" />
              </div>
            )}
            <div className="min-w-0">
              <div className="truncate text-sm font-medium">{site?.name || "未设置网站名称"}</div>
              <div className="truncate text-xs text-muted-foreground">{site?.subtitle || "未设置副标题"}</div>
            </div>
          </div>
          <Field label="站点名称">
            <Input defaultValue={site?.name || ""} name="site_name" required />
          </Field>
          <Field label="站点副标题">
            <Input defaultValue={site?.subtitle || ""} maxLength={240} name="site_subtitle" placeholder="显示在站点名称下方" />
          </Field>
          <Field label="站点图标">
            <Input accept=".png,.jpg,.jpeg,.webp,.svg" name="logo" type="file" />
          </Field>
          <Field label="蓝奏云统一密码">
            <Input autoComplete="off" defaultValue={site?.lanzou_password || ""} maxLength={80} name="lanzou_password" placeholder="所有字体下载链接共用" type="password" />
            <p className="text-xs text-muted-foreground">下载解析使用这里的统一密码；字体库里的“文档访问码”是腾讯文档随链接提取的 dkey，主要用于记录和排查。</p>
          </Field>
          <Button className="w-fit" type="submit">
            <Save className="size-4" />
            保存
          </Button>
        </form>
      </Panel>

      <Panel title="腾讯文档 OpenAPI">
        <form className="grid w-full max-w-2xl gap-4" key={`${tencentDocs.client_id || "docs"}-${tencentDocs.open_id || ""}-${tencentDocs.access_token_expires_at || ""}`} onSubmit={submitTencentDocs}>
          <div className="flex flex-wrap items-center gap-3 rounded-lg border p-3">
            <div className="flex size-10 items-center justify-center rounded-md bg-muted text-muted-foreground"><KeyRound className="size-5" /></div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 text-sm font-medium">
                私有文档授权
                <Badge variant={tencentDocs.configured ? "default" : "secondary"}>{tencentDocs.configured ? "已配置" : "未配置"}</Badge>
              </div>
              <p className="mt-1 text-xs text-muted-foreground">Access Token 会加密保存，仅由后端发送给腾讯文档官方接口。</p>
            </div>
          </div>
          <Field label="应用 ID（Client ID）">
            <Input defaultValue={tencentDocs.client_id || ""} maxLength={128} name="client_id" required />
          </Field>
          <Field label="Open ID">
            <Input defaultValue={tencentDocs.open_id || ""} maxLength={128} name="open_id" required />
          </Field>
          <Field label="Access Token">
            <Input autoComplete="off" maxLength={8192} name="access_token" placeholder={tencentDocs.configured ? "留空则保持当前 Token 不变" : "粘贴腾讯文档 Access Token"} type="password" />
            <p className="text-xs text-muted-foreground">{tencentDocs.access_token_expires_at ? `当前 Token 预计于 ${formatDateTime(tencentDocs.access_token_expires_at)} 过期；到期后需要在这里手动更新。` : "保存后即可解析已授权账号可访问的私有文档。"}</p>
          </Field>
          <Button className="w-fit" type="submit"><Save className="size-4" />保存腾讯文档凭据</Button>
        </form>
      </Panel>
    </div>
  )
}
