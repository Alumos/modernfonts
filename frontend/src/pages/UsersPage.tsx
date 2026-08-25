import type { FormEvent } from "react"
import { useEffect, useState } from "react"
import { KeyRound, Loader2, RefreshCw, UserPlus } from "lucide-react"
import { toast } from "sonner"

import { DataTable } from "@/components/common/DataTable"
import { Field } from "@/components/common/Field"
import { Panel } from "@/components/common/Panel"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { TableCell, TableRow } from "@/components/ui/table"
import { api, asArray } from "@/lib/api"
import { formatDateTime } from "@/lib/format"
import type { Account } from "@/types"

type UserAccount = Account & {
  created_at?: string
  last_login_at?: string | null
}

export function UsersPage() {
  const [users, setUsers] = useState<UserAccount[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [updatingId, setUpdatingId] = useState<number | null>(null)
  const [resetUser, setResetUser] = useState<UserAccount | null>(null)
  const [resetting, setResetting] = useState(false)

  async function load() {
    setLoading(true)
    try {
      const result = await api<{ users: UserAccount[] }>("/api/admin/users")
      setUsers(asArray(result.users))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "加载用户账号失败")
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function createUser(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formElement = event.currentTarget
    const form = new FormData(formElement)
    const username = String(form.get("username") || "").trim()
    const password = String(form.get("password") || "")

    setCreating(true)
    try {
      const result = await api<{ user: UserAccount }>("/api/admin/users", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      })
      setUsers((current) => [result.user, ...current.filter((user) => user.id !== result.user.id)])
      formElement.reset()
      toast.success(`用户 ${result.user.username} 已添加`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "添加用户失败")
    } finally {
      setCreating(false)
    }
  }

  async function setActive(user: UserAccount, active: boolean) {
    const status = active ? "active" : "disabled"
    setUpdatingId(user.id)
    try {
      const result = await api<{ user: UserAccount }>(`/api/admin/users/${user.id}/status`, {
        method: "PATCH",
        body: JSON.stringify({ status }),
      })
      setUsers((current) => current.map((item) => (item.id === result.user.id ? result.user : item)))
      toast.success(`${user.username} 已${active ? "启用" : "停用"}`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "更新用户状态失败")
    } finally {
      setUpdatingId(null)
    }
  }

  async function resetPassword(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!resetUser) return

    const user = resetUser
    const form = new FormData(event.currentTarget)
    const password = String(form.get("password") || "")
    const confirmation = String(form.get("password_confirmation") || "")
    if (password !== confirmation) {
      toast.error("两次输入的密码不一致")
      return
    }

    setResetting(true)
    try {
      const result = await api<{ user: UserAccount }>(`/api/admin/users/${user.id}/reset-password`, {
        method: "POST",
        body: JSON.stringify({ password }),
      })
      setUsers((current) => current.map((item) => (item.id === result.user.id ? result.user : item)))
      setResetUser(null)
      toast.success(`${user.username} 的密码已重置`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "重置密码失败")
    } finally {
      setResetting(false)
    }
  }

  return (
    <div className="grid gap-5">
      <Panel title="添加普通用户">
        <form className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]" onSubmit={createUser}>
          <Field label="账号">
            <Input autoComplete="off" maxLength={80} minLength={3} name="username" placeholder="至少 3 个字符" required />
          </Field>
          <Field label="初始密码">
            <Input autoComplete="new-password" maxLength={72} minLength={6} name="password" placeholder="至少 6 个字符" required type="password" />
          </Field>
          <div className="flex items-end">
            <Button className="w-full lg:w-auto" disabled={creating} type="submit">
              {creating ? <Loader2 className="size-4 animate-spin" /> : <UserPlus className="size-4" />}
              {creating ? "正在添加" : "添加账号"}
            </Button>
          </div>
        </form>
      </Panel>

      <Panel
        fullBleed
        title={`普通用户（${users.length}）`}
        action={
          <Button disabled={loading} size="sm" variant="outline" onClick={load}>
            <RefreshCw className={loading ? "size-4 animate-spin" : "size-4"} />
            刷新
          </Button>
        }
      >
        <DataTable headers={["账号", "状态", "最近登录", "创建时间", { label: "操作", className: "text-right" }]}>
          {users.map((user) => {
            const active = user.status === "active"
            const updating = updatingId === user.id
            return (
              <TableRow key={user.id}>
                <TableCell className="font-medium">{user.username}</TableCell>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <Checkbox
                      aria-label={`${active ? "停用" : "启用"}用户 ${user.username}`}
                      checked={active}
                      disabled={updating}
                      onCheckedChange={(checked) => setActive(user, checked === true)}
                    />
                    <Badge variant={active ? "default" : "secondary"}>{updating ? "更新中" : active ? "启用" : "停用"}</Badge>
                  </div>
                </TableCell>
                <TableCell>{formatDateTime(user.last_login_at)}</TableCell>
                <TableCell>{formatDateTime(user.created_at)}</TableCell>
                <TableCell className="text-right">
                  <Button disabled={updating} size="sm" variant="outline" onClick={() => setResetUser(user)}>
                    <KeyRound className="size-4" />
                    重置密码
                  </Button>
                </TableCell>
              </TableRow>
            )
          })}
          {loading && users.length === 0 ? (
            <TableRow>
              <TableCell className="h-24 text-center text-muted-foreground" colSpan={5}>
                <span className="inline-flex items-center gap-2"><Loader2 className="size-4 animate-spin" />正在加载用户账号</span>
              </TableCell>
            </TableRow>
          ) : null}
          {!loading && users.length === 0 ? (
            <TableRow>
              <TableCell className="h-24 text-center text-muted-foreground" colSpan={5}>暂无普通用户</TableCell>
            </TableRow>
          ) : null}
        </DataTable>
      </Panel>

      <Dialog open={resetUser !== null} onOpenChange={(open) => { if (!open && !resetting) setResetUser(null) }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>重置用户密码</DialogTitle>
            <DialogDescription>为 {resetUser?.username} 设置新密码，保存后该用户需要重新登录。</DialogDescription>
          </DialogHeader>
          <form className="grid gap-4" key={resetUser?.id} onSubmit={resetPassword}>
            <Field label="新密码">
              <Input autoComplete="new-password" maxLength={72} minLength={6} name="password" placeholder="至少 6 个字符" required type="password" />
            </Field>
            <Field label="确认新密码">
              <Input autoComplete="new-password" maxLength={72} minLength={6} name="password_confirmation" required type="password" />
            </Field>
            <DialogFooter>
              <Button disabled={resetting} type="button" variant="outline" onClick={() => setResetUser(null)}>取消</Button>
              <Button disabled={resetting} type="submit">
                {resetting ? <Loader2 className="size-4 animate-spin" /> : <KeyRound className="size-4" />}
                {resetting ? "正在保存" : "确认重置"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
