import type { components } from '../../shared/generated/api'

type Content = components['schemas']['LinkMediaResponse']
export type LinkMediaIdentity = {
  account: string
  conversation: string
  message: string
  url: string
  widget: boolean
}
type Entry = {
  identity: LinkMediaIdentity
  content: Content
  savedAt: number
  expiresAt: number
  bytes: number
}

const storageKey = 'ircord.linkMedia'
const version = 1
export const linkMediaFreshTime = 5 * 60_000
// Older metadata can bridge a refresh, but never outlive any resource it contains.
const retention = 15 * 60_000
const maxEntries = 64
const maxBytes = 128 * 1024
const maxEntryBytes = 24 * 1024
// External players do not use the server document's size-message capability.
const externalToken = '0'.repeat(48)
const entries = new Map<string, Entry>()
let owner = ''
let generation = 0
let timer: ReturnType<typeof setTimeout> | undefined
let listening = false

function record(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    throw new Error('Invalid metadata')
  return value as Record<string, unknown>
}
function text(value: unknown, limit: number, required = false): string | undefined {
  if (value === undefined && !required) return undefined
  if (typeof value !== 'string' || value.length > limit || (required && !value))
    throw new Error('Invalid metadata text')
  return value
}
function number(value: unknown, limit: number, required = false): number | undefined {
  if (value === undefined && !required) return undefined
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0 || value > limit)
    throw new Error('Invalid metadata number')
  return value
}
function deadline(value: unknown, now: number): string | undefined {
  if (value === undefined) return undefined
  const date = text(value, 64, true)!
  if (!Number.isFinite(Date.parse(date)) || Date.parse(date) <= now)
    throw new Error('Expired metadata')
  return date
}
function url(value: unknown, required = false): string | undefined {
  const raw = text(value, 4096, required)
  if (!raw && !required) return raw
  const parsed = new URL(raw!)
  // eslint-disable-next-line no-control-regex -- reject whitespace and controls in stored URLs
  const controls = /[\u0000-\u0020\u007f]/.test(raw!)
  if (
    !/^https?:$/.test(parsed.protocol) ||
    parsed.username ||
    parsed.password ||
    raw !== raw!.trim() ||
    controls ||
    // Upload capabilities and server widget documents must always use their live checks.
    /\/files\/[a-f0-9]{32}\/[a-f0-9]{48}(?:\/|$)/i.test(parsed.pathname) ||
    /\/api\//i.test(parsed.pathname)
  )
    throw new Error('Uncacheable metadata URL')
  return raw
}
function identity(value: unknown): LinkMediaIdentity {
  const item = record(value)
  if (typeof item.widget !== 'boolean') throw new Error('Invalid metadata identity')
  return {
    account: text(item.account, 256, true)!,
    conversation: text(item.conversation, 256, true)!,
    message: text(item.message, 256, true)!,
    url: url(item.url, true)!,
    widget: item.widget,
  }
}
function key(value: LinkMediaIdentity) {
  return JSON.stringify([value.account, value.conversation, value.message, value.url, value.widget])
}

