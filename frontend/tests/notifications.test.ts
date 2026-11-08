import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  defaultNotificationSettings,
  isHighlight,
  maskMatches,
  levelFor,
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
  const shared = (id: string, notifications: 'default' | 'all' | 'mentions' | 'none') => ({
    id,
    kind: 'channel' as const,
    channelSettings: { notifications, previewMode: 'client' as const },
  })
  assert.equal(notificationFor(s, shared('x', 'all'), message('hi'), 'karbowiak', 'mika'), 'message')
  assert.equal(
    notificationFor(s, shared('x', 'none'), message('karbowiak!'), 'karbowiak', 'mika'),
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
  assert.deepEqual(
    mediaLinks(
      'https://i.imgur.com/a.gif https://i.imgur.com/b.GIF?x=1 https://i.imgur.com/c.webp https://media.giphy.com/media/x/giphy.webp https://cdn.7tv.app/emote/x/4x.avif https://i.imgur.com/d.apng',
      'trusted',
      'https://ircord.test',
    ).map((l) => l.kind),
    ['gif', 'gif', 'image'],
  )
  assert.deepEqual(
    mediaLinks(
      'https://media.giphy.com/media/x/giphy.webp https://cdn.7tv.app/emote/x/4x.avif https://i.imgur.com/d.apng',
      'trusted',
      'https://ircord.test',
    ).map((l) => l.kind),
    ['gif', 'gif', 'gif'],
  )
})

test('a channel\'s trusted media hosts embed under trusted like the built-in hosts', () => {
  const text =
    'https://images.example.com/a.png https://images.example.com/b.mp4 https://other.example.com/c.png https://sub.images.example.com/d.png'
  const hosts = ['images.example.com']
  assert.deepEqual(
    mediaLinks(text, 'trusted', 'https://ircord.test', 10, hosts).map((l) => [l.url, l.kind]),
    [
      ['https://images.example.com/a.png', 'image'],
      ['https://images.example.com/b.mp4', 'video'],
    ],
  )
  // In another channel (no list) the same host is just a link, as before.
  assert.deepEqual(mediaLinks(text, 'trusted', 'https://ircord.test', 10), [])
  // Off and All do not change.
  assert.deepEqual(mediaLinks(text, 'off', 'https://ircord.test', 10, hosts), [])
  assert.equal(mediaLinks(text, 'all', 'https://ircord.test', 10, hosts).length, 4)
  assert.equal(mediaLinks(text, 'all', 'https://ircord.test', 10).length, 4)
})

test('GIF links are told apart from still images, including provider media hosts', () => {
  const links = mediaLinks(
    'https://media1.tenor.com/m/abc/cat.gif https://c.tenor.com/abc/cat.gif https://media4.giphy.com/media/x/giphy.GIF https://i.imgur.com/a.png https://tenor.com/view/cat-gif-1 https://eviltenor.com/a.gif',
    'trusted',
    'https://ircord.test',
  )
  assert.deepEqual(
    links.map((l) => [l.host, l.kind]),
    [
      ['media1.tenor.com', 'gif'],
      ['c.tenor.com', 'gif'],
      ['media4.giphy.com', 'gif'],
    ],
  )
  assert.deepEqual(
    mediaLinks(
      'https://i.imgur.com/a.png https://eviltenor.com/a.gif',
      'trusted',
      'https://x.test',
    ).map((l) => [l.host, l.kind]),
    [['i.imgur.com', 'image']],
  )
  // Provider subdomains serve animated WebP as a GIF format, like the listed GIF hosts.
  assert.equal(
    mediaLinks('https://media2.giphy.com/media/x/giphy.webp', 'trusted', 'https://x.test')[0]?.kind,
    'gif',
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

test('channel levels are channel-wide; Default inherits your own channel default', () => {
  // Personal per-channel values saved before levels became channel settings are ignored.
  const s = settings({ channelDefault: 'all', levels: { general: 'none', dm: 'none' } })
  const at = (notifications?: 'default' | 'all' | 'mentions' | 'none') => ({
    id: 'general',
    kind: 'channel' as const,
    ...(notifications ? { channelSettings: { notifications, previewMode: 'client' as const } } : {}),
  })
  assert.equal(levelFor(s, at()), 'all')
  assert.equal(levelFor(s, at('default')), 'all')
  assert.equal(levelFor(s, at('mentions')), 'mentions')
  assert.equal(levelFor(s, at('none')), 'none')
  assert.equal(notificationFor(s, at('mentions'), message('hello'), 'karbowiak', 'mika'), null)
  assert.equal(notificationFor(s, at('default'), message('hello'), 'karbowiak', 'mika'), 'message')
  // Direct messages keep the personal level.
  assert.equal(levelFor(s, dm), 'none')
})
