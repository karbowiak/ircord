import assert from 'node:assert/strict'
import test from 'node:test'
import {
  channelChoiceLabel,
  countPosted,
  linkDateRange,
  linkHostFilter,
  linkToday,
  linkEventEffect,
  linkQueryParams,
  linkThumbnail,
  linkTitle,
  openableLink,
} from '../shared/utils/link-catalog'

const origin = 'https://chat.example'

test('a card is titled by its stored preview, else by its own address', () => {
  assert.equal(
    linkTitle({
      url: 'https://a.test/x',
      preview: { url: 'https://a.test/x', title: 'An article' },
    }),
    'An article',
  )
  assert.equal(linkTitle({ url: 'https://www.a.test/caf%C3%A9?id=2' }), 'a.test/café?id=2')
  assert.equal(linkTitle({ url: 'https://a.test/' }), 'a.test')
  assert.equal(linkTitle({ url: 'https://a.test/%E0%A4%A' }), 'a.test/%E0%A4%A')
})

test('thumbnails follow the device preview setting and trusted hosts like the timeline', () => {
  const image = { url: 'https://i.imgur.com/cat.png', kind: 'image' as const }
  const untrusted = { url: 'https://pics.example/cat.png', kind: 'image' as const }
  assert.equal(linkThumbnail(image, 'off', origin), undefined)
  assert.equal(linkThumbnail(image, 'trusted', origin), 'https://i.imgur.com/cat.png')
  assert.equal(linkThumbnail(untrusted, 'trusted', origin), undefined)
  assert.equal(
    linkThumbnail(untrusted, 'trusted', origin, ['pics.example']),
    'https://pics.example/cat.png',
  )
  assert.equal(linkThumbnail(untrusted, 'all', origin), 'https://pics.example/cat.png')
  assert.equal(
    linkThumbnail({ url: 'https://youtu.be/dQw4w9WgXcQ', kind: 'youtube' }, 'trusted', origin),
    'https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg',
  )
  const page = { url: 'https://a.test/x', kind: 'link' as const }
  assert.equal(
    linkThumbnail(
      { ...page, preview: { url: page.url, title: 't', image: 'https://cdn.a.test/i.jpg' } },
      'trusted',
      origin,
    ),
    'https://cdn.a.test/i.jpg',
  )
  assert.equal(
    linkThumbnail(
      { ...page, preview: { url: page.url, title: 't', image: 'http://cdn.a.test/i.jpg' } },
      'all',
      origin,
    ),
    undefined,
  )
  assert.equal(
    linkThumbnail(
      { ...page, preview: { url: page.url, title: 't', image: 'https://cdn.a.test/i.jpg' } },
      'off',
      origin,
    ),
    undefined,
  )
  assert.equal(linkThumbnail(page, 'all', origin), undefined)
})

test('only web links open from the catalog', () => {
  assert.ok(openableLink('https://a.test/x?sig=1'))
  assert.ok(openableLink('http://a.test/'))
  assert.ok(!openableLink('javascript:alert(1)'))
  assert.ok(!openableLink('data:text/html,hi'))
})

test('filters become repeated query parameters and empty ones are left out', () => {
  assert.deepEqual(linkQueryParams({ conversations: [], network: '', kind: '', q: '' }), {
    conversation: undefined,
    network: undefined,
    kind: undefined,
    q: undefined,
    host: undefined,
    since: undefined,
    until: undefined,
    cursor: undefined,
    limit: 30,
  })
  assert.deepEqual(
    linkQueryParams({ conversations: ['n_dev'], network: 'n', kind: 'image', q: 'cat' }, 'c1'),
    {
      conversation: ['n_dev'],
      network: 'n',
      kind: ['image'],
      q: 'cat',
      host: undefined,
      since: undefined,
      until: undefined,
      cursor: 'c1',
      limit: 30,
    },
  )
  // Several channels, a host and a date range all reach the request.
  assert.deepEqual(
    linkQueryParams({
      conversations: ['n_dev', 'n_music'],
      network: '',
      kind: '',
      q: '',
      host: 'example.com',
      since: '2026-11-01T00:00:00.000Z',
      until: '2026-11-08T00:00:00.000Z',
    }),
    {
      conversation: ['n_dev', 'n_music'],
      network: undefined,
      kind: undefined,
      q: undefined,
      host: 'example.com',
      since: '2026-11-01T00:00:00.000Z',
      until: '2026-11-08T00:00:00.000Z',
      cursor: undefined,
      limit: 30,
    },
  )
})

