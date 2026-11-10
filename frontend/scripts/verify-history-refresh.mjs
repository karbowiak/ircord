import assert from 'node:assert/strict'
import { createApp } from 'vue'
import { createPinia, disposePinia, setActivePinia } from 'pinia'
import { PiniaColada, useQueryCache } from '@pinia/colada'
import { settleHistoryRefresh } from '../app/utils/history-refresh.ts'

const pinia = createPinia()
createApp({}).use(pinia).use(PiniaColada)
setActivePinia(pinia)
const cache = useQueryCache()
const notices = []
const unhandled = []
const onUnhandled = (error) => unhandled.push(error)
process.on('unhandledRejection', onUnhandled)
try {
  let complete
  const options = {
    key: ['fixture', 'latest'],
    query: ({ signal }) =>
      new Promise((resolve, reject) => {
        complete = resolve
        signal.addEventListener('abort', () => reject(signal.reason), { once: true })
      }),
  }
  const entry = cache.ensure(options)
  const first = settleHistoryRefresh(
    () => cache.fetch(entry),
    (error) => notices.push(error),
  )
  // Sending and the timeline pending-message watcher request the same newest window.
  cache.cancelQueries({ key: options.key, exact: true })
  const second = settleHistoryRefresh(
    () => cache.fetch(entry),
    (error) => notices.push(error),
  )
  complete({ items: [], nextCursor: null })
  assert.equal(await first, false, 'superseded history is a settled cancellation')
  assert.equal(await second, true)
  assert.equal(
    notices.length,
    0,
    'intentional navigation cancellation is not a user-facing failure',
  )

  const failure = new Error('Fixture server unavailable')
  const failed = cache.ensure({
    key: ['fixture', 'failed'],
    query: async () => {
      throw failure
    },
  })
  assert.equal(
    await settleHistoryRefresh(
      () => cache.fetch(failed),
      (error) => notices.push(error),
    ),
    false,
  )
  assert.deepEqual(notices, [failure], 'real query rejection remains visible')
  await new Promise((resolve) => setTimeout(resolve, 0))
  assert.equal(
    unhandled.length,
    0,
    'cancellation must not reach the dev overlay as an unhandled rejection',
  )
} finally {
  for (const entry of cache.getEntries()) cache.remove(entry)
  disposePinia(pinia)
  process.removeListener('unhandledRejection', onUnhandled)
}
console.log(
  'Real Colada history cancellation settles cleanly; successful refreshes and actual failures retain their behavior.',
)
