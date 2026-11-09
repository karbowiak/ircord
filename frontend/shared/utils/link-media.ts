import { textOutsideCode } from './irc'
import type { components } from '../generated/api'

type Widget = components['schemas']['LinkMediaWidget']

/** Upload metadata can identify media even when the storage URL has no extension. */
export function attachmentMediaAsset(attachment: components['schemas']['Attachment'], pageURL: string): components['schemas']['LinkMediaAsset'] | null {
  let source: string
  try { source = publicMediaURL(new URL(attachment.url, pageURL).href) } catch { return null }
  if (!source) return null
  let contentType = attachment.contentType?.split(';')[0]?.toLowerCase().trim() ?? ''
  if (!contentType || contentType === 'application/octet-stream') {
    const extension = attachment.name.split('.').at(-1)?.toLowerCase() ?? ''
    contentType = ({ gif: 'image/gif', png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', webp: 'image/webp', avif: 'image/avif', bmp: 'image/bmp', mp4: 'video/mp4', mov: 'video/quicktime', m4v: 'video/mp4', webm: 'video/webm', mp3: 'audio/mpeg', m4a: 'audio/mp4', ogg: 'audio/ogg', wav: 'audio/wav', flac: 'audio/flac' } as Record<string, string>)[extension] ?? ''
  }
  const kind = contentType.split('/')[0]
  if (kind !== 'image' && kind !== 'video' && kind !== 'audio') return null
  if (kind === 'image' && !['image/gif', 'image/png', 'image/jpeg', 'image/webp', 'image/avif', 'image/bmp'].includes(contentType)) return null
  return { url: source, kind, contentType, width: attachment.width, height: attachment.height }
}

export function publicMediaURL(raw: string) {
  try {
    const url = new URL(raw)
    return /^(https?:)$/.test(url.protocol) && !url.username && !url.password ? url.href : ''
  } catch { return '' }
}

/** Only actual links in prose resolve; code and repeated URLs stay quiet. */
export function messageMediaLinks(text: string) {
  const urls = new Set<string>()
  // eslint-disable-next-line no-control-regex -- control characters end a URL in IRC text
  for (const match of textOutsideCode(text).matchAll(/https?:\/\/[^\s<>"'\x00-\x1f]+/g)) {
    let raw = match[0].replace(/[.,!?;:]+$/, '')
    for (const [open, close] of [['(', ')'], ['[', ']'], ['{', '}']]) {
      while (raw.endsWith(close!) && raw.split(close!).length > raw.split(open!).length)
        raw = raw.slice(0, -1)
    }
    if (publicMediaURL(raw)) urls.add(raw)
  }
  return [...urls]
}

/** Raw HTML is never assigned to the application DOM or a same-origin srcdoc. */
export function widgetSource(widget: Widget, apiBase: string, pageURL: string) {
  try {
    const page = new URL(pageURL)
    if (widget.external) {
      const url = new URL(widget.url)
      return url.protocol === 'https:' && !url.username && !url.password && url.origin !== page.origin
        ? url.href : ''
    }
    if (!/^\/api\/v1\/link-media\/widgets\/[a-f0-9]{48}$/.test(widget.url)) return ''
    return new URL(widget.url, new URL(apiBase, page)).href
  } catch { return '' }
}

export function widgetHeight(event: Pick<MessageEvent, 'source' | 'origin' | 'data'>, source: Window | null, token: string) {
  if (!source || event.source !== source || event.origin !== 'null') return null
  const data: unknown = event.data
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  const value = data as Record<string, unknown>
  if (Object.keys(value).sort().join(',') !== 'height,token,type' || value.type !== 'ircord:widget-size' || value.token !== token) return null
  return typeof value.height === 'number' && Number.isInteger(value.height) && value.height >= 100 && value.height <= 800
    ? value.height : null
}

/** Provider resize messages must belong to this exact frame and displayed post. */
export function providerWidgetHeight(event: Pick<MessageEvent, 'source' | 'origin' | 'data'>, source: Window | null, raw: string) {
  if (!source || event.source !== source) return null
  let url: URL
  try { url = new URL(raw) } catch { return null }
  if (url.protocol !== 'https:' || event.origin !== url.origin) return null
  const data = event.data
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  let height: unknown
  if (url.hostname === 'platform.twitter.com' && url.pathname === '/embed/Tweet.html') {
    const message = data['twttr.embed']
    if (message?.method !== 'twttr.private.resize' || !Array.isArray(message.params) || message.params.length !== 1) return null
    if (message.params[0]?.data?.tweet_id !== url.searchParams.get('id')) return null
    height = message.params[0]?.height
  } else if (url.hostname === 'embed.bsky.app' && url.pathname.startsWith('/embed/')) {
    if (!url.searchParams.get('id') || data.id !== url.searchParams.get('id')) return null
    height = data.height
  }
  return typeof height === 'number' && Number.isFinite(height) && height >= 100 && height <= 2000
    ? Math.ceil(height) : null
}

/** Only the active TikTok frame may report its own documented player failure. */
export function widgetPlayerFailed(event: Pick<MessageEvent,'source'|'origin'|'data'>, source: Window | null, raw: string) {
  if(!source || event.source!==source) return false
  let url:URL
  try{url=new URL(raw)}catch{return false}
  if(!['www.tiktok.com','tiktok.com'].includes(url.hostname) || event.origin!==url.origin) return false
  const data=event.data as {type?:unknown;value?:{errorCode?:unknown};'x-tiktok-player'?:unknown}|null
  if(!data || data['x-tiktok-player']!==true || data.type!=='onPlayerError') return false
  const code=data.value?.errorCode
  return typeof code==='number' && Number.isInteger(code) && code>=1000 && code<10000
}

type NativeContent = Pick<components['schemas']['LinkMediaResponse'], 'url' | 'provider' | 'title' | 'author' | 'description'>

/**
 * What a native post card shows above its media: a byline, the title only when it says
 * something (not Instagram's “Post by name” or TikTok's truncated caption), and the caption
 * with runs of blank lines collapsed.
 */
export function nativePostText(content: NativeContent) {
  const caption = (content.description ?? '').replace(/[ \t]+\n/g, '\n').replace(/\n{3,}/g, '\n\n').trim()
  let title = (content.title ?? '').trim()
  const generic = /^(post|photo|video|reel|carousel)s? by (\S+)$/i.exec(title)
  const handleFromTitle = generic?.[2]
  const truncated = title.replace(/(\.\.\.|…)$/, '').trim()
  if (generic || /^tiktok video$/i.test(title) || (truncated && caption.startsWith(truncated))) title = ''
  let name = (content.author ?? '').trim()
  let handle: string
  let path = ''
  try { path = new URL(content.url).pathname } catch { /* a relative or malformed URL has no path */ }
  if (content.provider === 'reddit') {
    const subreddit = /^\/r\/([^/]+)/i.exec(path)?.[1]
    handle = subreddit ? `r/${subreddit}` : ''
    if (name) name = `u/${name.replace(/^u\//, '')}`
  } else {
    const fromPath = content.provider === 'tiktok' ? /^\/@([^/]+)/.exec(path)?.[1] : undefined
    const raw = handleFromTitle ?? fromPath ?? (/^[\w.]+$/.test(name) ? name : '')
    handle = raw ? `@${raw.replace(/^@/, '')}` : ''
    if (handle && name.replace(/^@/, '') === handle.slice(1)) name = ''
  }
  return { title, caption, name, handle }
}

/** Width over height for a set of assets, from the tallest one so paging never changes size. */
export function nativeStageRatio(assets: { width?: number; height?: number }[], fallback = 1) {
  const ratios = assets.filter(a => a.width && a.height).map(a => a.width! / a.height!)
  const ratio = ratios.length ? Math.min(...ratios) : fallback
  return Math.min(16 / 9, Math.max(9 / 16, ratio))
}
