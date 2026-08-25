export async function api<T = unknown>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: "include",
    headers: init?.body instanceof FormData ? undefined : { "Content-Type": "application/json" },
    ...init,
  })
  const contentType = response.headers.get("content-type") || ""
  const data = contentType.includes("application/json") ? await response.json() : {}
  if (!response.ok) {
    throw new Error(data.error || "请求失败")
  }
  return data as T
}

export function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}
