import type { Bootstrap, MessagePage } from '../../shared/types/chat'
import { defineQueryOptions } from '@pinia/colada'
import { searchParams, type SearchFilters } from '../../shared/utils/search-filter'
import { mergeMessagePage } from '../../shared/utils/message-page'
import { offlineStatus, readMessages, readSearch } from '../utils/offline-archive'

export const HISTORY_PAGE = 40
export const MAX_MESSAGES = 500

/** Workspace snapshot. Kept current by realtime events, so it is never stale on its own. */
export const bootstrapQuery = defineQueryOptions({
  key: ['bootstrap'],
  query: (): Promise<Bootstrap> => apiClient().bootstrap(),
  staleTime: Infinity,
  gcTime: false,
  refetchOnWindowFocus: false,
  refetchOnReconnect: false,
})

/**
 * Newest page of a conversation; older pages are merged in by the chat store. Realtime events
 * keep cached conversations current, and conversations nobody looks at for ten minutes are
 * evicted, which bounds memory without a hand-written LRU.
 */
export const messagesQuery = defineQueryOptions((id: string) => ({
  key: ['messages', id],
  enabled: !!id && !id.startsWith('server:'),
  query: async ({ signal, entry }): Promise<MessagePage> =>
    mergeMessagePage(
      await readMessages(id, { limit: HISTORY_PAGE }, signal),
      entry.state.value.data as MessagePage | undefined,
    ),
  staleTime: Infinity,
  gcTime: 10 * 60_000,
  refetchOnWindowFocus: false,
  refetchOnReconnect: false,
}))

/** A network's status window. Kept current by `server.line` events while cached. */
export const serverLogQuery = defineQueryOptions((networkId: string) => ({
  key: ['server-log', networkId],
  query: () => apiClient().serverLog(networkId, { limit: 100 }),
  staleTime: Infinity,
  gcTime: 10 * 60_000,
  refetchOnWindowFocus: false,
}))

/** Connection snapshots are kept current by private connection.updated frames. */
export const connectionQuery = defineQueryOptions((networkId: string) => ({
  key: ['connection', networkId],
  enabled: !!networkId,
  query: ({ signal }) => apiClient().connection(networkId, signal),
  staleTime: Infinity,
  gcTime: 10 * 60_000,
  refetchOnWindowFocus: false,
  refetchOnReconnect: false,
}))

/**
 * Keep-alive round trips over the last half hour. Not pushed (a sample every 20 seconds isn't
 * worth a replayed event), so the server page refreshes it while open.
 */
export const latencyQuery = defineQueryOptions((networkId: string) => ({
  key: ['connection-latency', networkId],
  enabled: !!networkId,
  query: ({ signal }) => apiClient().connectionLatency(networkId, signal),
  staleTime: 15_000,
  gcTime: 60_000,
}))

/** The network's public channel list (LIST), searched server-side. */
export const channelListQuery = defineQueryOptions(
  ({ networkId, q }: { networkId: string; q: string }) => ({
    key: ['channels', networkId, q],
    query: ({ signal }) => apiClient().channels(networkId, q, signal),
    staleTime: 2 * 60_000,
  }),
)

/**
 * Message search with filters (from:, in:, has:, dates…), for the Search page and a
 * conversation's search panel (which pins `in:` to itself).
 */
export const messageSearchQuery = defineQueryOptions(
  ({ text, filters }: { text: string; filters: SearchFilters }) => ({
    key: [
      'search',
      'messages',
      offlineStatus.value.offline ? 'offline' : 'online',
      text,
      JSON.stringify(searchParams('', filters)),
    ],
    query: ({ signal }) => readSearch(text, filters, undefined, signal),
    staleTime: 30_000,
    gcTime: 2 * 60_000,
  }),
)
/** A channel's ban/exception/invite-exception/quiet list. */
export const channelModeListQuery = defineQueryOptions(
  ({ conversationId, mode }: { conversationId: string; mode: string }) => ({
    key: ['channel-mode-list', conversationId, mode],
    query: () => apiClient().modeList(conversationId, mode),
    staleTime: 15_000,
  }),
)
