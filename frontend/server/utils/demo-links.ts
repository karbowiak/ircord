/**
 * Demo counterpart of Go's link catalog (backend/internal/msgindex/links.go and
 * storage/catalog.go): links outside code in the session's channel messages, grouped by
 * resource, newest first. Built on request from the in-memory history, which is small and
 * bounded. Known simplifications against Go: identities use the WHATWG URL path (which
 * normalizes `..` and some escapes), IRC formatting codes are not link terminators, the host
 * filter is not punycode-normalized.
 */
import type { LinkCard, LinkKind, LinkPage, LinkSource, Message } from '../../shared/types/chat'
import { stripFormatting, textOutsideCode } from '../../shared/utils/irc'
import type { DemoSession } from './demo-store'

interface Found {
  url: string
  identity: string
  host: string
  kind: LinkKind
}

/** Catalog identity only: no metadata fetch or media rendering. */
function youtubeID(url: URL): string | undefined {
  if (url.protocol !== 'https:' || url.username || url.password) return
  const parts = url.pathname.replace(/^\/+|\/+$/g, '').split('/')
  let id: string | undefined
  if (url.hostname === 'youtu.be' && parts.length === 1) id = parts[0]
  else if (
    [
      'youtube.com',
      'www.youtube.com',
      'm.youtube.com',
      'music.youtube.com',
      'youtube-nocookie.com',
      'www.youtube-nocookie.com',
    ].includes(url.hostname)
  ) {
    if (parts.length === 1 && parts[0] === 'watch') id = url.searchParams.get('v') ?? undefined
    else if (parts.length === 2 && ['embed', 'live', 'v', 'shorts'].includes(parts[0]!))
      id = parts[1]
  }
  return id && /^[A-Za-z0-9_-]{11}$/.test(id) ? id : undefined
}

function trimTrailing(raw: string) {
  for (;;) {
    const last = raw.at(-1)
    if (last && ".,;:!?'*_~`".includes(last)) raw = raw.slice(0, -1)
    else if (last && ')]}'.includes(last)) {
      const open = { ')': '(', ']': '[', '}': '{' }[last]!
      if (raw.split(open).length >= raw.split(last).length) return raw
      raw = raw.slice(0, -1)
    } else return raw
  }
}

function kindOf(url: URL): LinkKind {
  if (youtubeID(url)) return 'youtube'
  const animatedHost =
    [
      'media.tenor.com',
      'media.giphy.com',
      'i.giphy.com',
      'static.klipy.com',
      'cdn.7tv.app',
    ].includes(url.hostname) || /(^|\.)(tenor|giphy)\.com$/.test(url.hostname)
  if (/\.(gif|apng)$/i.test(url.pathname) || (animatedHost && /\.(webp|avif)$/i.test(url.pathname)))
    return 'gif'
  if (/\.(png|jpe?g|webp|avif)$/i.test(url.pathname)) return 'image'
  if (/\.(mp4|webm|mov|m4v)$/i.test(url.pathname)) return 'video'
  if (/\.(mp3|m4a|aac|ogg|oga|opus|wav|flac)$/i.test(url.pathname)) return 'audio'
  return 'link'
}

export function catalogLinks(text: string): Found[] {
  const found: Found[] = []
  for (const match of textOutsideCode(text).matchAll(/https?:\/\/[^\s<>"]+/g)) {
    const raw = trimTrailing(match[0])
    let url: URL
    try {
      url = new URL(raw)
    } catch {
      continue
    }
    if (url.username || url.password || raw.length > 2048) continue
    if (found.some((f) => f.url === raw)) continue
    const kind = kindOf(url)
    const video = kind === 'youtube' ? youtubeID(url) : undefined
    const identity = video
      ? `youtube:${video}`
      : `${url.protocol}//${url.host.toLowerCase()}${url.pathname}${url.search}`
    found.push({ url: raw, identity, host: url.hostname.toLowerCase(), kind })
    if (found.length === 20) break
  }
  return found
}

const linkable = (m: Message) =>
  !m.deleted && (m.kind === 'message' || m.kind === 'action' || m.kind === 'notice')

function postedWithin(createdAt: string, since: number, until: number) {
  const at = Date.parse(createdAt)
  return (!since || at >= since) && (!until || at < until)
}

export function listLinks(
  session: DemoSession,
  filter: {
    conversations: string[]
    network: string
    q: string
    host: string
    kinds: string[]
    /** Posted at or after (ms since the epoch), or before; 0 leaves that end open. */
    since: number
    until: number
  },
  cursor: string,
  limit: number,
): LinkPage {
  const groups = new Map<string, { card: LinkCard; posts: Set<string> }>()
  const channels = session.bootstrap.conversations.filter(
    (c) =>
      c.kind === 'channel' &&
      (!filter.conversations.length || filter.conversations.includes(c.id)) &&
      (!filter.network || c.networkId === filter.network),
  )
  const q = filter.q.toLowerCase()
  const posts: { message: Message; link: Found; networkId: string }[] = []
  for (const c of channels)
    for (const message of session.messages[c.id] ?? [])
      if (linkable(message) && postedWithin(message.createdAt, filter.since, filter.until))
        for (const link of catalogLinks(message.text))
          if (
            (!filter.kinds.length || filter.kinds.includes(link.kind)) &&
            (!filter.host || link.host === filter.host || link.host.endsWith(`.${filter.host}`)) &&
            (!q || link.url.toLowerCase().includes(q) || message.text.toLowerCase().includes(q))
          )
            posts.push({ message, link, networkId: c.networkId })
  posts.sort((a, b) => b.message.createdAt.localeCompare(a.message.createdAt))
  for (const { message, link, networkId } of posts) {
    let group = groups.get(link.identity)
    if (!group) {
      group = {
        card: {
          identity: link.identity,
          url: link.url,
          host: link.host,
          kind: link.kind,
          count: 0,
          lastPostedAt: message.createdAt,
          sources: [],
        },
        posts: new Set(),
      }
      groups.set(link.identity, group)
    }
    if (group.posts.has(message.id)) continue
    group.posts.add(message.id)
    group.card.count++
    if (group.card.sources.length < 5) {
      const source: LinkSource = {
        messageId: message.id,
        conversationId: message.conversationId,
        networkId,
        senderId: message.senderId,
        postedAt: message.createdAt,
        url: link.url,
        excerpt: stripFormatting(message.text).slice(0, 280),
      }
      group.card.sources.push(source)
    }
  }
  const cards = [...groups.values()].map((g) => g.card)
  const start = cursor ? cards.findIndex((c) => c.identity === cursor) + 1 : 0
  if (cursor && start === 0) throw createError({ statusCode: 400, statusMessage: 'invalid cursor' })
  const items = cards.slice(start, start + limit)
  return {
    items,
    indexing: false,
    nextCursor: start + limit < cards.length ? items.at(-1)!.identity : null,
  }
}
