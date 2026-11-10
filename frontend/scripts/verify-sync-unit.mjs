import assert from 'node:assert/strict'
import {
  mergeMessagePage,
  newestMessagePage,
  prependMessagePage,
  updateMessagePage,
} from '../shared/utils/message-page.ts'
import { boundaryMessage, messageBoundary } from '../shared/utils/history-cursor.ts'
import { readCheckpoint, writeCheckpoint } from '../app/utils/workspace-checkpoint.ts'

const message = (id, text, liveSeq) => ({
  id,
  conversationId: 'room',
  senderId: 'peer',
  kind: 'message',
  createdAt: '2026-11-01T12:00:00Z',
  text,
  reactions: [],
  liveSeq,
})
const page = { items: [message('one', 'old'), message('two', 'old')], nextCursor: null, cursor: 10 }
const current = {
  items: [
    { ...message('one', '', 11), deleted: true },
    { ...message('two', 'edited', 12), reactions: [{ emoji: '👍', memberIds: ['peer'] }] },
    message('three', 'new', 13),
  ],
  nextCursor: null,
}
const merged = mergeMessagePage(page, current)
assert.equal(merged.items.length, 3)
assert(merged.items[0].deleted)
assert.equal(merged.items.find((m) => m.id === 'two').text, 'edited')
assert.equal(merged.items.find((m) => m.id === 'two').reactions.length, 1)
assert.equal(mergeMessagePage({ ...page, cursor: 20 }, current).items[0].text, 'old')
assert.equal(
  mergeMessagePage(page, current, true).items.length,
  2,
  'detached history does not append new messages',
)
assert.equal(merged.items.find((m) => m.id === 'three').liveSeq, 13)
assert.equal(
  mergeMessagePage(page).items[0].liveSeq,
  10,
  'history revisions protect against older replay',
)

const values = new Map()
globalThis.localStorage = {
  getItem: (key) => values.get(key) ?? null,
  setItem: (key, value) => values.set(key, value),
}
const checkpoint = {
  version: 3,
  accountId: 'alice',
  savedAt: new Date().toISOString(),
  bootstrap: { version: 1, cursor: 13, user: {}, networks: [], members: [], conversations: [] },
  messages: { room: merged },
}
writeCheckpoint(checkpoint)
assert.equal(readCheckpoint('alice').bootstrap.cursor, 13)
assert.equal(readCheckpoint('bob'), undefined)
values.set('ircord.snapshot.bob', JSON.stringify(checkpoint))
assert.equal(readCheckpoint('bob'), undefined, 'account identity must match the storage key')
values.set('ircord.snapshot.alice', JSON.stringify({ ...checkpoint, version: 1 }))
assert.equal(readCheckpoint('alice'), undefined, 'old checkpoints lack a matching applied cursor')
values.set(
  'ircord.snapshot.alice',
  JSON.stringify({ ...checkpoint, messages: { room: { items: null } } }),
)
assert.equal(readCheckpoint('alice'), undefined)

const chronological = (index, seq = 1) => ({
  ...message(`m${String(index).padStart(4, '0')}`, `message ${index}`, seq),
  createdAt: new Date(Date.UTC(2026, 10, 1, 0, index)).toISOString(),
})
const liveWindow = {
  items: Array.from({ length: 500 }, (_, index) => chronological(index + 40)),
  nextCursor: 'older-page-cursor',
  cursor: 1,
}
const olderWindow = prependMessagePage(liveWindow, {
  items: Array.from({ length: 40 }, (_, index) => chronological(index)),
  nextCursor: 'previous-page-cursor',
  cursor: 1,
})
assert.equal(olderWindow.items.length, 500)
assert.equal(olderWindow.items[0].id, 'm0000')
assert.equal(olderWindow.items.at(-1).id, 'm0499')
assert(olderWindow.detached && olderWindow.hasNewer, 'dropping the live end marks an older window')
const skippedLive = updateMessagePage(olderWindow, chronological(540, 2), 2)
assert.equal(skippedLive.items, olderWindow.items, 'detached windows do not append across a gap')
assert.equal(skippedLive.newerLiveSeq, 2, 'the skipped event must be covered before reattaching')
const changedOld = updateMessagePage(olderWindow, { ...chronological(1), text: 'edited' }, 3)
assert.equal(changedOld.items[1].text, 'edited', 'edits still reach a detached window')
const latest = mergeMessagePage(
  {
    items: Array.from({ length: 40 }, (_, index) => chronological(index + 501)),
    nextCursor: 'server-latest-cursor',
    cursor: 2,
  },
  changedOld,
)
assert.equal(latest.items.length, 40, 'returning latest cannot reintroduce a disconnected old edit')
assert.equal(latest.items[0].id, 'm0501')
assert.equal(latest.items.at(-1).id, 'm0540')
assert.equal(latest.detached, false)

const appended = updateMessagePage(liveWindow, chronological(540), 2)
assert.equal(
  boundaryMessage(appended.nextCursor),
  appended.items[0].id,
  'live trimming moves the older-page boundary',
)
const checkpointPage = newestMessagePage(liveWindow, 100)
assert.equal(checkpointPage.items.length, 100)
assert.equal(
  boundaryMessage(checkpointPage.nextCursor),
  checkpointPage.items[0].id,
  'checkpoint clipping cannot reuse a cursor500 messages back',
)
assert.equal(boundaryMessage(messageBoundary('東京:abc/def')), '東京:abc/def')
assert.equal(boundaryMessage('offline:' + btoa(JSON.stringify({ id: 'boundary' }))), 'boundary')
assert.equal(boundaryMessage('offline:broken'), undefined)
values.set('ircord.snapshot.alice', JSON.stringify({ ...checkpoint, version: 2 }))
assert.equal(
  readCheckpoint('alice'),
  undefined,
  'old checkpoints may contain skipped pagination frontiers',
)
console.log(
  'History/live merge, bounded older/latest windows, exact checkpoint frontiers and account isolation pass.',
)
