import { defineQueryOptions, useQueryCache } from '@pinia/colada'
import { linkMediaCache, linkMediaFreshTime } from '../utils/link-media-cache'
import { createPreviewQueue } from '../utils/preview-queue'
import type { LinkMediaIdentity } from '../utils/link-media-cache'

const resolveVisible = createPreviewQueue(50)
const abortError = () => new DOMException('Link request cancelled.', 'AbortError')
export const linkMediaKey = (input: LinkMediaIdentity) => [
  'link-media',
  input.account,
  input.conversation,
  input.message,
  input.url,
  input.widget ? 'widget' : 'source',
]

export const linkMediaQuery = defineQueryOptions(
  (input: {
    account: string
    conversation: string
    message: string
    url: string
    widget: boolean
    enabled: boolean
  }) => {
    const cache = useQueryCache()
    const saved = linkMediaCache.read(input)
    return {
      key: linkMediaKey(input),
      enabled: input.enabled && !!input.account,
      initialData: () => saved?.data,
      initialDataUpdatedAt: saved?.updatedAt,
      query: async ({ signal, entry }) => {
        const ticket = linkMediaCache.generation()
        try {
          const value = await resolveVisible(signal, () =>
            apiClient().resolveMessageLink(
              input.conversation,
              input.message,
              input.url,
              input.widget,
              signal,
            ),
          )
          if (signal.aborted) throw signal.reason ?? abortError()
          // Admission/network/storage failures are retryable query errors, not successful
          // metadata to retain for five minutes while the reader returns to the preview.
          if (
            value.kind === 'card' &&
            ['fetch_failed', 'storage_unavailable'].includes(value.card?.reason ?? '')
          )
            throw new Error('Preview is temporarily unavailable')
          linkMediaCache.write(input, value, ticket)
          return value
        } catch (error) {
          if (signal.aborted) throw signal.reason ?? abortError()
          linkMediaCache.evict(input, ticket)
          // Colada normally retains stale data on error. A denied or failed source must disappear.
          const failure =
            error instanceof Error ? error : new Error('Preview is temporarily unavailable')
          cache.setEntryState(entry, { status: 'error', data: undefined, error: failure })
          throw failure
        }
      },
      staleTime: linkMediaFreshTime,
      gcTime: linkMediaFreshTime,
      refetchOnWindowFocus: false,
      refetchOnReconnect: false,
    }
  },
)
