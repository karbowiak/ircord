export function isImageUrl(url: string): boolean {
  const withoutQuery = url.split('?')[0]
  return /\.(png|jpe?g|gif|webp|avif)$/i.test(withoutQuery)
}

export function isVideoUrl(url: string): boolean {
  const withoutQuery = url.split('?')[0]
  return /\.(mp4|webm|gifv)$/i.test(withoutQuery)
}

export function isGifUrl(url: string): boolean {
  const withoutQuery = url.split('?')[0]
  return /\.gif$/i.test(withoutQuery)
}

export function isAnimatedMediaUrl(url: string): boolean {
  return isGifUrl(url) || isVideoUrl(url)
}

export function normalizeAnimatedUrl(url: string): string {
  if (/\.gifv(?:\?.*)?$/i.test(url)) {
    return url.replace(/\.gifv(?:\?.*)?$/i, '.mp4')
  }
  return url
}

export function proxyMediaUrl(url: string): string {
  return `/api/gif-proxy?url=${encodeURIComponent(normalizeAnimatedUrl(url))}`
}

export function isTenorPageUrl(url: string): boolean {
  try {
    const parsed = new URL(url)
    return parsed.hostname.includes('tenor.com') && parsed.pathname.startsWith('/view/')
  } catch {
    return false
  }
}

export function isGiphyPageUrl(url: string): boolean {
  try {
    const parsed = new URL(url)
    if (!parsed.hostname.includes('giphy.com')) return false
    return parsed.pathname.startsWith('/gifs/') || parsed.pathname.startsWith('/stickers/') || parsed.pathname.startsWith('/clips/')
  } catch {
    return false
  }
}

export function isResolvableMediaPageUrl(url: string): boolean {
  return isTenorPageUrl(url) || isGiphyPageUrl(url)
}
