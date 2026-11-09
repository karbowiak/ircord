import { splitCodeBlocks, textOutsideCode } from './irc'
import type { components } from '../generated/api'

type Widget = components['schemas']['LinkMediaWidget']

/** Singular posts remain compatible with history saved before upload galleries. */
export function messageAttachments(message: {
  attachment?: components['schemas']['Attachment']
  attachments?: components['schemas']['Attachment'][]
}) {
  return message.attachments?.length
    ? message.attachments
    : message.attachment
      ? [message.attachment]
      : []
}

const previewReasons = new Set([
  'unsupported_type',
  'type_mismatch',
  'too_large',
  'invalid_file',
  'fetch_failed',
  'storage_unavailable',
  'no_oembed',
  'restricted',
])

/** Unknown server reasons retain a stable, translated fallback. */
export function linkMediaReasonKey(reason: string) {
  return `mediaUi.linkPreview.reasons.${previewReasons.has(reason) ? reason : 'unavailable'}`
}

/** Name only the detected type; a filename extension is not evidence of file contents. */
export function linkFileTypeKey(contentType?: string) {
  const type = contentType?.split(';')[0]?.trim().toLowerCase() ?? ''
  const types: Record<string, string> = {
    'application/octet-stream': 'file',
    'application/pdf': 'pdf',
    'application/msword': 'doc',
    'application/vnd.openxmlformats-officedocument.wordprocessingml.document': 'docx',
    'application/zip': 'zip',
    'application/x-zip-compressed': 'zip',
    'application/vnd.rar': 'rar',
    'application/x-rar-compressed': 'rar',
    'application/x-7z-compressed': 'sevenZip',
    'application/x-tar': 'tar',
    'application/gzip': 'gzip',
    'application/x-gzip': 'gzip',
    'text/plain': 'text',
  }
  return types[type] ? `mediaUi.linkPreview.types.${types[type]}` : undefined
}

/** Expiring files and preview thumbnails share the native media refresh budget. */
export function linkMediaExpirations(content?: components['schemas']['LinkMediaResponse']) {
  if (!content) return undefined
  const entries: { url: string; expiresAt?: string }[] = [...content.assets]
  if (content.file?.url) entries.push({ url: content.file.url, expiresAt: content.file.expiresAt })
  const card = content.card
  if (card?.expiresAt) {
    for (const url of [card.image, card.icon]) {
      if (url) entries.push({ url, expiresAt: card.expiresAt })
    }
  }
  const widget = content.widget
  if (widget?.expiresAt) {
    entries.push({ url: widget.url, expiresAt: widget.expiresAt })
    if (widget.thumbnail) entries.push({ url: widget.thumbnail, expiresAt: widget.expiresAt })
  }
  return entries
}

/** Upload metadata can identify media even when the storage URL has no extension. */
export function attachmentMediaAsset(
  attachment: components['schemas']['Attachment'],
  pageURL: string,
): components['schemas']['LinkMediaAsset'] | null {
  let source: string
  try {
    source = publicMediaURL(new URL(attachment.url, pageURL).href)
  } catch {
    return null
  }
  if (!source) return null
  let contentType = attachment.contentType?.split(';')[0]?.toLowerCase().trim() ?? ''
  if (!contentType || contentType === 'application/octet-stream') {
    const extension = attachment.name.split('.').at(-1)?.toLowerCase() ?? ''
    contentType =
      (
        {
          gif: 'image/gif',
          png: 'image/png',
          jpg: 'image/jpeg',
          jpeg: 'image/jpeg',
          webp: 'image/webp',
          avif: 'image/avif',
          bmp: 'image/bmp',
          mp4: 'video/mp4',
          mov: 'video/quicktime',
          m4v: 'video/mp4',
          webm: 'video/webm',
          mp3: 'audio/mpeg',
          m4a: 'audio/mp4',
          ogg: 'audio/ogg',
          wav: 'audio/wav',
          flac: 'audio/flac',
        } as Record<string, string>
      )[extension] ?? ''
  }
  const kind = contentType.split('/')[0]
  if (kind !== 'image' && kind !== 'video' && kind !== 'audio') return null
  if (
    kind === 'image' &&
    !['image/gif', 'image/png', 'image/jpeg', 'image/webp', 'image/avif', 'image/bmp'].includes(
      contentType,
    )
  )
    return null
  return { url: source, kind, contentType, width: attachment.width, height: attachment.height }
}

