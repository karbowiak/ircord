import { defineQueryOptions, type QueryCache } from '@pinia/colada'
import type { InboxFilter, InboxPage } from '../../shared/types/chat'

export const INBOX_PAGE = 30

/** The inbox's unread indicator: a single count, refreshed when something may have changed it. */
export const inboxSummaryQuery = defineQueryOptions({
  key: ['inbox', 'summary'],
  query: ({ signal }) => apiClient().inboxSummary(signal),
  staleTime: Infinity,
  gcTime: false,
  refetchOnWindowFocus: false,
})

/** First page of one inbox view. Older pages load on demand. */
export const inboxListQuery = defineQueryOptions((filter: InboxFilter) => ({
  key: ['inbox', 'list', filter],
  query: ({ signal }): Promise<InboxPage> =>
    apiClient().inbox({ filter, limit: INBOX_PAGE }, signal),
  staleTime: Infinity,
  gcTime: 60_000,
  refetchOnWindowFocus: false,
}))

/** Bumped whenever the inbox may have changed, so loaded older pages are dropped and reread. */
export const inboxRevision = ref(0)

let timer: ReturnType<typeof setTimeout> | undefined
/**
 * Something the inbox shows changed: a new item, a dismissal on any device, a read, a deletion
 * or a removed conversation. Bursts collapse into one reread of the count and the open view.
 */
export function inboxChanged(cache: QueryCache, delay = 150) {
  clearTimeout(timer)
  timer = setTimeout(() => {
    inboxRevision.value++
    void cache.invalidateQueries({ key: ['inbox'] }).catch(() => {})
  }, delay)
}

/**
 * A conversation was read. Reading can only lower the unread count, so with nothing unread
 * there is nothing to refresh; busy channels read on every message.
 */
export function inboxRead(cache: QueryCache) {
  if (cache.getQueryData<{ unread: number }>(inboxSummaryQuery.key)?.unread === 0) return
  inboxChanged(cache)
}
