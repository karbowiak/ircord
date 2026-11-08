import type { LinkCard, LinkKind } from '../types/chat'
import { textOutsideCode } from './irc'

/** Link kinds as the links view names them, in filter order. */
export const linkKinds: { value: LinkKind; label: string; icon: string }[] = [
  { value: 'link', label: 'Pages', icon: 'link' },
  { value: 'image', label: 'Images', icon: 'image' },
  { value: 'gif', label: 'GIFs', icon: 'gif' },
  { value: 'video', label: 'Videos', icon: 'video' },
  { value: 'audio', label: 'Audio', icon: 'audio' },
  { value: 'youtube', label: 'YouTube', icon: 'play' },
]

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

/** What the links view lists: one channel or all of them, narrowed by the filters. */
export interface LinkFilter {
  /** Channels to list; empty lists every channel. */
  conversations: string[]
  network: string
  kind: LinkKind | ''
  q: string
}

export const linkQueryParams = (filter: LinkFilter, cursor?: string) => ({
  conversation: filter.conversations.length ? filter.conversations : undefined,
  network: filter.network || undefined,
  kind: filter.kind ? [filter.kind] : undefined,
  q: filter.q || undefined,
  cursor,
  limit: LINKS_PAGE,
})

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
