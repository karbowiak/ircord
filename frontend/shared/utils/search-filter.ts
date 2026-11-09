import type { Member, Message } from '../types/chat'
import { textOutsideCode } from './irc'

/**
 * Message search filters, Discord-style: free text plus `from:`, `mentions:`, `in:`, `has:`,
 * `before:`, `after:`, `on:` and `author:`. People and channels are kept by ID; dates are local
 * calendar days (YYYY-MM-DD).
 */
export const searchHasKinds = [
  { value: 'link', icon: 'link' },
  { value: 'image', icon: 'image' },
  { value: 'gif', icon: 'gif' },
  { value: 'video', icon: 'video' },
  { value: 'file', icon: 'file' },
] as const
export type SearchHas = (typeof searchHasKinds)[number]['value']
export type SearchAuthor = 'user' | 'bot'

export interface SearchFilters {
  from: string[]
  mentions: string[]
  in: string[]
  has: SearchHas[]
  before?: string
  after?: string
  on?: string
  author?: SearchAuthor
}
export const emptyFilters = (): SearchFilters => ({ from: [], mentions: [], in: [], has: [] })

export const filterCount = (f: SearchFilters) =>
  f.from.length +
  f.mentions.length +
  f.in.length +
  f.has.length +
  [f.before, f.after, f.on, f.author].filter(Boolean).length

/** One removable chip per value, in the order the search field shows them. */
export type FilterChip =
  | { key: 'from' | 'mentions' | 'in'; value: string }
  | { key: 'has'; value: SearchHas }
  | { key: 'before' | 'after' | 'on'; value: string }
  | { key: 'author'; value: SearchAuthor }
export function filterChips(f: SearchFilters): FilterChip[] {
  return [
    ...f.from.map((value) => ({ key: 'from' as const, value })),
    ...f.mentions.map((value) => ({ key: 'mentions' as const, value })),
    ...f.in.map((value) => ({ key: 'in' as const, value })),
    ...f.has.map((value) => ({ key: 'has' as const, value })),
    ...(['before', 'after', 'on'] as const).flatMap((key) =>
      f[key] ? [{ key, value: f[key]! }] : [],
    ),
    ...(f.author ? [{ key: 'author' as const, value: f.author }] : []),
  ]
}
export function withoutChip(f: SearchFilters, chip: FilterChip): SearchFilters {
  const next = {
    ...f,
    from: [...f.from],
    mentions: [...f.mentions],
    in: [...f.in],
    has: [...f.has],
  }
  if (chip.key === 'from' || chip.key === 'mentions' || chip.key === 'in')
    next[chip.key] = next[chip.key].filter((v) => v !== chip.value)
  else if (chip.key === 'has') next.has = next.has.filter((v) => v !== chip.value)
  else delete next[chip.key]
  return next
}
export function withChip(f: SearchFilters, chip: FilterChip): SearchFilters {
  const next = withoutChip(f, chip)
  if (chip.key === 'from' || chip.key === 'mentions' || chip.key === 'in')
    next[chip.key] = [...next[chip.key], chip.value]
  else if (chip.key === 'has') next.has = [...next.has, chip.value]
  else if (chip.key === 'author') next.author = chip.value
  else {
    next[chip.key] = chip.value
    // A single day replaces a range and a range replaces a single day.
    if (chip.key === 'on') {
      delete next.before
      delete next.after
    } else delete next.on
  }
  return next
}

/** The token keys the field recognises while typing, with what each one picks. */
export const filterKeys = [
  { key: 'from', icon: 'user' },
  { key: 'mentions', icon: 'at' },
  { key: 'in', icon: 'hash' },
  { key: 'has', icon: 'attachment' },
  { key: 'before', icon: 'calendar' },
  { key: 'after', icon: 'calendar' },
  { key: 'on', icon: 'calendar' },
  { key: 'author', icon: 'brand' },
] as const
export type FilterKey = (typeof filterKeys)[number]['key']

/** The token being typed at the end of the text, such as `from:jul` → key from, value jul. */
export function activeToken(text: string): { key: FilterKey; value: string; start: number } | null {
  const match = /(^|\s)(from|mentions|in|has|before|after|on|author):(\S*)$/i.exec(text)
  if (!match) return null
  return {
    key: match[2]!.toLowerCase() as FilterKey,
    value: match[3]!,
    start: match.index + match[1]!.length,
  }
}

export const isDay = (value: string) =>
  /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(new Date(`${value}T00:00:00`).getTime())

const startOfDay = (day: string) => new Date(`${day}T00:00:00`).getTime()
const nextDay = (day: string) => {
  const date = new Date(startOfDay(day))
  date.setDate(date.getDate() + 1)
  return date.toISOString()
}

/** What the API is asked for. Dates become instants at local midnight, as the user means them. */
export function searchParams(text: string, f: SearchFilters) {
  return {
    q: text.trim() || undefined,
    from: f.from.length ? f.from : undefined,
    mentions: f.mentions.length ? f.mentions : undefined,
    conversationId: f.in.length ? f.in : undefined,
    has: f.has.length ? f.has : undefined,
    before: f.on
      ? nextDay(f.on)
      : f.before
        ? new Date(startOfDay(f.before)).toISOString()
        : undefined,
    after: f.on ? new Date(startOfDay(f.on)).toISOString() : f.after ? nextDay(f.after) : undefined,
    author: f.author,
  }
}
export type SearchParams = ReturnType<typeof searchParams>

