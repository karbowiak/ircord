const uploadPath = /^\/files\/[0-9a-f]{32}\/[0-9a-f]{48}\/[^/?#\\\s]+$/
const maxDimension = 2048

function browserOrigin(): string | undefined {
  return typeof window === 'undefined' ? undefined : window.location.origin
}

/** Only unsigned, minted IRCord upload paths may acquire variant queries. */
export function isUploadImageUrl(value: unknown, origin = browserOrigin()): value is string {
  if (typeof value !== 'string') return false
  if (uploadPath.test(value)) return true
  return Boolean(
    origin &&
    /^https?:\/\//.test(value) &&
    value.startsWith(`${origin}/`) &&
    uploadPath.test(value.slice(origin.length)),
  )
}

/** Keep external images and signed URLs on the global single-density policy. */
export function uploadImageDensities(value: unknown, origin = browserOrigin()): '1x' | '1x 2x' {
  return isUploadImageUrl(value, origin) ? '1x 2x' : '1x'
}

function dimension(value: unknown): number | undefined {
  if (typeof value !== 'number' && typeof value !== 'string') return undefined
  if (!/^\d+$/.test(String(value))) return undefined
  const parsed = Number(value)
  if (!Number.isSafeInteger(parsed) || parsed < 1) return undefined
  return Math.min(parsed, maxDimension)
}

/** Return a display URL without rewriting the source URL or any query signature. */
export function uploadImageVariant(
  value: string,
  width?: unknown,
  height?: unknown,
  origin = browserOrigin(),
): string {
  if (!isUploadImageUrl(value, origin)) return value
  const w = dimension(width)
  const h = dimension(height)
  const query = [w ? `w=${w}` : '', h ? `h=${h}` : ''].filter(Boolean).join('&')
  return query ? `${value}?${query}` : value
}
