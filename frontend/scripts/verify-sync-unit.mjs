import assert from 'node:assert/strict'
import { mergeMessagePage } from '../shared/utils/message-page.ts'
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
  version: 2,
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
console.log(
  'History/live merge, message revisions, detached windows, and account-scoped checkpoints pass.',
)
