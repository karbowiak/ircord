import { defineQueryOptions } from '@pinia/colada'

type Task = { start: () => void; signal: AbortSignal }
const pending: Task[] = []
let active = 0
const abortError = () => new DOMException('Link request cancelled.', 'AbortError')
function pump() {
  while (active < 2 && pending.length) pending.shift()!.start()
}
async function resolveVisible<T>(signal: AbortSignal, run: () => Promise<T>): Promise<T> {
  await new Promise<void>((resolve, reject) => {
    if (signal.aborted) return reject(abortError())
    const task: Task = { signal, start: () => {
      signal.removeEventListener('abort', cancel)
      if (signal.aborted) { reject(abortError()); return }
      active++
      resolve()
    } }
    function cancel() {
      const index = pending.indexOf(task)
      if (index !== -1) pending.splice(index, 1)
      reject(abortError())
    }
    signal.addEventListener('abort', cancel, { once: true })
    pending.push(task)
    pump()
  })
  try { return await run() } finally { active--; pump() }
}

export const linkMediaQuery = defineQueryOptions((input: {
  account: string; conversation: string; message: string; url: string; widget: boolean; enabled: boolean
}) => ({
  key: ['link-media', input.account, input.conversation, input.message, input.url, input.widget ? 'widget' : 'source'],
  enabled: input.enabled && !!input.account,
  query: ({ signal }) => resolveVisible(signal, () => apiClient().resolveMessageLink(input.conversation, input.message, input.url, input.widget, signal)),
  staleTime: 5 * 60_000,
  gcTime: 5 * 60_000,
  refetchOnWindowFocus: false,
  refetchOnReconnect: false,
}))
