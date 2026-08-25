import type { FormEvent } from "react"
import { useState } from "react"
import { ArrowLeft, LogIn } from "lucide-react"
import { toast } from "sonner"

import { Field } from "@/components/common/Field"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { AuthFrame } from "@/layouts/AuthFrame"
import { api } from "@/lib/api"
import type { Account, Site } from "@/types"

export function LoginPage({ onDone, site }: { onDone: (account: Account) => Promise<void>; site?: Site }) {
  const [pending, setPending] = useState(false)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    setPending(true)
    try {
      const result = await api<{ account: Account }>("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ username: form.get("username"), password: form.get("password") }),
      })
      toast.success("登录成功")
      await onDone(result.account)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "登录失败")
    } finally {
      setPending(false)
    }
  }

  return (
    <AuthFrame title={site?.name || "腾讯文档字体解析"} description="使用管理员或用户账号登录。">
      <form className="grid gap-5" onSubmit={submit}>
        <Field label="账号">
          <Input autoComplete="username" maxLength={80} name="username" required />
        </Field>
        <Field label="密码">
          <Input autoComplete="current-password" maxLength={72} name="password" required type="password" />
        </Field>
        <Button className="w-full" disabled={pending} type="submit">
          <LogIn className="size-4" />
          {pending ? "正在登录" : "登录"}
        </Button>
        <Button asChild className="w-full" variant="outline">
          <a href="#/"><ArrowLeft className="size-4" />访客浏览</a>
        </Button>
      </form>
    </AuthFrame>
  )
}
