import assert from 'node:assert/strict'
import { mock } from 'bun:test'
import { effectScope, nextTick, ref } from 'vue'
import { createOfflineFileReads } from '../app/utils/offline-file-reads.ts'

const turn = async () => {
  for (let i = 0; i < 30; i++) await Promise.resolve()
  await nextTick()
}
const deferred = () => {
  let resolve, reject
  const promise = new Promise((yes, no) => {
    resolve = yes
    reject = no
  })
  return { promise, resolve, reject }
}

// Admission and cancellation use the real queue with controlled asynchronous reads.
const started = []
const reads = createOfflineFileReads(
  (url) => {
    const operation = { url, ...deferred() }
    started.push(operation)
    return operation.promise
  },
  { concurrency: 1, capacity: 3 },
)
const cancelled = new AbortController()
const first = reads.read('same', cancelled.signal).catch((error) => error.name)
const second = reads.read('same')
await turn()
assert.equal(started.length, 1, 'concurrent callers share one file lookup')
cancelled.abort()
assert.equal(await first, 'AbortError')
started[0].resolve('shared')
assert.equal(await second, 'shared', 'one reader leaving does not cancel another')
await turn()

const blocker = reads.read('blocker').catch((error) => error.name)
await turn()
const abandoned = new AbortController()
const queued = reads.read('abandoned', abandoned.signal).catch((error) => error.name)
const survivor = reads.read('survivor').catch((error) => error.name)
await assert.rejects(reads.read('overflow'), /busy/, 'pending URL storage is bounded')
abandoned.abort()
assert.equal(await queued, 'AbortError')
reads.reset()
assert.equal(await blocker, 'AbortError')
assert.equal(await survivor, 'AbortError')
const replacement = reads.read('same')
await turn()
assert.deepEqual(
  started.map((item) => item.url),
  ['same', 'blocker', 'same'],
)
started[1].resolve('old account')
started[2].resolve('new account')
assert.equal(await replacement, 'new account')
await turn()
assert(!started.some((item) => item.url === 'abandoned'), 'cancelled queued work never starts')
const repeated = reads.read('same')
await turn()
assert.equal(started.length, 4, 'completed Blob results are not retained in a cache')
started[3].resolve('new read')
await repeated
const immediate = reads.read('same')
await turn()
assert.equal(started.length, 5, 'an immediate reread cannot join an already-settled entry')
started[4].resolve('immediate read')
assert.equal(await immediate, 'immediate read')

// Exercise the real archive and composable; replace only browser I/O in this process.
mock.module('../app/utils/api.ts', () => ({
  apiClient: () => {
    throw new Error('This check must not contact an API')
  },
  errorStatus: (error) => error?.status,
}))
const workers = []
class ArchiveWorker {
  calls = []
  postMessage(data) {
    this.calls.push(data)
    if (data.method !== 'file')
      queueMicrotask(() =>
        this.reply(data, data.method === 'pendingFiles' ? [] : { messages: 0, files: 0, bytes: 0 }),
      )
  }
  reply(call, result) {
    this.onmessage?.({ data: { id: call.id, result } })
  }
  terminate() {}
  constructor() {
    workers.push(this)
  }
}
globalThis.Worker = ArchiveWorker
globalThis.window = {}
globalThis.document = { hidden: false }
globalThis.location = { href: 'https://ircord.invalid/' }
Object.defineProperty(globalThis, 'navigator', {
  configurable: true,
  value: { onLine: true },
})
const blob = new Blob(['fixture'], { type: 'image/png' })
globalThis.fetch = async () => ({ ok: true, blob: async () => blob })
const liveURLs = new Set()
let objectSerial = 0
URL.createObjectURL = () => {
  const url = `blob:fixture-${++objectSerial}`
  liveURLs.add(url)
  return url
}
URL.revokeObjectURL = (url) => liveURLs.delete(url)
const { offlineArchive, offlineRevision } = await import('../app/utils/offline-archive.ts')
const { useOfflineMedia } = await import('../app/composables/media/useOfflineMedia.ts')
offlineArchive.connection('disconnected')
await offlineArchive.start('fixture-a', '/api/v1')
const scopes = []
const mount = (url, enabled = ref(true)) => {
  const scope = effectScope()
  scopes.push(scope)
  const media = scope.run(() => useOfflineMedia(url, enabled))
  return { scope, media, enabled }
}
const callsFor = (worker, url) =>
  worker.calls.filter((call) => call.method === 'file' && call.args[0] === url)
