import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  defaultNotificationSettings,
  isHighlight,
  maskMatches,
  notificationFor,
  sanitizeNotificationSettings,
} from '../shared/notifications'
import { mediaLinks } from '../shared/utils/links'
import { parseChanmodes, toMask } from '../shared/utils/channel-modes'

const settings = (changes = {}) =>
  sanitizeNotificationSettings({ ...defaultNotificationSettings, ...changes })
const channel = { id: 'general', kind: 'channel' as const }
const dm = { id: 'dm', kind: 'direct' as const }
const message = (text: string, senderId = 'mika') => ({ text, kind: 'message' as const, senderId })

test('highlights match the nick or highlight words as whole words, ignoring formatting', () => {
  const s = settings({ highlightWords: ['ircord', 'go-team'] })
  assert.equal(isHighlight(s, 'hey karbowiak', 'karbowiak'), true)
  assert.equal(isHighlight(s, 'karbowiakish', 'karbowiak'), false)
  assert.equal(isHighlight(s, 'is \x02IRCord\x02 up?', 'karbowiak'), true)
  assert.equal(isHighlight(s, 'go-team!', 'karbowiak'), true)
  assert.equal(isHighlight(s, 'ircords', 'karbowiak'), false)
})

test('ignore masks accept nicks and nick!user@host wildcards', () => {
  assert.equal(maskMatches('troll', 'Troll'), true)
  assert.equal(maskMatches('troll', 'trolley'), false)
  assert.equal(maskMatches('*!*@spam.example', 'anyone', 'x', 'spam.example'), true)
  assert.equal(maskMatches('tr?ll!*@*', 'trill'), true)
  assert.equal(maskMatches('a.b', 'aXb'), false)
})

test('notification decisions follow levels, mentions, DMs and the ignore list', () => {
  const s = settings({ ignore: ['troll'], levels: { muted: 'none', loud: 'all' } })
  assert.equal(notificationFor(s, channel, message('hello'), 'karbowiak', 'mika'), null)
  assert.equal(
    notificationFor(s, channel, message('karbowiak: hi'), 'karbowiak', 'mika'),
    'mention',
  )
  assert.equal(notificationFor(s, dm, message('psst'), 'karbowiak', 'mika'), 'mention')
  assert.equal(
    notificationFor(s, { id: 'loud', kind: 'channel' }, message('hi'), 'karbowiak', 'mika'),
    'message',
  )
  assert.equal(
    notificationFor(
      s,
      { id: 'muted', kind: 'channel' },
      message('karbowiak!'),
      'karbowiak',
      'mika',
    ),
    null,
  )
  assert.equal(notificationFor(s, dm, message('hi'), 'karbowiak', 'troll'), null)
  assert.equal(notificationFor(s, dm, message('hi', 'you'), 'karbowiak', 'karbowiak'), null)
})

test('notification settings are sanitized', () => {
  const s = sanitizeNotificationSettings({
    channelDefault: 'loud',
    levels: { a: 'all', b: 'everything' },
    highlightWords: ['ok', '', 'x\ny', 'ok', 42],
  })
  assert.equal(s.channelDefault, 'mentions')
  assert.deepEqual(s.levels, { a: 'all' })
  assert.deepEqual(s.highlightWords, ['ok'])
})

test('host-mask ignores also suppress in-page notifications when sender identity is known', () => {
  const s = settings({ ignore: ['*!user@ignored.example'] })
  const now = new Date()
  assert.equal(
    notificationFor(s, dm, message('hello'), 'karbowiak', 'mika', now, {
      username: 'user',
      host: 'ignored.example',
    }),
    null,
  )
  assert.equal(
    notificationFor(s, dm, message('hello'), 'karbowiak', 'mika', now, {
      username: 'user',
      host: 'other.example',
    }),
    'mention',
  )
})

test('link previews respect the setting and only use https media links', () => {
  const text =
    'look https://i.imgur.com/a.png and https://example.com/b.jpg, http://i.imgur.com/c.gif https://site.test/page'
  assert.deepEqual(
    mediaLinks(text, 'trusted', 'https://ircord.test').map((l) => l.url),
    ['https://i.imgur.com/a.png'],
  )
  assert.deepEqual(
    mediaLinks(text, 'all', 'https://ircord.test').map((l) => l.url),
    ['https://i.imgur.com/a.png', 'https://example.com/b.jpg'],
  )
  assert.deepEqual(mediaLinks(text, 'off', 'https://ircord.test'), [])
  assert.equal(
    mediaLinks('https://ircord.test/api/v1/uploads/x.mp4', 'trusted', 'https://ircord.test')[0]
      ?.kind,
    'video',
  )
})

test('CHANMODES groups and masks', () => {
  assert.deepEqual(parseChanmodes({ CHANMODES: 'eIbq,k,flj,CFLMPQScgimnprstuz' }), {
    list: 'eIbq',
    always: 'k',
    whenSet: 'flj',
    flags: 'CFLMPQScgimnprstuz',
  })
  assert.deepEqual(parseChanmodes(undefined), {
    list: 'beI',
    always: 'k',
    whenSet: 'l',
    flags: 'imnpst',
  })
  assert.equal(toMask('troll'), 'troll!*@*')
  assert.equal(toMask('*!*@host'), '*!*@host')
  assert.equal(toMask('$a:account'), '$a:account')
})