export const canSearch = (text: string, filters: SearchFilters) =>
  [...text.trim()].length >= 2 || filterCount(filters) > 0

// eslint-disable-next-line no-control-regex -- control characters end a URL in IRC text
const URL_PATTERN = /\bhttps?:\/\/[^\s<>"'\x00-\x1f]+/g
const IMAGE = /\.(png|jpe?g|webp|avif|heic)(\?|#|$)/i
const GIF = /\.(gif|apng)(\?|#|$)|(^|\/\/|\.)(tenor|giphy)\.com\//i
const VIDEO = /\.(mp4|webm|mov|m4v)(\?|#|$)|(^|\/\/|\.)(youtube\.com|youtu\.be|vimeo\.com)\//i

/** Which kinds of content a message has, for `has:`. */
export function messageHas(message: Message): Set<SearchHas> {
  const kinds = new Set<SearchHas>()
  const urls = textOutsideCode(message.text).match(URL_PATTERN) ?? []
  if (urls.length) kinds.add('link')
  for (const url of urls) {
    if (GIF.test(url)) kinds.add('gif')
    else if (IMAGE.test(url)) kinds.add('image')
    else if (VIDEO.test(url)) kinds.add('video')
  }
  if (message.gif) kinds.add('gif')
  const type = message.attachment?.contentType?.split(';')[0]?.trim().toLowerCase() ?? ''
  if (message.attachment) {
    if (type === 'image/gif') kinds.add('gif')
    else if (type.startsWith('image/')) kinds.add('image')
    else if (type.startsWith('video/')) kinds.add('video')
    else kinds.add('file')
  }
  return kinds
}

const mentionPattern = (nick: string) =>
  new RegExp(
    `(^|[^\\p{L}\\p{N}_-])@?${nick.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}($|[^\\p{L}\\p{N}_-])`,
    'iu',
  )

/** Demo-mode equivalent of the API's literal phrase and final-word prefix. */
export const searchTokens = (value: string) =>
  Array.from(value.toLowerCase(), (letter) =>
    /\p{Script=Latin}/u.test(letter) ? letter.normalize('NFD').replace(/\p{M}/gu, '') : letter,
  )
    .join('')
    .replace(/(\p{Script=Latin})\p{M}+/gu, '$1')
    .match(/[\p{L}\p{N}\p{Co}]+/gu) ?? []

export function searchTextMatches(text: string, query: string) {
  const words = searchTokens(text),
    terms = searchTokens(query)
  return (
    terms.length > 0 &&
    words.some((_, i) =>
      terms.every((term, j) =>
        j === terms.length - 1 ? words[i + j]?.startsWith(term) : words[i + j] === term,
      ),
    )
  )
}

/**
 * Demo-mode search filtering. Live search is handled entirely by the backend.
 */
export function matchesSearch(
  message: Message,
  text: string,
  f: SearchFilters,
  member: (id: string) => Member | undefined,
) {
  const q = text.trim().toLowerCase()
  if (q && !searchTextMatches(message.text, q)) return false
  if (f.from.length && !f.from.includes(message.senderId)) return false
  if (f.in.length && !f.in.includes(message.conversationId)) return false
  if (f.mentions.length) {
    const named = f.mentions.some((id) => {
      const nick = member(id)?.nick
      return !!nick && mentionPattern(nick).test(message.text)
    })
    if (!named) return false
  }
  if (f.has.length) {
    const has = messageHas(message)
    if (!f.has.some((kind) => has.has(kind))) return false
  }
  const at = new Date(message.createdAt).getTime()
  const p = searchParams('', f)
  if (p.before && at >= new Date(p.before).getTime()) return false
  if (p.after && at < new Date(p.after).getTime()) return false
  if (f.author && (member(message.senderId)?.role === 'bot') !== (f.author === 'bot')) return false
  return true
}

/** A past search, kept on this device to run again. */
export interface RecentSearch {
  text: string
  filters: SearchFilters
}
export const MAX_RECENT_SEARCHES = 8
export function sanitizeRecent(value: unknown): RecentSearch[] {
  if (!Array.isArray(value)) return []
  const strings = (v: unknown) =>
    Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string').slice(0, 20) : []
  const day = (v: unknown) => (typeof v === 'string' && isDay(v) ? v : undefined)
  return value
    .filter(
      (r): r is { text?: unknown; filters?: Record<string, unknown> } =>
        !!r && typeof r === 'object',
    )
    .map((r): RecentSearch => {
      const f = (r.filters ?? {}) as Record<string, unknown>
      const kinds = searchHasKinds.map((k) => k.value as string)
      return {
        text: typeof r.text === 'string' ? r.text.slice(0, 200) : '',
        filters: {
          from: strings(f.from),
          mentions: strings(f.mentions),
          in: strings(f.in),
          has: strings(f.has).filter((k): k is SearchHas => kinds.includes(k)),
          before: day(f.before),
          after: day(f.after),
          on: day(f.on),
          author: f.author === 'bot' || f.author === 'user' ? f.author : undefined,
        },
      }
    })
    .filter((r) => r.text || filterCount(r.filters))
    .slice(0, MAX_RECENT_SEARCHES)
}
/** Newest first, without repeating the same search. */
export function rememberSearch(list: RecentSearch[], entry: RecentSearch) {
  const same = JSON.stringify(entry)
  return [entry, ...list.filter((r) => JSON.stringify(r) !== same)].slice(0, MAX_RECENT_SEARCHES)
}
