// The picker library: saved GIFs, emoji usage and skin tone. Shared by the
// Nitro routes (validation) and the browser (local mirror).
import type { GifItem } from './types/media'
import { validGif } from './utils/media'

export interface SavedGif extends GifItem {
  savedAt: string
}
export interface EmojiUsage {
  id: string
  count: number
  lastUsed: string
}
export interface PickerLibrary {
  gifs: SavedGif[]
  /** How often each emoji was picked; drives “Frequently used” and autocomplete ranking. */
  emojiUsage: EmojiUsage[]
  skinTone: string
  updatedAt: string | null
}

export const libraryLimits = { gifs: 200, emojiUsage: 120 } as const
export const skinTones = ['0', '1', '2', '3', '4', '5'] as const

export const emptyLibrary = (): PickerLibrary => ({
  gifs: [],
  emojiUsage: [],
  skinTone: '0',
  updatedAt: null,
})

export const validEmojiId = (id: unknown): id is string =>
  typeof id === 'string' && id.length > 0 && id.length <= 120 && !/[\s<>"]/.test(id)

/** Recent heavy use outranks old heavy use: counts fade with a two-week half-life. */
export function usageScore(usage: EmojiUsage, now = Date.now()) {
  const days = Math.max(0, now - new Date(usage.lastUsed).getTime()) / 86_400_000
  return usage.count * 0.5 ** (days / 14)
}

/** Adds uses (id → times) and keeps the highest-scoring entries. */
export function recordUsage(list: EmojiUsage[], uses: Record<string, number>, now = new Date()) {
  const byId = new Map(list.map((u) => [u.id, { ...u }]))
  for (const [id, times] of Object.entries(uses)) {
    if (!validEmojiId(id) || !Number.isInteger(times) || times < 1) continue
    const current = byId.get(id)
    byId.set(id, {
      id,
      count: Math.min(1_000_000, (current?.count ?? 0) + Math.min(times, 100)),
      lastUsed: now.toISOString(),
    })
  }
  return [...byId.values()]
    .sort((a, b) => usageScore(b, now.getTime()) - usageScore(a, now.getTime()))
    .slice(0, libraryLimits.emojiUsage)
}

export function sanitizeLibrary(input: unknown): PickerLibrary {
  const value = (input && typeof input === 'object' ? input : {}) as Record<string, unknown>
  const seen = new Set<string>()
  const gifs = (Array.isArray(value.gifs) ? value.gifs : [])
    .filter((gif): gif is SavedGif => validGif(gif) && !seen.has(gif.id) && !!seen.add(gif.id))
    .slice(0, libraryLimits.gifs)
    .map((gif) => ({ ...savedGif(gif), savedAt: validDate(gif.savedAt) }))
  return {
    gifs,
    emojiUsage: (Array.isArray(value.emojiUsage) ? value.emojiUsage : [])
      .filter(
        (u): u is EmojiUsage =>
          !!u &&
          validEmojiId(u.id) &&
          Number.isInteger(u.count) &&
          u.count > 0 &&
          u.count <= 1_000_000,
      )
      .filter((u, i, all) => all.findIndex((x) => x.id === u.id) === i)
      .slice(0, libraryLimits.emojiUsage)
      .map((u) => ({ id: u.id, count: u.count, lastUsed: validDate(u.lastUsed) })),
    skinTone: skinTones.includes(value.skinTone as never) ? (value.skinTone as string) : '0',
    updatedAt: typeof value.updatedAt === 'string' ? validDate(value.updatedAt) : null,
  }
}

/** Copies only the known GIF fields so stored favourites cannot carry extra data. */
export function savedGif(gif: GifItem, savedAt = new Date().toISOString()): SavedGif {
  const { id, slug, title, url, previewUrl, stillUrl, width, height, provider } = gif
  return { id, slug, title, url, previewUrl, stillUrl, width, height, provider, savedAt }
}

function validDate(value: unknown) {
  const date = typeof value === 'string' ? new Date(value) : new Date()
  return Number.isNaN(date.getTime()) ? new Date().toISOString() : date.toISOString()
}
