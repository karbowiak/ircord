import assert from 'node:assert/strict'
import { mock } from 'bun:test'

const calls = []
let failure
const serverPage = { items: [], nextCursor: 'server-next', cursor: 7 }
mock.module('../app/utils/api.ts', () => ({
  apiClient: () => ({
    async messages(id, query) {
      calls.push({ id, query })
      if (failure) throw failure
      return query.around ? { ...serverPage, nextCursor: 'server-boundary' } : serverPage
    },
  }),
  errorStatus: (error) => error?.status,
}))
Object.defineProperty(globalThis, 'navigator', {
  configurable: true,
  value: { onLine: true },
})
const { readMessages, offlineArchive, offlineStatus } =
  await import('../app/utils/offline-archive.ts')
const saved = { items: [], nextCursor: null, offline: true }
let offlineReads = 0
offlineArchive.available = () => true
offlineArchive.history = async () => {
  offlineReads++
  return saved
}
offlineStatus.value = { ...offlineStatus.value, offline: true }
assert.equal(await readMessages('room'), serverPage)
assert.equal(offlineReads, 0, 'socket reconnect must not force reachable HTTP reads into IndexedDB')

calls.length = 0
const local = 'offline:' + btoa(JSON.stringify({ id: 'saved-boundary' }))
assert.equal(await readMessages('room', { before: local, limit: 40 }), serverPage)
assert.deepEqual(
  calls.map((call) => call.query),
  [
    { around: 'saved-boundary', limit: 1 },
    { before: 'server-boundary', limit: 40 },
  ],
)
assert.equal(offlineReads, 0, 'an online reader must leave offline pagination when HTTP recovers')

calls.length = 0
await readMessages('room', { before: 'message:clipped%3Aid' })
assert.equal(calls[0].query.around, 'clipped:id')
assert.equal(calls[1].query.before, 'server-boundary')

failure = Object.assign(new Error('Forbidden'), { status: 403 })
await assert.rejects(readMessages('room'))
assert.equal(offlineReads, 0, 'denied server access must not fall back to saved history')
failure = new TypeError('Network unavailable')
assert.equal(await readMessages('room'), saved)
assert.equal(offlineReads, 1)
const abort = new AbortController()
abort.abort()
await assert.rejects(readMessages('room', {}, abort.signal))
assert.equal(offlineReads, 1, 'aborted window requests must not revive an old offline page')

navigator.onLine = false
failure = undefined
calls.length = 0
assert.equal(await readMessages('room'), saved)
assert.equal(calls.length, 0)
console.log('HTTP-first history, offline recovery boundaries, denied access and aborts pass.')
