import type { GalleryItem, Message } from '../types/chat'
import { textOutsideCode } from './irc'
import { animatedImageHosts, gifProviderDomains, isTrustedMediaUrl } from './links'

/**
 * The images and GIFs a message contributes to its conversation's gallery. This is the
 * TypeScript copy of the backend's msgindex.Images, used by the demo backend; both are checked
 * against tests/fixtures/gallery-parity.json.
 */
export type GalleryEntry = Pick<
  GalleryItem,
  'url' | 'kind' | 'source' | 'fileId' | 'width' | 'height'
>

export const maxGalleryPerMessage = 10
const imagePath = /\.(png|jpe?g|webp|avif)$/i
const gifPath = /\.(gif|apng)$/i
const animatedPath = /\.(webp|avif)$/i
const filePath = /^\/files\/([A-Za-z0-9_-]{1,128})\/[^/]+\//

const animatedHost = (host: string) =>
  animatedImageHosts.includes(host.toLowerCase()) ||
  gifProviderDomains.some((d) => host.toLowerCase() === d || host.toLowerCase().endsWith(`.${d}`))

function httpsUrl(raw: string | undefined): URL | null {
  if (!raw || raw.length > 4096) return null
  try {
    const url = new URL(raw)
    return url.protocol === 'https:' && url.hostname && !url.username && !url.password ? url : null
  } catch {
    return null
  }
}

export function messageImages(message: Message): GalleryEntry[] {
  if (message.deleted || !['message', 'action', 'notice'].includes(message.kind)) return []
  const out: GalleryEntry[] = []
  const add = (entry: GalleryEntry) => {
    if (out.length < maxGalleryPerMessage && !out.some((e) => e.url === entry.url)) out.push(entry)
  }
  const a = message.attachment
  let upload = ''
  if (a?.url) {
    const ct = a.contentType ?? ''
    const ext = /\.[^./]*$/.exec(a.name)?.[0] ?? ''
    const named = !ct && gifPath.test(ext)
    if (ct.startsWith('image/') || named || (!ct && imagePath.test(ext))) {
      let fileId: string | undefined
      try {
        fileId = filePath.exec(new URL(a.url, 'https://invalid.example').pathname)?.[1]
      } catch {}
      upload = a.url
      add({
        url: a.url,
        kind: ct === 'image/gif' || ct === 'image/apng' || named ? 'gif' : 'image',
        source: 'upload',
        ...(fileId ? { fileId } : {}),
        ...(a.width ? { width: a.width } : {}),
        ...(a.height ? { height: a.height } : {}),
      })
    }
  }
  const g = message.gif
  if (g && httpsUrl(g.url))
    add({ url: g.url, kind: 'gif', source: 'gif', width: g.width, height: g.height })
  if (!g)
    for (const match of textOutsideCode(message.text).matchAll(/https?:\/\/[^\s<>"']+/g)) {
      const raw = match[0].replace(/[),.!?]+$/, '')
      if (upload && raw.endsWith(upload)) continue
      const url = httpsUrl(raw)
      if (!url) continue
      const kind =
        gifPath.test(url.pathname) ||
        (animatedPath.test(url.pathname) && animatedHost(url.hostname))
          ? 'gif'
          : imagePath.test(url.pathname)
            ? 'image'
            : null
      if (kind) add({ url: raw, kind, source: 'link' })
    }
  const p = message.previewSource === 'server' ? message.preview : undefined
  if (p?.media && httpsUrl(p.media.url))
    add({
      url: p.media.url,
      kind: 'gif',
      source: 'preview',
      ...(p.media.width ? { width: p.media.width } : {}),
      ...(p.media.height ? { height: p.media.height } : {}),
    })
  for (const image of p?.images ?? [])
    if (httpsUrl(image.url))
      add({
        url: image.url,
        kind: 'image',
        source: 'preview',
        ...(image.width ? { width: image.width } : {}),
        ...(image.height ? { height: image.height } : {}),
      })
  return out
}

/**
 * Whether a gallery image may load on this device, by the same rules as the timeline: uploads
 * and picked GIFs always do; server preview media follow the preview setting being on; direct
 * links follow it exactly (“trusted” loads only trusted hosts). Anything else is shown as a link
 * and nothing is requested from its host.
 */
export function galleryLoads(
  item: Pick<GalleryItem, 'url' | 'source'>,
  mode: 'off' | 'trusted' | 'all',
  origin: string,
  channelHosts: readonly string[] = [],
): boolean {
  if (item.source === 'upload' || item.source === 'gif') return true
  if (mode === 'off') return false
  if (item.source === 'preview' || mode === 'all') return true
  return isTrustedMediaUrl(item.url, origin, channelHosts)
}