export function publicMediaURL(raw: string) {
  try {
    const url = new URL(raw)
    return /^(https?:)$/.test(url.protocol) && !url.username && !url.password ? url.href : ''
  } catch {
    return ''
  }
}

/** Only actual links in prose resolve; code and repeated URLs stay quiet. */
export function messageMediaLinks(text: string) {
  const urls = new Set<string>()
  // eslint-disable-next-line no-control-regex -- control characters end a URL in IRC text
  for (const match of textOutsideCode(text).matchAll(/https?:\/\/[^\s<>"'\x00-\x1f]+/g)) {
    const raw = mediaLinkText(match[0])
    if (publicMediaURL(raw)) urls.add(raw)
  }
  return [...urls]
}

/** Keep sentence punctuation outside the link, consistently for resolution and presentation. */
function mediaLinkText(text: string) {
  let raw = text.replace(/[.,!?;:]+$/, '')
  for (const [open, close] of [
    ['(', ')'],
    ['[', ']'],
    ['{', '}'],
  ]) {
    while (raw.endsWith(close!) && raw.split(close!).length > raw.split(open!).length)
      raw = raw.slice(0, -1)
  }
  return raw
}

/** Presentation only: hide successfully rendered prose links without changing code or IRC controls. */
export function messageMediaText(text: string, rendered: ReadonlySet<string>) {
  if (!rendered.size) return text
  return splitCodeBlocks(text)
    .map((segment) => {
      if (segment.kind === 'code') return segment.raw
      let mono = false
      // eslint-disable-next-line no-control-regex -- preserve IRC monospace/reset codes and URL boundaries
      return segment.raw.replace(/\x11|\x0f|https?:\/\/[^\s<>"'\x00-\x1f]+/g, (token) => {
        if (token === '\x11') mono = !mono
        else if (token === '\x0f') mono = false
        else if (!mono) {
          const url = mediaLinkText(token)
          if (rendered.has(url)) return token.slice(url.length)
        }
        return token
      })
    })
    .join('')
}

/** Raw HTML is never assigned to the application DOM or a same-origin srcdoc. */
export function widgetSource(widget: Widget, apiBase: string, pageURL: string) {
  try {
    const page = new URL(pageURL)
    if (!/^[a-f0-9]{48}$/.test(widget.token)) return ''
    if (widget.external) {
      const url = new URL(widget.url)
      return url.protocol === 'https:' && url.origin !== page.origin ? publicMediaURL(url.href) : ''
    }
    if (widget.url !== `/api/v1/link-media/widgets/${widget.token}`) return ''
    const url = new URL(widget.url, new URL(apiBase, page))
    return publicMediaURL(url.href)
  } catch {
    return ''
  }
}

export function widgetHeight(
  event: Pick<MessageEvent, 'source' | 'origin' | 'data'>,
  source: Window | null,
  token: string,
) {
  if (!source || event.source !== source || event.origin !== 'null') return null
  const data: unknown = event.data
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  const value = data as Record<string, unknown>
  if (
    Object.keys(value).sort().join(',') !== 'height,token,type' ||
    value.type !== 'ircord:widget-size' ||
    value.token !== token
  )
    return null
  return typeof value.height === 'number' &&
    Number.isInteger(value.height) &&
    value.height >= 100 &&
    value.height <= 800
    ? value.height
    : null
}

type NativeContent = Pick<
  components['schemas']['LinkMediaResponse'],
  'url' | 'provider' | 'title' | 'author' | 'description'
>

/**
 * What a native post card shows above its media: a byline, the title only when it says
 * something (not Instagram's “Post by name” or TikTok's truncated caption), and the caption
 * with runs of blank lines collapsed.
 */
export function nativePostText(content: NativeContent) {
  const caption = (content.description ?? '')
    .replace(/[ \t]+\n/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
  let title = (content.title ?? '').trim()
  const generic = /^(post|photo|video|reel|carousel)s? by (\S+)$/i.exec(title)
  const handleFromTitle = generic?.[2]
  const truncated = title.replace(/(\.\.\.|…)$/, '').trim()
  if (generic || /^tiktok video$/i.test(title) || (truncated && caption.startsWith(truncated)))
    title = ''
  let name = (content.author ?? '').trim()
  let handle: string
  let path = ''
  try {
    path = new URL(content.url).pathname
  } catch {
    /* a relative or malformed URL has no path */
  }
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
  const ratios = assets.filter((a) => a.width && a.height).map((a) => a.width! / a.height!)
  const ratio = ratios.length ? Math.min(...ratios) : fallback
  return Math.min(16 / 9, Math.max(9 / 16, ratio))
}
