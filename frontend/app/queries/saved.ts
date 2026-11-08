import { defineQueryOptions, type QueryCache } from '@pinia/colada'
import type { Message, SavedItem, SavedPage, SavedRef } from '../../shared/types/chat'

export const SAVED_PAGE = 30
type SavedRefs = { items: SavedRef[] }

/** Which messages you saved (references only, bounded by the server). Kept current by events. */
export const savedIdsQuery = defineQueryOptions({
  key: ['saved', 'ids'],
  query: (): Promise<SavedRefs> => apiClient().savedMessageIds(),
  staleTime: Infinity,
  gcTime: false,
  refetchOnWindowFocus: false,
  refetchOnReconnect: false,
})

/** First page of your saved messages, optionally filtered by text. Older pages load on demand. */
export const savedListQuery = defineQueryOptions((q: string) => ({
  key: ['saved', 'list', q],
  query: ({ signal }) =>
    apiClient().savedMessages({ q: q || undefined, limit: SAVED_PAGE }, signal),
  staleTime: Infinity,
  gcTime: 60_000,
  refetchOnWindowFocus: false,
}))

/** Bumped when the set of saved messages changed, so loaded older pages are dropped and reread. */
export const savedRevision = ref(0)
/** The latest saved message whose source changed; the list patches it in place. */
export const savedPatch = shallowRef<Message>()

export function putSavedRef(cache: QueryCache, ref: SavedRef) {
  const current = cache.getQueryData<SavedRefs>(savedIdsQuery.key)
  if (!current) return
  cache.setQueryData(savedIdsQuery.key, {
    items: [ref, ...current.items.filter((r) => r.messageId !== ref.messageId)],
  })
}
export function dropSavedRef(cache: QueryCache, messageId: string) {
  const current = cache.getQueryData<SavedRefs>(savedIdsQuery.key)
  if (current)
    cache.setQueryData(savedIdsQuery.key, {
      items: current.items.filter((r) => r.messageId !== messageId),
    })
}
export const isSavedRef = (cache: QueryCache, messageId: string) =>
  !!cache.getQueryData<SavedRefs>(savedIdsQuery.key)?.items.some((r) => r.messageId === messageId)

/** The listed content changed (a new bookmark, an edited or deleted source, a removed network). */
export function savedChanged(cache: QueryCache) {
  savedRevision.value++
  void cache.invalidateQueries({ key: ['saved', 'list'] }).catch(() => {})
}

/** A listed item with its source message's new content: deleted becomes a stub. */
export function patchSavedItem(item: SavedItem, message: Message): SavedItem {
  if (item.messageId !== message.id || item.status === 'unavailable') return item
  if (message.deleted)
    return {
      ...item,
      status: 'deleted',
      message: {
        id: message.id,
        conversationId: message.conversationId,
        senderId: message.senderId,
        createdAt: message.createdAt,
        kind: message.kind,
        text: '',
        deleted: true,
        reactions: [],
      },
    }
  return { ...item, status: 'available', message }
}

/**
 * A saved message's source changed (an edit, a deletion, a resolved link preview). Cached
 * list pages are patched in place rather than reloaded, so the reader keeps their place.
 */
export function patchSaved(cache: QueryCache, message: Message) {
  for (const entry of cache.getEntries({ key: ['saved', 'list'] })) {
    const page = entry.state.value.data as SavedPage | undefined
    if (page?.items.some((i) => i.messageId === message.id))
      cache.setQueryData(entry.key, {
        ...page,
        items: page.items.map((i) => patchSavedItem(i, message)),
      })
  }
  savedPatch.value = message
}
