// Inline previews for links to images and videos. IRC messages are plain text, so the only
// signal is the URL itself; nothing is fetched to guess.

/** Well-known image hosts previewed under the default “trusted” setting. */
export const trustedMediaHosts = [
  'i.imgur.com',
  'i.redd.it',
  'preview.redd.it',
  'pbs.twimg.com',
  'media.tenor.com',
  'media.giphy.com',
  'i.giphy.com',
  'static.klipy.com',
  'cdn.discordapp.com',
  'media.discordapp.net',
  'upload.wikimedia.org',
  'images.unsplash.com',
  'cdn.7tv.app',
]

export interface MediaLink {
  url: string
  host: string
  /**
   * `gif` is any image known to be animated: a .gif or .apng, or a .webp/.avif from a GIF host.
   * Other images may still turn out animated, so every image is held on a still frame anyway.
   */
  kind: 'image' | 'gif' | 'video'
}

/** Hosts that serve animated WebP and AVIF as their GIF formats. */
export const animatedImageHosts = [
  'media.tenor.com',
  'media.giphy.com',
  'i.giphy.com',
  'static.klipy.com',
  'cdn.7tv.app',
]
const imagePath = /\.(png|jpe?g|webp|avif)$/i
const gifPath = /\.(gif|apng)$/i
const animatedPath = /\.(webp|avif)$/i
const videoPath = /\.(mp4|webm|mov|m4v)$/i

/** Whether an image URL is known to be animated, by its extension and host. */
export function isAnimatedImageUrl(href: string): boolean {
  try {
    const url = new URL(href)
    return (
      gifPath.test(url.pathname) ||
      (animatedPath.test(url.pathname) && animatedImageHosts.includes(url.hostname))
    )
  } catch {
    return false
  }
}

/**
 * Up to `limit` previewable links in a message. `mode` is the user's link-preview setting;
 * `origin` is this app's origin, whose own uploads are always previewed.
 */
export function mediaLinks(
  text: string,
  mode: 'off' | 'trusted' | 'all',
  origin: string,
  limit = 3,
): MediaLink[] {
  if (mode === 'off') return []
  const found: MediaLink[] = []
  for (const match of text.matchAll(/https?:\/\/[^\s<>"']+/g)) {
    let url: URL
    try {
      url = new URL(match[0].replace(/[),.!?]+$/, ''))
    } catch {
      continue
    }
    if (url.protocol !== 'https:' && url.origin !== origin) continue
    const kind = isAnimatedImageUrl(url.href)
      ? 'gif'
      : imagePath.test(url.pathname)
        ? 'image'
        : videoPath.test(url.pathname)
          ? 'video'
          : null
    if (!kind) continue
    const trusted = url.origin === origin || trustedMediaHosts.includes(url.hostname)
    if (mode === 'trusted' && !trusted) continue
    if (found.some((f) => f.url === url.href)) continue
    found.push({ url: url.href, host: url.hostname, kind })
    if (found.length >= limit) break
  }
  return found
}
