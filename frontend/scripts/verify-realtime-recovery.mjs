import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { Transpiler } from 'bun'
import { boundedRead, createRealtimeRecovery, RECOVERY } from '../app/utils/realtime-recovery.ts'
import { mergeMessagePage } from '../shared/utils/message-page.ts'

const flush = async () => {
  for (let i = 0; i < 30; i++) await Promise.resolve()
}
const deferred = () => {
  let resolve, reject
  const promise = new Promise((yes, no) => {
    resolve = yes
    reject = no
  })
  return { promise, resolve, reject }
}
const real = {
  now: Date.now,
  setTimeout: globalThis.setTimeout,
  clearTimeout: globalThis.clearTimeout,
  random: Math.random,
}
let clock = 0,
  serial = 0
const timers = new Map()
Date.now = () => clock
Math.random = () => 0
globalThis.setTimeout = (fn, delay) => {
  const id = ++serial
  timers.set(id, { fn, at: clock + delay })
  return id
}
globalThis.clearTimeout = (id) => timers.delete(id)
async function advance(ms) {
  const end = clock + ms
  await flush()
  for (let steps = 0; steps < 1000; steps++) {
    const next = [...timers.entries()]
      .filter(([, t]) => t.at <= end)
      .sort((a, b) => a[1].at - b[1].at)[0]
    if (!next) {
      clock = end
      await flush()
      return
    }
    clock = Math.max(clock, next[1].at)
    timers.delete(next[0])
    next[1].fn()
    await flush()
  }
  throw new Error('Timer loop did not settle')
}
function fixture() {
  const f = {
    online: true,
    visible: true,
    sockets: [],
    states: [],
    failed: false,
    expired: 0,
    snapshots: [],
    snapshotSignals: [],
    sessionSignals: [],
    barriers: [],
    invalidations: 0,
    snapshot: async () => ({ cursor: 90 }),
    session: async () => true,
  }
  const recovery = createRealtimeRecovery({
    enabled: () => true,
    online: () => f.online,
    visible: () => f.visible,
    open: () => {
      const socket = {
        closed: false,
        close() {
          this.closed = true
        },
      }
      f.sockets.push(socket)
      return socket
    },
    socket: (socket) => {
      f.socket = socket
    },
    receive: (raw) => {
      if (raw === 'ready') recovery.ready()
      else if (raw === 'synced') recovery.synced()
      else if (raw === 'progress') recovery.progress()
      else if (raw === 'resync') recovery.resync()
      else throw new Error('Synthetic invalid frame')
    },
    snapshot: (signal) => {
      f.snapshotSignals.push(signal)
      return f.snapshot(signal)
    },
    invalidate: () => f.invalidations++,
    install: (snapshot) => f.snapshots.push(snapshot),
    session: (signal) => {
      f.sessionSignals.push(signal)
      return f.session(signal)
    },
    expired: () => f.expired++,
    status: (state) => {
      f.state = state
      f.states.push(state)
    },
    failure: (failed) => {
      f.failed = failed
    },
    lost: () => {},
    synced: (resumed) => f.barriers.push(resumed),
  })
  f.recovery = recovery
  f.receive = (type) => f.socket.onmessage({ data: type })
  f.synced = () => {
    f.receive('ready')
    f.receive('synced')
  }
  f.stop = () => {
    recovery.dispose()
    assert.equal(timers.size, 0, 'dispose releases every controller timer')
  }
  recovery.start()
  return f
}

