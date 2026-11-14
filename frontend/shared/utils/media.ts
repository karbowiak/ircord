import type { EmojiItem, GifItem, GifPage, InstanceCatalog } from '../types/media'
import { klipyMediaHosts } from './gif-hosts'

export function isKlipyMediaUrl(value: unknown): value is string {
  if (typeof value !== 'string' || value.length > 4096) return false
  try {
    const url = new URL(value)
    return (
      url.protocol === 'https:' &&
      (klipyMediaHosts as readonly string[]).includes(url.hostname) &&
      !url.username &&
      !url.password
    )
  } catch {
    return false
  }
}
export function validGif(value: unknown): value is GifItem {
  if (!value || typeof value !== 'object') return false
  const item = value as GifItem
  return (
    item.provider === 'klipy' &&
    typeof item.id === 'string' &&
    item.id.length > 0 &&
    item.id.length <= 100 &&
    typeof item.slug === 'string' &&
    /^[\w-]{1,180}$/.test(item.slug) &&
    typeof item.title === 'string' &&
    item.title.length <= 200 &&
    isKlipyMediaUrl(item.url) &&
    isKlipyMediaUrl(item.previewUrl) &&
    isKlipyMediaUrl(item.stillUrl) &&
    Number.isFinite(item.width) &&
    item.width > 0 &&
    item.width <= 8192 &&
    Number.isFinite(item.height) &&
    item.height > 0 &&
    item.height <= 8192
  )
}

export function normalizeKlipyPage(
  response: unknown,
  translate?: (key: string) => string,
): GifPage {
  const message = (key: string, fallback: string) => translate?.(`mediaUi.klipy.${key}`) ?? fallback
  /* eslint-disable @typescript-eslint/no-explicit-any -- Klipy rows are third-party JSON read defensively */
  const result = response as {
    result?: boolean
    data?:
      | {
          data?: Record<string, any>[]
          current_page?: number
          has_next?: boolean
          next_page_url?: string
          last_page?: number
        }
      | Record<string, any>[]
  }
  /* eslint-enable @typescript-eslint/no-explicit-any */
  if (!result || result.result === false || !result.data)
    throw new Error(message('loadFailed', 'Klipy could not load these GIFs. Please try again.'))
  const wrapper = Array.isArray(result.data) ? undefined : result.data
  const rows = Array.isArray(result.data) ? result.data : result.data.data
  if (!Array.isArray(rows))
    throw new Error(message('unreadable', 'Klipy returned an unreadable response.'))
  const items = rows.map((row) => {
    const file = row.file ?? {}
    const preview =
      file.sm?.webp ??
      file.xs?.webp ??
      file.sm?.gif ??
      file.md?.webp ??
      file.hd?.webp ??
      file.hd?.gif
    const full = file.hd?.gif ?? file.md?.gif ?? file.hd?.webp ?? preview
    const still = file.sm?.jpg ?? file.xs?.jpg ?? file.hd?.jpg ?? file.sm?.png ?? preview
    const item: GifItem = {
      id: String(row.id ?? row.slug),
      slug: String(row.slug ?? ''),
      title: String(row.title ?? 'GIF').slice(0, 200),
      url: full?.url,
      previewUrl: preview?.url,
      stillUrl: still?.url,
      width: Number(full?.width ?? 320),
      height: Number(full?.height ?? 240),
      provider: 'klipy',
    }
    if (!validGif(item))
      throw new Error(message('unsafeGif', 'Klipy returned a GIF that cannot be displayed safely.'))
    return item
  })
  const page = Number(wrapper?.current_page ?? 1)
  const hasNext = wrapper?.has_next ?? Boolean(wrapper?.next_page_url)
  return {
    items,
    nextPage: hasNext || (wrapper?.last_page && page < wrapper.last_page) ? page + 1 : null,
  }
}

export function emojiText(item: EmojiItem, skinTone = '0') {
  return item.unicode ? (item.variants?.[skinTone] ?? item.unicode) : `:${item.shortcode}:`
}
export function searchEmoji(items: EmojiItem[], query: string, group = 'all') {
  const words = query.toLowerCase().replace(/:/g, '').trim().split(/\s+/).filter(Boolean)
  return items.filter(
    (item) =>
      (group === 'all' || item.group === group || item.source === group) &&
      words.every((word) =>
        `${item.label} ${item.shortcode} ${item.aliases.join(' ')}`.toLowerCase().includes(word),
      ),
  )
}

