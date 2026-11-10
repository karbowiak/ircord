import assert from 'node:assert/strict'
import { createPreviewQueue, retainPreview } from '../app/utils/preview-queue.ts'

const run = createPreviewQueue(50)
const controllers = Array.from({ length: 52 }, () => new AbortController())
const release = []
const started = []
const job = (index) =>
  run(
    controllers[index].signal,
    () =>
      new Promise((resolve, reject) => {
        started.push(index)
        release[index] = resolve
        controllers[index].signal.addEventListener(
          'abort',
          () => reject(controllers[index].signal.reason),
          { once: true },
        )
      }),
  )
const tasks = Array.from({ length: 51 }, (_, index) => job(index))
const outcomes = tasks.map((task) => task.catch(() => 'cancelled'))
await Promise.resolve()
assert.equal(started.length, 50, '50 independent previews can start together')
assert(!started.includes(50), 'a 51st request must wait')
controllers[50].abort()
const visible = job(51)
await Promise.resolve()
assert(!started.includes(51))
controllers[0].abort()
await outcomes[0]
await Promise.resolve()
assert(started.includes(51), 'cancelling obsolete active work releases its slot')
assert(!started.includes(50), 'cancelled queued work never starts')
release[51]('visible')
assert.equal(await visible, 'visible')
for (let index = 1; index < 50; index++) release[index]('done')
await Promise.all(outcomes)

let cancelled = 0
const first = retainPreview(['account', 'message'], () => cancelled++)
const second = retainPreview(['account', 'message'], () => cancelled++)
first()
assert.equal(cancelled, 0, 'a second visible consumer still owns the request')
second()
assert.equal(cancelled, 1)
second()
assert.equal(cancelled, 1, 'releasing twice is harmless')
console.log('50-preview admission, obsolete queue cancellation and shared-reader ownership pass.')
