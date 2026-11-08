import { defineQueryOptions, type QueryCache } from '@pinia/colada'
import type { GalleryPage, Message } from '../../shared/types/chat'

export const GALLERY_PAGE = 40

/** First page of a conversation's gallery. Older pages load as the grid scrolls. */
export const galleryQuery = defineQueryOptions((conversationId: string) => ({
  key: ['gallery', conversationId],
  query: ({ signal }) => apiClient().gallery(conversationId, { limit: GALLERY_PAGE }, signal),
  staleTime: 30_000,
  gcTime: 60_000,
  refetchOnWindowFocus: false,
}))

/**
 * Per conversation, bumped when its listed images may have changed or gone (an edit, a deletion,
 * an ignore). An open gallery then drops the pages it loaded past the first and rereads them.
 * New posts do not bump it: they only re-read the first page, and the gallery keeps its place.
 */
export const galleryRevisions = reactive(new Map<string, number>())
/** Message IDs an open gallery has loaded, per conversation, beyond what the cache knows. */
export const galleryLoaded = new Map<string, Set<string>>()

const bump = (conversationId: string) =>
  galleryRevisions.set(conversationId, (galleryRevisions.get(conversationId) ?? 0) + 1)

/** Whether a message could add gallery images. */
const mayCarryImages = (message: Message) =>
  !!message.attachment || !!message.gif || !!message.preview || /https?:\/\//.test(message.text)

/**
 * A message of the conversation arrived or changed (an edit, a deletion, a resolved preview).
 * The gallery is private to this account and rebuilt by the backend, so a loaded gallery rereads
 * it; nothing is fetched when no gallery of that conversation is loaded.
 */
export function galleryMessage(cache: QueryCache, message: Message, updated: boolean) {
  const conversationId = message.conversationId
  const key = ['gallery', conversationId]
  const entry = cache.getEntries({ key, exact: true })[0]
  const loaded = galleryLoaded.get(conversationId)
  if (!entry && !loaded) return
  if (!updated) {
    if (mayCarryImages(message)) void cache.invalidateQueries({ key, exact: true }).catch(() => {})
    return
  }
  const page = entry?.state.value.data as GalleryPage | undefined
  const listed = !!loaded?.has(message.id) || !!page?.items.some((i) => i.messageId === message.id)
  if (!listed && !mayCarryImages(message)) return
  bump(conversationId)
  void cache.invalidateQueries({ key, exact: true }).catch(() => {})
}

/** Conversations, networks or ignored people changed: every loaded gallery is reread. */
export function galleriesChanged(cache: QueryCache) {
  const ids = new Set(galleryLoaded.keys())
  for (const entry of cache.getEntries({ key: ['gallery'] }))
    if (typeof entry.key[1] === 'string') ids.add(entry.key[1])
  for (const id of ids) bump(id)
  void cache.invalidateQueries({ key: ['gallery'] }).catch(() => {})
}
