import { test } from 'node:test'
import assert from 'node:assert/strict'
import { defaultNotificationSettings, type NotificationSettings } from '../shared/notifications'
import {
  enqueueBounded,
  maxReadAloudQueue,
  maxSpokenLength,
  shouldReadAloud,
  spokenText,
} from '../shared/read-aloud'

const settings = (changes: Partial<NotificationSettings> = {}): NotificationSettings => ({
  ...defaultNotificationSettings,
  ...changes,
})
const channel = { id: 'c1', kind: 'channel' as const }
const direct = { id: 'd1', kind: 'direct' as const }
const message = (text: string, senderId = 'bob') => ({ text, kind: 'message' as const, senderId })
const noon = new Date('2026-01-01T12:00:00Z')

test('highlights are read: your nick or a highlight word, whole words only', () => {
  assert.equal(shouldReadAloud(settings(), channel, message('alice: ping'), 'alice', 'bob', noon), true)
  assert.equal(shouldReadAloud(settings(), channel, message('hey @Alice'), 'alice', 'bob', noon), true)
  assert.equal(shouldReadAloud(settings(), channel, message('malice aforethought'), 'alice', 'bob', noon), false)
  assert.equal(shouldReadAloud(settings(), channel, message('just chatting'), 'alice', 'bob', noon), false)
  const words = settings({ highlightWords: ['deploy'] })
  assert.equal(shouldReadAloud(words, channel, message('Deploy is done'), 'alice', 'bob', noon), true)
})

test('your own messages, events, ignored senders, muted conversations and quiet hours are not read', () => {
  assert.equal(shouldReadAloud(settings(), channel, message('alice: me', 'you'), 'alice', 'alice', noon), false)
  assert.equal(
    shouldReadAloud(settings(), channel, { text: 'alice joined', kind: 'event', senderId: 'bob' }, 'alice', 'bob', noon),
    false,
  )
  assert.equal(shouldReadAloud(settings({ ignore: ['bob'] }), channel, message('alice: hi'), 'alice', 'bob', noon), false)
  assert.equal(
    shouldReadAloud(settings({ ignore: ['*!*@spam.example'] }), channel, message('alice: hi'), 'alice', 'bob', noon, { username: 'b', host: 'spam.example' }),
    false,
  )
  assert.equal(shouldReadAloud(settings({ channelDefault: 'none' }), channel, message('alice: hi'), 'alice', 'bob', noon), false)
  const quiet = settings({ quietHours: { enabled: true, start: '11:00', end: '13:00', timeZone: 'UTC' } })
  assert.equal(shouldReadAloud(quiet, channel, message('alice: hi'), 'alice', 'bob', noon), false)
})

test('a direct message is read only when it highlights you', () => {
  assert.equal(shouldReadAloud(settings(), direct, message('hello there'), 'alice', 'bob', noon), false)
  assert.equal(shouldReadAloud(settings(), direct, message('alice, you around?'), 'alice', 'bob', noon), true)
})

test('spoken text names the sender and channel, drops formatting and shortens links', () => {
  assert.equal(
    spokenText({ sender: 'bob', channel: '#dev', text: '\x02alice\x02: see https://example.com/a/very/long?path=1 now' }),
    'bob in #dev: alice: see a link to example.com now',
  )
  assert.equal(spokenText({ sender: undefined, text: 'alice?' }), 'Someone: alice?')
  assert.equal(spokenText({ sender: 'bob', text: '', gif: true }), 'bob: sent a GIF')
})

test('spoken text is capped', () => {
  const long = spokenText({ sender: 'bob', channel: '#dev', text: `alice ${'word '.repeat(200)}` })
  assert.ok(long.length <= maxSpokenLength && long.length > maxSpokenLength - 5)
  assert.ok(long.endsWith('…'))
})

test('a burst keeps only the newest few, in order', () => {
  const queue: number[] = []
  for (let i = 1; i <= maxReadAloudQueue + 3; i++) enqueueBounded(queue, i)
  assert.deepEqual(queue, [4, 5, 6, 7, 8])
})