try {
  const resumeSamples = []
  {
    const f = fixture()
    f.synced()
    f.recovery.visibility(true)
    f.visible = false
    f.socket.onerror()
    // Frozen timers become runnable before the visibilitychange task on thaw.
    clock += 60_000
    await advance(0)
    const hiddenSockets = f.sockets.length - 1
    f.visible = true
    const foreground = clock
    f.recovery.visibility(false)
    const foregroundSockets = f.sockets.length
    for (
      let ms = 0;
      foregroundSockets === hiddenSockets + 1 &&
      f.sockets.length === foregroundSockets &&
      ms < 16_000;
      ms += 100
    )
      await advance(100)
    f.synced()
    resumeSamples.push({
      scenario: 'thawed retry before visibility',
      hiddenSockets,
      foregroundDelayMs: clock - foreground,
    })
    f.stop()
  }
  {
    const f = fixture()
    f.synced()
    f.recovery.visibility(true)
    f.visible = false
    for (let i = 0; i < 6; i++) {
      f.socket?.onerror()
      await advance(16_000)
    }
    const hiddenSockets = f.sockets.length - 1
    f.visible = true
    f.recovery.visibility(false)
    f.socket.onerror()
    const foreground = clock
    for (let ms = 0; !f.socket && ms < 16_000; ms += 100) await advance(100)
    f.synced()
    resumeSamples.push({
      scenario: 'hidden failures then fast foreground failure',
      hiddenSockets,
      foregroundDelayMs: clock - foreground,
    })
    f.stop()
  }
  console.log(JSON.stringify({ resumeSamples }))
  assert.equal(resumeSamples[0].hiddenSockets, 1, 'background transport recovery remains available')
  assert.equal(
    resumeSamples[0].foregroundDelayMs,
    0,
    'a hidden retry cannot suppress the foreground attempt',
  )
  assert.equal(
    resumeSamples[1].hiddenSockets,
    7,
    'background retries retain their existing bounded backoff',
  )
  assert.equal(
    resumeSamples[1].foregroundDelayMs,
    800,
    'foreground failures restart with the initial backoff',
  )
  {
    const f = fixture()
    for (const milliseconds of [800, 1600, 3200, 6400, 12_800, 15_000, 15_000]) {
      f.socket.onerror()
      const count = f.sockets.length
      f.recovery.resume()
      f.recovery.resume(true)
      await advance(milliseconds - 1)
      assert.equal(f.sockets.length, count, 'real foreground outages retain exponential backoff')
      await advance(1)
      assert.equal(f.sockets.length, count + 1)
    }
    f.stop()
  }
  {
    const f = fixture()
    f.socket.onerror()
    const queuedRetry = [...timers.values()].find((timer) => timer.at === clock + 800).fn
    f.recovery.visibility(true)
    f.visible = false
    clock += 60_000
    // A throttled background retry can run before the visibilitychange listener.
    f.visible = true
    queuedRetry()
    assert.equal(f.sockets.length, 2, 'background pages can still restore their live transport')
    f.recovery.visibility(false)
    assert.equal(f.sockets.length, 3, 'the pending visibility event starts recovery immediately')
    queuedRetry()
    assert.equal(
      f.sockets.length,
      3,
      'an obsolete queued retry cannot replace the foreground generation',
    )
    f.stop()
  }
  for (const visibleFirst of [false, true]) {
    const f = fixture()
    f.synced()
    const staleMessage = f.socket.onmessage
    f.recovery.visibility(true)
    f.visible = false
    f.recovery.freeze()
    assert(f.sockets[0].closed, 'freeze releases an apparently open socket')
    clock += 60_000
    await advance(0)
    assert.equal(f.failed, false, 'frozen time does not consume the warning grace')
    assert.equal(f.sockets.length, 1)
    if (visibleFirst) {
      f.visible = true
      f.recovery.visibility(false)
      assert.equal(f.sockets.length, 1, 'visibility alone cannot reconnect a still-frozen page')
    }
    f.recovery.thaw()
    if (!visibleFirst) {
      assert.equal(f.sockets.length, 1, 'resume while hidden waits for visibility')
      f.visible = true
      f.recovery.visibility(false)
    }
    for (let i = 0; i < 5; i++) {
      f.recovery.thaw()
      f.recovery.resume(true)
      f.recovery.visibility(false)
    }
    assert.equal(f.sockets.length, 2, 'either lifecycle ordering creates one replacement')
    staleMessage({ data: 'synced' })
    assert.equal(f.state, 'reconnecting', 'pre-freeze handlers cannot complete a replacement')
    f.receive('ready')
    assert.equal(f.state, 'syncing', 'foreground still waits for the replay barrier')
    f.receive('progress')
    f.receive('synced')
    assert.deepEqual(f.barriers, [false, true])
    f.stop()
  }
  {
    const f = fixture(),
      old = deferred(),
      fresh = deferred()
    let count = 0
    f.snapshot = () => (++count === 1 ? old.promise : fresh.promise)
    f.recovery.resync()
    await flush()
    f.recovery.freeze()
    assert(f.snapshotSignals[0].aborted, 'freeze aborts a pending replacement snapshot')
    f.recovery.thaw()
    await flush()
    old.resolve({ cursor: 999 })
    await flush()
    assert.equal(f.snapshots.length, 0, 'pre-freeze data cannot install after thaw')
    fresh.resolve({ cursor: 103 })
    await flush()
    assert.deepEqual(f.snapshots, [{ cursor: 103 }])
    f.synced()
    f.stop()
  }
  {
    const f = fixture()
    // A fresh attempt made before a short hide must not swallow the only foreground event.
    f.recovery.visibility(true)
    f.visible = false
    await advance(100)
    f.visible = true
    f.recovery.visibility(false)
    f.recovery.resume()
    f.recovery.resume(true)
    assert.equal(f.sockets.length, 2)
    f.stop()
  }
  {
    const f = fixture(),
      hung = deferred()
    f.synced()
    f.session = () => hung.promise
    const staleClose = f.socket.onclose
    f.socket.onerror()
    await flush()
    assert.equal(f.state, 'reconnecting')
    const oldSignal = f.sessionSignals.at(-1)
    await advance(800)
    assert.equal(f.sockets.length, 2, 'HTTP session lookup cannot block a new socket')
    assert(oldSignal.aborted, 'next attempt aborts the old session check')
    f.synced()
    hung.resolve(false)
    staleClose({ code: 1006 })
    await flush()
    assert.equal(f.expired, 0, 'obsolete session result cannot expire the healthy replacement')
    assert.equal(f.state, 'connected')
    f.stop()
  }
  {
    const f = fixture()
    f.session = async () => false
    f.socket.onerror()
    await flush()
    assert.equal(f.expired, 1)
    assert.equal(timers.size, 0, 'confirmed expiry cancels recovery')
    f.recovery.reconnect()
    f.recovery.resume(true)
    assert.equal(f.sockets.length, 1)
    f.stop()
  }
  {
    const f = fixture()
    await advance(RECOVERY.ready)
    assert.equal(f.socket, undefined, 'missing session.ready times out')
    await advance(800)
    f.receive('ready')
    await advance(RECOVERY.replayIdle - 1)
    f.receive('progress')
    await advance(RECOVERY.replayIdle - 1)
    assert.equal(f.state, 'syncing', 'durable replay progress extends the idle deadline')
    assert.equal(f.failed, true, 'ready/progress do not reset the failure episode')
    await advance(1)
    assert.equal(f.state, 'reconnecting', 'stalled replay retries')
    await advance(1600)
    f.synced()
    assert.equal(f.failed, false, 'only the completed barrier clears the warning episode')
    f.stop()
  }
  {
    const f = fixture()
    f.receive('ready')
    for (let i = 0; i < 4; i++) {
      await advance(10_000)
      f.receive('progress')
    }
    await advance(5000)
    assert.equal(f.state, 'reconnecting', 'total replay cap prevents endless progress trickles')
    f.stop()
  }
  {
    const f = fixture()
    await advance(1000)
    // pagehide can precede visibilityState changing; explicit hidden state must pause grace.
    f.recovery.visibility(true)
    await advance(60_000)
    assert.equal(f.failed, false, 'background time does not consume foreground-online grace')
    f.recovery.visibility(false)
    await advance(RECOVERY.grace - 1001)
    assert.equal(f.failed, false)
    await advance(1)
    assert.equal(f.failed, true)
    f.stop()
  }
  {
    const f = fixture()
    f.online = false
    f.recovery.offline()
    f.online = true
    f.recovery.resume(true)
    assert.equal(
      f.sockets.length,
      2,
      'instant offline/online bounce cannot lose the only recovery trigger',
    )
    f.recovery.resume(true)
    assert.equal(
      f.sockets.length,
      2,
      'the following online/focus event coalesces with that active attempt',
    )
    f.stop()
  }
  {
    const f = fixture()
    f.synced()
    await advance(1000)
    f.recovery.resume()
    assert.equal(f.sockets.length, 1, 'ordinary focus preserves a healthy fresh socket')
    f.recovery.visibility(true)
    f.visible = false
    await advance(RECOVERY.staleAfter)
    f.visible = true
    f.recovery.visibility(false)
    f.recovery.resume(true)
    f.recovery.resume()
    assert.equal(f.sockets.length, 2, 'resume/focus/pageshow/online burst coalesces')
    assert.equal(f.sockets.filter((socket) => !socket.closed).length, 1)
    f.synced()
    await advance(1000)
    f.recovery.resume(true)
    assert.equal(f.sockets.length, 3, 'persisted pageshow can replace an apparently open socket')
    f.stop()
  }
  {
    const f = fixture()
    f.synced()
    const first = deferred(),
      second = deferred()
    let count = 0
    f.snapshot = () => (++count === 1 ? first.promise : second.promise)
    f.recovery.resync()
    await flush()
    const oldSignal = f.snapshotSignals[0]
    f.recovery.reconnect()
    await flush()
    assert(oldSignal.aborted, 'explicit Retry supersedes a held snapshot')
    first.resolve({ cursor: 999 })
    await flush()
    assert.equal(f.snapshots.length, 0, 'superseded snapshot cannot install an obsolete cursor')
    second.resolve({ cursor: 100 })
    await flush()
    assert.deepEqual(f.snapshots, [{ cursor: 100 }])
    f.synced()
    assert.equal(f.state, 'connected')
    f.stop()
  }
  {
    const f = fixture(),
      old = deferred()
    let count = 0
    f.snapshot = () => (++count === 1 ? old.promise : Promise.resolve({ cursor: 101 }))
    f.recovery.resync()
    await advance(RECOVERY.snapshot)
    assert.equal(f.state, 'reconnecting', 'snapshot timeout is not falsely device-offline')
    assert(f.snapshotSignals[0].aborted)
    await advance(800)
    assert.deepEqual(f.snapshots, [{ cursor: 101 }], 'snapshot failure automatically retries')
    old.resolve({ cursor: 998 })
    await flush()
    assert.equal(f.snapshots.length, 1)
    f.stop()
  }
  {
    const f = fixture()
    f.snapshot = async () => {
      throw new Error('Synthetic HTTP 503')
    }
    f.recovery.resync()
    await flush()
    assert.equal(f.state, 'reconnecting')
    await advance(800)
    assert.equal(f.snapshotSignals.length, 2)
    f.online = false
    f.recovery.offline()
    await advance(30_000)
    assert.equal(f.state, 'offline')
    assert.equal(f.failed, false)
    const count = f.sockets.length
    f.online = true
    f.snapshot = async () => ({ cursor: 102 })
    f.recovery.resume(true)
    await flush()
    assert.equal(f.sockets.length, count + 1)
    f.stop()
  }

  // Execute the actual store history-refresh functions with synthetic cache and read adapters.
  const source = await readFile(new URL('../app/stores/chat.ts', import.meta.url), 'utf8')
  const start = source.indexOf('  function cancelRecoveryHistory()')
  const end = source.indexOf('  watch(', start)
  assert(start > 0 && end > start)
  const transpiler = new Transpiler({ loader: 'ts' })
  {
    const f = fixture()
    f.synced()
    f.recovery.visibility(true)
    await advance(RECOVERY.staleAfter)
    const from = source.indexOf('  const onPageShow =')
    const to = source.indexOf('  const onPageHide =', from)
    const pageShow = new Function(
      'recovery',
      `${transpiler.transformSync(source.slice(from, to))}; return onPageShow`,
    )(f.recovery)
    pageShow({ persisted: true })
    assert.equal(
      f.sockets.length,
      2,
      'persisted pageshow clears pagehide even without visibilitychange',
    )
    f.stop()
  }
  const createHistory = new Function(
    'env',
    `with (env) {
    let historyRefresh
    ${transpiler.transformSync(source.slice(start, end))}
    return { refreshHistory, cancelRecoveryHistory }
  }`,
  )
  const history = (current, read) => {
    const f = {
      disposed: false,
      activeId: { value: 'room' },
      historyRefreshingId: { value: '' },
      historyFailedId: { value: '' },
      boundedRead,
      RECOVERY,
      HISTORY_PAGE: 40,
      mergeMessagePage,
      messagesQuery: (id) => ({ key: ['messages', id] }),
      readMessages: read,
      queryCache: {
        getQueryData: () => f.page,
        setQueryData: (_key, page) => {
          f.page = page
        },
      },
      page: current,
    }
    return { ...createHistory(f), f }
  }
  const message = (id, seq) => ({
    id,
    text: id,
    createdAt: new Date(Date.UTC(2026, 10, 10, 10, 0, seq)).toISOString(),
    liveSeq: seq,
  })
  {
    const pending = deferred()
    const original = { items: [message('one', 40)], cursor: 40, nextCursor: null }
    let reads = 0
    const h = history(original, () => {
      reads++
      return pending.promise
    })
    const a = h.refreshHistory(),
      b = h.refreshHistory()
    await flush()
    assert.equal(reads, 1, 'overlapping barriers/history events deduplicate the latest read')
    h.f.page = { ...original, items: [...original.items, message('live', 42)] }
    pending.resolve({ items: [message('one', 41)], cursor: 41, nextCursor: null })
    await Promise.all([a, b])
    assert(
      h.f.page.items.some((item) => item.id === 'live'),
      'late history preserves newer live updates',
    )
    assert.equal(h.f.historyFailedId.value, '')
  }
  {
    const original = {
      items: [message('older-anchor', 1)],
      cursor: 1,
      nextCursor: 'older',
      detached: true,
      hasNewer: true,
    }
    const h = history(original, async () => ({
      items: [message('latest', 41)],
      cursor: 41,
      nextCursor: null,
    }))
    await h.refreshHistory()
    assert.equal(h.f.page, original, 'latest reconciliation never navigates an older window')
  }
  {
    const original = { items: [message('loaded', 1)], cursor: 1, nextCursor: null }
    let failed = true
    const h = history(original, async () => {
      if (failed) throw new Error('Synthetic history unavailable')
      return { items: [message('latest', 2)], cursor: 2, nextCursor: null }
    })
    await h.refreshHistory()
    assert.equal(h.f.page, original, 'refresh failure keeps loaded messages')
    assert.equal(h.f.historyFailedId.value, 'room')
    failed = false
    await h.refreshHistory()
    assert.equal(
      h.f.historyFailedId.value,
      '',
      'the separate refresh action can clear a history failure',
    )
  }
  {
    const original = { items: [message('loaded', 1)], cursor: 1, nextCursor: null }
    const held = deferred()
    const h = history(original, () => held.promise)
    const refresh = h.refreshHistory()
    await advance(RECOVERY.history)
    await refresh
    assert.equal(h.f.page, original)
    assert.equal(h.f.historyFailedId.value, 'room')
    held.resolve({ items: [], cursor: 99, nextCursor: null })
    await flush()
    assert.equal(h.f.page, original, 'timed-out history cannot replace the current window later')
  }
  assert.equal(timers.size, 0)
  console.log(
    'Recovery deadlines, nonblocking auth, stale fencing, foreground grace, coalesced resume, resync retries/disposal and latest-history preservation pass.',
  )
} finally {
  Date.now = real.now
  Math.random = real.random
  globalThis.setTimeout = real.setTimeout
  globalThis.clearTimeout = real.clearTimeout
}
