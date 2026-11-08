import type { components } from '../generated/api'
export type GifItem = components['schemas']['GIF']
export interface GifPage {
  items: GifItem[]
  nextPage: number | null
}
export interface GifCategory {
  name: string
  query: string
  previewUrl: string
}
export interface EmojiItem {
  id: string
  label: string
  shortcode: string
  aliases: string[]
  group: string
  unicode?: string
  url?: string
  staticUrl?: string
  /** instance: the Go backend's shared catalog; workspace and 7tv come from the frontend bundle. */
  source: 'unicode' | '7tv' | 'workspace' | 'instance'
  /** Where an instance emote was imported from (7tv, ffz); absent for uploads. */
  provider?: string
  sourceUrl?: string
  creator?: string
  animated?: boolean
  variants?: Record<string, string>
}
export type InstanceEmote = components['schemas']['Emote']
export type InstanceCatalog = components['schemas']['EmoteCatalog']
export interface EmojiGroup {
  id: string
  label: string
  icon: string
}
export interface EmojiCatalog {
  items: EmojiItem[]
  standardUrl: string
  sources: string[]
}
