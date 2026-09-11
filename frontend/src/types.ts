export type Site = { id?: number; name: string; subtitle?: string; logo_path?: string; lanzou_password?: string }

export type TencentDocsSettings = {
  configured: boolean
  client_id?: string
  open_id?: string
  access_token_expires_at?: string | null
}

export type Status = { installed: boolean; site?: Site }

export type AccountRole = "admin" | "user"

export type Account = {
  id: number
  username: string
  role: AccountRole
  status: "active" | "disabled"
  must_change_credentials: boolean
  last_login_at?: string | null
  created_at?: string
  updated_at?: string
}

export type DocumentSource = {
  id: number
  url: string
  title: string
  refresh_interval_minutes: number
  enabled: boolean
  last_parsed_at?: string | null
  next_run_at?: string | null
  last_error: string
}

export type FontItem = {
  id: number
  source_id: number
  font_name: string
  download_url: string
  access_code: string
  first_seen_at: string
  last_seen_at: string
}

export type PublicFont = Pick<FontItem, "id" | "font_name" | "first_seen_at">

export type LanzouDownloadFile = {
  name: string
  size: string
  path?: string
  url?: string
  error?: string
}
export type ArchiveFile = { name: string; path: string; size: number }

export type ParseRun = {
  id: number
  source_id: number
  status: string
  total_found: number
  inserted: number
  error: string
  started_at: string
  finished_at: string
}