/** Whitelist the wire fields: never carry HTML, unknown fields, or document tokens to disk. */
function content(value: unknown, original: string, now: number): Content {
  const item = record(value)
  if (item.url !== original || !Array.isArray(item.assets) || item.assets.length > 30)
    throw new Error('Invalid metadata source')
  const result: Content = {
    url: url(item.url, true)!,
    provider: text(item.provider, 128, true)!,
    title: text(item.title, 1024),
    description: text(item.description, 4096),
    author: text(item.author, 512),
    kind: item.kind as Content['kind'],
    assets: item.assets.map((value) => {
      const asset = record(value)
      if (asset.kind !== 'image' && asset.kind !== 'video' && asset.kind !== 'audio')
        throw new Error('Invalid media kind')
      return {
        kind: asset.kind,
        url: url(asset.url, true)!,
        contentType: text(asset.contentType, 256),
        width: number(asset.width, 100_000),
        height: number(asset.height, 100_000),
        duration: number(asset.duration, 365 * 24 * 60 * 60),
        expiresAt: deadline(asset.expiresAt, now),
      }
    }),
  }
  if (item.kind === 'media') {
    if (!result.assets.length) throw new Error('Missing media')
  } else if (item.kind === 'card') {
    const card = record(item.card)
    // Failed resolutions should retry rather than survive a reload as a cached failure.
    if (card.reason) throw new Error('Unavailable preview')
    result.card = {
      site: text(card.site, 512, true)!,
      title: text(card.title, 1024, true)!,
      description: text(card.description, 4096),
      image: url(card.image),
      icon: url(card.icon),
      imageWidth: number(card.imageWidth, 100_000),
      imageHeight: number(card.imageHeight, 100_000),
      expiresAt: deadline(card.expiresAt, now),
    }
  } else if (item.kind === 'file') {
    const file = record(item.file)
    if (file.reason) throw new Error('Unavailable file')
    result.file = {
      name: text(file.name, 1024, true)!,
      contentType: text(file.contentType, 256, true)!,
      size: number(file.size, Number.MAX_SAFE_INTEGER, true)!,
      url: url(file.url, true)!,
      expiresAt: deadline(file.expiresAt, now),
    }
  } else if (item.kind === 'widget') {
    const widget = record(item.widget)
    const source = url(widget.url, true)!
    const parsed = new URL(source)
    if (
      widget.external !== true ||
      parsed.protocol !== 'https:' ||
      parsed.origin === window.location.origin ||
      !widget.expiresAt ||
      (widget.clickToLoad !== undefined && typeof widget.clickToLoad !== 'boolean')
    )
      throw new Error('Uncacheable widget')
    result.widget = {
      url: source,
      external: true,
      token: externalToken,
      height: number(widget.height, 800, true)!,
      aspectRatio: number(widget.aspectRatio, 20),
      clickToLoad: widget.clickToLoad as boolean | undefined,
      thumbnail: url(widget.thumbnail),
      expiresAt: deadline(widget.expiresAt, now),
    }
    if (result.widget.height < 100 || (result.widget.aspectRatio ?? 1) < 0.05)
      throw new Error('Invalid widget dimensions')
  } else throw new Error('Invalid preview kind')
  return result
}
function expiresAt(value: Content, savedAt: number) {
  const deadlines = [
    ...value.assets.map((asset) => asset.expiresAt),
    value.card?.expiresAt,
    value.file?.expiresAt,
    value.widget?.expiresAt,
  ]
    .filter((value): value is string => !!value)
    .map(Date.parse)
  return Math.min(savedAt + retention, ...deadlines)
}
function stored(entry: Pick<Entry, 'identity' | 'content' | 'savedAt'>) {
  // A fresh server response supplies a new document token. External hydration needs none.
  const widget = entry.content.widget
  return {
    identity: entry.identity,
    savedAt: entry.savedAt,
    content: {
      ...entry.content,
      widget: widget
        ? {
            url: widget.url,
            external: true,
            height: widget.height,
            aspectRatio: widget.aspectRatio,
            clickToLoad: widget.clickToLoad,
            thumbnail: widget.thumbnail,
            expiresAt: widget.expiresAt,
          }
        : undefined,
    },
  }
}
function makeEntry(source: LinkMediaIdentity, value: unknown, savedAt: number, now: number): Entry {
  const preview = content(value, source.url, now)
  const entry = {
    identity: source,
    content: preview,
    savedAt,
    expiresAt: expiresAt(preview, savedAt),
  }
  const bytes = JSON.stringify(stored(entry)).length * 2
  if (bytes > maxEntryBytes) throw new Error('Oversized preview')
  return { ...entry, bytes }
}
function removeStored() {
  try {
    localStorage.removeItem(storageKey)
  } catch {
    /* Site storage is optional. */
  }
}
function trim() {
  const now = Date.now()
  for (const [key, entry] of entries) if (entry.expiresAt <= now) entries.delete(key)
  // Include envelope/array overhead; serialization and storage writes stay small as well.
  let bytes = 4096
  for (const entry of entries.values()) bytes += entry.bytes + 2
  while (entries.size > maxEntries || bytes > maxBytes) {
    const oldest = entries.keys().next().value!
    bytes -= entries.get(oldest)!.bytes + 2
    entries.delete(oldest)
  }
}
function flush() {
  clearTimeout(timer)
  timer = undefined
  if (!owner) return
  trim()
  const raw = JSON.stringify({
    version,
    account: owner,
    entries: [...entries.values()].map(stored),
  })
  try {
    if (entries.size && raw.length * 2 <= maxBytes) localStorage.setItem(storageKey, raw)
    else removeStored()
  } catch {
    /* Quotas and disabled storage must not interrupt previews. */
  }
}
function schedule() {
  if (!timer) timer = setTimeout(flush, 250)
}
function pageHidden() {
  if (timer) flush()
}
function storageChanged(event: StorageEvent) {
  if (event.key === storageKey && event.newValue === null) reset()
}

