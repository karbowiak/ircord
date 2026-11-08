import { test } from 'node:test'
import assert from 'node:assert/strict'
import { emptyLibrary, recordUsage, sanitizeLibrary, savedGif, usageScore } from '../shared/library'
import type { GifItem } from '../shared/types/media'

const gif = (id: string): GifItem => ({
  id,
  slug: `slug-${id}`,
  title: `GIF ${id}`,
  url: `https://static.klipy.com/${id}.gif`,
  previewUrl: `https://static.klipy.com/${id}.webp`,
  stillUrl: `https://static.klipy.com/${id}.jpg`,
  width: 320,
  height: 240,
  provider: 'klipy',
})

test('library sanitizing drops unsafe GIFs, duplicates and invalid ids', () => {
  const library = sanitizeLibrary({
    gifs: [
      savedGif(gif('a')),
      savedGif(gif('a')),
      { ...gif('b'), url: 'https://evil.example/x.gif' },
      { ...savedGif(gif('c')), injected: true },
    ],
    emojiUsage: [
      { id: '1F600', count: 3, lastUsed: '2026-01-01T00:00:00Z' },
      { id: '1F600', count: 9, lastUsed: '2026-01-01T00:00:00Z' },
      { id: 'has space', count: 1, lastUsed: '2026-01-01T00:00:00Z' },
      { id: '1F601', count: -2, lastUsed: '2026-01-01T00:00:00Z' },
    ],
    skinTone: '9',
  })
  assert.deepEqual(
    library.gifs.map((g) => g.id),
    ['a', 'c'],
  )
  assert.equal('injected' in library.gifs[1]!, false)
  assert.deepEqual(
    library.emojiUsage.map((u) => [u.id, u.count]),
    [['1F600', 3]],
  )
  assert.equal(library.skinTone, '0')
})

test('an empty library is valid', () => {
  assert.deepEqual(sanitizeLibrary(emptyLibrary()), emptyLibrary())
  assert.deepEqual(sanitizeLibrary('garbage'), emptyLibrary())
})

test('emoji usage counts accumulate and recent use outranks stale heavy use', () => {
  const now = new Date('2026-06-01T00:00:00Z')
  const old = { id: 'old', count: 10, lastUsed: '2026-01-01T00:00:00Z' }
  let list = recordUsage([old], { fresh: 1 }, now)
  list = recordUsage(list, { fresh: 2, 'bad id': 5 }, now)
  assert.equal(list.find((u) => u.id === 'fresh')?.count, 3)
  assert.equal(
    list.some((u) => u.id === 'bad id'),
    false,
  )
  assert.equal(list[0]!.id, 'fresh')
  assert.ok(usageScore(old, now.getTime()) < 1)
})

test('picks saved in one batch keep their order', async () => {
  const { recentEmojiIds, usageOrder } = await import('../shared/library')
  const now = new Date('2026-03-01T00:00:00.000Z')
  // B was picked before A within the same debounced save: A is the more recent one.
  const list = recordUsage([], { A: 1, B: 1 }, now, ['B', 'A'])
  assert.deepEqual(recentEmojiIds(list), ['A', 'B'])
  assert.deepEqual(recentEmojiIds(recordUsage([], { A: 1, B: 1 }, now, ['A', 'B'])), ['B', 'A'])
  // The order a client sends only keeps ids that this update uses.
  assert.deepEqual(usageOrder(['B', 'x', 7, 'toString', 'A'], { A: 1, B: 1 }), ['B', 'A'])
  assert.deepEqual(usageOrder('A', { A: 1 }), [])
})

test('quick reactions follow the most recent distinct picks and fall back to defaults', async () => {
  const { pickQuickReactions, recentEmojiIds, defaultReactions } = await import('../shared/library')
  const item = (id: string, extra = {}) => ({
    id,
    label: id,
    shortcode: id.toLowerCase(),
    aliases: [],
    group: 'g',
    source: 'unicode' as const,
    unicode: id,
    ...extra,
  })
  const usage = [
    { id: 'A', count: 50, lastUsed: '2026-01-01T00:00:00.000Z' },
    { id: 'gone', count: 9, lastUsed: '2026-01-05T00:00:00.000Z' },
    { id: 'B', count: 1, lastUsed: '2026-01-04T00:00:00.000Z' },
    { id: 'C', count: 3, lastUsed: '2026-01-04T00:00:00.000Z' },
  ]
  // Recency, not lifetime popularity; equal times keep the stored order.
  assert.deepEqual(recentEmojiIds(usage), ['gone', 'B', 'C', 'A'])
  const byId = new Map(['A', 'B', 'C'].map((id) => [id, item(id)]))
  // A removed custom emoji is skipped rather than shown broken.
  assert.deepEqual(
    pickQuickReactions(recentEmojiIds(usage), byId, 3).map((x) => x.id),
    ['B', 'C', 'A'],
  )
  assert.deepEqual(
    pickQuickReactions(['B'], byId, 3).map((x) => x.id),
    ['B', defaultReactions[0]!.id, defaultReactions[1]!.id],
  )
  // No history (or no catalogue yet): the defaults, with skin-tone variants intact.
  const defaults = pickQuickReactions([], new Map(), 5)
  assert.deepEqual(defaults.map((x) => x.unicode), ['👍', '❤️', '😂', '😮', '😢'])
  assert.equal(defaults[0]!.variants?.['3'], '👍🏽')
  // Defaults already among the recent picks are not repeated.
  const thumbs = item('1F44D', { unicode: '👍' })
  assert.deepEqual(
    pickQuickReactions(['1F44D'], new Map([['1F44D', thumbs]]), 3).map((x) => x.id),
    ['1F44D', '2764', '1F602'],
  )
})
