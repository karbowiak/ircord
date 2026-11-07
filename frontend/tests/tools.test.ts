import { test } from 'node:test'
import assert from 'node:assert/strict'
import { networkNameFromHost, parseIrcUrl } from '../shared/utils/irc-url'
import { nextUnread } from '../shared/utils/navigation'
import { formatLog, logTime } from '../shared/utils/log'
import {
  defaultNotificationSettings,
  isQuiet,
  notificationFor,
  sanitizeNotificationSettings,
} from '../shared/notifications'
import type { Message } from '../shared/types/chat'

test('irc:// links parse host, port, TLS, channel, key and nick targets', () => {
  assert.deepEqual(parseIrcUrl('ircs://irc.libera.chat/rust'), {
    host: 'irc.libera.chat',
    port: 6697,
    tls: true,
    channel: '#rust',
  })
  assert.deepEqual(parseIrcUrl('irc://Irc.Example.org:7000/%23secret?hunter2'), {
    host: 'irc.example.org',
    port: 7000,
    tls: false,
    channel: '#secret',
    key: 'hunter2',
  })
  assert.equal(parseIrcUrl('irc://irc.example.org/&local')?.channel, '&local')
  assert.deepEqual(parseIrcUrl('ircs://irc.libera.chat/jules,isnick'), {
    host: 'irc.libera.chat',
    port: 6697,
    tls: true,
    nick: 'jules',
  })
  assert.deepEqual(parseIrcUrl('irc://irc.libera.chat'), {
    host: 'irc.libera.chat',
    port: 6667,
    tls: false,
  })
})

test('irc:// parsing rejects other schemes, bad hosts and ports', () => {
  assert.equal(parseIrcUrl('https://irc.libera.chat/rust'), null)
  assert.equal(parseIrcUrl('irc://bad_host!/rust'), null)
  assert.equal(parseIrcUrl('irc://irc.libera.chat:70000/rust'), null)
  assert.equal(parseIrcUrl('irc://irc.libera.chat:0'), null)
  assert.equal(networkNameFromHost('irc.libera.chat'), 'libera.chat')
  assert.equal(networkNameFromHost('chat.freenode.net'), 'freenode.net')
})

test('next unread prefers mentions, wraps around and starts from either end', () => {
  const list = [
    { id: 'a', mentionCount: 0, unreadCount: 2 },
    { id: 'b', mentionCount: 0, unreadCount: 0 },
    { id: 'c', mentionCount: 0, unreadCount: 1 },
  ]
  assert.equal(nextUnread(list, 'a', 1)?.id, 'c')
  assert.equal(nextUnread(list, 'c', 1)?.id, 'a')
  assert.equal(nextUnread(list, 'b', 1)?.id, 'a')
  assert.equal(nextUnread(list, 'b', -1)?.id, 'c')
  const mentioned = [...list, { id: 'd', mentionCount: 1, unreadCount: 1 }]
  assert.equal(nextUnread(mentioned, 'a', 1)?.id, 'd')
  assert.equal(
    nextUnread(
      list.map((c) => ({ ...c, unreadCount: 0 })),
      'a',
      1,
    ),
    null,
  )
})

test('log export writes irssi-style lines without formatting codes', () => {
  const at = new Date(2026, 10, 7, 9, 5).toISOString()
  const base = { conversationId: 'c', createdAt: at, reactions: [] }
  const messages = [
    { ...base, id: '1', senderId: 'jules', kind: 'message', text: '\x02hello\x02\nthere' },
    { ...base, id: '2', senderId: 'jules', kind: 'action', text: 'waves' },
    { ...base, id: '3', senderId: 'server', kind: 'notice', text: 'maintenance' },
    { ...base, id: '4', senderId: 'jules', kind: 'event', text: 'jules joined' },
    {
      ...base,
      id: '5',
      senderId: 'you',
      kind: 'message',
      text: 'look',
      attachment: { url: 'https://files.example/a.png' },
    },
  ] as unknown as Message[]
  assert.equal(logTime(at), '2026-11-07 09:05')
  assert.equal(
    formatLog(messages, (id) => (id === 'you' ? 'me' : id), '# #general'),
    [
      '# #general',
      '',
      '[2026-11-07 09:05] <jules> hello ↵ there',
      '[2026-11-07 09:05] * jules waves',
      '[2026-11-07 09:05] -server- maintenance',
      '[2026-11-07 09:05] *** jules joined',
      '[2026-11-07 09:05] <me> look https://files.example/a.png',
      '',
    ].join('\n'),
  )
})

test('quiet hours handle ranges across midnight in the configured time zone', () => {
  const quiet = (start: string, end: string, timeZone = 'UTC') =>
    sanitizeNotificationSettings({
      ...defaultNotificationSettings,
      quietHours: { enabled: true, start, end, timeZone },
    })
  const at = (hhmm: string) => new Date(`2026-11-07T${hhmm}:00Z`)
  assert.equal(isQuiet(quiet('22:00', '07:00'), at('23:30')), true)
  assert.equal(isQuiet(quiet('22:00', '07:00'), at('06:59')), true)
  assert.equal(isQuiet(quiet('22:00', '07:00'), at('07:00')), false)
  assert.equal(isQuiet(quiet('09:00', '17:00'), at('12:00')), true)
  assert.equal(isQuiet(quiet('09:00', '17:00'), at('17:00')), false)
  // 08:00 UTC is 09:00 in Copenhagen in November.
  assert.equal(isQuiet(quiet('09:00', '10:00', 'Europe/Copenhagen'), at('08:00')), true)
  assert.equal(isQuiet(quiet('09:00', '09:00'), at('09:00')), false)
  assert.equal(isQuiet(defaultNotificationSettings, at('23:30')), false)
})

test('quiet hours silence mentions and DMs', () => {
  const s = sanitizeNotificationSettings({
    ...defaultNotificationSettings,
    quietHours: { enabled: true, start: '22:00', end: '07:00', timeZone: 'UTC' },
  })
  const dm = { id: 'dm', kind: 'direct' as const }
  const msg = { text: 'hi', kind: 'message' as const, senderId: 'mika' }
  const night = new Date('2026-11-07T23:00:00Z')
  const day = new Date('2026-11-07T12:00:00Z')
  assert.equal(notificationFor(s, dm, msg, 'you', 'mika', night), null)
  assert.notEqual(notificationFor(s, dm, msg, 'you', 'mika', day), null)
})
