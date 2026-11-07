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

/**
 * GIF providers whose every subdomain is trusted and serves GIF formats: they use many numbered
 * hosts (media1.tenor.com, c.tenor.com, media0-4.giphy.com, …). Matches the backend's gifProviders.
 */
export const gifProviderDomains = ['tenor.com', 'giphy.com']

const inProviderDomain = (host: string) =>
  gifProviderDomains.some((domain) => host === domain || host.endsWith(`.${domain}`))

export interface MediaLink {
  url: string
  host: string
  /**
   * `gif` is any image known to be animated: a .gif or .apng, or a .webp/.avif from a GIF host.
   * Other images may still turn out animated, so every image is held on a still frame anyway.
   */
  kind: 'image' | 'gif' | 'video'
  /** Intrinsic size, when the source declares one (server-resolved share pages do). */
  width?: number
  height?: number
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
      (animatedPath.test(url.pathname) &&
        (animatedImageHosts.includes(url.hostname) || inProviderDomain(url.hostname)))
    )
  } catch {
    return false
  }
}

/** Link kinds in order of precedence: a .webp on a GIF host is a GIF, not a still image. */
const kinds: [(url: URL) => boolean, MediaLink['kind']][] = [
  [(url) => isAnimatedImageUrl(url.href), 'gif'],
  [(url) => imagePath.test(url.pathname), 'image'],
  [(url) => videoPath.test(url.pathname), 'video'],
]

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
    const kind = kinds.find(([matches]) => matches(url))?.[1]
    if (!kind) continue
    const trusted =
      url.origin === origin ||
      trustedMediaHosts.includes(url.hostname) ||
      inProviderDomain(url.hostname)
    if (mode === 'trusted' && !trusted) continue
    if (found.some((f) => f.url === url.href)) continue
    found.push({ url: url.href, host: url.hostname, kind })
    if (found.length >= limit) break
  }
  return found
}
