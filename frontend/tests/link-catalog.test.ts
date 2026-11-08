import assert from 'node:assert/strict'
import test from 'node:test'
import {
  countPosted,
  linkEventEffect,
  linkQueryParams,
  linkTitle,
  openableLink,
} from '../shared/utils/link-catalog'

test('a card is titled by its own address', () => {
  assert.equal(linkTitle({ url: 'https://www.a.test/caf%C3%A9?id=2' }), 'a.test/café?id=2')
  assert.equal(linkTitle({ url: 'https://a.test/' }), 'a.test')
  assert.equal(linkTitle({ url: 'https://a.test/%E0%A4%A' }), 'a.test/%E0%A4%A')
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
      cursor: 'c1',
      limit: 30,
    },
  )
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
