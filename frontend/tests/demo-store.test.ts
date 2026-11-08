import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  createSession,
  sendMessage,
  history,
  toggleReaction,
  redactMessage,
  ForbiddenError,
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
test('only the author can delete a message, and replies stop quoting it', () => {
  const s = createSession()
  const events: { type: string; data: unknown }[] = []
  s.listeners.add((event) => events.push(event as { type: string; data: unknown }))
  const parent = sendMessage(s, { clientId: 'to-delete', conversationId: 'general', text: 'oops' })
  const reply = sendMessage(s, {
    clientId: 'reply-to-delete',
    conversationId: 'general',
    text: 'quoting',
    replyToId: parent.id,
  })
  const foreign = history(s, 'general').items.find(
    (m) => m.senderId !== 'you' && m.kind === 'message',
  )!
  assert.throws(() => redactMessage(s, 'general', foreign.id), ForbiddenError)
  assert.throws(() => redactMessage(s, 'random', parent.id), /not found/)
  const result = redactMessage(s, 'general', parent.id)
  assert.equal(result.message.deleted, true)
  assert.equal(result.message.text, 'Message deleted')
  assert.equal(result.attachmentRetained, false)
  const items = history(s, 'general').items
  assert.equal(items.find((m) => m.id === parent.id)?.deleted, true)
  assert.equal(items.find((m) => m.id === reply.id)?.replyTo?.text, 'Message deleted')
  assert.ok(
    events.some((e) => e.type === 'message.updated' && (e.data as { id: string }).id === parent.id),
  )
  assert.throws(() => redactMessage(s, 'general', parent.id), /already deleted/)
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
test('marking a message unread counts from it until the conversation is read', async () => {
  const { patchConversation } = await import('../server/utils/demo-store')
  const s = createSession()
  // A known tail: your chosen message, a system row, an event, someone else's reply and yours.
  const base = history(s, 'general').items.at(-1)!
  const at = Date.parse(base.createdAt)
  const row = (id: string, senderId: string, kind: string, i: number) => ({
    ...base,
    id,
    senderId,
    kind,
    text: id,
    createdAt: new Date(at + (i + 1) * 1000).toISOString(),
  })
  s.messages.general!.push(
    row('own', 'you', 'message', 0) as never,
    row('sys', '', 'system', 1) as never,
    row('ev', '', 'event', 2) as never,
    row('peer', base.senderId === 'you' ? 'echo' : base.senderId, 'message', 3) as never,
    row('own-2', 'you', 'action', 4) as never,
  )
  const c = patchConversation(s, 'general', { unreadFrom: 'own' })
  assert.equal(c.unreadFrom, 'own')
  // Only the chosen message and the other person's reply count.
  assert.equal(c.unreadCount, 2)
  assert.throws(() => patchConversation(s, 'general', { unreadFrom: 'sys' }), /no longer/)
  assert.throws(() => patchConversation(s, 'general', { unreadFrom: 'missing' }), /no longer/)
  const read = patchConversation(s, 'general', { read: true })
  assert.equal(read.unreadCount, 0)
  assert.equal(read.unreadFrom, undefined)
})
