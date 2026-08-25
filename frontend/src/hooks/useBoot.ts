import { useEffect, useState } from "react"

import { api } from "@/lib/api"
import type { Account, Status } from "@/types"

export function useBoot() {
  const [status, setStatus] = useState<Status | null>(null)
  const [account, setAccount] = useState<Account | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = async () => {
    setLoading(true)
    try {
      const next = await api<Status>("/api/system/status")
      setStatus(next)
      if (next.installed) {
        try {
          const me = await api<{ account: Account }>("/api/auth/me")
          setAccount(me.account)
        } catch {
          setAccount(null)
        }
      } else {
        setAccount(null)
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    refresh()
  }, [])

  return { status, account, loading, refresh }
}
