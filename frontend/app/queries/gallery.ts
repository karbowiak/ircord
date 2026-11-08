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

/** Bumped when a conversation's gallery may have changed, so loaded older pages are reread. */
export const galleryRevision = ref(0)

/** Whether a message could add, change or remove gallery images. */
const mayCarryImages = (message: Message) =>
  !!message.attachment || !!message.gif || !!message.preview || /https?:\/\//.test(message.text)

/**
 * A message of the conversation arrived or changed (an edit, a deletion, a resolved preview).
 * The gallery is private to this account and rebuilt by the backend, so an open gallery just
 * rereads it; nothing is fetched when no gallery of that conversation is loaded.
 */
export function galleryMessage(cache: QueryCache, message: Message, updated: boolean) {
  const key = ['gallery', message.conversationId]
  const entry = cache.getEntries({ key, exact: true })[0]
  if (!entry) return
  const page = entry.state.value.data as GalleryPage | undefined
  const listed = !!page?.items.some((i) => i.messageId === message.id)
  if (!mayCarryImages(message) && !(updated && listed) && !message.deleted) return
  galleryRevision.value++
  void cache.invalidateQueries({ key, exact: true }).catch(() => {})
}

/** Conversations or networks went away: every loaded gallery is reread. */
export function galleriesChanged(cache: QueryCache) {
  galleryRevision.value++
  void cache.invalidateQueries({ key: ['gallery'] }).catch(() => {})
}
