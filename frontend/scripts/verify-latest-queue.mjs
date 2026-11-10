import assert from 'node:assert/strict'
import { createLatestQueue } from '../app/utils/latest-queue.ts'

const originalSet = globalThis.setTimeout
const originalClear = globalThis.clearTimeout
const timers = new Map()
let nextTimer = 0
globalThis.setTimeout = (fn) => {
  const id = ++nextTimer
  timers.set(id, fn)
  return id
}
globalThis.clearTimeout = (id) => timers.delete(id)
const settle = async () => {
  for (let i = 0; i < 5; i++) await Promise.resolve()
}
const fire = async () => {
  for (const [id, fn] of [...timers]) {
    timers.delete(id)
    fn()
  }
  await settle()
}
try {
  const sent = [],
    saved = [],
    errors = []
  const flights = []
  let active = 0,
    maxActive = 0,
    dirty = true
  const queue = createLatestQueue(
    (value) => {
      sent.push(value)
      maxActive = Math.max(maxActive, ++active)
      return new Promise((resolve, reject) => flights.push({ resolve, reject })).finally(
        () => active--,
      )
    },
    {
      delay: 400,
      onSaved: (value, result) => {
        saved.push([value, result])
        dirty = false
      },
      onError: (error) => errors.push(error),
    },
  )
  queue.push('first')
  queue.push('second')
  await fire()
  assert.deepEqual(sent, ['second'])
  queue.push('third')
  await fire()
  flights[0].resolve('second saved')
  await settle()
  assert.deepEqual(saved, [])
  assert.deepEqual(sent, ['second', 'third'])
  assert.equal(maxActive, 1)
  flights[1].resolve('third saved')
  await settle()
  assert.deepEqual(saved, [['third', 'third saved']])

  queue.push('pending sync')
  queue.cancel()
  await fire()
  assert.equal(sent.length, 2)

  queue.push('in flight')
  await fire()
  queue.cancel()
  dirty = true // A local edit after sync was disabled must remain unsaved.
  flights[2].resolve('obsolete response')
  await settle()
  assert.equal(dirty, true)
  assert.equal(saved.length, 1)

  queue.push('old failure')
  await fire()
  queue.cancel()
  queue.push('new generation')
  await fire()
  flights[3].reject(new Error('obsolete failure'))
  await settle()
  assert.deepEqual(errors, [])
  assert.equal(maxActive, 1)
  assert.equal(sent.at(-1), 'new generation')
  flights[4].reject(new Error('current failure'))
  await settle()
  assert.equal(errors.length, 1)

  queue.push('disposed before send')
  queue.dispose()
  queue.push('ignored')
  await fire()
  assert.equal(queue.busy, false)
  assert.equal(timers.size, 0)
  assert.equal(sent.length, 5)
  console.log(
    'Latest settings queue: coalescing, single flight, cancellation, dirty-state preservation, failures and disposal passed.',
  )
} finally {
  globalThis.setTimeout = originalSet
  globalThis.clearTimeout = originalClear
}
