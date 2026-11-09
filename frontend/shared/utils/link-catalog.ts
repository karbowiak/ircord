import type { LinkCard, LinkKind } from '../types/chat'
import { textOutsideCode } from './irc'

/** Link kinds in filter order; labels belong to the active locale at the UI call site. */
export const linkKinds: { value: LinkKind; icon: string }[] = [
  { value: 'link', icon: 'link' },
  { value: 'image', icon: 'image' },
  { value: 'gif', icon: 'gif' },
  { value: 'video', icon: 'video' },
  { value: 'audio', icon: 'audio' },
  { value: 'youtube', icon: 'play' },
]

export type LinkCatalogTranslator = (
  key: string,
  values?: Record<string, string | number>,
  plural?: number,
) => string
const english = (key: string, values?: Record<string, string | number>, plural?: number) => {
  const messages: Record<string, string> = {
    'collectionsUi.links.hostHint': 'Enter a host such as example.com.',
    'collectionsUi.links.dateFormatError': 'Enter dates as shown, such as 2026-11-08.',
    'collectionsUi.links.dateOrderError': 'The end date is before the start date.',
    'collectionsUi.links.anyChannel': 'Any channel',
    'collectionsUi.links.oneChannel': '#{channel}',
    'collectionsUi.links.selectedChannels':
      'No channels selected | {count} channel selected | {count} channels selected',
  }
  const choices = (messages[key] ?? key).split('|').map((part) => part.trim())
  const selected =
    plural === undefined || choices.length === 1
      ? choices[0]
      : plural === 0
        ? choices[0]
        : plural === 1
          ? (choices[1] ?? choices[0])
          : (choices[2] ?? choices.at(-1) ?? choices[0])
  return Object.entries(values ?? {}).reduce(
    (message, [name, value]) => message.replaceAll(`{${name}}`, String(value)),
    selected ?? key,
  )
}

/** The card's heading uses the posted link's own host and path. */
export function linkTitle(card: Pick<LinkCard, 'url'>) {
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

/** Whether a link may be opened from the catalog: only web links, never script or data URLs. */
export const openableLink = (url: string) => /^https?:\/\/[^\s]+$/i.test(url)

export const LINKS_PAGE = 30

/** What the links view lists: some channels or all of them, narrowed by the filters. */
export interface LinkFilter {
  /** Channels to list; empty lists every channel. */
  conversations: string[]
  network: string
  /** Kinds to list; empty lists every kind. */
  kinds: LinkKind[]
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
  kind: filter.kinds.length ? filter.kinds : undefined,
  q: filter.q || undefined,
  host: filter.host || undefined,
  since: filter.since || undefined,
  until: filter.until || undefined,
  cursor,
  limit: LINKS_PAGE,
})

/** One host-name label: letters and digits (any script), hyphens only inside. */
const HOST_LABEL = /^[\p{L}\p{N}](?:[\p{L}\p{N}\p{M}-]{0,61}[\p{L}\p{N}\p{M}])?$/u

/** Whether the URL parser accepts a host (it rejects malformed IPv6 and punycode). */
function parsesAsHost(host: string) {
  try {
    new URL(`http://${host}/`)
    return true
  } catch {
    return false
  }
}

/**
 * The host filter as typed: a host name or IP address, or a pasted web address (its host is
 * used). Only well-formed names (letters, digits and inner hyphens per label) and addresses
 * pass, so most values the server would refuse get an explanation here instead of a failed
 * request. The server still has the last word, and normalizes case and punycode itself.
 */
export function linkHostFilter(
  raw: string,
  translate: LinkCatalogTranslator = english,
): { host: string } | { error: string } {
  let host = raw.trim()
  if (!host) return { host: '' }
  if (/^https?:\/\//i.test(host)) {
    try {
      host = new URL(host).hostname
    } catch {
      return { error: translate('collectionsUi.links.hostHint') }
    }
  }
  host = host.replace(/^\[(.*)\]$/, '$1').replace(/\.$/, '')
  if (!host || host.length > 253) return { error: translate('collectionsUi.links.hostHint') }
  if (host.includes(':'))
    return /^[0-9a-f:.]+$/i.test(host) && parsesAsHost(`[${host}]`)
      ? { host: host.toLowerCase() }
      : { error: translate('collectionsUi.links.hostHint') }
  if (!host.split('.').every((label) => HOST_LABEL.test(label)) || !parsesAsHost(host))
    return { error: translate('collectionsUi.links.hostHint') }
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
  translate: LinkCatalogTranslator = english,
): { since?: string; until?: string } | { error: string } {
  const start = from ? localDay(from) : undefined
  const end = to ? localDay(to, 1) : undefined
  if ((from && !start) || (to && !end))
    return { error: translate('collectionsUi.links.dateFormatError') }
  if (start && end && end <= start)
    return { error: translate('collectionsUi.links.dateOrderError') }
  return { since: start?.toISOString(), until: end?.toISOString() }
}

/**
 * Whether a date range can still gain posts: open-ended, or ending after now. A range ending
 * today ends at the next midnight, so links posted today still count as new.
 */
export const linkRangeOpen = (until: string | undefined, now = Date.now()) =>
  !until || Date.parse(until) > now

/** The server takes at most this many channels at once. */
export const MAX_LINK_CHANNELS = 50

/** Whether a channel's box may be ticked: it already is, or there is room for one more. */
export const channelSelectable = (picked: readonly string[], id: string, max = MAX_LINK_CHANNELS) =>
  picked.includes(id) || picked.length < max

/** What the channel picker's button says for the chosen channels. */
export function channelChoiceLabel(
  names: readonly string[],
  translate: LinkCatalogTranslator = english,
) {
  if (!names.length) return translate('collectionsUi.links.anyChannel')
  if (names.length === 1) return translate('collectionsUi.links.oneChannel', { channel: names[0]! })
  return translate('collectionsUi.links.selectedChannels', { count: names.length }, names.length)
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
