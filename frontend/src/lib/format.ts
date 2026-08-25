export function formatDate(value: string) {
  if (!value) return "-"
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium" }).format(new Date(value))
}

export function formatDateTime(value?: string | null) {
  if (!value) return "-"
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value))
}

export function dateInputValue(value: string) {
  if (!value) return ""
  return new Date(value).toISOString().slice(0, 10)
}
