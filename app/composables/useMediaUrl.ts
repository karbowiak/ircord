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
