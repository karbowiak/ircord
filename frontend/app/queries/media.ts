import { $fetch } from '#build/fetch'
import { defineQueryOptions } from '@pinia/colada'
import type { EmojiItem, InstanceCatalog } from '../../shared/types/media'

/** Workspace + 7TV emotes. Public and slow-changing. */
export const emojiCatalogQuery = defineQueryOptions({
  key: ['emoji', 'catalog'],
  query: () => appClient.emojiCatalog(),
  staleTime: 60 * 60_000,
  gcTime: false,
  refetchOnWindowFocus: false,
})

/** The backend's shared emote catalog. emotes.updated and session.ready invalidate it. */
export const instanceEmotesQuery = defineQueryOptions({
  key: ['emoji', 'instance'],
  query: (): Promise<InstanceCatalog> => apiClient().emotes(),
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
