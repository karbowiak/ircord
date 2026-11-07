import { test } from 'node:test'
import assert from 'node:assert/strict'
import { normalizeTransport, realtimeUrl } from '../app/utils/transport'
import { channelTarget } from '../shared/utils/irc'

test('channel commands retain the actual prefix rather than reconstructing it from display names', () => {
  assert.equal(channelTarget({ name: 'local', target: '&local' }), '&local')
  assert.equal(channelTarget({ name: '&local', target: '#&local' }), '#&local')
  assert.equal(channelTarget({ name: 'general' }), '#general')
})

test('normalizes only identity fields while preserving account authorization and message text', () => {
  const snapshot = normalizeTransport({
    user: { id: 'account-123', nick: 'me', displayName: 'Me' },
    members: [
      { id: 'account-123', nick: 'me', displayName: 'Me' },
      { id: 'peer', nick: 'other' },
    ],
    conversations: [
      { id: 'room', memberIds: ['account-123', 'peer'], memberModes: { 'account-123': '@' } },
    ],
  })
  assert.equal(snapshot.user.id, 'you')
  assert.deepEqual(snapshot.conversations[0]!.memberModes, { you: '@' })
  const message = normalizeTransport({
    senderId: 'account-123',
    text: 'account-123',
    reactions: [{ memberIds: ['account-123'] }],
    event: { actorId: 'account-123', targetId: 'peer' },
  })
  assert.equal(message.senderId, 'you')
  assert.equal(message.text, 'account-123')
  assert.deepEqual(message.reactions[0]!.memberIds, ['you'])
  assert.equal(normalizeTransport({ account: { id: 'account-123' } }).account.id, 'account-123')
  normalizeTransport({ user: { id: 'second-account', nick: 'next' } })
  assert.equal(normalizeTransport({ senderId: 'account-123' }).senderId, 'account-123')
})

test('derives sockets from the API origin and honors explicit reverse-proxy URLs', () => {
  const location = { href: 'https://chat.example/workspace' }
  assert.equal(realtimeUrl('/api/v1', '', location), 'wss://chat.example/api/v1/events')
  assert.equal(
    realtimeUrl('http://127.0.0.1:8080/api/v1', '', location),
    'ws://127.0.0.1:8080/api/v1/events',
  )
  assert.equal(
    realtimeUrl('/api/v1', 'wss://events.example/socket', location),
    'wss://events.example/socket',
  )
})
