import { defineQueryOptions, useQueryCache } from '@pinia/colada'
import { linkMediaCache, linkMediaFreshTime } from '../utils/link-media-cache'

type Task = { start: () => void; signal: AbortSignal }
const pending: Task[] = []
let active = 0
const abortError = () => new DOMException('Link request cancelled.', 'AbortError')
function pump() {
  while (active < 2 && pending.length) pending.shift()!.start()
}
async function resolveVisible<T>(signal: AbortSignal, run: () => Promise<T>): Promise<T> {
  await new Promise<void>((resolve, reject) => {
    if (signal.aborted) return reject(signal.reason ?? abortError())
    const task: Task = {
      signal,
      start: () => {
        signal.removeEventListener('abort', cancel)
        if (signal.aborted) {
          reject(signal.reason ?? abortError())
          return
        }
        active++
        resolve()
      },
    }
    function cancel() {
      const index = pending.indexOf(task)
      if (index !== -1) pending.splice(index, 1)
      reject(signal.reason ?? abortError())
    }
    signal.addEventListener('abort', cancel, { once: true })
    pending.push(task)
    pump()
  })
  try {
    return await run()
  } finally {
    active--
    pump()
  }
}

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
      key: [
        'link-media',
        input.account,
        input.conversation,
        input.message,
        input.url,
        input.widget ? 'widget' : 'source',
      ],
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
          linkMediaCache.write(input, value, ticket)
          return value
        } catch (error) {
          if (signal.aborted) throw signal.reason ?? abortError()
          linkMediaCache.evict(input, ticket)
          // Colada normally retains stale data on error. A denied or failed source must disappear.
          cache.setEntryState(entry, { status: 'pending', data: undefined, error: null })
          throw error
        }
      },
      staleTime: linkMediaFreshTime,
      gcTime: linkMediaFreshTime,
      refetchOnWindowFocus: false,
      refetchOnReconnect: false,
    }
  },
)
