import assert from 'node:assert/strict'
import { createHistoryPagination } from '../app/utils/history-pagination.ts'
import {
  appendMessagePage,
  mergeMessagePage,
  updateMessagePage,
} from '../shared/utils/message-page.ts'
import { boundaryMessage, messageBoundary } from '../shared/utils/history-cursor.ts'

const message = (index, room = 'room') => ({
  id: `m${String(index).padStart(4, '0')}`,
  conversationId: room,
  senderId: 'peer',
  kind: 'message',
  text: `Fixture ${index}`,
  createdAt: new Date(Date.UTC(2026, 10, 1, 0, 0, Math.floor(index / 3))).toISOString(),
  position: index % 3,
  reactions: [],
})
const retained = Array.from({ length: 1500 }, (_, index) => message(index))
function window(query, items = retained, cursor = 10) {
  const boundary = query.around ?? boundaryMessage(query.before)
  const at = boundary ? items.findIndex((item) => item.id === boundary) : items.length
  assert(at >= 0)
  const start = query.around
    ? Math.max(0, at - Math.floor((query.limit - 1) / 2))
    : Math.max(0, at - query.limit)
  const end = query.around ? Math.min(items.length, at + Math.ceil((query.limit - 1) / 2) + 1) : at
  return {
    items: items.slice(start, end),
    nextCursor: start ? messageBoundary(items[start].id) : null,
    hasNewer: !!query.around && end < items.length,
    cursor,
  }
}
const cache = new Map([['room', window({ limit: 40 })]])
const notices = [],
  visited = new Set(),
  requests = []
let direction,
  active = 0,
  maximum = 0
let read = async (id, query) => {
  assert.equal(id, 'room')
  requests.push(query)
  active++
  maximum = Math.max(maximum, active)
  await Promise.resolve()
  active--
  return window(query)
}
const pager = createHistoryPagination({
  pageSize: 40,
  limit: 500,
  get: (id) => cache.get(id),
  set: (id, page) => {
    assert(page.items.length <= 500)
    assert.equal(new Set(page.items.map((item) => item.id)).size, page.items.length)
    cache.set(id, page)
    for (const item of page.items) visited.add(item.id)
  },
  read: (...args) => read(...args),
  changed: (value) => {
    direction = value
  },
  report: (error) => notices.push(error),
})
for (const item of cache.get('room').items) visited.add(item.id)
let olderPages = 0,
  newerPages = 0
while (cache.get('room').nextCursor) {
  assert.equal(await pager.load('room', 'older'), true)
  assert(++olderPages < 100)
}
assert.equal(cache.get('room').items[0].id, 'm0000')
assert(cache.get('room').detached && cache.get('room').hasNewer)
while (cache.get('room').hasNewer) {
  assert.equal(await pager.load('room', 'newer'), true)
  assert(++newerPages < 100)
}
assert.equal(cache.get('room').items.at(-1).id, 'm1499')
assert.equal(cache.get('room').detached, false)
assert.equal(visited.size, retained.length, 'every retained message is reachable without jumping')
assert.equal(boundaryMessage(cache.get('room').nextCursor), cache.get('room').items[0].id)
assert.equal(maximum, 1)
assert(requests.every((query) => query.limit <= 81))

// An authoritative end read must cover events skipped while that read was in flight.
let page = { ...window({ around: 'm0020', limit: 41 }, retained.slice(0, 80)), detached: true }
page = updateMessagePage(page, message(80), 11)
page = updateMessagePage(page, { ...message(20), text: 'Latest edit' }, 12)
assert(!page.items.some((item) => item.id === 'm0080'))
const endAtOldCut = window({ around: page.items.at(-1).id, limit: 81 }, retained.slice(0, 80))
page = appendMessagePage(page, endAtOldCut)
assert(
  page.detached && page.hasNewer,
  'a stale end response cannot erase the skipped-event watermark',
)
assert.equal(page.items.find((item) => item.id === 'm0020').text, 'Latest edit')
page = appendMessagePage(page, window({ around: 'm0079', limit: 81 }, retained.slice(0, 81), 12))
assert.equal(page.items.at(-1).id, 'm0080')
assert.equal(page.detached, false)
assert.equal(page.newerLiveSeq, undefined)
const newerEvent = updateMessagePage({ ...page, detached: true }, message(81), 13)
assert(
  mergeMessagePage({ ...endAtOldCut, cursor: 12 }, newerEvent).detached,
  'newest refreshes also retain the uncovered live watermark',
)

const savedEnd = appendMessagePage(
  { ...page, detached: true, hasNewer: true },
  {
    ...window({ around: 'm0080', limit: 81 }, retained.slice(0, 81), 12),
    offline: true,
  },
)
assert.equal(savedEnd.hasNewer, false, 'the saved archive has its own finite end')
assert.equal(savedEnd.detached, true, 'a local archive end cannot prove the live edge')
assert.equal(
  appendMessagePage(savedEnd, window({ around: 'm0080', limit: 81 }, retained.slice(0, 81), 12))
    .detached,
  false,
)

// Cancelled navigation must neither write a stale page nor release a newer request's slot.
cache.set('room', { ...window({ around: 'm0100', limit: 41 }), detached: true })
const blocked = []
read = (id, query, signal) => new Promise((resolve) => blocked.push({ id, query, signal, resolve }))
const oldRead = pager.load('room', 'newer')
assert.equal(await pager.load('other-account-room', 'older'), undefined)
pager.cancel()
assert(blocked[0].signal.aborted)
const freshRead = pager.load('room', 'newer')
const beforeStale = cache.get('room')
blocked[0].resolve(window(blocked[0].query))
assert.equal(await oldRead, undefined)
assert.equal(cache.get('room'), beforeStale)
assert.equal(direction, 'newer', 'settling an aborted request must not clear its replacement')
blocked[1].resolve(window(blocked[1].query))
assert.equal(await freshRead, true)
assert.equal(direction, undefined)

// A newer/around replacement with different boundaries rejects the prior response.
const staleRead = pager.load('room', 'newer')
const replacement = { ...window({ around: 'm0800', limit: 41 }), detached: true }
cache.set('room', replacement)
blocked[2].resolve(window(blocked[2].query))
assert.equal(await staleRead, undefined)
assert.equal(cache.get('room'), replacement)
read = async () => ({ items: [message(999)], nextCursor: null, hasNewer: false, cursor: 10 })
assert.equal(await pager.load('room', 'newer'), false, 'disjoint pages fail without creating a gap')
assert.equal(cache.get('room'), replacement)
read = async () => ({
  items: [replacement.items.at(-1)],
  nextCursor: null,
  hasNewer: true,
  cursor: 10,
})
assert.equal(
  await pager.load('room', 'newer'),
  false,
  'a non-advancing continuation fails without looping',
)
assert.equal(notices.length, 2)
read = async () => ({ ...replacement, items: [] })
assert.equal(
  await pager.load('room', 'older'),
  false,
  'a repeated older boundary fails without looping',
)
assert.equal(cache.get('room'), replacement)
assert.equal(notices.length, 3)
cache.set('room', { ...replacement, partial: true })
assert.equal(
  await pager.load('room', 'newer'),
  undefined,
  'an unverified checkpoint cannot start paging',
)

console.log(
  JSON.stringify({
    retainedMessages: retained.length,
    visited: visited.size,
    olderPages,
    newerPages,
    bound: 500,
    maximumConcurrentReads: maximum,
    liveRevisions: 'pass',
    readCutRace: 'pass',
    offlineEnd: 'pass',
    staleCancellation: 'pass',
    gaps: 'rejected',
  }),
)
