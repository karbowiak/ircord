import assert from 'node:assert/strict'
import test from 'node:test'
import {
  CHECKPOINT_INTERVAL,
  CHECKPOINT_SETTLE,
  createCheckpointSchedule,
} from './checkpoint-schedule'

function fakeTimers() {
  let now = 0
  let next = 1
  const pending = new Map<number, { at: number; run: () => void }>()
  return {
    now: () => now,
    setTimeout(run: () => void, ms: number) {
      const id = next++
      pending.set(id, { at: now + ms, run })
      return id
    },
    clearTimeout(handle: unknown) {
      pending.delete(handle as number)
    },
    advance(ms: number) {
      const until = now + ms
      for (;;) {
        const due = [...pending].filter(([, t]) => t.at <= until).sort((a, b) => a[1].at - b[1].at)
        if (!due.length) break
        const [id, timer] = due[0]!
        pending.delete(id)
        now = timer.at
        timer.run()
      }
      now = until
    },
  }
}

test('a burst of changes writes once, shortly after the first', () => {
  const timers = fakeTimers()
  let writes = 0
  const schedule = createCheckpointSchedule(() => (writes++, true), timers)
  for (let i = 0; i < 50; i++) {
    schedule.changed()
    timers.advance(10)
  }
  timers.advance(CHECKPOINT_SETTLE)
  assert.equal(writes, 1)
})

test('a steady stream of changes writes at most once a minute', () => {
  const timers = fakeTimers()
  let writes = 0
  const schedule = createCheckpointSchedule(() => (writes++, true), timers)
  // One event every 100ms for five minutes.
  for (let t = 0; t < 5 * CHECKPOINT_INTERVAL; t += 100) {
    schedule.changed()
    timers.advance(100)
  }
  assert.ok(writes <= 6, `wrote ${writes} times`)
  assert.ok(writes >= 5, `wrote ${writes} times`)
})

test('hiding the page writes at once and replaces the waiting write', () => {
  const timers = fakeTimers()
  let writes = 0
  const schedule = createCheckpointSchedule(() => (writes++, true), timers)
  schedule.changed()
  schedule.flush()
  assert.equal(writes, 1)
  timers.advance(CHECKPOINT_INTERVAL * 2)
  assert.equal(writes, 1)
  // Hiding saves even within the interval; the next change waits for the interval again.
  schedule.changed()
  timers.advance(CHECKPOINT_SETTLE)
  assert.equal(writes, 2)
  schedule.flush()
  assert.equal(writes, 3)
  schedule.changed()
  timers.advance(CHECKPOINT_INTERVAL - 1)
  assert.equal(writes, 3)
  timers.advance(1)
  assert.equal(writes, 4)
})

test('a change that cannot be saved yet does not use up the interval', () => {
  const timers = fakeTimers()
  let ready = false
  let writes = 0
  const schedule = createCheckpointSchedule(() => (ready ? (writes++, true) : undefined), timers)
  schedule.changed()
  timers.advance(CHECKPOINT_SETTLE)
  assert.equal(writes, 0)
  ready = true
  schedule.changed()
  timers.advance(CHECKPOINT_SETTLE)
  assert.equal(writes, 1)
})

test('a failed write waits for the interval before trying again', () => {
  const timers = fakeTimers()
  let attempts = 0
  const schedule = createCheckpointSchedule(() => (attempts++, false), timers)
  schedule.changed()
  timers.advance(CHECKPOINT_SETTLE)
  assert.equal(attempts, 1)
  schedule.changed()
  timers.advance(CHECKPOINT_INTERVAL / 2)
  assert.equal(attempts, 1)
  timers.advance(CHECKPOINT_INTERVAL)
  assert.equal(attempts, 2)
})
