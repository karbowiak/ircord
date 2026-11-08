import type { LinkCard, LinkKind } from '../types/chat'
import { textOutsideCode } from './irc'
import { mediaLinks, youtubeThumbnail, youtubeVideo } from './links'

/** Link kinds as the links view names them, in filter order. */
export const linkKinds: { value: LinkKind; label: string; icon: string }[] = [
  { value: 'link', label: 'Pages', icon: 'link' },
  { value: 'image', label: 'Images', icon: 'image' },
  { value: 'gif', label: 'GIFs', icon: 'gif' },
  { value: 'video', label: 'Videos', icon: 'video' },
  { value: 'audio', label: 'Audio', icon: 'audio' },
  { value: 'youtube', label: 'YouTube', icon: 'play' },
]

const https = (value?: string) => !!value && /^https:\/\/[^\s]+$/i.test(value)

/** The card's heading: the stored preview title, otherwise the link's own host and path. */
export function linkTitle(card: Pick<LinkCard, 'url' | 'preview'>) {
  if (card.preview?.title) return card.preview.title
  try {
    const url = new URL(card.url)
    let path = url.pathname === '/' ? '' : url.pathname
    try {
      path = decodeURIComponent(path)
    } catch {
      // A malformed escape keeps the path as posted.
    }
    return `${url.hostname.replace(/^www\./, '')}${path}${url.search}`
  } catch {
    return card.url
  }
}

/**
 * The picture a card may show, under the same rules as the timeline: nothing while the
 * device's preview setting is off; direct images only from hosts that setting allows (the
 * built-in trusted hosts, this app's uploads and the channel's extra media hosts under
 * “trusted”); YouTube thumbnails from i.ytimg.com; a stored preview's HTTPS image otherwise.
 */
export function linkThumbnail(
  card: Pick<LinkCard, 'url' | 'kind' | 'preview'>,
  mode: 'off' | 'trusted' | 'all',
  origin: string,
  channelHosts: readonly string[] = [],
): string | undefined {
  if (mode === 'off') return undefined
  if (card.kind === 'youtube') {
    const video = youtubeVideo(card.url)
    return video ? youtubeThumbnail(video.id) : undefined
  }
  if (card.kind === 'image' || card.kind === 'gif')
    return mediaLinks(card.url, mode, origin, 1, channelHosts)[0]?.url
  return https(card.preview?.image) ? card.preview!.image : undefined
}

/** Whether a link may be opened from the catalog: only web links, never script or data URLs. */
export const openableLink = (url: string) => /^https?:\/\/[^\s]+$/i.test(url)

export const LINKS_PAGE = 30

/** What the links view lists: some channels or all of them, narrowed by the filters. */
export interface LinkFilter {
  /** Channels to list; empty lists every channel. */
  conversations: string[]
  network: string
  kind: LinkKind | ''
  q: string
  /** A host name or IP address; its subdomains match too. */
  host?: string
  /** Posted at or after this instant (ISO 8601). */
  since?: string
  /** Posted before this instant (ISO 8601). */
  until?: string
}

export const linkQueryParams = (filter: LinkFilter, cursor?: string) => ({
  conversation: filter.conversations.length ? filter.conversations : undefined,
  network: filter.network || undefined,
  kind: filter.kind ? [filter.kind] : undefined,
  q: filter.q || undefined,
  host: filter.host || undefined,
  since: filter.since || undefined,
  until: filter.until || undefined,
  cursor,
  limit: LINKS_PAGE,
})

const HOST_HINT = 'Enter a host such as example.com.'

/**
 * The host filter as typed: a host name or IP address, or a pasted web address (its host is
 * used). Rejects what the server would (paths, ports, credentials, spaces), so the view can say
 * why instead of failing the request. The server still normalizes case and punycode.
 */
export function linkHostFilter(raw: string): { host: string } | { error: string } {
  let host = raw.trim()
  if (!host) return { host: '' }
  if (/^https?:\/\//i.test(host)) {
    try {
      host = new URL(host).hostname
    } catch {
      return { error: HOST_HINT }
    }
  }
  host = host.replace(/^\[(.*)\]$/, '$1').replace(/\.$/, '')
  if (!host || host.length > 253) return { error: HOST_HINT }
  if (host.includes(':')) return /^[0-9a-f:.]+$/i.test(host) ? { host } : { error: HOST_HINT }
  if (/[\s/\\@?#[\]%]/.test(host) || host.startsWith('.') || host.includes('..'))
    return { error: HOST_HINT }
  return { host: host.toLowerCase() }
}

/** A calendar day (`YYYY-MM-DD`, as a date input gives it) as local midnight, or undefined. */
function localDay(day: string, offset = 0) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day)
  if (!m) return undefined
  const date = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]) + offset)
  return Number.isNaN(date.getTime()) ? undefined : date
}

/** Today as a date input's value, in local time. */
export function linkToday(now = new Date()) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

/**
 * The date filter: from the start of the first day to the end of the last, both inclusive and
 * in the viewer's time zone. Either end may be left open.
 */
export function linkDateRange(
  from: string,
  to: string,
): { since?: string; until?: string } | { error: string } {
  const start = from ? localDay(from) : undefined
  const end = to ? localDay(to, 1) : undefined
  if ((from && !start) || (to && !end))
    return { error: 'Enter dates as shown, such as 2026-11-08.' }
  if (start && end && end <= start) return { error: 'The end date is before the start date.' }
  return { since: start?.toISOString(), until: end?.toISOString() }
}

/** What the channel picker's button says for the chosen channels. */
export function channelChoiceLabel(names: readonly string[]) {
  if (!names.length) return 'Any channel'
  if (names.length === 1) return `#${names[0]}`
  return `${names.length} channels`
}

/** Whether a message's text has a link the catalog lists (outside code, like the backend). */
export const catalogsLinks = (text: string) => /https?:\/\//.test(textOutsideCode(text))

/** What a live message event means for the link catalog. */
export interface LinkEvent {
  updated: boolean
  /** Posted in a channel (the catalog lists channels only). */
  channel: boolean
  /** From someone the account ignores (the catalog leaves them out). */
  ignored: boolean
  deleted?: boolean
  text: string
  /** The client's copy before this event, when it had one. */
  previousText?: string
  /** One of the posts the open view lists. */
  listed: boolean
}

/**
 * `posted`: a new link the view may offer. `changed`: the list must be reread (a listed post
 * was deleted or its text changed, or an edit added a link). Reactions and resolved previews
 * leave the text alone and mean nothing here.
 */
export function linkEventEffect(e: LinkEvent): 'posted' | 'changed' | null {
  if (!e.channel) return null
  if (!e.updated) return !e.ignored && catalogsLinks(e.text) ? 'posted' : null
  const textChanged = e.previousText === undefined || e.previousText !== e.text
  if (e.listed && (e.deleted || textChanged)) return 'changed'
  if (!e.deleted && textChanged && catalogsLinks(e.text)) return 'changed'
  return null
}

/**
 * How many new link posts a view shows: in its channels (every channel when none are named),
 * on its network when it has one. `posted` counts posts per conversation.
 */
export function countPosted(
  posted: ReadonlyMap<string, number>,
  conversations: readonly string[],
  network: string,
  networkOf: (conversationId: string) => string | undefined,
) {
  let n = 0
  for (const [id, count] of posted)
    if (
      (!conversations.length || conversations.includes(id)) &&
      (!network || networkOf(id) === network)
    )
      n += count
  return n
}
