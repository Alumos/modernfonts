import type { FormEvent } from "react"
import { useEffect, useState } from "react"
import { ImageIcon, Save } from "lucide-react"
import { toast } from "sonner"

import { Field } from "@/components/common/Field"
import { Panel } from "@/components/common/Panel"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { api } from "@/lib/api"
import type { Site } from "@/types"

export function SettingsPage({ refresh }: { refresh: () => Promise<void> }) {
  const [site, setSite] = useState<Site | null>(null)

  async function load() {
    const nextSite = await api<Site>("/api/admin/settings/site")
    setSite(nextSite)
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

  return (
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
  )
}
