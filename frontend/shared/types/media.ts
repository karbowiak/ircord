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
  source: 'unicode' | '7tv' | 'workspace'
  sourceUrl?: string
  creator?: string
  variants?: Record<string, string>
}
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
