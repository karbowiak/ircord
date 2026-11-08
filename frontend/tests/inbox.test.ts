import { test } from 'node:test'
import assert from 'node:assert/strict'
import { appendMessage, createSession, patchConversation, removeNetwork } from '../server/utils/demo-store'
import { inboxUnread, listInbox, recordInbox, setInboxDismissed } from '../server/utils/demo-inbox'
import type { Message } from '../shared/types/chat'

const incoming = (conversationId: string, text: string, extra: Partial<Message> = {}): Message => ({
  id: `in-${Math.random().toString(36).slice(2)}`,
  conversationId,
  senderId: 'mika',
  text,
  createdAt: new Date().toISOString(),
  kind: 'message',
  reactions: [],
  ...extra,
})

test('the fixtures seed mentions and direct messages, read state included', () => {
  const s = createSession()
  const all = listInbox(s)
  assert.ok(all.items.some((i) => i.conversationId === 'development' && i.reasons.includes('mention')))
  assert.ok(all.items.some((i) => i.conversationId === 'dm-jules' && i.reasons.includes('direct') && i.unread))
  // dm-echo has no unread messages, so its item is already read.
  assert.ok(all.items.some((i) => i.conversationId === 'dm-echo' && !i.unread))
  assert.equal(all.unread, inboxUnread(s))
})

test('one item per message, with every reason, recorded once', () => {
  const s = createSession()
  const events: string[] = []
  s.listeners.add((e) => events.push(e.type))
  const before = listInbox(s).items.length
  const dm = incoming('dm-mika', 'hey karbowiak')
  appendMessage(s, dm)
  // The same message again (a replay) records nothing more.
  recordInbox(s, dm)
  appendMessage(s, incoming('general', 'nothing to see'))
  const item = listInbox(s).items.find((i) => i.messageId === dm.id)!
  assert.deepEqual(item.reasons, ['mention', 'direct'])
  assert.equal(listInbox(s).items.length, before + 1)
  assert.equal(events.filter((t) => t === 'inbox.added').length, 1)
})

test('a reply to your message is a reply; muted channels record nothing', () => {
  const s = createSession()
  const mine = s.messages.general!.find((m) => m.senderId === 'you')!
  const reply = incoming('general', 'good question', { replyTo: { id: mine.id, nick: 'karbowiak', text: mine.text } })
  appendMessage(s, reply)
  assert.deepEqual(listInbox(s, 'replies').items.map((i) => i.messageId), [reply.id])
  s.bootstrap.conversations.find((c) => c.id === 'general')!.channelSettings!.notifications = 'none'
  const muted = incoming('general', 'karbowiak?')
  appendMessage(s, muted)
  assert.ok(!listInbox(s).items.some((i) => i.messageId === muted.id))
})

test('dismissing is separate from reading, and reading does not dismiss', () => {
  const s = createSession()
  const m = incoming('development', 'karbowiak, ping')
  appendMessage(s, m)
  const unread = inboxUnread(s)
  setInboxDismissed(s, m.id, true)
  assert.equal(inboxUnread(s), unread - 1)
  // The conversation is untouched by the dismissal.
  assert.ok(s.bootstrap.conversations.find((c) => c.id === 'development')!.unreadCount > 0)
  assert.deepEqual(listInbox(s, 'dismissed').items.map((i) => i.messageId), [m.id])
  setInboxDismissed(s, m.id, false)
  patchConversation(s, 'development', { read: true })
  const item = listInbox(s).items.find((i) => i.messageId === m.id)!
  assert.equal(item.unread, false)
  assert.equal(item.dismissed, false)
  assert.throws(() => setInboxDismissed(s, 'nope', true), /not found/)
})

test('excerpts are bounded, pages continue, removed networks take their items', () => {
  const s = createSession()
  const long = incoming('dm-mika', 'x'.repeat(1000))
  appendMessage(s, long)
  const item = listInbox(s).items.find((i) => i.messageId === long.id)!
  assert.ok(item.truncated && [...item.excerpt!].length <= 281)
  const first = listInbox(s, 'all', '', 2)
  assert.equal(first.items.length, 2)
  const next = listInbox(s, 'all', first.nextCursor!, 100)
  assert.ok(!next.items.some((i) => first.items.some((f) => f.messageId === i.messageId)))
  removeNetwork(s, 'libera')
  assert.equal(listInbox(s).items.length, 0)
})
