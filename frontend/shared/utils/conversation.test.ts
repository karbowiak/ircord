import assert from 'node:assert/strict'
import test from 'node:test'
import type { Conversation } from '../types/chat'
import { sameConversationSettings } from './conversation'

const channel: Conversation = {
  id: 'c1',
  networkId: 'net',
  kind: 'channel',
  name: '#ircord',
  topic: 'Welcome',
  unreadCount: 0,
  mentionCount: 0,
  favorite: false,
  joined: true,
  memberIds: ['you', 'net:1'],
  memberModes: { you: '@' },
  modes: 'nt',
}

test('unread state and member lists do not count as a settings change', () => {
  assert.ok(
    sameConversationSettings(channel, {
      ...channel,
      unreadCount: 4,
      mentionCount: 1,
      unreadFrom: 'm1',
      memberIds: ['you', 'net:1', 'net:2'],
    }),
  )
})

test('a fresh copy of the same member modes is the same', () => {
  assert.ok(sameConversationSettings(channel, { ...channel, memberModes: { you: '@' } }))
})

test('changes rows depend on are settings changes', () => {
  assert.ok(!sameConversationSettings(channel, { ...channel, joined: false }))
  assert.ok(!sameConversationSettings(channel, { ...channel, modes: 'ntm' }))
  assert.ok(!sameConversationSettings(channel, { ...channel, memberModes: { you: '+' } }))
  assert.ok(
    !sameConversationSettings(channel, { ...channel, memberModes: { you: '@', 'net:1': '@' } }),
  )
  assert.ok(
    !sameConversationSettings(channel, {
      ...channel,
      channelSettings: { groupMentions: 'off', notifications: 'default', nsfw: false },
    }),
  )
})