const file = { blob, refs: [{ id: 'fixture-message', pinned: true }] }
const url = 'https://files.invalid/a.png'
const other = 'https://files.invalid/b.png'
try {
  const a = mount(url)
  const b = mount(url)
  const unrelated = mount(other)
  const workerA = workers.at(-1)
  await turn()
  assert.equal(callsFor(workerA, url).length, 1, 'wrappers share the same-account file RPC')
  assert.equal(callsFor(workerA, other).length, 1)
  workerA.reply(callsFor(workerA, url)[0], file)
  workerA.reply(callsFor(workerA, other)[0], file)
  await turn()
  const stableSource = a.media.source.value
  const stableRefs = a.media.refs.value
  assert(a.media.saved.value && b.media.saved.value && unrelated.media.saved.value)
  const revision = offlineRevision.value
  await offlineArchive.download({
    url,
    conversationId: 'fixture-room',
    messageId: 'fixture-message',
  })
  await turn()
  assert.equal(offlineRevision.value, revision, 'one download does not invalidate every media URL')
  assert.equal(callsFor(workerA, url).length, 2)
  assert.equal(callsFor(workerA, other).length, 1, 'unrelated media does not reread per download')
  assert.equal(a.media.source.value, stableSource)
  assert.equal(a.media.refs.value, stableRefs)
  assert(a.media.saved.value, 'saved state is stable while revalidating the same source')
  assert.equal(a.media.loading.value, false)
  workerA.reply(callsFor(workerA, url)[1], file)
  await turn()
  assert.equal(
    a.media.source.value,
    stableSource,
    'same-source revalidation retains its object URL',
  )

  offlineArchive.online = true
  await offlineArchive.downloadFiles()
  offlineArchive.online = false
  await turn()
  assert.equal(offlineRevision.value, revision, 'an empty download queue does not revalidate media')
  assert.equal(callsFor(workerA, url).length, 2)
  assert.equal(callsFor(workerA, other).length, 1)

  // A deletion invalidation supersedes a read that was already dispatched.
  await offlineArchive.download({
    url,
    conversationId: 'fixture-room',
    messageId: 'fixture-message',
  })
  await turn()
  const obsolete = callsFor(workerA, url).at(-1)
  offlineArchive.event({
    seq: 1,
    type: 'message.updated',
    data: { id: 'fixture-message', deleted: true },
  })
  await offlineArchive.flush()
  await turn()
  const current = callsFor(workerA, url).at(-1)
  assert.notEqual(current.id, obsolete.id)
  workerA.reply(current, undefined)
  workerA.reply(obsolete, file)
  workerA.reply(callsFor(workerA, other).at(-1), file)
  await turn()
  assert.equal(
    a.media.saved.value,
    false,
    'deleted saved bytes cannot be restored by an older read',
  )
  assert(!liveURLs.has(stableSource))

  // Disable while waiting behind the four admitted worker requests.
  const busy = Array.from({ length: 4 }, (_, i) => mount(`https://files.invalid/busy-${i}`))
  await turn()
  const waitingURL = 'https://files.invalid/waiting'
  const waiting = mount(waitingURL)
  await turn()
  assert.equal(callsFor(workerA, waitingURL).length, 0)
  waiting.enabled.value = false
  assert.equal(waiting.media.loading.value, false, 'disabled media leaves no pending UI state')
  for (let i = 0; i < busy.length; i++) {
    workerA.reply(callsFor(workerA, `https://files.invalid/busy-${i}`)[0], undefined)
  }
  await turn()
  assert.equal(
    callsFor(workerA, waitingURL).length,
    0,
    'disabled queued media never reaches the worker',
  )
  for (const item of busy) item.scope.stop()
  waiting.scope.stop()

  await offlineArchive.download({
    url,
    conversationId: 'fixture-room',
    messageId: 'fixture-message',
  })
  await turn()
  const stale = callsFor(workerA, url).at(-1)
  const standalone = offlineArchive.file('standalone').catch((error) => error.name)
  await turn()
  const switchAccount = offlineArchive.start('fixture-b', '/api/v1')
  assert.equal(liveURLs.size, 0, 'account switch revokes existing object URLs synchronously')
  assert.equal(a.media.saved.value, false)
  await switchAccount
  await turn()
  assert.equal(await standalone, 'AbortError')
  const workerB = workers.at(-1)
  workerA.reply(stale, file)
  await turn()
  assert.equal(a.media.saved.value, false, 'old-account results cannot reach the new session')
  workerB.reply(callsFor(workerB, url).at(-1), file)
  await turn()
  assert(a.media.saved.value)
  const beforeDisable = a.media.source.value
  a.enabled.value = false
  assert.equal(a.media.loading.value, false)
  assert.equal(a.media.saved.value, false)
  assert(!liveURLs.has(beforeDisable), 'disabling releases owned bytes')
  a.enabled.value = true
  assert.equal(a.media.loading.value, true)
  assert.equal(a.media.source.value, undefined, 'reenabling checks saved bytes before the original')
  await turn()
  workerB.reply(callsFor(workerB, url).at(-1), file)
  await turn()
  assert.equal(a.media.saved.value, true)
  b.scope.stop()
  assert.equal(b.media.loading.value, false)
  assert.equal(b.media.source.value, undefined)
} finally {
  for (const scope of scopes) scope.stop()
  offlineArchive.stop()
}
assert.equal(liveURLs.size, 0, 'scope disposal leaves no object URLs')
console.log(
  'Offline file deduplication, bounded admission, targeted refresh, stable saved state, cancellation, deletion and account isolation pass.',
)