/**
 * Orders matches for autocomplete and search: exact shortcode, prefix, contained in the
 * shortcode, matched only by label or alias; within each, the user's most used, then shorter.
 */
export function rankEmoji(items: EmojiItem[], usage: Map<string, number>, query = '') {
  const q = query.toLowerCase().replace(/:/g, '')
  const tier = (item: EmojiItem) => {
    const code = item.shortcode.toLowerCase()
    return code === q ? 0 : code.startsWith(q) ? 1 : code.includes(q) ? 2 : 3
  }
  return [...items].sort(
    (a, b) =>
      tier(a) - tier(b) ||
      (usage.get(a.id) ?? Infinity) - (usage.get(b.id) ?? Infinity) ||
      a.shortcode.length - b.shortcode.length,
  )
}

/** Picker and renderer entries for the backend's shared catalog; disabled entries are left out. */
export function instanceEmojiItems(catalog: InstanceCatalog | undefined): EmojiItem[] {
  return (catalog?.items ?? [])
    .filter((emote) => emote.enabled)
    .map((emote) => ({
      id: `instance:${emote.id}`,
      label: emote.name,
      shortcode: emote.shortcode,
      aliases: emote.aliases ?? [],
      group: 'workspace',
      url: emote.url,
      staticUrl: emote.staticUrl,
      source: 'instance',
      provider: emote.source === 'upload' ? undefined : emote.source,
      sourceUrl: emote.sourceUrl,
      creator: emote.creator,
      animated: emote.animated,
    }))
}

/** Reads a 7TV or FrankerFaceZ emote page link (or a bare ID) into an import request. */
export function parseProviderEmote(
  value: string,
): { provider: '7tv' | 'ffz'; providerId: string } | undefined {
  const text = value.trim()
  const sevenTv =
    /^(?:https?:\/\/(?:www\.)?(?:7tv\.app|7tv\.io\/v3)\/emotes\/)?([0-9A-HJKMNP-TV-Z]{26})\/?$/i.exec(
      text,
    )
  if (sevenTv) return { provider: '7tv', providerId: sevenTv[1]!.toUpperCase() }
  const ffz =
    /^(?:https?:\/\/(?:www\.)?frankerfacez\.com\/emoticon\/|https?:\/\/api\.frankerfacez\.com\/v1\/emote\/|ffz:)?(\d{1,10})(?:-[\w-]*)?\/?$/.exec(
      text,
    )
  if (ffz) return { provider: 'ffz', providerId: ffz[1]! }
  return undefined
}

export const providerName = (provider?: string) =>
  provider === '7tv' ? '7TV' : provider === 'ffz' ? 'FrankerFaceZ' : provider === 'emoji.gg' ? 'emoji.gg' : undefined

/**
 * Resolves message `:names:` to custom emoji. Later sources win: 7TV globals, then the bundled
 * workspace set, then the backend catalog, whose names are unique. Former names of a backend
 * emote render old messages but never shadow a name that is in use, including `reserved`
 * (the standard Unicode shortcodes and aliases).
 */
export function customShortcodes(
  bundled: EmojiItem[],
  instance: EmojiItem[],
  catalog: InstanceCatalog | undefined,
  reserved: ReadonlySet<string> = new Set(),
) {
  const map = new Map<string, EmojiItem>()
  const byId = new Map(instance.map((item) => [item.id, item]))
  for (const emote of catalog?.items ?? []) {
    const item = byId.get(`instance:${emote.id}`)
    if (item) for (const name of emote.retired ?? []) if (!reserved.has(name)) map.set(name, item)
  }
  for (const item of bundled.filter((item) => item.source === '7tv')) map.set(item.shortcode, item)
  for (const item of bundled.filter((item) => item.source === 'workspace'))
    map.set(item.shortcode, item)
  for (const item of instance) {
    for (const alias of item.aliases) map.set(alias, item)
    map.set(item.shortcode, item)
  }
  return map
}
