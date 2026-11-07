import { defineQueryOptions } from '@pinia/colada'
import type { EmojiItem } from '../../shared/types/media'

/** Workspace + 7TV emotes. Public and slow-changing. */
export const emojiCatalogQuery = defineQueryOptions({
  key: ['emoji', 'catalog'],
  query: () => appClient.emojiCatalog(),
  staleTime: 60 * 60_000,
  gcTime: false,
  refetchOnWindowFocus: false,
})

/** The generated Unicode dataset; immutable for a given build. */
export const standardEmojiQuery = defineQueryOptions({
  key: ['emoji', 'standard'],
  query: () => $fetch<EmojiItem[]>('/data/emoji-v18.json'),
  staleTime: Infinity,
  gcTime: false,
  refetchOnWindowFocus: false,
})
