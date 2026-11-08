// Inline previews for links to images, video, audio and YouTube. IRC messages are plain text,
// so the only signal is the URL itself; nothing is fetched to guess. The supported link kinds,
// limits and fallbacks are documented in README.md (“Link previews”).

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
  // YouTube thumbnails; players load from youtube-nocookie.com only when someone presses play.
  'i.ytimg.com',
  // GitHub's Open Graph images: the share image of issue, pull request and repository cards.
  'opengraph.githubassets.com',
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
  kind: 'image' | 'gif' | 'video' | 'audio' | 'youtube'
  /** The video a YouTube link names. */
  youtube?: YouTubeVideo
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
const audioPath = /\.(mp3|m4a|aac|ogg|oga|opus|wav|flac)$/i

export interface YouTubeVideo {
  id: string
  /** Offset in seconds from a t= or start= parameter. */
  start: number
  /** A vertical Short. */
  short: boolean
}
const youtubeHosts = new Set([
  'youtube.com',
  'www.youtube.com',
  'm.youtube.com',
  'music.youtube.com',
  'youtube-nocookie.com',
  'www.youtube-nocookie.com',
])
const youtubeId = /^[\w-]{11}$/
/** "90", "90s" or "1h2m3s"; anything else, or more than a day, is no offset. Matches the backend. */
function startOffset(value: string | null) {
  const match = value ? /^(?:(\d{1,3})h)?(?:(\d{1,4})m)?(?:(\d{1,6})s?)?$/.exec(value) : null
  if (!match) return 0
  const total = Number(match[1] ?? 0) * 3600 + Number(match[2] ?? 0) * 60 + Number(match[3] ?? 0)
  return total > 86400 ? 0 : total
}
/** The YouTube video a link names, normalizing its URL variants (as the backend's ParseYouTube). */
export function youtubeVideo(href: string | URL): YouTubeVideo | null {
  let url: URL
  try {
    url = new URL(href)
  } catch {
    return null
  }
  if (url.protocol !== 'https:' || url.username || url.password) return null
  const host = url.hostname.toLowerCase()
  const segments = url.pathname.replace(/^\/+|\/+$/g, '').split('/')
  let id: string | null = null
  let short = false
  if (host === 'youtu.be') id = segments[0] ?? null
  else if (youtubeHosts.has(host)) {
    if (segments.length === 1 && segments[0] === 'watch') id = url.searchParams.get('v')
    else if (segments.length === 2 && ['embed', 'live', 'v'].includes(segments[0]!))
      id = segments[1]!
    else if (segments.length === 2 && segments[0] === 'shorts') [id, short] = [segments[1]!, true]
  }
  if (!id || !youtubeId.test(id)) return null
  return {
    id,
    short,
    // An empty t= falls back to start=, as Go's Query().Get does.
    start: startOffset(url.searchParams.get('t') || url.searchParams.get('start')),
  }
}
/** The privacy-enhanced player, started by the click that created it. */
export function youtubeEmbedUrl(video: YouTubeVideo) {
  const params = new URLSearchParams({ autoplay: '1', rel: '0', playsinline: '1' })
  if (video.start) params.set('start', String(video.start))
  return `https://www.youtube-nocookie.com/embed/${video.id}?${params}`
}
export const youtubeThumbnail = (id: string) => `https://i.ytimg.com/vi/${id}/hqdefault.jpg`
/** The canonical watch link, for “Open on YouTube”. */
export function youtubeWatchUrl(video: YouTubeVideo) {
  const base = video.short
    ? `https://www.youtube.com/shorts/${video.id}`
    : `https://www.youtube.com/watch?v=${video.id}`
  return video.start && !video.short ? `${base}&t=${video.start}s` : base
}

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

/**
 * Whether a media URL loads under the default “trusted” setting: this app's own files, the
 * well-known hosts, the GIF providers' domains and the channel's extra trusted media hosts.
 */
export function isTrustedMediaUrl(
  href: string | URL,
  origin: string,
  channelHosts: readonly string[] = [],
): boolean {
  try {
    const url = new URL(href)
    return (
      url.origin === origin ||
      trustedMediaHosts.includes(url.hostname) ||
      channelHosts.includes(url.hostname) ||
      inProviderDomain(url.hostname)
    )
  } catch {
    return false
  }
}

/**
 * Whether a link is an image the timeline shows, and of which kind: `gif` for a known animated
 * format, `image` for other images, null otherwise. Galleries catalogue links by the same rule.
 */
export function imageLinkKind(url: URL): 'image' | 'gif' | null {
  if (isAnimatedImageUrl(url.href)) return 'gif'
  return imagePath.test(url.pathname) ? 'image' : null
}

/** Link kinds in order of precedence: a .webp on a GIF host is a GIF, not a still image. */
const kinds: [(url: URL) => boolean, MediaLink['kind']][] = [
  [(url) => imageLinkKind(url) === 'gif', 'gif'],
  [(url) => imageLinkKind(url) === 'image', 'image'],
  [(url) => videoPath.test(url.pathname), 'video'],
  [(url) => audioPath.test(url.pathname), 'audio'],
]

/**
 * Up to `limit` previewable links in a message. `mode` is the user's link-preview setting;
 * `origin` is this app's origin, whose own uploads are always previewed. `channelHosts` are the
 * extra trusted media hosts the message's channel operators set, treated exactly like
 * `trustedMediaHosts`. Several links to the same file, or to the same YouTube video in different
 * forms, count once.
 */
export function mediaLinks(
  text: string,
  mode: 'off' | 'trusted' | 'all',
  origin: string,
  limit = 3,
  channelHosts: readonly string[] = [],
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
    const youtube = youtubeVideo(url)
    if (youtube) {
      // The thumbnail comes from YouTube's image host, trusted like the other well-known hosts.
      if (!found.some((f) => f.youtube?.id === youtube.id))
        found.push({ url: url.href, host: 'YouTube', kind: 'youtube', youtube })
      if (found.length >= limit) break
      continue
    }
    const kind = kinds.find(([matches]) => matches(url))?.[1]
    if (!kind) continue
    if (mode === 'trusted' && !isTrustedMediaUrl(url, origin, channelHosts)) continue
    if (found.some((f) => f.url === url.href)) continue
    found.push({ url: url.href, host: url.hostname, kind })
    if (found.length >= limit) break
  }
  return found
}

/** Media links a message considers: up to ten images for a gallery, players bounded below. */
export const maxMediaLinks = 10
/** Players (video, audio, YouTube) shown per message; further links stay links. */
export const maxPlayers = 3
/**
 * Splits a message's media into what is shown: still images and GIFs in link order (shown as one
 * gallery when there are several), and at most `maxPlayers` players.
 */
export function groupMedia(links: MediaLink[]) {
  const images = links.filter((link) => link.kind === 'image' || link.kind === 'gif')
  const players = links
    .filter((link) => link.kind === 'video' || link.kind === 'audio' || link.kind === 'youtube')
    .slice(0, maxPlayers)
  return { images, players }
}
