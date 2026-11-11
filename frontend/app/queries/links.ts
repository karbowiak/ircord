import { defineQueryOptions, type QueryCache } from '@pinia/colada'
import type { LinkPage, Message } from '../../shared/types/chat'
import {
  countPosted,
  linkEventEffect,
  linkQueryParams,
  type LinkEvent,
  type LinkFilter,
} from '../../shared/utils/link-catalog'

export { LINKS_PAGE, linkQueryParams, type LinkFilter } from '../../shared/utils/link-catalog'

/**
 * First page of the link catalog for a filter. Older pages load on demand. A cached page is
 * shown at once when the view reopens and refreshed behind it, so a link posted while the
 * view was closed is never missing.
 */
export const linksQuery = defineQueryOptions((filter: LinkFilter) => ({
  key: [
    'links',
    filter.conversations.join('\n'),
    filter.network,
    filter.kinds.join(','),
    filter.q,
    filter.host ?? '',
    filter.since ?? '',
    filter.until ?? '',
  ],
  query: ({ signal }): Promise<LinkPage> => apiClient().links(linkQueryParams(filter), signal),
  staleTime: 0,
  gcTime: 30_000,
  refetchOnWindowFocus: false,
}))

/**
 * Links posted in each channel since the app loaded, so an open view offers the ones posted
 * after its list loaded in the channels it shows, without moving what is being read.
 */
export const linksPosted = reactive(new Map<string, number>())
/** Bumped when listed content may be gone (a deleted post, a removed network or channel). */
export const linksRevision = ref(0)
/** Retained conversations with changed settings need a fresh source classification. */
export const linksClassificationOverrides = reactive(new Map<string, boolean | undefined>())
export const linkSourceKey = (networkId: string, conversationId: string) =>
  JSON.stringify([networkId, conversationId])

/** Record fresh classifications for sources invalidated by a retained conversation update. */
export function updateLinkClassifications(page: LinkPage, revision: number) {
  if (revision !== linksRevision.value) return
  for (const card of page.items)
    for (const source of card.sources) {
      const key = linkSourceKey(source.networkId, source.conversationId)
      if (linksClassificationOverrides.has(key))
        linksClassificationOverrides.set(key, source.nsfwKnown === true ? source.nsfw : undefined)
    }
}
/** Messages the open view lists, so a change to one of them refreshes it. */
export const linksListed = new Set<string>()

/** A source post was deleted or a network or channel left: listed excerpts must not linger. */
export function linksChanged(cache: QueryCache) {
  linksRevision.value++
  cache.cancelQueries({ key: ['links'] })
  void cache.invalidateQueries({ key: ['links'] }).catch(() => {})
}

/**
 * A message arrived or changed (see linkEventEffect): new links are counted per channel for
 * the view's offer, and changes to listed or link-bearing posts reread the list.
 */
export function linksMessage(
  cache: QueryCache,
  message: Message,
  event: Omit<LinkEvent, 'text' | 'deleted' | 'listed'>,
) {
  const effect = linkEventEffect({
    ...event,
    text: message.text,
    deleted: message.deleted,
    listed: linksListed.has(message.id),
  })
  if (effect === 'posted')
    linksPosted.set(message.conversationId, (linksPosted.get(message.conversationId) ?? 0) + 1)
  else if (effect === 'changed') linksChanged(cache)
}

/** New link posts in what a view shows (see countPosted). */
export const postedIn = (
  filter: LinkFilter,
  networkOf: (conversationId: string) => string | undefined,
) => countPosted(linksPosted, filter.conversations, filter.network, networkOf)
