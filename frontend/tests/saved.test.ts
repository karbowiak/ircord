import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  createSession,
  history,
  listSaved,
  redactMessage,
  saveMessage,
  savedRefs,
  sendMessage,
  unsaveMessage,
} from '../server/utils/demo-store'

test('saving is idempotent, private to the session and emits one event', () => {
  const a = createSession(),
    b = createSession()
  const events: { type: string }[] = []
  a.listeners.add((event) => events.push(event))
  const target = history(a, 'general').items.find((m) => m.kind === 'message')!
  const first = saveMessage(a, 'general', target.id)
  const again = saveMessage(a, 'general', target.id)
  assert.equal(first.status, 'available')
  assert.equal(again.savedAt, first.savedAt)
  assert.equal(savedRefs(a).items.length, 1)
  assert.deepEqual(
    events.map((e) => e.type),
    ['saved.added'],
  )
  assert.equal(savedRefs(b).items.length, 0)
  assert.throws(() => saveMessage(b, 'missing', target.id), /not found/i)
  unsaveMessage(a, target.id)
  unsaveMessage(a, target.id)
  assert.deepEqual(
    events.map((e) => e.type),
    ['saved.added', 'saved.removed'],
  )
})

test('a deleted source becomes a stub and drops out of a text filter', () => {
  const session = createSession()
  const mine = sendMessage(session, {
    clientId: 'saved-delete',
    conversationId: 'general',
    text: 'remember the milk',
  })
  saveMessage(session, 'general', mine.id)
  assert.equal(listSaved(session, '', 'milk').items.length, 1)
  redactMessage(session, 'general', mine.id)
  const [item] = listSaved(session).items
  assert.equal(item?.status, 'deleted')
  assert.equal(item?.message?.text, '')
  assert.equal(listSaved(session, '', 'milk').items.length, 0)
})

test('saved messages page newest first without duplicates', () => {
  const session = createSession()
  const ids = history(session, 'general')
    .items.filter((m) => m.kind === 'message')
    .slice(0, 5)
    .map((m) => m.id)
  ids.forEach((id) => saveMessage(session, 'general', id))
  const seen: string[] = []
  let cursor = ''
  for (let i = 0; i < 5; i++) {
    const page = listSaved(session, cursor, '', 2)
    seen.push(...page.items.map((x) => x.messageId))
    if (!page.nextCursor) break
    cursor = page.nextCursor
  }
  // Every saved message exactly once, newest saved first.
  assert.equal(seen.length, ids.length)
  assert.deepEqual(seen, [...ids].reverse())
  assert.throws(() => listSaved(session, 'forged'), /invalid cursor/)
})

test('history around a message returns a window and says newer messages exist', () => {
  const session = createSession()
  const all = history(session, 'general', undefined, 100).items
  const target = all[2]!
  const page = history(session, 'general', undefined, 3, target.id)
  assert.deepEqual(
    page.items.map((m) => m.id),
    all.slice(1, 4).map((m) => m.id),
  )
  assert.equal(page.hasNewer, true)
  assert.throws(() => history(session, 'general', undefined, 3, 'missing'), /not found/i)
})
