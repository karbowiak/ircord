import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  createSession,
  sendMessage,
  history,
  toggleReaction,
  addConversation,
  addNetwork,
} from '../server/utils/demo-store'

test('sessions own independent history and profiles', () => {
  const a = createSession(),
    b = createSession()
  sendMessage(a, { clientId: 'a-message', conversationId: 'general', text: 'private to session a' })
  assert.equal(history(a, 'general').items.at(-1)?.text, 'private to session a')
  assert.equal(
    history(b, 'general').items.some((m) => m.text === 'private to session a'),
    false,
  )
  a.bootstrap.user.bio = 'changed'
  assert.notEqual(b.bootstrap.user.bio, 'changed')
})
test('retrying a message command acknowledges the same message once', () => {
  const session = createSession()
  const events: unknown[] = []
  session.listeners.add((event) => events.push(event))
  const input = { clientId: 'unique-command', conversationId: 'general', text: 'hi' }
  const a = sendMessage(session, input),
    b = sendMessage(session, input)
  assert.equal(a.id, b.id)
  // Durable events only; the echo bot's typing notice is ephemeral.
  assert.equal(events.filter((e) => 'seq' in (e as object)).length, 1)
})
test('history cursors page without duplicates and reject unknown cursors', () => {
  const session = createSession()
  const newest = history(session, 'general', undefined, 3)
  const older = history(session, 'general', newest.nextCursor!, 3)
  assert.equal(newest.items.length, 3)
  assert.equal(older.items.length, 3)
  assert.equal(
    older.items.some((m) => newest.items.some((n) => n.id === m.id)),
    false,
  )
  assert.throws(() => history(session, 'general', 'unknown'), /cursor has expired/)
})
test('commands cannot target conversations outside their session', () => {
  const a = createSession(),
    b = createSession()
  const conversation = addConversation(a, { networkId: 'libera', name: 'private-new-channel' })
  assert.throws(
    () => sendMessage(b, { clientId: 'unauthorized', conversationId: conversation.id, text: 'hi' }),
    /not found/,
  )
})
test('replies refer only to messages in the same conversation', () => {
  const s = createSession()
  const parent = history(s, 'general').items[0]!
  const m = sendMessage(s, {
    clientId: 'reply',
    conversationId: 'general',
    text: 'hello',
    replyToId: parent.id,
  })
  const cross = sendMessage(s, {
    clientId: 'cross-reply',
    conversationId: 'design',
    text: 'hello',
    replyToId: parent.id,
  })
  assert.equal(m.replyTo?.id, parent.id)
  assert.equal(cross.replyTo, undefined)
})
test('reactions toggle only the acting member and leave other reactions intact', () => {
  const s = createSession()
  const m = history(s, 'general').items.find((m) => m.reactions.length)!
  toggleReaction(s, 'general', m.id, '☕')
  assert.equal(m.reactions.find((r) => r.emoji === '☕')?.memberIds.includes('you'), true)
  toggleReaction(s, 'general', m.id, '☕')
  assert.equal(
    m.reactions.some((r) => r.emoji === '☕'),
    false,
  )
  assert.ok(m.reactions.length)
})
test('message and network input bounds reject invalid values', () => {
  const s = createSession()
  assert.throws(
    () => sendMessage(s, { clientId: 'empty', conversationId: 'general', text: ' ' }),
    /between/,
  )
  assert.throws(
    () => sendMessage(s, { clientId: 'large', conversationId: 'general', text: 'x'.repeat(4001) }),
    /between/,
  )
  assert.throws(
    () => addNetwork(s, { name: 'test', host: 'invalid/host', port: 6697, tls: true }),
    /valid port/,
  )
})