/** One bounded slot for the current account; changing owners discards the previous slot. */
function open(account: string) {
  if (!account || account === owner || typeof window === 'undefined') return
  clearTimeout(timer)
  timer = undefined
  entries.clear()
  owner = account
  generation++
  if (!listening) {
    listening = true
    window.addEventListener('pagehide', pageHidden)
    window.addEventListener('storage', storageChanged)
  }
  try {
    const raw = localStorage.getItem(storageKey)
    if (!raw) return
    if (raw.length * 2 > maxBytes) throw new Error('Oversized preview cache')
    const cache = record(JSON.parse(raw))
    if (
      cache.version !== version ||
      cache.account !== account ||
      !Array.isArray(cache.entries) ||
      cache.entries.length > maxEntries
    )
      throw new Error('Obsolete preview cache')
    const now = Date.now()
    for (const value of cache.entries) {
      try {
        const item = record(value)
        if (JSON.stringify(item).length * 2 > maxEntryBytes) continue
        const source = identity(item.identity)
        const savedAt = number(item.savedAt, now, true)!
        if (source.account !== account || savedAt + retention <= now) continue
        entries.set(key(source), makeEntry(source, item.content, savedAt, now))
      } catch {
        /* A corrupt or expired entry does not discard the other previews. */
      }
    }
    // Also discard expired/invalid persisted entries without scanning unrelated site data.
    schedule()
  } catch {
    entries.clear()
    removeStored()
  }
}
function reset() {
  clearTimeout(timer)
  timer = undefined
  entries.clear()
  owner = ''
  generation++
}

export function clearLinkMediaCache() {
  reset()
  removeStored()
}

if (import.meta.hot)
  import.meta.hot.dispose(() => {
    if (timer) flush()
    reset()
    if (listening) {
      window.removeEventListener('pagehide', pageHidden)
      window.removeEventListener('storage', storageChanged)
      listening = false
    }
  })

export const linkMediaCache = {
  /** Reuse only dimensions, within the same account, when the same source is posted again. */
  imageSize(account: string, raw: string) {
    open(account)
    trim()
    if (!account || account !== owner) return undefined
    let size: { width: number; height: number } | undefined
    for (const entry of entries.values()) {
      if (entry.identity.url !== raw || entry.content.kind !== 'media') continue
      const [asset] = entry.content.assets
      if (
        entry.content.assets.length === 1 &&
        asset?.kind === 'image' &&
        asset.width &&
        asset.height
      )
        size = { width: asset.width, height: asset.height }
    }
    return size
  },
  read(source: LinkMediaIdentity) {
    open(source.account)
    trim()
    const entry = source.account === owner ? entries.get(key(source)) : undefined
    return entry
      ? {
          data: entry.content,
          updatedAt: Math.min(entry.savedAt, entry.expiresAt - linkMediaFreshTime),
        }
      : undefined
  },
  generation: () => generation,
  write(source: LinkMediaIdentity, value: Content, ticket: number) {
    if (ticket !== generation || source.account !== owner) return
    const entryKey = key(source)
    entries.delete(entryKey)
    try {
      const normalized = identity(source)
      const now = Date.now()
      entries.set(entryKey, makeEntry(normalized, value, now, now))
    } catch {
      /* Sensitive, invalid, and unavailable responses stay in the live query only. */
    }
    trim()
    schedule()
  },
  evict(source: LinkMediaIdentity, ticket: number) {
    if (ticket === generation && source.account === owner && entries.delete(key(source))) schedule()
  },
}
