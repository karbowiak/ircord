// Browser storage can be unavailable (private windows, blocked site data); never let that break the app.

export function readLocal<T>(
  key: string,
  sanitize: (value: unknown) => T,
  area: 'local' | 'session' = 'local',
): T {
  try {
    const raw = (area === 'local' ? localStorage : sessionStorage).getItem(key)
    return sanitize(raw === null ? undefined : JSON.parse(raw))
  } catch {
    return sanitize(undefined)
  }
}

export function writeLocal(key: string, value: unknown, area: 'local' | 'session' = 'local') {
  try {
    ;(area === 'local' ? localStorage : sessionStorage).setItem(key, JSON.stringify(value))
    return true
  } catch {
    return false
  }
}
