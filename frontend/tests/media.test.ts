import { test } from 'node:test'
import assert from 'node:assert/strict'
import { normalizeKlipyPage, isKlipyMediaUrl, searchEmoji, emojiText } from '../shared/utils/media'
import type { EmojiItem } from '../shared/types/media'

const row = (slug: string) => ({
  id: slug,
  slug,
  title: slug,
  file: {
    hd: {
      gif: {
        url: `https://static.klipy.com/${slug}.gif?delivery=keep-this`,
        width: 400,
        height: 300,
      },
    },
    sm: {
      webp: { url: `https://static.klipy.com/${slug}.webp`, width: 150, height: 110 },
      jpg: { url: `https://static.klipy.com/${slug}.jpg`, width: 150, height: 110 },
    },
  },
})
test('Klipy results retain provider order, delivery parameters and pagination', () => {
  const data = normalizeKlipyPage({
    result: true,
    data: { data: [row('second'), row('first')], current_page: 2, has_next: true },
  })
  assert.deepEqual(
    data.items.map((i) => i.slug),
    ['second', 'first'],
  )
  assert.equal(data.items[0]!.url, 'https://static.klipy.com/second.gif?delivery=keep-this')
  assert.equal(data.nextPage, 3)
  assert.equal(normalizeKlipyPage({ result: true, data: [row('saved')] }).items[0]!.slug, 'saved')
})
test('malformed provider data and unsafe media URLs are rejected', () => {
  assert.equal(isKlipyMediaUrl('https://static.klipy.com.evil.test/image.gif'), false)
  assert.equal(isKlipyMediaUrl('javascript:alert(1)'), false)
  assert.throws(() =>
    normalizeKlipyPage({ result: true, data: { data: [{ ...row('bad'), file: {} }] } }),
  )
  assert.throws(() => normalizeKlipyPage({ result: false }))
})
test('emoji search includes aliases, source groups, skin tones and textual custom fallback', () => {
  const unicode: EmojiItem = {
    id: 'wave',
    label: 'waving hand',
    shortcode: 'wave',
    aliases: ['hello'],
    group: 'people-body',
    unicode: '👋',
    variants: { '3': '👋🏽' },
    source: 'unicode',
  }
  const custom: EmojiItem = {
    id: 'custom',
    label: 'Party',
    shortcode: 'party',
    aliases: ['celebrate'],
    group: '7tv',
    source: '7tv',
    url: 'https://cdn.7tv.app/example.webp',
  }
  assert.deepEqual(searchEmoji([unicode, custom], ':hello:'), [unicode])
  assert.deepEqual(searchEmoji([unicode, custom], '', '7tv'), [custom])
  assert.equal(emojiText(unicode, '3'), '👋🏽')
  assert.equal(emojiText(custom), ':party:')
})

test('emoji ranking prefers exact, then prefix, then the user’s most used', async () => {
  const { rankEmoji } = await import('../shared/utils/media')
  const item = (id: string, shortcode: string, aliases: string[] = []) =>
    ({
      id,
      shortcode,
      label: shortcode,
      aliases,
      group: 'x',
      source: 'unicode',
      unicode: id,
    }) as EmojiItem
  const items = [
    item('key', 'key', ['jo']),
    item('banjo', 'banjo'),
    item('joy_cat', 'joy_cat'),
    item('joy', 'joy'),
  ]
  assert.deepEqual(
    rankEmoji(items, new Map(), 'jo').map((i) => i.id),
    ['joy', 'joy_cat', 'banjo', 'key'],
  )
  assert.deepEqual(
    rankEmoji(items, new Map([['joy_cat', 0]]), 'jo')
      .map((i) => i.id)
      .slice(0, 2),
    ['joy_cat', 'joy'],
  )
})