test('the host filter takes a host, an address or a pasted web link, and says what it refuses', () => {
  assert.deepEqual(linkHostFilter(''), { host: '' })
  assert.deepEqual(linkHostFilter('  News.Example  '), { host: 'news.example' })
  assert.deepEqual(linkHostFilter('example.com.'), { host: 'example.com' })
  assert.deepEqual(linkHostFilter('https://www.example.com/a/b?c=1'), { host: 'www.example.com' })
  assert.deepEqual(linkHostFilter('127.0.0.1'), { host: '127.0.0.1' })
  assert.deepEqual(linkHostFilter('[::1]'), { host: '::1' })
  assert.deepEqual(linkHostFilter('http://[::1]:8443/x'), { host: '::1' })
  assert.deepEqual(linkHostFilter('bücher.example'), { host: 'bücher.example' })
  // The same shapes the server refuses: paths, credentials, spaces, ports, empty labels.
  for (const bad of ['a/b', 'user@a.test', 'a b', 'a.test:80', '?#', '.a.test', 'a..test'])
    assert.ok('error' in linkHostFilter(bad), bad)
})

test('the date filter covers whole local days, either end open', () => {
  const day = (y: number, m: number, d: number) => new Date(y, m - 1, d).toISOString()
  assert.deepEqual(linkDateRange('', ''), { since: undefined, until: undefined })
  assert.deepEqual(linkDateRange('2026-11-01', ''), { since: day(2026, 11, 1), until: undefined })
  // The last day is included: the range ends at the next midnight.
  assert.deepEqual(linkDateRange('', '2026-11-08'), { since: undefined, until: day(2026, 11, 9) })
  assert.deepEqual(linkDateRange('2026-11-08', '2026-11-08'), {
    since: day(2026, 11, 8),
    until: day(2026, 11, 9),
  })
  assert.deepEqual(linkDateRange('2026-12-31', '2026-12-31').until, day(2027, 1, 1))
  assert.ok('error' in linkDateRange('2026-11-08', '2026-11-01'))
  assert.ok('error' in linkDateRange('yesterday', ''))
  assert.equal(linkToday(new Date(2026, 0, 5, 23, 59)), '2026-01-05')
})

test('the channel picker names what it lists', () => {
  assert.equal(channelChoiceLabel([]), 'Any channel')
  assert.equal(channelChoiceLabel(['dev']), '#dev')
  assert.equal(channelChoiceLabel(['dev', 'music', 'general']), '3 channels')
})

test('live events: new channel links are offered; listed posts and new links reread the list', () => {
  const base = { updated: false, channel: true, ignored: false, listed: false }
  assert.equal(linkEventEffect({ ...base, text: 'see https://a.test/' }), 'posted')
  assert.equal(linkEventEffect({ ...base, text: 'no link here' }), null)
  // Direct messages, ignored senders and links in code are never listed.
  assert.equal(linkEventEffect({ ...base, channel: false, text: 'https://a.test/' }), null)
  assert.equal(linkEventEffect({ ...base, ignored: true, text: 'https://a.test/' }), null)
  assert.equal(linkEventEffect({ ...base, text: '\x11https://a.test/\x11' }), null)
  assert.equal(linkEventEffect({ ...base, text: 'HTTPS://a.test/' }), null)

  const update = { ...base, updated: true }
  // A reaction or a resolved preview leaves the text alone: nothing moves.
  assert.equal(
    linkEventEffect({
      ...update,
      listed: true,
      text: 'https://a.test/',
      previousText: 'https://a.test/',
    }),
    null,
  )
  assert.equal(
    linkEventEffect({ ...update, text: 'https://a.test/', previousText: 'https://a.test/' }),
    null,
  )
  // A listed post deleted, or edited (even to drop its link), rereads the list.
  assert.equal(
    linkEventEffect({
      ...update,
      listed: true,
      deleted: true,
      text: 'Message deleted',
      previousText: 'https://a.test/',
    }),
    'changed',
  )
  assert.equal(
    linkEventEffect({ ...update, listed: true, text: 'gone', previousText: 'https://a.test/' }),
    'changed',
  )
  // An edit that adds a link rereads it; one without links, unlisted, does not.
  assert.equal(
    linkEventEffect({ ...update, text: 'now https://b.test/', previousText: 'now' }),
    'changed',
  )
  assert.equal(linkEventEffect({ ...update, text: 'typo fixed', previousText: 'typo fixd' }), null)
  // Without the previous copy, a listed post or a link-bearing one is assumed changed.
  assert.equal(linkEventEffect({ ...update, listed: true, text: 'x' }), 'changed')
  assert.equal(linkEventEffect({ ...update, channel: false, listed: true, text: 'x' }), null)
})

test('new link posts are counted only for the channels and network a view shows', () => {
  const posted = new Map([
    ['n1_dev', 2],
    ['n1_ops', 1],
    ['n2_dev', 4],
  ])
  const networkOf = (id: string) => id.split('_')[0]
  assert.equal(countPosted(posted, [], '', networkOf), 7)
  assert.equal(countPosted(posted, ['n1_dev'], '', networkOf), 2)
  assert.equal(countPosted(posted, [], 'n1', networkOf), 3)
  assert.equal(countPosted(posted, ['n2_dev'], 'n1', networkOf), 0)
})
